package db

import (
	"errors"
	"lsmdb/internal/config"
	"os"
	"path/filepath"
	"testing"

	"lsmdb/internal/memtable"
	"lsmdb/internal/sstable"
)

type ent struct {
	key  string
	seq  uint64
	kind config.Kind
	val  string
}

// writeTable plays the flusher: it puts the entries in a memtable and writes it as an SSTable.
func writeTable(t *testing.T, path string, entries ...ent) {
	t.Helper()
	m := memtable.New()
	for _, e := range entries {
		m.Put([]byte(e.key), e.seq, e.kind, []byte(e.val))
	}
	if err := sstable.WriteFile(path, m.NewEntryIterator(), config.MaxSSTableSizeInBytes); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestOpenDeletesLeftoverTempFiles(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "000001.sst.tmp")
	os.WriteFile(tmp, []byte("half a flush"), 0o644) // a crash in the middle of a flush

	d := mustOpen(t, dir)
	if exists(tmp) {
		t.Fatal("a leftover .sst.tmp must be deleted on Open")
	}
	if len(d.tables) != 0 {
		t.Fatalf("tables = %d, a temp file is not a table", len(d.tables))
	}
}

func TestOpenLoadsTablesAndRestoresSeq(t *testing.T) {
	dir := t.TempDir()
	// older table, then a newer one that overwrites x and deletes y
	writeTable(t, filepath.Join(dir, "000001.sst"),
		ent{"x", 1, config.KindSet, "old"}, ent{"y", 3, config.KindSet, "y1"}, ent{"only-old", 2, config.KindSet, "o"})
	writeTable(t, filepath.Join(dir, "000002.sst"),
		ent{"x", 5, config.KindSet, "new"}, ent{"y", 8, config.KindDelete, ""})

	d := mustOpen(t, dir)
	if len(d.tables) != 2 || d.seq != 8 || d.nextFile != 3 {
		t.Fatalf("tables=%d seq=%d nextFile=%d, want 2, 8, 3", len(d.tables), d.seq, d.nextFile)
	}
	if got, err := d.Get([]byte("x")); err != nil || string(got) != "new" {
		t.Fatalf("x = %q, %v; the newer table must win", got, err)
	}
	if got, err := d.Get([]byte("only-old")); err != nil || string(got) != "o" {
		t.Fatalf("only-old = %q, %v", got, err)
	}
	if _, err := d.Get([]byte("y")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("y: err = %v; a tombstone in a newer table must hide the older value", err)
	}
	_ = d.Put([]byte("z"), []byte("1"))
	if d.seq != 9 {
		t.Fatalf("next write got seq %d, want 9", d.seq)
	}
}

// Crash after the flush finished and the old logs were deleted: the ONLY record of the
// highest sequence number is the table's footer.
func TestSeqComesFromTableFooterWhenLogsAreGone(t *testing.T) {
	dir := t.TempDir()
	d := mustOpenNoFlush(t, dir)
	d.memLimit = 1 // every write rotates
	_ = d.Put([]byte("a"), []byte("1"))
	_ = d.Put([]byte("b"), []byte("2"))
	a, b := d.imm[0], d.imm[1]
	d.Close()

	for i, fm := range []frozenMem{a, b} { // finish both flushes, then delete their logs
		if err := sstable.WriteFile(filepath.Join(dir, []string{"000001.sst", "000002.sst"}[i]), fm.mem.NewEntryIterator(), config.MaxSSTableSizeInBytes); err != nil {
			t.Fatal(err)
		}
		os.Remove(fm.walPath)
	}

	d = mustOpen(t, dir)
	if d.seq != 2 {
		t.Fatalf("seq = %d, want 2 (restored from the footers)", d.seq)
	}
	for k, want := range map[string]string{"a": "1", "b": "2"} {
		if got, err := d.Get([]byte(k)); err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", k, got, err)
		}
	}
}

// Crash between the rename and the log delete: the table AND its log both exist.
func TestFinishedTableWithUndeletedLogIsHarmless(t *testing.T) {
	dir := t.TempDir()
	d := mustOpenNoFlush(t, dir)
	d.memLimit = 1
	_ = d.Put([]byte("a"), []byte("1"))
	_ = d.Put([]byte("a"), []byte("2")) // a second version of the same key
	first := d.imm[0]
	d.Close()

	if err := sstable.WriteFile(filepath.Join(dir, "000001.sst"), first.mem.NewEntryIterator(), config.MaxSSTableSizeInBytes); err != nil {
		t.Fatal(err) // note: first.walPath is deliberately NOT deleted
	}

	d = mustOpen(t, dir)
	if got, err := d.Get([]byte("a")); err != nil || string(got) != "2" {
		t.Fatalf("a = %q, %v; the newest version must win", got, err)
	}
	if d.seq != 2 {
		t.Fatalf("seq = %d, want 2", d.seq)
	}
}

func TestOpenFailsOnDamagedTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "000001.sst")
	writeTable(t, path, ent{"k", 1, config.KindSet, "v"})
	good, _ := os.ReadFile(path)

	os.WriteFile(path, good[:len(good)-5], 0o644) // truncated footer
	if _, err := Open(dir); !errors.Is(err, sstable.ErrCorrupt) {
		t.Fatalf("truncated table: err = %v, want ErrCorrupt", err)
	}

	bad := append([]byte(nil), good...)
	bad[2] ^= 0xff // inside the data block: Open cannot see it, Get must
	os.WriteFile(path, bad, 0o644)
	d := mustOpen(t, dir)
	if _, err := d.Get([]byte("k")); !errors.Is(err, sstable.ErrCorrupt) {
		t.Fatalf("damaged block: err = %v, want ErrCorrupt and no wrong answer", err)
	}

	os.WriteFile(filepath.Join(dir, "garbage.sst"), good, 0o644)
	d.Close()
	if _, err := Open(dir); err == nil {
		t.Fatal("a .sst file with a non-numeric name should be rejected")
	}
}
