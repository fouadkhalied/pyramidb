package db

import (
	"errors"
	"lsmdb/internal/sstable"
	"sync"

	"lsmdb/internal/memtable"
	"lsmdb/internal/wal"
)

var (
	ErrNotFound = errors.New("db: key not found")
	ErrClosed   = errors.New("db: closed")
)

// frozenMem is a full memtable together with the log that holds exactly its records.
type frozenMem struct {
	mem     *memtable.MemTable
	walPath string
}

type DB struct {
	mu     sync.Mutex // guards seq and closed, and keeps queue order == seq order
	dir    string
	seq    uint64
	closed bool

	wal *wal.Writer // only the write worker uses it (and Close, after the worker exits)

	mem      *memtable.MemTable
	view     sync.RWMutex // guards mem and imm
	imm      []frozenMem  // frozen memtables, oldest first
	memLimit int

	sst         *sstable.Writer
	flushCh     chan frozenMem
	flushChDone chan struct{}

	requests chan request
	done     chan struct{}
}
