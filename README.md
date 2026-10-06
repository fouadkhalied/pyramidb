# lsmdb

An LSM-tree storage engine in Go, built alongside shipthatcode's "Build an LSM Tree" course.

    go test -race ./...

CI runs `gofmt`, `go vet` and the tests with the race detector on every push.

| Course section                  | Package                        | Status      | Notes |
|---------------------------------|--------------------------------|-------------|-------|
| 1. MemTable foundations         | internal/memtable, internal/keys | done      | Skip list with versioned entries (seq + kind), size accounting |
| 2. Write-ahead log              | internal/wal                   | done        | Checksummed records, group commit (one fsync per batch), crash recovery with torn-tail trimming |
| 3. SSTable on-disk format       | internal/sstable               | in progress | In-memory placeholder with block packing; on-disk format, block index and footer next |
| 4. Flush & read path            | db, internal/sstable           | in progress | Memtable freezing and per-memtable WAL rotation done; flusher, SSTable reads and WAL cleanup next |
| 5. Bloom filters                | internal/bloom                 |             | |
| 6. Compaction                   | internal/compaction            |             | |
| 7. Range queries & iterators    | internal/iterator              |             | |
| 8. Tombstones & deletes         | memtable + sstable + iterator  |             | |

## How a write works

1. `Put` assigns the next sequence number and queues the record.
2. A single writer goroutine appends the queued records to the log, then calls fsync once for the whole batch.
3. After the fsync, the records are applied to the memtable and the writers are told "done".
4. When the memtable reaches its size limit, it is frozen into an immutable list and a new memtable and a new log are started.

On startup, `Open` replays the logs in order, trims a torn tail on the newest one, restores the sequence counter, and starts a fresh log.

## Layout

    db/                  public API: Open, Put, Get, Close; write loop; rotation
    internal/keys        entry type, kinds, log record encoding
    internal/memtable    skip list and the memtable wrapper
    internal/wal         log writer, replay and recovery helpers
    internal/sstable     blocks and tables (in progress)
    internal/bloom       (not started)
    internal/compaction  (not started)
    internal/iterator    (not started)