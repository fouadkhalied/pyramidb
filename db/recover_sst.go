package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"lsmdb/internal/sstable"
)

func (db *DB) recoverTables() (err error) {
	tmps, err := filepath.Glob(filepath.Join(db.dir, "*.sst.tmp"))
	if err != nil {
		return err
	}
	for _, f := range tmps {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	files, err := filepath.Glob(filepath.Join(db.dir, "*.sst"))
	if err != nil {
		return err
	}
	sort.Strings(files) // zero-padded names: alphabetical order is numeric order, oldest first

	defer func() { // do not leak file handles if one table is bad
		if err != nil {
			db.closeTables()
		}
	}()

	var maxFile uint64
	for _, f := range files {
		n, perr := strconv.ParseUint(strings.TrimSuffix(filepath.Base(f), ".sst"), 10, 64)
		if perr != nil {
			return fmt.Errorf("db: unexpected file name %s", f)
		}
		t, oerr := sstable.Open(f)
		if oerr != nil {
			return fmt.Errorf("db: %s: %w", f, oerr)
		}

		if t.HighestSeq() > db.flushedSeq {
			db.flushedSeq = t.HighestSeq()
		}
		if t.HighestSeq() > db.seq {
			db.seq = t.HighestSeq() // if every log was deleted, the footers are the only memory of seq
		}

		db.tables = append([]*sstable.Table{t}, db.tables...) // newest first
		if t.HighestSeq() > db.seq {
			db.seq = t.HighestSeq()
		}
		if n > maxFile {
			maxFile = n
		}
	}
	db.nextFile = maxFile + 1
	return nil
}

func (db *DB) closeTables() error {
	var first error
	for _, t := range db.tables {
		if err := t.Close(); err != nil && first == nil {
			first = err
		}
	}
	db.tables = nil
	return first
}
