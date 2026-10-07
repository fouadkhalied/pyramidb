package sstable

import (
	"bufio"
	"os"
)

// Index is the small summary section at the end of the file.
type Internal struct {
	key []byte
}
type Entry struct {
	Key      []byte // user key
	Internal Internal
	Value    []byte
	Size     uint64
}

type IndexEntry struct {
	Largest []byte // largest INTERNAL key in the block (user key + seq/kind trailer)
	Offset  uint64 // where the block starts in the file
	Length  uint32 // block length in bytes, including its trailing CRC
}

type SSTable struct {
	f      *os.File
	footer Footer
	index  []IndexEntry
}

type Footer struct {
	IndexOffset uint64
	IndexLength uint64
	HighestSeq  uint64 // lets Open restore the sequence counter after the log is gone
	EntryCount  uint64
	Magic       uint64
}

type Block struct {
	Entries Entry
	Size    int
}

type Writer struct {
	f         *os.File
	bw        *bufio.Writer // buffering is fine here: nothing is visible until the final fsync + rename
	path      string        // final name; the data goes to path+".tmp" first
	blockSize int

	block        []byte // encoded entries of the block being built (CRC added when it is sealed)
	blockLargest []byte // internal key of the last entry added to that block
	offset       uint64 // bytes written so far = where the next block will start
	index        []IndexEntry

	count      uint64
	highestSeq uint64
	lastKey    []byte // previous internal key, to reject unsorted input
}

func (t *SSTable) Open(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f}, nil
}

func Init() *SSTable { return &SSTable{} }
