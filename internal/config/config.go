package config

type Kind uint8

const (
	KindDelete             Kind   = 0
	KindSet                Kind   = 1
	KindMax                       = KindSet // highest kind, used when seeking
	MaxMemTableSizeInBytes        = 4 * 1024
	MaxSSTableSizeInBytes         = 4 * 1024
	DefaultBlockSize              = 4 * 1024           // target size of one data block, in encoded bytes
	FooterSize                    = 5 * 8              // five fixed-width uint64 fields
	Magic                  uint64 = 0x4C534D5353543031 // "LSMSST01"
	HeaderLen                     = 4 + 1 + 8 + 4      // crc + type + seq + klen
	VlenLen                       = 4
	TrailerLen                    = 8
)
