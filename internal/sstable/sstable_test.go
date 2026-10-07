package sstable

//
//import "testing"
//
//const budget = 8 // block budget used by these tests
//
//// put adds an entry whose len(key)+len(value) == size (size >= 1, key is 1 byte).
//func put(t *SSTable, i, size int) {
//	key := string(rune('a' + i))
//	t.Put([]byte(key), make([]byte, size-len(key)))
//}
//
//func newTable() *SSTable {
//	s := Init()
//	s.MaxBlock = budget // don't depend on the package constant's value
//	return s
//}
//
//func TestPackingRule(t *testing.T) {
//	cases := []struct {
//		name  string
//		sizes []int
//		want  []int // number of entries in each block
//	}{
//		{"first entry on an empty table", []int{3}, []int{1}},
//		{"exactly the budget still fits", []int{3, 5}, []int{2}},
//		{"one byte over starts a new block", []int{3, 6}, []int{1, 1}},
//		{"fills a block step by step", []int{3, 2, 3}, []int{3}},
//		{"overflow on the third entry", []int{3, 2, 4}, []int{2, 1}},
//		{"single entry exactly the budget", []int{8}, []int{1}},
//		{"entry after a full block starts a new one", []int{8, 1}, []int{1, 1}},
//		{"oversized entry gets its own block", []int{3, 10, 2, 3}, []int{1, 1, 2}},
//		{"oversized entry first", []int{10, 2}, []int{1, 1}},
//		{"two oversized in a row", []int{10, 12}, []int{1, 1}},
//	}
//	for _, c := range cases {
//		s := newTable()
//		for i, sz := range c.sizes {
//			put(s, i, sz)
//		}
//
//		got := make([]int, len(s.Data))
//		for i, b := range s.Data {
//			got[i] = len(b.Entries)
//		}
//		if len(got) != len(c.want) {
//			t.Errorf("%s: blocks = %v, want %v", c.name, got, c.want)
//			continue
//		}
//		for i := range got {
//			if got[i] != c.want[i] {
//				t.Errorf("%s: blocks = %v, want %v", c.name, got, c.want)
//				break
//			}
//		}
//
//		// Nothing lost, duplicated or reordered, and Size matches the entries.
//		next := 0
//		for bi, b := range s.Data {
//			sum := 0
//			for _, e := range b.Entries {
//				if want := string(rune('a' + next)); string(e.Key) != want {
//					t.Errorf("%s: entry %d has key %q, want %q", c.name, next, e.Key, want)
//				}
//				sum += len(e.Key) + len(e.Value)
//				next++
//			}
//			if b.Size != sum {
//				t.Errorf("%s: block %d Size=%d but entries add up to %d", c.name, bi, b.Size, sum)
//			}
//			if b.Size > budget && len(b.Entries) != 1 {
//				t.Errorf("%s: block %d is over budget (%d) but holds %d entries", c.name, bi, b.Size, len(b.Entries))
//			}
//		}
//		if next != len(c.sizes) {
//			t.Errorf("%s: stored %d entries, want %d", c.name, next, len(c.sizes))
//		}
//	}
//}
//
//func TestPutCopiesBuffers(t *testing.T) {
//	s := newTable()
//	k, v := []byte("k"), []byte("val")
//	s.Put(k, v)
//	k[0], v[0] = 'X', 'X' // the caller reuses its buffers
//	e := s.Data[0].Entries[0]
//	if string(e.Key) != "k" || string(e.Value) != "val" {
//		t.Fatal("table aliased the caller's buffers")
//	}
//}
