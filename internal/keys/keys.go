package keys

import (
	"encoding/binary"
	"hash/crc32"
)

type Kind uint8

const (
	KindDelete             Kind = 0
	KindSet                Kind = 1
	KindMax                     = KindSet // highest kind, used when seeking
	MaxMemTableSizeInBytes      = 4 * 1024
)

const (
	headerLen = 4 + 1 + 8 + 4 // crc + type + seq + klen
	vlenLen   = 4
)

type Entry struct { // make it key and value only for now
	Key []byte // user key
	//Seq   uint64
	//Kind  Kind
	Value []byte
}

func Encode(typ Kind, seq uint64, key, value []byte) []byte {
	buf := make([]byte, headerLen+len(key)+vlenLen+len(value))
	buf[4] = byte(typ)
	binary.LittleEndian.PutUint64(buf[5:], seq)
	binary.LittleEndian.PutUint32(buf[13:], uint32(len(key)))
	copy(buf[headerLen:], key)
	off := headerLen + len(key)
	binary.LittleEndian.PutUint32(buf[off:], uint32(len(value)))
	copy(buf[off+vlenLen:], value)
	binary.LittleEndian.PutUint32(buf[:4], crc32.ChecksumIEEE(buf[4:])) // crc of everything after itself
	return buf
}

// Decode reads one record from the start of data and returns the bytes it used.
func Decode(data []byte) (typ Kind, seq uint64, key, value []byte, n int, err error) {
	if len(data) < headerLen {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}

	klen := int(binary.LittleEndian.Uint32(data[13:]))

	if klen < 0 || len(data) < headerLen+klen+vlenLen {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}

	off := headerLen + klen

	vlen := int(binary.LittleEndian.Uint32(data[off:]))

	n = off + vlenLen + vlen

	if vlen < 0 || n > len(data) {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}

	if crc32.ChecksumIEEE(data[4:n]) != binary.LittleEndian.Uint32(data[:4]) {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}

	return Kind(data[4]), binary.LittleEndian.Uint64(data[5:]),
		data[headerLen : headerLen+klen], data[off+vlenLen : n], n, nil
}
