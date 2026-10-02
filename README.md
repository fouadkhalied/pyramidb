# lsmdb

An LSM-tree storage engine in Go, built alongside shipthatcode's "Build an LSM Tree" course.

    go test ./...

| Course section                  | Package                      | Status |
|---------------------------------|------------------------------|--------|
| 1. MemTable foundations         | internal/memtable            | done   |
| 2. Write-ahead log              | internal/wal                 | done   |
| 3. SSTable on-disk format       | internal/sstable             |        |
| 4. Flush & read path            | db.go + internal/sstable     |        |
| 5. Bloom filters                | internal/bloom               |        |
| 6. Compaction                   | internal/compaction          |        |
| 7. Range queries & iterators    | internal/iterator            |        |
| 8. Tombstones & deletes         | memtable + sstable + iterator|        |
