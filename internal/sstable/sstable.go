package sstable

import (
	"bytes"
	"lsmdb/internal/keys"
)

// Index is the small summary section at the end of the file.
type Index struct {
	NumEntries int
	Smallest   []byte // first user key in Data
	Largest    []byte // last user key in Data
}

type SSTable struct {
	MaxBlock int
	Data     []Block
	Index    Index
}

type Block struct {
	Entries []keys.Entry
	Size    int
}

const MaxBlockSize int = 4 * 1024 // 4KB

func Init() *SSTable { return &SSTable{MaxBlock: MaxBlockSize} }

func (t *SSTable) Len() int { return len(t.Data) }

func (t *SSTable) Put(key, value []byte) {
	size := len(key) + len(value)
	e := keys.Entry{Key: bytes.Clone(key), Value: bytes.Clone(value)}

	n := len(t.Data)
	if n == 0 || t.Data[n-1].Size+size > t.MaxBlock {
		t.Data = append(t.Data, Block{Entries: []keys.Entry{e}, Size: size})
		return
	}
	last := &t.Data[n-1]
	last.Entries = append(last.Entries, e)
	last.Size += size
}
