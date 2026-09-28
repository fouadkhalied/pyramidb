// Package lsmdb is the public API of the storage engine.
// Right now it is memtable-only; each course section adds a layer underneath.
package lsmdb

import (
	"errors"

	"lsmdb/internal/memtable"
)

var ErrNotFound = errors.New("lsmdb: key not found")

type DB struct {
	dir string // unused until the WAL and SSTables arrive
	mem *memtable.MemTable
}

func Open(dir string) (*DB, error) {
	return &DB{dir: dir, mem: memtable.New()}, nil
}

func (db *DB) Put(key, value []byte) error {
	db.mem.Put(key, value)
	return nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	if v, ok := db.mem.Get(key); ok {
		return v, nil
	}
	return nil, ErrNotFound
}
