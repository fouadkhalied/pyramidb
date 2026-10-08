package keys

import (
	"bytes"
	"encoding/binary"
	"lsmdb/internal/config"
)

// Parse splits an internal key
func Parse(ikey []byte) (userKey []byte, seq uint64, kind config.Kind, ok bool) {
	n := len(ikey) - config.TrailerLen
	if n < 0 {
		return nil, 0, 0, false
	}
	t := binary.LittleEndian.Uint64(ikey[n:])
	return ikey[:n], t >> 8, config.Kind(t & 0xff), true
}

// Compare orders internal keys: user key ascending, then newest version first.
func Compare(a, b []byte) int {
	if c := bytes.Compare(a[:len(a)-config.TrailerLen], b[:len(b)-config.TrailerLen]); c != 0 {
		return c
	}
	ta := binary.LittleEndian.Uint64(a[len(a)-config.TrailerLen:])
	tb := binary.LittleEndian.Uint64(b[len(b)-config.TrailerLen:])
	switch {
	case ta > tb:
		return -1 // higher trailer = newer = sorts first
	case ta < tb:
		return 1
	}
	return 0
}
