package sstable

import (
	"errors"
	"fmt"
	"lsmdb/internal/config"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

type sliceIter struct {
	es []Entry
	i  int
}

func (s *sliceIter) Valid() bool  { return s.i < len(s.es) }
func (s *sliceIter) Entry() Entry { return s.es[s.i] }
func (s *sliceIter) Next()        { s.i++ }

// entry builds a test entry from a user key, seq and kind.
func entry(key string, seq uint64, kind config.Kind, val string) Entry {
	return Entry{Internal: Make([]byte(key), seq, kind), Value: []byte(val)}
}

// sampleEntries returns 20 keys with 3 versions each, in file order
// (user key ascending, newest version first). Every 7th key's newest version is a tombstone.
func sampleEntries() []Entry {
	var es []Entry
	for k := 0; k < 20; k++ {
		for v := 3; v >= 1; v-- { // newest first
			seq := uint64(k*10 + v)
			key := fmt.Sprintf("key-%02d", k)
			if v == 3 && k%7 == 0 {
				es = append(es, entry(key, seq, config.KindDelete, ""))
			} else {
				es = append(es, entry(key, seq, config.KindSet, fmt.Sprintf("v%d", seq)))
			}
		}
	}
	return es
}

// expect is the brute-force answer: the first (= newest) version of key with Seq <= snap.
func expect(es []Entry, key []byte, snap uint64) (Entry, bool) {
	for _, e := range es {
		if string(e.UserKey()) == string(key) && e.Seq() <= snap {
			return e, true
		}
	}
	return Entry{}, false
}

func writeTable(t *testing.T, es []Entry, blockSize int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "000001.sst")
	if err := WriteFile(path, &sliceIter{es: es}, blockSize); err != nil {
		t.Fatal(err)
	}
	return path
}

func openTable(t *testing.T, path string) *Table {
	t.Helper()
	tb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tb.Close() })
	return tb
}

func TestRoundTripAllVersions(t *testing.T) {
	es := sampleEntries()
	tb := openTable(t, writeTable(t, es, 64))
	if len(tb.index) < 10 {
		t.Fatalf("only %d blocks; the budget should force many", len(tb.index))
	}
	if tb.EntryCount() != uint64(len(es)) || tb.HighestSeq() != 193 {
		t.Fatalf("footer: count=%d highest=%d", tb.EntryCount(), tb.HighestSeq())
	}
	for k := 0; k < 20; k++ {
		key := []byte(fmt.Sprintf("key-%02d", k))
		for snap := uint64(0); snap < 210; snap++ {
			want, wantOK := expect(es, key, snap)
			got, ok, err := tb.Get(key, snap)
			if err != nil {
				t.Fatalf("Get(%s,%d): %v", key, snap, err)
			}
			if ok != wantOK || (ok && (got.Seq() != want.Seq() || got.Kind() != want.Kind() || string(got.Value) != string(want.Value))) {
				t.Fatalf("Get(%s,%d) = %+v,%v; want %+v,%v", key, snap, got, ok, want, wantOK)
			}
		}
	}
}

// With a tiny budget every version of a key lands in its own block.
func TestVersionsStraddleBlocks(t *testing.T) {
	var es []Entry
	for _, seq := range []uint64{60, 50, 40, 30, 20, 10} {
		es = append(es, entry("k", seq, config.KindSet, fmt.Sprintf("val-%d", seq)))
	}
	tb := openTable(t, writeTable(t, es, 40))
	if len(tb.index) != 6 {
		t.Fatalf("blocks = %d, want 6 (one version per block)", len(tb.index))
	}
	cases := []struct {
		snap uint64
		want string
	}{{100, "val-60"}, {60, "val-60"}, {59, "val-50"}, {45, "val-40"}, {10, "val-10"}, {9, ""}}
	for _, c := range cases {
		got, ok, err := tb.Get([]byte("k"), c.snap)
		if err != nil {
			t.Fatal(err)
		}
		if c.want == "" && ok || c.want != "" && (!ok || string(got.Value) != c.want) {
			t.Fatalf("snap %d: got %q,%v want %q", c.snap, got.Value, ok, c.want)
		}
	}
}

func TestMissingKeys(t *testing.T) {
	es := []Entry{
		entry("b", 5, config.KindSet, "B"),
		entry("d", 6, config.KindSet, "D"),
	}
	tb := openTable(t, writeTable(t, es, 20))
	for _, c := range []struct {
		key  string
		snap uint64
		hit  bool
	}{{"a", 99, false}, {"b", 99, true}, {"b", 5, true}, {"b", 4, false}, {"c", 99, false}, {"d", 99, true}, {"e", 99, false}, {"", 99, false}} {
		_, ok, err := tb.Get([]byte(c.key), c.snap)
		if err != nil || ok != c.hit {
			t.Fatalf("Get(%q,%d) = %v,%v; want %v", c.key, c.snap, ok, err, c.hit)
		}
	}
}

func TestAddRejectsUnsortedInput(t *testing.T) {
	w, err := NewWriter(filepath.Join(t.TempDir(), "x.sst"), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Abort()
	add := func(k string, seq uint64) error {
		return w.Add(entry(k, seq, config.KindSet, "v"))
	}
	if err := add("b", 5); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		k   string
		seq uint64
		ok  bool
	}{{"b", 5, false}, {"b", 9, false}, {"a", 1, false}, {"b", 4, true}, {"c", 1, true}} {
		if err := add(c.k, c.seq); (err == nil) != c.ok || (!c.ok && !errors.Is(err, ErrUnsorted)) {
			t.Fatalf("Add(%s,%d) = %v; ok should be %v", c.k, c.seq, err, c.ok)
		}
	}
}

func TestFinishAndAbortLeaveNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "000001.sst")
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }

	if err := WriteFile(path, &sliceIter{es: sampleEntries()}, 64); err != nil {
		t.Fatal(err)
	}
	if !exists(path) || exists(path+".tmp") {
		t.Fatal("after Finish the .sst must exist and the .tmp must not")
	}

	other := filepath.Join(dir, "000002.sst")
	w, _ := NewWriter(other, 64)
	_ = w.Add(sampleEntries()[0])
	w.Abort()
	if exists(other) || exists(other+".tmp") {
		t.Fatal("Abort must leave nothing behind")
	}

	empty := filepath.Join(dir, "000003.sst")
	if err := WriteFile(empty, &sliceIter{}, 64); !errors.Is(err, ErrEmpty) || exists(empty) || exists(empty+".tmp") {
		t.Fatalf("empty write: err=%v", err)
	}

	stale := filepath.Join(dir, "000004.sst")
	os.WriteFile(stale+".tmp", []byte("junk"), 0o644)
	if _, err := NewWriter(stale, 64); err == nil {
		t.Fatal("NewWriter must refuse to reuse an existing temp file")
	}
}

func TestOpenRejectsDamage(t *testing.T) {
	path := writeTable(t, sampleEntries(), 64)
	good, _ := os.ReadFile(path)
	ft, err := decodeFooter(good[len(good)-footerSize:])
	if err != nil {
		t.Fatal(err)
	}
	tryOpen := func(b []byte) error {
		p := filepath.Join(t.TempDir(), "d.sst")
		os.WriteFile(p, b, 0o644)
		tb, err := Open(p)
		if err == nil {
			tb.Close()
		}
		return err
	}

	for cut := 0; cut < len(good); cut++ { // every truncation must be rejected, never a panic
		if err := tryOpen(good[:cut]); err == nil {
			t.Fatalf("a file cut to %d bytes was accepted", cut)
		}
	}
	flip := func(i int) []byte { b := append([]byte(nil), good...); b[i] ^= 0xff; return b }

	for i := int(ft.IndexOffset); i < int(ft.IndexOffset+ft.IndexLength); i++ { // index block, CRC included
		if tryOpen(flip(i)) == nil {
			t.Fatalf("flipping index byte %d was accepted", i)
		}
	}
	for i := len(good) - footerSize; i < len(good)-footerSize+16; i++ { // index offset and length
		if tryOpen(flip(i)) == nil {
			t.Fatalf("flipping footer byte %d was accepted", i)
		}
	}
	for i := len(good) - 8; i < len(good); i++ { // magic
		if tryOpen(flip(i)) == nil {
			t.Fatalf("flipping magic byte %d was accepted", i)
		}
	}

	// A damaged data block opens fine (only the index is read) and fails when it is read.
	bad := flip(3)
	p := filepath.Join(t.TempDir(), "d.sst")
	os.WriteFile(p, bad, 0o644)
	tb, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer tb.Close()
	if _, _, err := tb.Get([]byte("key-00"), 1000); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("reading a damaged block: err = %v, want ErrCorrupt", err)
	}
}

func TestDecodeEntryNeverPanics(t *testing.T) {
	ik := Make([]byte("user:1"), 7, config.KindSet)
	enc := appendEntry(nil, ik.Key, []byte("hello"))
	if _, _, n, err := decodeEntry(enc); err != nil || n != len(enc) {
		t.Fatalf("full entry: n=%d err=%v", n, err)
	}
	for cut := 0; cut < len(enc); cut++ {
		if _, _, _, err := decodeEntry(enc[:cut]); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("prefix of %d bytes: err = %v, want ErrCorrupt", cut, err)
		}
	}
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 5000; i++ { // random garbage must be an error or a clean decode, never a panic
		b := make([]byte, rnd.Intn(40))
		rnd.Read(b)
		decodeEntry(b)
	}
}

func TestAddRejectsKeyWithoutTrailer(t *testing.T) {
	w, err := NewWriter(filepath.Join(t.TempDir(), "x.sst"), 100)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Abort()
	if err := w.Add(Entry{Internal: Internal{
		Key: []byte("short"),
	}, Value: []byte("v")}); !errors.Is(err, ErrBadKey) {
		t.Fatalf("err = %v, want ErrBadKey", err)
	}
}

//func TestAddDoesNotKeepTheCallersBuffer(t *testing.T) {
//	path := filepath.Join(t.TempDir(), "x.sst")
//	w, _ := NewWriter(path, 1000)
//	e := entry("a", 1, config.KindSet, "v")
//	if err := w.Add(e); err != nil {
//		t.Fatal(err)
//	}
//	e.Internal.Key = 'Z' // the caller reuses its buffer for the next entry
//	if err := w.Add(entry("b", 1, config.KindSet, "v")); err != nil {
//		t.Fatalf("a mutated caller buffer broke the sort check: %v", err)
//	}
//	if err := w.Finish(); err != nil {
//		t.Fatal(err)
//	}
//}
