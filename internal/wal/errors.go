package wal

import "errors"

// ErrCorrupt means a record is truncated or fails its checksum.
var ErrCorrupt = errors.New("keys: corrupt or truncated record")
