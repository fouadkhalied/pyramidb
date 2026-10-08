package db

import "lsmdb/internal/memtable"

func (db *DB) flushLoop() {
	defer close(db.flushChDone)
	//var batch []frozenMem
	for first := range db.flushCh {
		batch := []frozenMem{first}
	drain:
		for {
			select {
			case r := <-db.flushCh:
				batch = append(batch, r)
			default:
				break drain
			}
		}

		for _, r := range batch {
			err := db.flushMem(r.mem.NewIterator())
			if err != nil {
				return
			}
		}
	}
}

func (db *DB) flushMem(mem *memtable.Iterator) error {
	return nil
}

func (db *DB) DeleteWal() error {
	// wal should delete this
	return nil
}
