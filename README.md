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

## Project Structure

frozenDB follows standard Go project layout conventions:

```
frozenDB/
├── pkg/frozendb/          # PUBLIC API - Import from your applications
│   ├── frozendb.go        # Core database operations (open, close)
│   ├── transaction.go     # Transaction management
│   ├── errors.go          # Error types and handling
│   └── finder.go          # Query strategies
│
├── internal/frozendb/     # INTERNAL - Implementation details (not importable)
│   └── *.go               # All implementation code
│
├── cmd/frozendb/          # CLI TOOLS
│   └── main.go            # Command-line interface
│
├── examples/              # EXAMPLES
│   └── getting_started/   # Quick start guide
│       └── main.go        # Working example code
│
└── docs/                  # DOCUMENTATION
    ├── v1_file_format.md  # File format specification
    └── ...
```

**Import Path**: `github.com/susu-dot-dev/frozenDB/pkg/frozendb`

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

## Installation

```bash
go get github.com/susu-dot-dev/frozenDB/pkg/frozendb
```

## Quick Start

For a complete working example, see [examples/getting_started/main.go](examples/getting_started/main.go).

### Using the Public API

```go
package main

import (
    "encoding/json"
    "log"
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

type MyData struct {
    Message string `json:"message"`
}

func main() {
    // Open an existing database
    // (Database creation is done via CLI or internal package - see examples/)
    db, err := frozendb.NewFrozenDB(
        "/path/to/database.fdb",
        frozendb.MODE_WRITE,
        frozendb.FinderStrategySimple,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Begin a transaction
    tx, err := db.BeginTx()
    if err != nil {
        log.Fatal(err)
    }

    // Add a row with UUIDv7 key
    key := uuid.Must(uuid.NewV7())
    data := MyData{Message: "Hello, frozenDB!"}
    jsonData, _ := json.Marshal(data)
    
    if err := tx.AddRow(key, jsonData); err != nil {
        log.Fatal(err)
    }

    // Commit the transaction
    if err := tx.Commit(); err != nil {
        log.Fatal(err)
    }

    // Query the data
    var retrieved MyData
    if err := db.Get(key, &retrieved); err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Retrieved: %s", retrieved.Message)
}
```

### Building the CLI

```bash
# Build the frozendb CLI binary
make build-cli

# Binary is output to: dist/frozendb
./dist/frozendb

# Build example binaries
make build-examples

# Run the getting_started example
./dist/examples/getting_started
```

## Use Cases

- **Audit trails** - Immutable compliance records that must never be altered
- **Event sourcing** - Append-only event logs for replayable state reconstruction
- **Security logging** - Tamper-evident security event records
- **Compliance records** - Regulatory requirements for permanent, unmodifiable data

## API Design

### Core Database Operations

```go
// Open an existing database
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error)

// Access modes
const (
    MODE_READ  string = "r"  // Read-only access, multiple readers allowed
    MODE_WRITE string = "w"  // Read-write access, exclusive lock
)

// Query strategies
const (
    FinderStrategySimple       // Fixed memory, O(n) lookups
    FinderStrategyInMemory     // ~40 bytes/row, O(1) lookups
    FinderStrategyBinarySearch // O(log n) with time-based optimization
)

// Database methods
func (db *FrozenDB) BeginTx() (*Transaction, error)
func (db *FrozenDB) Get(key uuid.UUID, value any) error
func (db *FrozenDB) Close() error

// Transaction methods
func (tx *Transaction) AddRow(key uuid.UUID, value []byte) error
func (tx *Transaction) Commit() error
func (tx *Transaction) Rollback() error
func (tx *Transaction) Validate() error
```

### Database Creation

**Note**: Database creation is currently done via the internal package (see [examples/getting_started/](examples/getting_started/)) or will be available through the CLI in future releases.

```go
// TEMPORARY: Using internal package for creation
import internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"

config := internal.NewCreateConfig("/path/to/db.fdb", 256, 5000)
err := internal.Create(config)

// FUTURE: Use CLI for creation
// $ frozendb create --path /path/to/db.fdb --row-size 256 --skew-ms 5000
```

Database creation requires sudo privileges to set the append-only file attribute, ensuring immutability at the filesystem level.

### Error Handling

frozenDB uses structured error types for different failure scenarios:

```go
import "github.com/susu-dot-dev/frozenDB/pkg/frozendb"

func handleError(err error) {
    switch err.(type) {
    case *frozendb.InvalidInputError:
        // Invalid parameters or input validation failures
        fmt.Println("Invalid input:", err)
        
    case *frozendb.PathError:
        // Filesystem path issues
        fmt.Println("Path error:", err)
        
    case *frozendb.WriteError:
        // File write operations, sudo permissions, disk space
        fmt.Println("Write error:", err)
        
    case *frozendb.CorruptDatabaseError:
        // Database file corruption detected
        fmt.Println("Corrupt database:", err)
        
    case *frozendb.KeyNotFoundError:
        // Key does not exist in database
        fmt.Println("Key not found:", err)
        
    case *frozendb.TombstonedError:
        // Key exists but is marked as deleted
        fmt.Println("Key tombstoned:", err)
        
    default:
        fmt.Println("Unexpected error:", err)
    }
}
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
