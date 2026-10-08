package sstable

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// File layout:
//
//	[ data block 0 ][ data block 1 ] ... [ index block ][ footer ]
//
// data block  = entry entry ... | 4B crc of the entries
// entry       = 4B key length | internal key | 4B value length | value
// index block = (4B key length | largest internal key of a block | 8B offset | 4B length) ... | 4B crc
// footer      = 5 x uint64: index offset, index length, highest seq, entry count, magic
//
// Everything is little-endian. An internal key is the user key plus the 8-byte
// seq/kind trailer made by keys.Make.
const (
	footerSize        = 5 * 8              // five fixed-width uint64 fields
	magic      uint64 = 0x4C534D5353543031 // "LSMSST01"
	crcLen            = 4
	minKeyLen         = 8 // an internal key always holds at least the trailer
)

// ---- entries ----

func entrySize(ikey, value []byte) int { return 4 + len(ikey) + 4 + len(value) }

func appendEntry(dst, ikey, value []byte) []byte {
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(ikey)))
	dst = append(dst, ikey...)
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(value)))
	return append(dst, value...)
}

// decodeEntry reads one entry from the start of src and says how many bytes it used.
// ikey and value point into src. It never panics: any length that does not fit is ErrCorrupt.
func decodeEntry(src []byte) (ikey, value []byte, n int, err error) {
	if len(src) < 4 {
		return nil, nil, 0, ErrCorrupt
	}
	klen := uint64(binary.LittleEndian.Uint32(src))
	if klen < minKeyLen || klen > uint64(len(src))-4 {
		return nil, nil, 0, ErrCorrupt
	}
	off := 4 + int(klen)
	if len(src)-off < 4 {
		return nil, nil, 0, ErrCorrupt
	}
	vlen := uint64(binary.LittleEndian.Uint32(src[off:]))
	off += 4
	if vlen > uint64(len(src)-off) {
		return nil, nil, 0, ErrCorrupt
	}
	end := off + int(vlen)
	return src[4 : 4+int(klen)], src[off:end], end, nil
}

// ---- blocks (data and index share the "payload + crc" shape) ----

// sealBlock appends the CRC of payload to payload.
func sealBlock(payload []byte) []byte {
	return binary.LittleEndian.AppendUint32(payload, crc32.ChecksumIEEE(payload))
}

// openBlock checks the trailing CRC and returns the payload without it.
func openBlock(raw []byte) ([]byte, error) {
	if len(raw) < crcLen {
		return nil, ErrCorrupt
	}
	payload := raw[:len(raw)-crcLen]
	if crc32.ChecksumIEEE(payload) != binary.LittleEndian.Uint32(raw[len(raw)-crcLen:]) {
		return nil, ErrCorrupt
	}
	return payload, nil
}

func encodeIndex(index []IndexEntry) []byte {
	var b []byte
	for _, ie := range index {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(ie.Largest)))
		b = append(b, ie.Largest...)
		b = binary.LittleEndian.AppendUint64(b, ie.Offset)
		b = binary.LittleEndian.AppendUint32(b, ie.Length)
	}
	return sealBlock(b)
}

func decodeIndex(payload []byte) ([]IndexEntry, error) {
	var out []IndexEntry
	for len(payload) > 0 {
		if len(payload) < 4 {
			return nil, ErrCorrupt
		}
		klen := uint64(binary.LittleEndian.Uint32(payload))
		if klen < minKeyLen || klen > uint64(len(payload))-4 || uint64(len(payload))-4-klen < 12 {
			return nil, ErrCorrupt
		}
		off := 4 + int(klen)
		out = append(out, IndexEntry{
			Largest: bytes.Clone(payload[4:off]),
			Offset:  binary.LittleEndian.Uint64(payload[off:]),
			Length:  binary.LittleEndian.Uint32(payload[off+8:]),
		})
		payload = payload[off+12:]
	}
	return out, nil
}

// ---- footer ----

func (f Footer) encode() []byte {
	b := make([]byte, 0, footerSize)
	for _, v := range []uint64{f.IndexOffset, f.IndexLength, f.HighestSeq, f.EntryCount, f.Magic} {
		b = binary.LittleEndian.AppendUint64(b, v)
	}
	return b
}

func decodeFooter(b []byte) (Footer, error) {
	if len(b) != footerSize {
		return Footer{}, ErrCorrupt
	}
	f := Footer{
		IndexOffset: binary.LittleEndian.Uint64(b[0:]),
		IndexLength: binary.LittleEndian.Uint64(b[8:]),
		HighestSeq:  binary.LittleEndian.Uint64(b[16:]),
		EntryCount:  binary.LittleEndian.Uint64(b[24:]),
		Magic:       binary.LittleEndian.Uint64(b[32:]),
	}
	if f.Magic != magic {
		return Footer{}, ErrCorrupt // not an SSTable, or damaged
	}
	return f, nil
}
