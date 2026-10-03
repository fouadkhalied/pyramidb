package db

import "lsmdb/internal/keys"

type request struct {
	seq        uint64
	rec        []byte // the encoded WAL record
	key, value []byte
	reply      chan error
}

func (db *DB) writeLoop() {
	defer close(db.done)
	for first := range db.requests {
		batch := []request{first}
	drain:
		for {
			select {
			case r := <-db.requests:
				batch = append(batch, r)
			default:
				break drain
			}
		}

		var err error
		for _, r := range batch {
			if err = db.wal.Append(r.rec); err != nil {
				break
			}
		}
		if err == nil {
			err = db.wal.FSync() // one fsync for the whole batch
		}
		if err == nil {
			for _, r := range batch { // in order, only after the sync
				db.mem.Put(r.key, r.seq, keys.KindSet, r.value)
				// temporary load data to sstable
				db.sst.Put(r.key, r.value)
			}
		}
		for _, r := range batch {
			r.reply <- err
		}
	}
}
