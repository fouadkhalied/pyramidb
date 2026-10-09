package db

import (
	"errors"
	"fmt"
	"lsmdb/internal/config"
	"lsmdb/internal/wal"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func mustOpen(t *testing.T, dir string) *DB {
	t.Helper()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func mustOpenNoFlush(t *testing.T, dir string) *DB {
	t.Helper()
	d, err := open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPutGet(t *testing.T) {
	d := mustOpen(t, t.TempDir())
	if _, err := d.Get([]byte("missing")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_ = d.Put([]byte("name"), []byte("v1"))
	_ = d.Put([]byte("name"), []byte("v2"))
	if got, err := d.Get([]byte("name")); err != nil || string(got) != "v2" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestRecoveryAfterReopen(t *testing.T) {
	dir := t.TempDir()
	d := mustOpen(t, dir)
	for _, kv := range [][2]string{{"a", "1"}, {"b", "2"}, {"c", "3"}} {
		if err := d.Put([]byte(kv[0]), []byte(kv[1])); err != nil {
			t.Fatal(err)
		}
	}
	d.Close()

	d = mustOpen(t, dir)
	for k, want := range map[string]string{"a": "1", "b": "2", "c": "3"} {
		if got, err := d.Get([]byte(k)); err != nil || string(got) != want {
			t.Fatalf("Get(%q) = %q, %v; want %q", k, got, err, want)
		}
	}
	if d.seq != 3 {
		t.Fatalf("seq = %d, want 3", d.seq)
	}
	_ = d.Put([]byte("d"), []byte("4"))
	if d.seq != 4 {
		t.Fatalf("seq after next Put = %d, want 4", d.seq)
	}
}

func TestTornTailTruncated(t *testing.T) {
	dir := t.TempDir()
	d := mustOpen(t, dir)
	_ = d.Put([]byte("a"), []byte("1"))
	_ = d.Put([]byte("b"), []byte("2"))
	d.Close()

	log := filepath.Join(dir, "000001.log")
	before, _ := os.Stat(log)
	f, _ := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write([]byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02})
	f.Close()

	d = mustOpen(t, dir)
	after, _ := os.Stat(log)
	if after.Size() != before.Size() {
		t.Fatalf("log size %d, want %d (torn tail should be cut)", after.Size(), before.Size())
	}
	if got, err := d.Get([]byte("b")); err != nil || string(got) != "2" {
		t.Fatalf("b = %q, %v", got, err)
	}
	if d.seq != 2 {
		t.Fatalf("seq = %d, want 2", d.seq)
	}
}

func TestDamagedOlderLogFailsOpen(t *testing.T) {
	dir := t.TempDir()
	d := mustOpen(t, dir)
	_ = d.Put([]byte("a"), []byte("1"))
	d.Close()
	d = mustOpen(t, dir) // creates a second, newer log
	_ = d.Put([]byte("b"), []byte("2"))
	d.Close()

	f, _ := os.OpenFile(filepath.Join(dir, "000001.log"), os.O_APPEND|os.O_WRONLY, 0o644)
	f.Write([]byte{1, 2, 3})
	f.Close()

	if _, err := Open(dir); err == nil {
		t.Fatal("Open should fail when an older log is damaged")
	}
}

func TestRotationKeepsOldDataReadable(t *testing.T) {
	d := mustOpenNoFlush(t, t.TempDir())

	keysIn := []string{"key-0", "key-1", "key-2", "key-3", "key-4"}
	put := func(k string) {
		if err := d.Put([]byte(k), []byte("value-"+k[4:])); err != nil {
			t.Fatal(err)
		}
	}
	put(keysIn[0])
	d.memLimit = 2 * d.mem.Size() // every entry costs the same, so every 2nd write rotates
	for _, k := range keysIn[1:] {
		put(k)
	}
	if len(d.imm) != 2 {
		t.Fatalf("frozen memtables = %d, want 2", len(d.imm))
	}
	for _, k := range keysIn { // old data must still be visible
		got, err := d.Get([]byte(k))
		if err != nil || string(got) != "value-"+k[4:] {
			t.Fatalf("Get(%q) = %q, %v", k, got, err)
		}
	}

	_ = d.Put([]byte("key-0"), []byte("NEW")) // newer version beats the one in a frozen memtable
	if got, _ := d.Get([]byte("key-0")); string(got) != "NEW" {
		t.Fatalf("key-0 = %q, want NEW", got)
	}
}

// countRecords returns how many records a log file holds.
// logKeys returns the keys of every record in a log file, in order.
func logKeys(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	_, _, err := wal.Replay(path, func(_ config.Kind, _ uint64, key, _ []byte) {
		out = append(out, string(key))
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRotationStartsNewLog(t *testing.T) {
	dir := t.TempDir()
	d := mustOpenNoFlush(t, t.TempDir())
	d.memLimit = 1 // the very first write fills the memtable

	if err := d.Put([]byte("a"), []byte("1")); err != nil {
		t.Fatal(err)
	}
	if len(d.imm) != 1 {
		t.Fatalf("frozen memtables = %d, want 1", len(d.imm))
	}
	oldPath, newPath := d.imm[0].walPath, d.wal.GetWriterPath()
	if oldPath == newPath {
		t.Fatalf("new log reuses the old log's name: %s", oldPath)
	}
	if logs, _ := wal.ListLogs(dir); len(logs) != 2 {
		t.Fatalf("log files = %d, want 2", len(logs))
	}
	if got := logKeys(t, oldPath); len(got) != 1 || got[0] != "a" {
		t.Fatalf("old log holds %v, want [a]", got)
	}
	if got := logKeys(t, newPath); len(got) != 0 {
		t.Fatalf("new log should be empty, holds %v", got)
	}

	// Writes after the rotation must reach the NEW log (a closed writer would fail here).
	if err := d.Put([]byte("b"), []byte("2")); err != nil {
		t.Fatal(err)
	}
	if got := logKeys(t, d.imm[len(d.imm)-1].walPath); len(got) != 1 {
		t.Fatalf("second frozen log holds %v, want exactly [b]", got)
	}
}

func TestReopenAfterSeveralRotations(t *testing.T) {
	dir := t.TempDir()
	d := mustOpenNoFlush(t, t.TempDir())
	d.memLimit = 1 // every write rotates

	const n = 6
	for i := 0; i < n; i++ {
		if err := d.Put([]byte(fmt.Sprintf("k%d", i)), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if len(d.imm) != n {
		t.Fatalf("frozen memtables = %d, want %d", len(d.imm), n)
	}
	d.Close()

	d = mustOpen(t, dir)
	for i := 0; i < n; i++ {
		got, err := d.Get([]byte(fmt.Sprintf("k%d", i)))
		if err != nil || string(got) != fmt.Sprintf("v%d", i) {
			t.Fatalf("k%d = %q, %v", i, got, err)
		}
	}
	if d.seq != n {
		t.Fatalf("seq = %d, want %d", d.seq, n)
	}
}

// Under concurrent writes, batches form. Each log must still hold exactly the
func TestEachLogMatchesItsMemtableUnderLoad(t *testing.T) {
	d := mustOpenNoFlush(t, t.TempDir())
	d.memLimit = 300

	const writers, perWriter = 20, 25
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				k := []byte(fmt.Sprintf("w%02d-%02d", w, i))
				if err := d.Put(k, []byte("value")); err != nil {
					t.Error(err)
					return
				}
			}
		}(w)
	}
	wg.Wait()

	if len(d.imm) < 3 {
		t.Fatalf("only %d rotations; the test should force many", len(d.imm))
	}
	total := 0
	for i, f := range d.imm {
		if got, want := len(logKeys(t, f.walPath)), f.mem.Len(); got != want {
			t.Fatalf("frozen %d: log has %d records but its memtable has %d keys", i, got, want)
		}
		total += f.mem.Len()
	}
	if got, want := len(logKeys(t, d.wal.GetWriterPath())), d.mem.Len(); got != want {
		t.Fatalf("active: log has %d records but memtable has %d keys", got, want)
	}
	total += d.mem.Len()
	if total != writers*perWriter {
		t.Fatalf("total keys = %d, want %d", total, writers*perWriter)
	}
}
