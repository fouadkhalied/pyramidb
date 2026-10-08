package sstable

import (
	"bytes"
	"errors"
	"io"
	"lsmdb/internal/config"
	"os"
	"sort"

	"lsmdb/internal/keys"
)

// Table is an open SSTable. It keeps the footer and the index in memory;
// a lookup reads at most one data block from disk.
type Table struct {
	f      *os.File
	path   string
	footer Footer
	index  []IndexEntry
}

// Open validates the footer and the index. Data blocks are checked when they are read.
func Open(path string) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	t, err := load(f, path)
	if err != nil {
		f.Close()
		return nil, err
	}
	return t, nil
}

func load(f *os.File, path string) (*Table, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := uint64(st.Size())
	if size < footerSize+crcLen {
		return nil, ErrCorrupt
	}

	fb := make([]byte, footerSize)
	if err := readAt(f, fb, int64(size-footerSize)); err != nil {
		return nil, err
	}
	ft, err := decodeFooter(fb)
	if err != nil {
		return nil, err
	}
	// The index must end exactly where the footer begins.
	if ft.IndexLength < crcLen || ft.IndexLength > size || ft.IndexOffset > size ||
		ft.IndexOffset+ft.IndexLength != size-footerSize {
		return nil, ErrCorrupt
	}

	raw := make([]byte, ft.IndexLength)
	if err := readAt(f, raw, int64(ft.IndexOffset)); err != nil {
		return nil, err
	}
	payload, err := openBlock(raw)
	if err != nil {
		return nil, err
	}
	index, err := decodeIndex(payload)
	if err != nil || len(index) == 0 {
		return nil, ErrCorrupt
	}

	// The blocks must tile the data section exactly, and their largest keys must increase.
	next := uint64(0)
	for i, ie := range index {
		if ie.Length < crcLen || ie.Offset != next {
			return nil, ErrCorrupt
		}
		next += uint64(ie.Length)
		if i > 0 && keys.Compare(index[i-1].Largest, ie.Largest) >= 0 {
			return nil, ErrCorrupt
		}
	}
	if next != ft.IndexOffset {
		return nil, ErrCorrupt
	}
	return &Table{f: f, path: path, footer: ft, index: index}, nil
}

// readAt fills buf from off. A short file is ErrCorrupt.
func readAt(f *os.File, buf []byte, off int64) error {
	n, err := f.ReadAt(buf, off)
	if n == len(buf) {
		return nil // ReadAt may report io.EOF together with a complete read
	}
	if err == nil || errors.Is(err, io.EOF) {
		return ErrCorrupt
	}
	return err
}

// Get returns the newest version of userKey with seq <= snap, which may be a tombstone
// (check Kind). ok is false if this table holds no such version.
func (t *Table) Get(userKey []byte, snap uint64) (e Entry, ok bool, err error) {
	// The first entry at or after this target is the newest visible version, if any:
	// higher sequence numbers sort first, and KindMax is the highest kind.
	target := Make(userKey, snap, config.KindMax)

	// First block whose largest key is >= target. A key's versions can straddle two
	// blocks, which is why the index stores internal keys and not user keys.
	i := sort.Search(len(t.index), func(i int) bool { return keys.Compare(t.index[i].Largest, target.Key) >= 0 })
	if i == len(t.index) {
		return Entry{}, false, nil
	}

	ie := t.index[i]
	raw := make([]byte, ie.Length)
	if err := readAt(t.f, raw, int64(ie.Offset)); err != nil {
		return Entry{}, false, err
	}
	payload, err := openBlock(raw)
	if err != nil {
		return Entry{}, false, err
	}

	for len(payload) > 0 {
		ik, val, n, err := decodeEntry(payload)
		if err != nil {
			return Entry{}, false, err
		}
		if keys.Compare(ik, target.Key) >= 0 {
			uk, _, _, _ := keys.Parse(ik) // decodeEntry guarantees the trailer is there
			if !bytes.Equal(uk, userKey) {
				return Entry{}, false, nil // the next key in this file is a different one
			}
			return Entry{Internal: Internal{Key: ik}, Value: val}, true, nil
		}
		payload = payload[n:]
	}
	// The index promised an entry >= target in this block but there is none.
	return Entry{}, false, ErrCorrupt
}

func (t *Table) HighestSeq() uint64 { return t.footer.HighestSeq }
func (t *Table) EntryCount() uint64 { return t.footer.EntryCount }
func (t *Table) Path() string       { return t.path }
func (t *Table) Close() error       { return t.f.Close() }

func Init() *SSTable { return &SSTable{} }

func (t *Writer) Close() error {
	return t.f.Close()
}
