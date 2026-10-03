package sstable

import (
	"bytes"
	"sort"

	"lsmdb/internal/keys"
)

// Index is the small summary section at the end of the file.
type Index struct {
	NumEntries int
	Smallest   []byte // first user key in Data
	Largest    []byte // last user key in Data
}

type SSTable struct {
	Data  []keys.Entry // sorted: user key ascending, then seq descending
	Index Index
}

func Init() *SSTable { return &SSTable{Data: make([]keys.Entry, 0)} }

func (t *SSTable) Len() int { return len(t.Data) }

// Sort orders entries by user key ascending, then newest version first.
func (t *SSTable) Sort() {
	sort.Slice(t.Data, func(i, j int) bool {
		if c := bytes.Compare(t.Data[i].Key, t.Data[j].Key); c != 0 {
			return c < 0
		}
		return t.Data[i].Seq > t.Data[j].Seq
	})
}

// Put keeps its own copies, so callers can reuse their buffers.
func (t *SSTable) Put(key []byte, seq uint64, kind keys.Kind, value []byte) {
	t.Data = append(t.Data, keys.Entry{
		Key:   bytes.Clone(key),
		Seq:   seq,
		Kind:  kind,
		Value: bytes.Clone(value),
	})
}

// Compact sorts the data and fills in the index.
func (t *SSTable) Compact() {
	t.Sort()
	t.Index.NumEntries = len(t.Data)
	if len(t.Data) == 0 {
		t.Index.Smallest, t.Index.Largest = nil, nil
		return
	}
	t.Index.Smallest = t.Data[0].Key
	t.Index.Largest = t.Data[len(t.Data)-1].Key
}
