package sstable

import (
	"bufio"
	"encoding/binary"
	"lsmdb/internal/config"
	"lsmdb/internal/keys"
	"os"
)

// Index is the small summary section at the end of the file.
type Internal struct {
	Key []byte
}

// Make builds an internal key. It always allocates, so the caller's slice is never touched.
func Make(userKey []byte, seq uint64, kind config.Kind) Internal {
	ikey := make([]byte, len(userKey)+config.TrailerLen)
	copy(ikey, userKey)
	binary.LittleEndian.PutUint64(ikey[len(userKey):], seq<<8|uint64(kind))
	return Internal{
		Key: ikey,
	}
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
	Entries []Entry
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
	tmp        string
}

func (e Entry) UserKey() []byte   { uk, _, _, _ := keys.Parse(e.Internal.Key); return uk }
func (e Entry) Seq() uint64       { _, seq, _, _ := keys.Parse(e.Internal.Key); return seq }
func (e Entry) Kind() config.Kind { _, _, kind, _ := keys.Parse(e.Internal.Key); return kind }
