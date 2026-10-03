package db

import (
	"errors"
	"sync"

	"lsmdb/internal/memtable"
	"lsmdb/internal/sstable"
	"lsmdb/internal/wal"
)

var (
	ErrNotFound = errors.New("db: key not found")
	ErrClosed   = errors.New("db: closed")
)

type DB struct {
	mu       sync.Mutex // guards seq and closed, and keeps queue order == seq order
	dir      string
	seq      uint64 // global sequence counter
	closed   bool   // set by Close, checked by Put
	wal      *wal.Writer
	mem      *memtable.MemTable
	sst      *sstable.SSTable
	requests chan request  // this DB's queue of writes for the worker
	done     chan struct{} // closed by the worker when it exits
}
