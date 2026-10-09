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
	mem, imm, tables := db.mem, db.imm, db.tables // one consistent view of the active + frozen memtables
	db.view.RUnlock()

	// check mem and imm first
	if v, ok := mem.Get(key, snap); ok { // newest first
		return v, nil
	}
	for i := len(imm) - 1; i >= 0; i-- {
		if v, ok := imm[i].mem.Get(key, snap); ok {
			return v, nil
		}
	}

	// check sstables next
	for _, t := range tables { // newest table first
		e, ok, err := t.Get(key, snap)
		if err != nil {
			return nil, err
		}
		if ok {
			if e.Kind() == config.KindDelete {
				return nil, ErrNotFound // a tombstone hides every older version
			}
			return e.Value, nil
		}
	}
	return nil, ErrNotFound
}
