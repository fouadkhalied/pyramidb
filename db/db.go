// Package lsmdb is the public API of the storage engine.
// Right now it is memtable-only; each course section adds a layer underneath.
package lsmdb

import (
	"errors"
	"fmt"
	"lsmdb/internal/keys"
	"lsmdb/internal/wal"
	"os"
	"path/filepath"
	"sync"

	"lsmdb/internal/memtable"
)

var ErrNotFound = errors.New("lsmdb: key not found")
var ErrCorrupt = errors.New("wal: corrupt or truncated record")

type DB struct {
	mu  sync.Mutex
	dir string
	seq uint64
	wal *wal.Writer
	mem *memtable.MemTable
}

func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db := &DB{dir: dir, mem: memtable.New()}

	files, err := wal.ListLogs(dir)
	if err != nil {
		return nil, err
	}
	for i, f := range files {
		good, maxSeq, err := wal.Replay(f, func(kind keys.Kind, seq uint64, key, value []byte) {
			db.mem.Put(key, seq, kind, value)
		})
		if err != nil {
			return nil, err
		}
		if maxSeq > db.seq {
			db.seq = maxSeq
		}

		info, err := os.Stat(f)
		if err != nil {
			return nil, err
		}
		if int64(good) < info.Size() { // bytes left after the last good record
			if i != len(files)-1 {
				return nil, fmt.Errorf("wal: %s is damaged and is not the newest log", f)
			}
			if err := os.Truncate(f, int64(good)); err != nil {
				return nil, err
			}
		}
	}

	w, err := wal.Open(filepath.Join(dir, fmt.Sprintf("%06d.log", db.seq+1)))
	if err != nil {
		return nil, err
	}
	db.wal = w
	return db, nil
}

func (db *DB) Put(key, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.seq++ // 1. seq = ++global_seq
	rec := keys.Encode(keys.KindSet, db.seq, key, value)
	if err := db.wal.Append(rec); err != nil { // 2. append, 3. fsync
		return err
	}
	db.mem.Put(key, db.seq, keys.KindSet, value) // 4. memtable
	return nil                                   // 5. ack
}

func (db *DB) Get(key []byte) ([]byte, error) {
	if v, ok := db.mem.Get(key); ok {
		return v, nil
	}
	return nil, ErrNotFound
}
