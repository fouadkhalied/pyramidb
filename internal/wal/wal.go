package wal

import (
	"os"
)

type Writer struct{ f *os.File }

func Open(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{f: f}, nil
}

func (w *Writer) Append(rec []byte) error {
	if _, err := w.f.Write(rec); err != nil { // one Write for the whole record
		return err
	}
	return w.f.Sync() // the durability boundary
}

func (w *Writer) Close() error {
	return w.f.Close()
}
