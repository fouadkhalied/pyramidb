package sstable

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"

	"lsmdb/internal/keys"
)

// Iterator is what WriteFile needs. It is declared here, by the consumer, so
// sstable never imports memtable.
type Iterator interface {
	Valid() bool
	Entry() Entry
	Next()
}

// NewWriter creates path+".tmp". O_EXCL makes it fail if that file already exists,
// so a bug can never write over somebody else's data.
func NewWriter(path string, blockSize int) (*Writer, error) {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f, bw: bufio.NewWriter(f), path: path, tmp: tmp, blockSize: blockSize}, nil
}

// Add appends one entry. Entries must come in strictly increasing internal-key order.
func (w *Writer) Add(e Entry) error {
	_, seq, _, ok := keys.Parse(e.Internal.Key)
	if !ok {
		return ErrBadKey
	}
	ik := bytes.Clone(e.Internal.Key) // the writer keeps this key, so it must not alias the caller's buffer
	if w.lastKey != nil && keys.Compare(w.lastKey, ik) >= 0 {
		return ErrUnsorted
	}

	// Packing rule: a block is never empty, and an entry that would push a
	// non-empty block over the budget starts a new block. A single oversized
	// entry therefore gets a block of its own.
	if len(w.block) > 0 && len(w.block)+entrySize(ik, e.Value) > w.blockSize {
		if err := w.sealBlock(); err != nil {
			return err
		}
	}
	w.block = appendEntry(w.block, ik, e.Value)
	w.blockLargest = ik
	w.lastKey = ik
	w.count++
	if seq > w.highestSeq {
		w.highestSeq = seq
	}
	return nil
}

func (w *Writer) sealBlock() error {
	w.block = sealBlock(w.block)
	if _, err := w.bw.Write(w.block); err != nil {
		return err
	}
	w.index = append(w.index, IndexEntry{Largest: w.blockLargest, Offset: w.offset, Length: uint32(len(w.block))})
	w.offset += uint64(len(w.block))
	w.block = w.block[:0]
	return nil
}

// Finish writes the last block, the index and the footer, makes the file durable,
// and installs it under its final name. On any error the temp file is removed.
func (w *Writer) Finish() error {
	if w.count == 0 {
		w.Abort()
		return ErrEmpty
	}
	if err := w.finish(); err != nil {
		w.Abort()
		return err
	}
	return nil
}

func (w *Writer) finish() error {
	if len(w.block) > 0 {
		if err := w.sealBlock(); err != nil {
			return err
		}
	}
	idx := encodeIndex(w.index)
	footer := Footer{
		IndexOffset: w.offset,
		IndexLength: uint64(len(idx)),
		HighestSeq:  w.highestSeq,
		EntryCount:  w.count,
		Magic:       magic,
	}
	if _, err := w.bw.Write(idx); err != nil {
		return err
	}
	if _, err := w.bw.Write(footer.encode()); err != nil {
		return err
	}
	if err := w.bw.Flush(); err != nil {
		return err
	}
	if err := w.f.Sync(); err != nil { // the file's bytes are on disk...
		return err
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	if err := os.Rename(w.tmp, w.path); err != nil { // ...then it appears under its real name, atomically
		return err
	}
	return syncDir(filepath.Dir(w.path)) // ...and the rename itself is made durable
}

// Abort throws the half-written file away. It is safe to call more than once.
func (w *Writer) Abort() {
	w.f.Close()
	os.Remove(w.tmp)
}

func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil // Windows cannot fsync a directory
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// WriteFile writes every entry of it as one SSTable at path.
func WriteFile(path string, it Iterator, blockSize int) error {
	w, err := NewWriter(path, blockSize)
	if err != nil {
		return err
	}
	for ; it.Valid(); it.Next() {
		if err := w.Add(it.Entry()); err != nil {
			w.Abort()
			return err
		}
	}
	return w.Finish()
}
