package sstable

import "errors"

var (
	ErrCorrupt  = errors.New("sstable: corrupt or truncated")
	ErrUnsorted = errors.New("sstable: entries must be added in strictly increasing order")
	ErrEmpty    = errors.New("sstable: no entries to write")
	ErrBadKey   = errors.New("sstable: internal key is too short to hold a seq/kind trailer")
)
