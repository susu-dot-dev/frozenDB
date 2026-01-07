# frozenDB

A simple, immutable key-value store built on append-only files with UUIDv7 keys for audit trails and event sourcing.

## Overview

frozenDB is a single-file key-value store where rows can only be added, never removed or modified. By requiring UUIDv7 keys (which are time-ordered), frozenDB enables efficient lookups through binary search with bounded linear probing within a configurable time skew window. This makes it ideal for audit trails, event sourcing, security logs, and compliance records where immutability and chronological ordering are essential.

## Key Features

- **Append-only immutability** - Data written to frozenDB is permanent and tamper-resistant
- **Fixed-width rows** - Enables direct seeks to an individual row, and simple corruption detection
- **UUIDv7 keys** - Time-ordered identifiers that enable binary search without traditional indexing
- **Configurable time skew** - Handles clock drift and race conditions in distributed systems
- **Binary search + linear probe** - O(log n) lookups with bounded linear search within skew window
- **Single-file storage** - Simple deployment and backup strategies
- **Text-based data format** - Easy to parse the database file, recover errors. Any JSON serializable type is allowed

## Architecture

```
┌─────────────────────────────────────┐
│           frozenDB File             │
├─────────────────────────────────────┤
│  Header (row size, metadata)        │
├─────────────────────────────────────┤
│  R0: [tx_0|uuid7|json|rw_end_0]     │
│  R1: [rw_1|uuid7|json|tx_cmt_1]     │
│  R2: [tx_1|uuid7|json|tx_cxl_1]     │
│  ...                                │
└─────────────────────────────────────┘
```

1. **Insert**: Direct append to end of file. Multiple insert atomicity guaranteed by transaction headers
2. **Lookup**: Binary search to find key range within time skew, then linear probe within that range
3. **Concurrency**: Append-only means that reads can happen concurrently with writes, (avoiding any open transaction)
3. **Corruption detection**: Sentinel bytes validate transaction integrity

## Use Cases

- **Audit trails** - Immutable compliance records that must never be altered
- **Event sourcing** - Append-only event logs for replayable state reconstruction
- **Security logging** - Tamper-evident security event records
- **Compliance records** - Regulatory requirements for permanent, unmodifiable data

## API Design (coming soon)

```go
type DB struct { Path string, ... }

func NewDB(path string, rowSize int, opts ...Option) (*DB, error)
func (db *DB) BeginTx() error
func (db *DB) CommitTx() error
func (db *DB) AbortTx() error
func (db *DB) Add(key uuid.UUID, value interface{}) error
func (db *DB) Get(key uuid.UUID) (interface{}, bool, error)
func (db *DB) Enumerate() <-chan KeyValue
func (db *DB) Count() (int64, error)
func (db *DB) Close() error
```

## Design Philosophy

frozenDB explores how append-only storage combined with UUIDv7's time-ordered keys can enable efficient key-value lookups without traditional indexing. By embracing immutability and leveraging the time component in UUIDv7, frozenDB provides a simple yet powerful solution for scenarios where data must never be altered and chronological ordering is essential.

## Performance
In order to utilize single-file storage (for on-disk simplicity), along with an append-only data structure, frozenDB has extra logic to ensure performant operations. Since rows are never deleted, it is simple to carry in memory caches to reduce disk seeks. Next, since the rows are roughly ordered (+- time skew), a binary search can be utilized to find matching rows. Disk seeks and searches can be further optimized by caching primary binary search keys, LRUs around keys, being efficient with disk reads (corresponding to sector size) and so on. The design of frozenDB is that memory usage should be fixed, and not scale with the database size.

## Disk reliability
frozenDB can be opened in read or read+write mode. Only one process can open the file in write mode, enforced via OS-level locks (flocks). Thus, from a reliability perspective, the only remaining failure case would be a partial failure to write or flush to disk. This is detected by missing per-row sentinels, which can detect invalid rows. Reads do not need any special care around reliability. The append-only nature of the file means that any number of concurrent readers can read from disk, independent of writes. Readers just need to ensure they drop the last transaction should it not be complete. The append-only nature also means that disk flushing does not need to be awaited on before reading. Instead, fsync can just be called periodically only as a data-preservation mechanism

## Status

Currently in development - coming soon!

## License

MIT
