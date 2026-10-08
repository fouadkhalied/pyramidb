package memtable

import (
	"lsmdb/internal/sstable"
)

// EntryIterator yields every version of every key:
// user key ascending, newest version first. That is exactly the order an
// SSTable stores, so flush needs no sorting.
type EntryIterator struct {
	n *node
	i int // index into n.versions, counting down (newest is last)
}

func (s *SkipList) NewEntryIterator() *EntryIterator {
	it := &EntryIterator{n: s.head.next[0]}
	if it.n != nil {
		it.i = len(it.n.versions) - 1
	}
	return it
}

func (it *EntryIterator) Valid() bool { return it.n != nil }

func (it *EntryIterator) Entry() sstable.Entry {
	v := it.n.versions[it.i]
	return sstable.Entry{Internal: sstable.Make(it.n.key, v.seq, v.kind), Value: v.val}
}

func (it *EntryIterator) Next() {
	it.i--
	if it.i < 0 {
		it.n = it.n.next[0]
		if it.n != nil {
			it.i = len(it.n.versions) - 1
		}
	}
}
