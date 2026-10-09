package db

import (
	"path/filepath"
	"testing"
)

func TestOpenDropsLogsAlreadyInTables(t *testing.T) {
	dir := t.TempDir()
	d := mustOpenNoFlush(t, dir)
	d.memLimit = 1
	_ = d.Put([]byte("a"), []byte("1")) // log 000001.log
	_ = d.Put([]byte("b"), []byte("2")) // log 000002.log
	a, b := d.imm[0], d.imm[1]
	d.Close()
	writeTable(t, filepath.Join(dir, "000001.sst"), ent{"a", 1, 1, "1"}, ent{"b", 2, 1, "2"})

	d = mustOpen(t, dir)
	defer d.Close()
	if exists(a.walPath) || exists(b.walPath) {
		t.Fatal("logs fully inside a table should be deleted on Open")
	}
	if d.mem.Len() != 0 {
		t.Fatalf("%d keys replayed into the memtable; the table already has them", d.mem.Len())
	}
	// ... Get a and b: both come from the table
}
