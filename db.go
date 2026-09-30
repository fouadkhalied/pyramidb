// Package lsmdb is the public API of the storage engine.
// Right now it is memtable-only; each course section adds a layer underneath.
package lsmdb

import (
	"errors"
	"lsmdb/internal/keys"
	"lsmdb/internal/wal"
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
	w, err := wal.Open(filepath.Join(dir, "000001.wal"))
	if err != nil {
		return nil, err
	}
	return &DB{dir: dir, wal: w, mem: memtable.New()}, nil
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
