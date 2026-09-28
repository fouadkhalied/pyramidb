package memtable

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

func TestSkipListPutGet(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	if _, ok := s.Get([]byte("a")); ok {
		t.Fatal("empty list should miss")
	}
	s.Put([]byte("b"), []byte("2"))
	s.Put([]byte("a"), []byte("1"))
	s.Put([]byte("c"), []byte("3"))
	for k, want := range map[string]string{"a": "1", "b": "2", "c": "3"} {
		got, ok := s.Get([]byte(k))
		if !ok || string(got) != want {
			t.Fatalf("Get(%q) = %q, %v; want %q", k, got, ok, want)
		}
	}
	if s.Len() != 3 {
		t.Fatalf("Len = %d, want 3", s.Len())
	}
}

func TestSkipListOverwrite(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	if s.Put([]byte("k"), []byte("v1")) {
		t.Fatal("first Put should not report replaced")
	}
	if !s.Put([]byte("k"), []byte("v2")) {
		t.Fatal("second Put should report replaced")
	}
	if got, _ := s.Get([]byte("k")); string(got) != "v2" {
		t.Fatalf("got %q, want v2", got)
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1", s.Len())
	}
}

func TestSkipListSortedIteration(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	rnd := rand.New(rand.NewSource(42))
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("key-%04d", rnd.Intn(500))
		seen[k] = true
		s.Put([]byte(k), []byte("v"))
	}
	want := make([]string, 0, len(seen))
	for k := range seen {
		want = append(want, k)
	}
	sort.Strings(want)

	i := 0
	it := s.NewIterator()
	for it.SeekToFirst(); it.Valid(); it.Next() {
		if i >= len(want) || string(it.Key()) != want[i] {
			t.Fatalf("position %d: got %q", i, it.Key())
		}
		i++
	}
	if i != len(want) || s.Len() != len(want) {
		t.Fatalf("iterated %d, Len %d, want %d", i, s.Len(), len(want))
	}
}

func TestSkipListSeek(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	for _, k := range []string{"b", "d", "f"} {
		s.Put([]byte(k), []byte(k))
	}
	it := s.NewIterator()
	cases := []struct{ target, want string }{{"a", "b"}, {"c", "d"}, {"d", "d"}, {"e", "f"}}
	for _, c := range cases {
		it.Seek([]byte(c.target))
		if !it.Valid() || string(it.Key()) != c.want {
			t.Fatalf("Seek(%q) landed wrong, want %q", c.target, c.want)
		}
	}
	it.Seek([]byte("g"))
	if it.Valid() {
		t.Fatal("Seek past the last key should be invalid")
	}
}
