package wal

import (
	"lsmdb/internal/config"
	"os"
	"path/filepath"
	"sort"
)

// ListLogs returns the *.log files in dir, oldest first.
func ListLogs(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func Replay(path string, apply func(kind config.Kind, seq uint64, key, value []byte)) (goodLen int, maxSeq uint64, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	off := 0
	for off < len(data) {
		kind, seq, key, value, n, err := Decode(data[off:])
		if err != nil {
			break // torn tail: stop, keep what we have
		}
		apply(kind, seq, key, value)
		if seq > maxSeq {
			maxSeq = seq
		}
		off += n
	}
	return off, maxSeq, nil
}
