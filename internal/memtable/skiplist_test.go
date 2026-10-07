package memtable

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"lsmdb/internal/config"
)

// set is a test helper: write a KindSet version of k at the given seq.
func set(s *SkipList, seq uint64, k, v string) bool {
	return s.Put([]byte(k), seq, config.KindSet, []byte(v))
}

func TestSkipListPutGet(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	if _, ok := s.Get([]byte("a")); ok {
		t.Fatal("empty list should miss")
	}
	set(s, 1, "b", "2")
	set(s, 2, "a", "1")
	set(s, 3, "c", "3")
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

func TestSkipListNewVersionSameKey(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	if set(s, 1, "k", "v1") {
		t.Fatal("first Put should report existed=false")
	}
	if !set(s, 2, "k", "v2") {
		t.Fatal("second Put should report existed=true")
	}
	if got, _ := s.Get([]byte("k")); string(got) != "v2" {
		t.Fatalf("Get = %q, want v2", got)
	}
	if s.Len() != 1 {
		t.Fatalf("Len = %d, want 1 distinct key", s.Len())
	}
	if n := s.findGE([]byte("k"), nil); len(n.versions) != 2 {
		t.Fatalf("versions = %d, want 2 (old one must be kept)", len(n.versions))
	}
}

func TestSkipListGetAt(t *testing.T) {
	// The example from the lesson, written in global seq order.
	s := NewSkipList(BytewiseCompare, 1)
	set(s, 12, "user:1", "alice@v1")
	set(s, 87, "user:1", "alice@v2")
	set(s, 99, "user:2", "bob")
	set(s, 105, "user:1", "alice@v3")

	cases := []struct {
		key  string
		snap uint64
		want string // "" means not visible
	}{
		{"user:1", 11, ""},
		{"user:1", 12, "alice@v1"},
		{"user:1", 86, "alice@v1"},
		{"user:1", 90, "alice@v2"},
		{"user:1", 200, "alice@v3"},
		{"user:2", 90, ""}, // written at 99, after this snapshot
		{"user:2", 99, "bob"},
		{"user:3", 200, ""}, // never written
	}
	for _, c := range cases {
		v, ok := s.GetAt([]byte(c.key), c.snap)
		if c.want == "" {
			if ok {
				t.Fatalf("GetAt(%q, %d) = %q, want miss", c.key, c.snap, v.val)
			}
			continue
		}
		if !ok || string(v.val) != c.want {
			t.Fatalf("GetAt(%q, %d) = %q, %v; want %q", c.key, c.snap, v.val, ok, c.want)
		}
	}
}

func TestSkipListDeleteVersion(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	set(s, 10, "a", "v1")
	s.Put([]byte("a"), 20, config.KindDelete, nil)

	if v, ok := s.GetAt([]byte("a"), 15); !ok || v.kind != config.KindSet {
		t.Fatalf("at snap 15 want the Set version, got %+v, %v", v, ok)
	}
	if v, ok := s.GetAt([]byte("a"), 25); !ok || v.kind != config.KindDelete {
		t.Fatalf("at snap 25 want the tombstone, got %+v, %v", v, ok)
	}
}

func TestSkipListSortedIteration(t *testing.T) {
	s := NewSkipList(BytewiseCompare, 1)
	rnd := rand.New(rand.NewSource(42))
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("key-%04d", rnd.Intn(500))
		seen[k] = true
		set(s, uint64(i+1), k, "v")
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
	for i, k := range []string{"b", "d", "f"} {
		set(s, uint64(i+1), k, k)
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
