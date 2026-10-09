package memtable

import (
	"lsmdb/internal/config"
	"sync"
	"unsafe"
)

// MemTable is the mutable write buffer in front of the on-disk SSTables.
// It tracks approximate size so the engine knows when to flush it.
type MemTable struct {
	mu   sync.RWMutex
	list *SkipList
	size int // approximate bytes of keys + values
}

var (
	versionOverhead = int(unsafe.Sizeof(version{}))                                 // 40 bytes
	nodeOverhead    = int(unsafe.Sizeof(node{})) + 2*int(unsafe.Sizeof(uintptr(0))) // 88 bytes
)

func New() *MemTable {
	return &MemTable{list: NewSkipList(BytewiseCompare, 1)}
}

func (m *MemTable) Put(key []byte, seq uint64, kind config.Kind, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existed := m.list.Put(key, seq, kind, value)
	m.size += versionOverhead + len(value) // every write adds a version
	if !existed {
		m.size += nodeOverhead + len(key) // the key is stored once, in its node
	}
}

func (m *MemTable) Get(key []byte, snap uint64) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.list.GetAt(key, snap)
	if !ok || v.kind != config.KindSet { // a tombstone counts as "not found"
		return nil, false
	}
	return v.val, true
}

// ApproximateSize is what the engine compares against its flush threshold.
func (m *MemTable) ApproximateSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

func (m *MemTable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.list.Len()
}

func (m *MemTable) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.size
}

// NewEntryIterator is what flush will use. It takes no lock, so only iterate a
func (m *MemTable) NewEntryIterator() *EntryIterator { return m.list.NewEntryIterator() }
