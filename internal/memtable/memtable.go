package memtable

import "sync"

// MemTable is the mutable write buffer in front of the on-disk SSTables.
// It tracks approximate size so the engine knows when to flush it.
type MemTable struct {
	mu   sync.RWMutex
	list *SkipList
	size int // approximate bytes of keys + values
}

func New() *MemTable {
	return &MemTable{list: NewSkipList(BytewiseCompare, 1)}
}

func (m *MemTable) Put(key, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.list.Get(key); ok {
		m.size += len(value) - len(old)
	} else {
		m.size += len(key) + len(value)
	}
	m.list.Put(key, value)
}

func (m *MemTable) Get(key []byte) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.list.Get(key)
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

// NewIterator is what flush will use. It takes no lock, so only iterate a
// memtable that has been frozen (no more writes) — we'll formalize that later.
func (m *MemTable) NewIterator() *Iterator { return m.list.NewIterator() }
