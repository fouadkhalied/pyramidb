package memtable

import (
	"testing"

	"lsmdb/internal/config"
)

// Every write keeps the old version, so the size must grow even when the
// new value is shorter than the old one.
func TestSizeOnlyGrows(t *testing.T) {
	m := New()
	m.Put([]byte("k"), 1, config.KindSet, []byte("a-long-value-here"))
	before := m.Size()

	m.Put([]byte("k"), 2, config.KindSet, []byte("x")) // shorter value
	after := m.Size()

	if after <= before {
		t.Fatalf("size went %d -> %d; a new version must add to it", before, after)
	}
	if got, want := after-before, versionOverhead+1; got != want {
		t.Fatalf("second write added %d, want %d (version overhead + value)", got, want)
	}
}

func TestSizeCountsKeyOncePerKey(t *testing.T) {
	m := New()
	m.Put([]byte("key"), 1, config.KindSet, []byte("v"))
	first := m.Size()
	if want := nodeOverhead + len("key") + versionOverhead + 1; first != want {
		t.Fatalf("new key cost %d, want %d", first, want)
	}

	m.Put([]byte("key"), 2, config.KindSet, []byte("v"))
	if added, want := m.Size()-first, versionOverhead+1; added != want {
		t.Fatalf("existing key added %d, want %d (no key bytes, no node)", added, want)
	}
}

func TestGetTreatsTombstoneAsMissing(t *testing.T) {
	m := New()
	m.Put([]byte("a"), 1, config.KindSet, []byte("v"))
	m.Put([]byte("a"), 2, config.KindDelete, nil)

	if _, ok := m.Get([]byte("a"), 1); !ok {
		t.Fatal("at snap 1 the value should be visible")
	}
	if _, ok := m.Get([]byte("a"), 2); ok {
		t.Fatal("at snap 2 the tombstone should hide it")
	}
}
