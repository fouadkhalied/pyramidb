package db

import (
	"errors"
	"os"
	"path/filepath"
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
	d := mustOpen(t, t.TempDir())

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
