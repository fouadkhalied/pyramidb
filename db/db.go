// Package db is the public API of the storage engine.
package db

import (
	"errors"
	"sync"

	"lsmdb/internal/memtable"
	"lsmdb/internal/wal"
)

var ErrNotFound = errors.New("db: key not found")

type DB struct {
	mu       sync.Mutex // guards seq and keeps log order == memtable order
	dir      string
	seq      uint64 // global sequence counter
	wal      *wal.Writer
	mem      *memtable.MemTable
	requests chan request
}
