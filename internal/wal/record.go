package wal

import (
	"encoding/binary"
	"hash/crc32"
	"lsmdb/internal/config"
)

func Encode(typ config.Kind, seq uint64, key, value []byte) []byte {
	buf := make([]byte, config.HeaderLen+len(key)+config.VlenLen+len(value))
	buf[4] = byte(typ)
	binary.LittleEndian.PutUint64(buf[5:], seq)
	binary.LittleEndian.PutUint32(buf[13:], uint32(len(key)))
	copy(buf[config.HeaderLen:], key)
	off := config.HeaderLen + len(key)
	binary.LittleEndian.PutUint32(buf[off:], uint32(len(value)))
	copy(buf[off+config.VlenLen:], value)
	binary.LittleEndian.PutUint32(buf[:4], crc32.ChecksumIEEE(buf[4:])) // crc of everything after itself
	return buf
}

// Decode reads one record from the start of data and returns the bytes it used.
func Decode(data []byte) (typ config.Kind, seq uint64, key, value []byte, n int, err error) {
	if len(data) < config.HeaderLen {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}
	klen := int(binary.LittleEndian.Uint32(data[13:]))
	if klen < 0 || len(data) < config.HeaderLen+klen+config.VlenLen {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}
	off := config.HeaderLen + klen
	vlen := int(binary.LittleEndian.Uint32(data[off:]))
	n = off + config.VlenLen + vlen
	if vlen < 0 || n > len(data) {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}
	if crc32.ChecksumIEEE(data[4:n]) != binary.LittleEndian.Uint32(data[:4]) {
		return 0, 0, nil, nil, 0, ErrCorrupt
	}
	return config.Kind(data[4]), binary.LittleEndian.Uint64(data[5:]),
		data[config.HeaderLen : config.HeaderLen+klen], data[off+config.VlenLen : n], n, nil
}
