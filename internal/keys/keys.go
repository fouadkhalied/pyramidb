package keys

import (
	"bytes"
	"encoding/binary"
)

type Kind uint8

const (
	KindDelete Kind = 0
	KindSet    Kind = 1
	KindMax         = KindSet // highest kind, used when seeking
)

const trailerLen = 8

func Make(userKey []byte, seq uint64, kind Kind) []byte {
	ikey := make([]byte, len(userKey)+trailerLen)
	copy(ikey, userKey)
	binary.LittleEndian.PutUint64(ikey[len(userKey):], seq<<8|uint64(kind))
	return ikey
}

func Parse(ikey []byte) (userKey []byte, seq uint64, kind Kind) {
	n := len(ikey) - trailerLen
	t := binary.LittleEndian.Uint64(ikey[n:])
	return ikey[:n], t >> 8, Kind(t & 0xff)
}

func Compare(a, b []byte) int {
	if c := bytes.Compare(a[:len(a)-trailerLen], b[:len(b)-trailerLen]); c != 0 {
		return c // different user keys: normal order
	}
	ta := binary.LittleEndian.Uint64(a[len(a)-trailerLen:])
	tb := binary.LittleEndian.Uint64(b[len(b)-trailerLen:])
	switch {
	case ta > tb:
		return -1 // newer write sorts first
	case ta < tb:
		return 1
	}
	return 0
}