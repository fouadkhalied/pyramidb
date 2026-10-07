package db

import (
	"lsmdb/internal/config"
	"lsmdb/internal/wal"
)

// Put follows the WAL protocol: seq, append + fsync, memtable, ack.
func (db *DB) Put(key, value []byte) error {
	reply := make(chan error, 1)

	db.mu.Lock()
	db.seq++
	db.requests <- request{
		seq:   db.seq,
		rec:   wal.Encode(config.KindSet, db.seq, key, value),
		key:   key,
		value: value,
		reply: reply,
	}
	db.mu.Unlock() // queue order now matches seq order

	return <-reply // wait here, outside the lock
}

// Get reads the newest value as of the latest sequence number.
func (db *DB) Get(key []byte) ([]byte, error) {
	db.mu.Lock()
	snap := db.seq
	db.mu.Unlock()

	db.view.RLock()
	mem, imm := db.mem, db.imm // one consistent view of the active + frozen memtables
	db.view.RUnlock()

	if v, ok := mem.Get(key, snap); ok { // newest first
		return v, nil
	}
	for i := len(imm) - 1; i >= 0; i-- {
		if v, ok := imm[i].mem.Get(key, snap); ok {
			return v, nil
		}
	}
	return nil, ErrNotFound
}
