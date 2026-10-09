package db

import (
	"fmt"
	"lsmdb/internal/config"
	"os"
	"path/filepath"

	"lsmdb/internal/memtable"
	"lsmdb/internal/wal"
)

// Open recovers any existing logs into a fresh memtable, then starts a new log.
func Open(dir string) (*DB, error) { return open(dir, false) }

func open(dir string, noFlush bool) (*DB, error) {
	// ...
	db := &DB{dir: dir, mem: memtable.New(), memLimit: config.MaxMemTableSizeInBytes, noFlush: noFlush}

	// Tables first: they say how much of the logs is already durable.
	if err := db.recoverTables(); err != nil {
		return nil, err
	}
	if err := db.recover(); err != nil {
		db.closeTables()
		return nil, err
	}

	w, err := wal.Open(filepath.Join(dir, fmt.Sprintf("%06d.log", db.seq+1)))
	if err != nil {
		db.closeTables()
		return nil, err
	}
	db.wal = w

	db.flushCh = make(chan frozenMem, 2)
	db.flushChDone = make(chan struct{})
	go db.flushLoop()

	db.requests = make(chan request, 1024)
	db.done = make(chan struct{})
	go db.writeLoop()
	return db, nil
}

// rotate freezes the full memtable and starts a new memtable with a new log.
// nextSeq must be (last seq of the batch just applied) + 1, so the new log's
func (db *DB) rotate(nextSeq uint64) (frozenMem, error) {
	// 1. Everything that can fail goes first. If it fails, nothing has changed.
	w, err := wal.Open(filepath.Join(db.dir, fmt.Sprintf("%06d.log", nextSeq)))
	if err != nil {
		return frozenMem{}, err
	}

	// 2. Swap all the state together, holding the lock only for pointer changes.
	db.view.Lock()
	old := db.wal
	fm := frozenMem{mem: db.mem, walPath: old.GetWriterPath()} // built once, used twice
	db.imm = append(db.imm, fm)
	db.mem = memtable.New()
	db.wal = w
	db.view.Unlock()

	// 3. Close the old log last. It was fsynced before this batch was applied, so
	_ = old.Close()
	return fm, nil
}

// recover replays every log, oldest first, and trims a torn tail on the newest one.
func (db *DB) recover() error {
	files, err := wal.ListLogs(db.dir)
	if err != nil {
		return err
	}
	for i, f := range files {
		good, maxSeq, err := wal.Replay(f, func(kind config.Kind, seq uint64, key, value []byte) {
			if seq <= db.flushedSeq {
				return // already inside a table: replaying it would duplicate the version
			}

			db.mem.Put(key, seq, kind, value)
		})
		if err != nil {
			return err
		}
		if maxSeq > db.seq {
			db.seq = maxSeq
		}

		if maxSeq > 0 && maxSeq <= db.flushedSeq {
			// Crash between "table renamed" and "log deleted": finish the job.
			if err := os.Remove(f); err != nil {
				return err
			}
			continue
		}

		info, err := os.Stat(f)
		if err != nil {
			return err
		}
		if int64(good) < info.Size() { // bytes left after the last good record
			if i != len(files)-1 {
				return fmt.Errorf("db: %s is damaged and is not the newest log", f)
			}
			if err := os.Truncate(f, int64(good)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil
	}
	db.closed = true
	close(db.requests) // 1. no new writes
	db.mu.Unlock()

	<-db.done         // 2. the write worker has stopped: nothing sends on flushCh any more
	close(db.flushCh) // 3. tell the flusher no more memtables are coming
	<-db.flushChDone  // 4. wait until it has finished everything already queued

	err := db.closeTables() // 5. close the table files
	if werr := db.wal.Close(); err == nil {
		err = werr
	}
	return err
}
