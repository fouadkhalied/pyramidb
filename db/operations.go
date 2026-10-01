package db

import "lsmdb/internal/keys"

// Put follows the WAL protocol: seq, append + fsync, memtable, ack.
func (db *DB) Put(key, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.seq++
	rec := keys.Encode(keys.KindSet, db.seq, key, value)
	if err := db.wal.Append(rec); err != nil {
		return err
	}
	db.mem.Put(key, db.seq, keys.KindSet, value)
	return nil
}

// Get reads the newest value as of the latest sequence number.
func (db *DB) Get(key []byte) ([]byte, error) {
	db.mu.Lock()
	snap := db.seq
	db.mu.Unlock()

	v, ok := db.mem.Get(key, snap)
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (db *DB) Close() error { return db.wal.Close() }
