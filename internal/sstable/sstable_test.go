package sstable

import (
	"bytes"
	"testing"

	"lsmdb/internal/keys"
)

// set is a test helper: write a KindSet version of k at the given seq.
func set(s *SSTable, key []byte, seq uint64, kind keys.Kind, value []byte) bool {
	s.Put(key, seq, kind, value)
	return true
}

func TestSSTableSortsKeysAndVersions(t *testing.T) {
	s := Init()
	s.Put([]byte("b"), 1, keys.KindSet, []byte("x"))
	s.Put([]byte("a"), 5, keys.KindSet, []byte("old"))
	s.Put([]byte("c"), 3, keys.KindSet, []byte("x"))
	s.Put([]byte("a"), 9, keys.KindSet, []byte("new"))
	s.Compact()

	want := []struct {
		key string
		seq uint64
	}{{"a", 9}, {"a", 5}, {"b", 1}, {"c", 3}} // newest version of a first
	for i, w := range want {
		if string(s.Data[i].Key) != w.key || s.Data[i].Seq != w.seq {
			t.Fatalf("entry %d = %s/%d, want %s/%d", i, s.Data[i].Key, s.Data[i].Seq, w.key, w.seq)
		}
	}
	if !bytes.Equal(s.Index.Smallest, []byte("a")) || !bytes.Equal(s.Index.Largest, []byte("c")) {
		t.Fatalf("index range = %q..%q", s.Index.Smallest, s.Index.Largest)
	}
}

func TestSSTableCompactEmpty(t *testing.T) {
	s := Init()
	s.Compact() // must not panic
	if s.Index.NumEntries != 0 || s.Index.Smallest != nil {
		t.Fatal("empty table should have an empty index")
	}
}

func TestSSTablePutCopiesBuffers(t *testing.T) {
	s := Init()
	k, v := []byte("key"), []byte("val")
	s.Put(k, 1, keys.KindSet, v)
	k[0], v[0] = 'X', 'X' // the caller reuses its buffers
	if string(s.Data[0].Key) != "key" || string(s.Data[0].Value) != "val" {
		t.Fatal("table aliased the caller's buffers")
	}
}
