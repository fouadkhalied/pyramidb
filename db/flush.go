package db

import (
	"fmt"
	"lsmdb/internal/config"
	"lsmdb/internal/memtable"
	"lsmdb/internal/sstable"
	"os"
	"path/filepath"
)

func tablePath(dir string, num uint64) string {
	return filepath.Join(dir, fmt.Sprintf("%06d.sst", num))
}

func (db *DB) flushLoop() {
	defer close(db.flushChDone)
	failed := false
	for fm := range db.flushCh {
		if db.noFlush || failed {
			continue
		}
		if err := db.flushOne(fm); err != nil {
			failed = true
			db.view.Lock()
			db.bgErr = err
			db.view.Unlock()
		}
	}
}

func (db *DB) flushOne(fm frozenMem) error {
	path := tablePath(db.dir, db.nextFile)
	if err := sstable.WriteFile(path, fm.mem.NewEntryIterator(), config.MaxSSTableSizeInBytes); err != nil {
		return err // WriteFile already removed its .tmp
	}
	db.nextFile++

	t, err := sstable.Open(path)
	if err != nil {
		return err
	}

	db.view.Lock()
	db.tables = append([]*sstable.Table{t}, db.tables...)
	rest := make([]frozenMem, 0, len(db.imm))
	for _, m := range db.imm {
		if m.mem != fm.mem {
			rest = append(rest, m)
		}
	}
	db.imm = rest
	db.view.Unlock()

	return os.Remove(fm.walPath)
}

func (db *DB) flushMem(mem *memtable.Iterator) error {
	return nil
}

func (db *DB) DeleteWal() error {
	// wal should delete this
	return nil
}
