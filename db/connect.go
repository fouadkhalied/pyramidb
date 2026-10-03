package db

import (
	"fmt"
	"os"
	"path/filepath"

	"lsmdb/internal/keys"
	"lsmdb/internal/memtable"
	"lsmdb/internal/sstable"
	"lsmdb/internal/wal"
)

// Open recovers any existing logs into a fresh memtable, then starts a new log.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db := &DB{dir: dir, mem: memtable.New()}

	if err := db.recover(); err != nil {
		return nil, err
	}

	db.sst = sstable.Init()

	w, err := wal.Open(filepath.Join(dir, fmt.Sprintf("%06d.log", db.seq+1)))
	if err != nil {
		return nil, err
	}
	db.wal = w

	db.requests = make(chan request, 1024) // this DB's queue
	db.done = make(chan struct{})
	go db.writeLoop() // start only after recovery is finished
	return db, nil
}

// recover replays every log, oldest first, and trims a torn tail on the newest one.
func (db *DB) recover() error {
	files, err := wal.ListLogs(db.dir)
	if err != nil {
		return err
	}
	for i, f := range files {
		good, maxSeq, err := wal.Replay(f, func(kind keys.Kind, seq uint64, key, value []byte) {
			db.mem.Put(key, seq, kind, value)
		})
		if err != nil {
			return err
		}
		if maxSeq > db.seq {
			db.seq = maxSeq
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
	close(db.requests)
	db.mu.Unlock()

	<-db.done
	return db.wal.Close()
}
