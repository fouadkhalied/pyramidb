// Package iterator holds SSTable iterators and the merge iterator for range scans (section 7).
package iterator

import (
	"lsmdb/internal/memtable"
	"lsmdb/internal/sstable"
)

type Iterator struct {
	mem *memtable.MemTable
	sst *sstable.SSTable
}

func NewIterator(mem *memtable.MemTable, sst *sstable.SSTable) *Iterator {
	return &Iterator{mem: mem, sst: sst}
}

func (i *Iterator) Next() bool {

}