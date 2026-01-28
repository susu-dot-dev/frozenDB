# API Contract: Project Structure Refactor & CLI

**Feature**: 028-pkg-internal-cli-refactor  
**Date**: 2026-01-28  
**Status**: Complete

## Overview

This document specifies the **minimal public API** for frozenDB after the refactor. The public API is exposed through `/pkg/frozendb` and includes ONLY the core operations needed for working with existing databases. Database creation is excluded from the public API as it will be handled by CLI commands in future releases.

## Public Package: `pkg/frozendb`

**Import Path**: `github.com/susu-dot-dev/frozenDB/pkg/frozendb`

**Package Declaration**: `package frozendb`

**Philosophy**: Minimal surface area - expose only what's needed for core database operations (open, transaction, query, close).

---

## Core Database Types

### FrozenDB

**Type**: Re-exported from internal implementation

```go
type FrozenDB = internal_frozendb.FrozenDB
```

**Description**: Represents an open connection to a frozenDB database file. Instance methods are NOT thread-safe - use one instance per goroutine. Close() is thread-safe.

**Public Methods**:
- `BeginTransaction() error` - Start a new transaction
- `GetTransaction() (*Transaction, error)` - Get the active transaction
- `Close() error` - Close the database and release resources

**Usage**:
```go
db, err := frozendb.NewFrozenDB("/data/mydb.fdb", frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

err = db.BeginTransaction()
// ... work with transaction
```

---

### Transaction

**Type**: Re-exported from internal implementation (method subset only)

```go
type Transaction = internal_frozendb.Transaction
```

**Description**: Represents a database transaction. Supports adding rows and committing/rolling back changes.

**Public Methods**:
- `Begin() error` - Initialize the transaction
- `AddRow(key uuid.UUID, value []byte) error` - Add a key-value pair to the transaction
- `Commit() error` - Commit the transaction to disk
- `Rollback() error` - Rollback the transaction
- `IsTombstoned() bool` - Check if transaction is in error state
- `Validate() error` - Validate transaction state

**NOT Exposed** (internal-only):
- ❌ `GetEmptyRow() *NullRow` - Exposes internal type
- ❌ `GetRows() []DataRow` - Exposes internal type

**Usage**:
```go
tx, err := db.GetTransaction()
if err != nil {
    log.Fatal(err)
}

key := uuid.Must(uuid.NewV7())
value := []byte(`{"data": "value"}`)

err = tx.AddRow(key, value)
if err != nil {
    log.Fatal(err)
}

err = tx.Commit()
if err != nil {
    log.Fatal(err)
}
```

---

## Constants

### Access Modes

```go
const (
    // MODE_READ opens database in read-only mode with no lock
    // Multiple readers can access the same file concurrently
    MODE_READ = "read"
    
    // MODE_WRITE opens database in read-write mode with exclusive lock
    // Only one writer can access the file at a time
    MODE_WRITE = "write"
)
```

**Usage**:
```go
// Read-only access
db, err := frozendb.NewFrozenDB("/path/to/db.fdb", frozendb.MODE_READ, frozendb.FinderStrategySimple)

// Read-write access
db, err := frozendb.NewFrozenDB("/path/to/db.fdb", frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
```

---

### Finder Strategies

```go
type FinderStrategy = string

const (
    // FinderStrategySimple: O(row_size) fixed memory regardless of DB size
    // GetIndex O(n), GetTransactionStart/End O(k) where k ≤ 101
    // Use when DB is large or memory is bounded
    FinderStrategySimple FinderStrategy = "simple"
    
    // FinderStrategyInMemory: ~40 bytes per row (uuid map + tx boundary maps)
    // GetIndex, GetTransactionStart, GetTransactionEnd all O(1)
    // Use when DB fits in memory and read-heavy workloads need low latency
    FinderStrategyInMemory FinderStrategy = "inmemory"
    
    // FinderStrategyBinarySearch: Optimized for time-ordered UUID lookups
    // Uses binary search on chronologically ordered keys
    // GetIndex O(log n) with time-based optimizations
    FinderStrategyBinarySearch FinderStrategy = "binary_search"
)
```

**Usage**:
```go
// For large databases with memory constraints
db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ, frozendb.FinderStrategySimple)

// For in-memory optimized fast lookups
db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ, frozendb.FinderStrategyInMemory)

// For time-ordered query optimization
db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ, frozendb.FinderStrategyBinarySearch)
```

---

## Constructor Functions

### NewFrozenDB

**Signature**:
```go
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error)
```

**Description**: Opens an existing frozenDB database file with specified access mode and finder strategy.

**Parameters**:
- `path`: Filesystem path to existing frozenDB database file (.fdb extension required)
- `mode`: Access mode - MODE_READ for read-only, MODE_WRITE for read-write
- `strategy`: FinderStrategySimple, FinderStrategyInMemory, or FinderStrategyBinarySearch

**Returns**:
- `*FrozenDB`: Database instance ready for operations
- `error`: InvalidInputError (invalid strategy), PathError, CorruptDatabaseError, or WriteError

**Thread Safety**: Safe for concurrent calls on different files

**Example**:
```go
db, err := frozendb.NewFrozenDB("/data/mydb.fdb", frozendb.MODE_READ, frozendb.FinderStrategySimple)
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

---

## Query Methods (via Finder)

The FrozenDB instance provides access to query methods through its internal Finder interface. These methods are accessible via the FrozenDB instance:

### GetIndex

**Description**: Returns the index of the first row containing the specified UUID key.

**Parameters**:
- `key`: The UUIDv7 key to search for

**Returns**:
- `index`: Zero-based index of the matching row
- `error`: KeyNotFoundError if not found, InvalidInputError for invalid UUID

**Usage**:
```go
key := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
index, err := db.GetIndex(key)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Found at index: %d\n", index)
```

### GetTransactionStart

**Description**: Returns the index of the first row in the transaction containing the specified index.

**Parameters**:
- `index`: Index of a row within the transaction

**Returns**:
- `startIndex`: Index of the transaction start row
- `error`: InvalidInputError, CorruptDatabaseError, or ReadError

### GetTransactionEnd

**Description**: Returns the index of the last row in the transaction containing the specified index.

**Parameters**:
- `index`: Index of a row within the transaction

**Returns**:
- `endIndex`: Index of the transaction end row
- `error`: InvalidInputError, TransactionActiveError, CorruptDatabaseError, or ReadError

---

## Error Types

All error types are re-exported from internal implementation. Each error embeds `FrozenDBError` base type with Code, Message, and Err fields.

### Error Type Re-exports

```go
type FrozenDBError = internal_frozendb.FrozenDBError
type InvalidInputError = internal_frozendb.InvalidInputError
type InvalidActionError = internal_frozendb.InvalidActionError
type PathError = internal_frozendb.PathError
type WriteError = internal_frozendb.WriteError
type CorruptDatabaseError = internal_frozendb.CorruptDatabaseError
type KeyOrderingError = internal_frozendb.KeyOrderingError
type TombstonedError = internal_frozendb.TombstonedError
type ReadError = internal_frozendb.ReadError
type KeyNotFoundError = internal_frozendb.KeyNotFoundError
type TransactionActiveError = internal_frozendb.TransactionActiveError
type InvalidDataError = internal_frozendb.InvalidDataError
```

### Error Constructor Functions

```go
func NewInvalidInputError(message string, err error) *InvalidInputError
func NewInvalidActionError(message string, err error) *InvalidActionError
func NewPathError(message string, err error) *PathError
func NewWriteError(message string, err error) *WriteError
func NewCorruptDatabaseError(message string, err error) *CorruptDatabaseError
func NewKeyOrderingError(message string, err error) *KeyOrderingError
func NewTombstonedError(message string, err error) *TombstonedError
func NewReadError(message string, err error) *ReadError
func NewKeyNotFoundError(message string, err error) *KeyNotFoundError
func NewTransactionActiveError(message string, err error) *TransactionActiveError
func NewInvalidDataError(message string, err error) *InvalidDataError
```

**Error Handling Example**:
```go
db, err := frozendb.NewFrozenDB(path, frozendb.MODE_READ, frozendb.FinderStrategySimple)
if err != nil {
    var pathErr *frozendb.PathError
    var corruptErr *frozendb.CorruptDatabaseError
    
    switch {
    case errors.As(err, &pathErr):
        log.Printf("Path error: %v", err)
    case errors.As(err, &corruptErr):
        log.Printf("Database corrupted: %v", err)
    default:
        log.Printf("Unexpected error: %v", err)
    }
    return
}
```

---

## NOT in Public API

The following types and functions are **intentionally excluded** from the public API and remain in `/internal/frozendb`:

### Database Creation (CLI-only in future)
- ❌ `CreateFrozenDB()` - Create new database files
- ❌ `CreateConfig` - Configuration for database creation
- ❌ `NewCreateConfig()` - Constructor for CreateConfig
- ❌ `SudoContext` - Sudo environment details
- ❌ `FILE_EXTENSION`, `FILE_PERMISSIONS` constants

**Rationale**: Database creation will be handled by CLI commands (e.g., `frozendb create`) in future releases. Library users typically work with existing databases.

### Internal Row Types
- ❌ `NullRow` - Empty transaction row structure
- ❌ `DataRow` - Data row structure
- ❌ `PartialDataRow` - Partial row structure
- ❌ `Row` - Row union type
- ❌ `RowUnion` - Row type wrapper

**Rationale**: These are file format implementation details that should not be exposed to users.

### Internal Transaction Methods
- ❌ `Transaction.GetEmptyRow()` - Returns internal NullRow
- ❌ `Transaction.GetRows()` - Returns internal DataRow slice

**Rationale**: Exposes internal row structures that are implementation details.

### File Format Details
- ❌ `Header` - Database header structure
- ❌ `HEADER_SIZE`, `HEADER_FORMAT` constants
- ❌ Checksum types and functions

**Rationale**: File format details are implementation concerns, not user concerns.

---

## CLI Binary

### Installation

**Build from source**:
```bash
make build-cli
```

**Output**: `frozendb` binary in repository root

### Usage

**Current (v028)**:
```bash
./frozendb
# Output: Hello world
```

**Future Commands** (not in scope for v028):
```bash
./frozendb create --path /data/db.fdb --row-size 256 --skew-ms 5000
./frozendb open --path /data/db.fdb --mode read
./frozendb query --path /data/db.fdb --key <uuid>
```

---

## Complete Usage Example

**See**: `examples/getting_started/main.go` for complete working example

**Note**: Example uses internal package for database creation (temporary until CLI supports it)

**Basic Workflow**:

```go
package main

import (
    "log"
    
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
    internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

func main() {
    // 1. Create database (using internal package - temporary)
    config, err := internal.NewCreateConfig("/tmp/example.fdb", 256, 5000)
    if err != nil {
        log.Fatal(err)
    }
    
    err = internal.CreateFrozenDB(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. Open database for writing (using public API)
    db, err := frozendb.NewFrozenDB("/tmp/example.fdb", frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 3. Begin transaction (public API)
    err = db.BeginTransaction()
    if err != nil {
        log.Fatal(err)
    }
    
    tx, err := db.GetTransaction()
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. Add rows (public API)
    key := uuid.Must(uuid.NewV7())
    value := []byte(`{"name": "example", "count": 42}`)
    
    err = tx.AddRow(key, value)
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Commit transaction (public API)
    err = tx.Commit()
    if err != nil {
        log.Fatal(err)
    }
    
    // 6. Query (public API - via finder methods)
    index, err := db.GetIndex(key)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Successfully created, wrote, and queried database. Key found at index: %d\n", index)
}
```

**Future** (once CLI supports creation):
```go
package main

import (
    "github.com/susu-dot-dev/frozenDB/pkg/frozendb"  // ONLY public API needed
)

func main() {
    // Step 1: User runs CLI to create database
    // $ frozendb create --path /tmp/example.fdb --row-size 256 --skew-ms 5000
    
    // Step 2: Code only uses public API
    db, err := frozendb.NewFrozenDB("/tmp/example.fdb", frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
    // ... rest of code uses only public API
}
```

---

## Implementation Files (Internal Reference)

### pkg/frozendb/ (Minimal Shim Layer - 4 files)

**File**: `pkg/frozendb/frozendb.go`
- Re-exports: FrozenDB, NewFrozenDB, MODE_READ, MODE_WRITE, Close

**File**: `pkg/frozendb/transaction.go`
- Re-exports: Transaction (methods: Begin, AddRow, Commit, Rollback, IsTombstoned, Validate)
- Excludes: GetEmptyRow, GetRows

**File**: `pkg/frozendb/errors.go`
- Re-exports: All error types and constructors

**File**: `pkg/frozendb/finder.go`
- Re-exports: FinderStrategy, constants (Simple, InMemory, BinarySearch)
- Finder methods accessible via FrozenDB instance

### internal/frozendb/ (Full Implementation - 50+ files)

All implementation files including create.go, header.go, row types, etc. moved from `/frozendb/` to `/internal/frozendb/` with import path updates only.

---

## Breaking Changes from Previous API

### Removed from Public API

1. **Database Creation Functions** ❌
   - `CreateFrozenDB()`, `CreateConfig`, `NewCreateConfig()`, `SudoContext`
   - **Reason**: CLI will handle creation in future
   - **Workaround**: Use internal package temporarily, or wait for CLI support

2. **Transaction.GetEmptyRow()** ❌
   - **Reason**: Exposes internal NullRow type
   - **Migration**: Not needed - method had no documented external use

3. **Transaction.GetRows()** ❌
   - **Reason**: Exposes internal DataRow slice
   - **Migration**: Not needed - method had no documented external use

4. **Header Type** ❌
   - **Reason**: File format implementation detail
   - **Migration**: Not needed for external users

5. **Internal Row Types** ❌
   - `NullRow`, `DataRow`, `PartialDataRow`
   - **Reason**: File format implementation details
   - **Migration**: Not needed for external users

### Import Path Changes

**Old** (deprecated):
```go
import "github.com/susu-dot-dev/frozenDB/frozendb"
```

**New** (for public API):
```go
import "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
```

**New** (for internal - temporary for examples):
```go
import internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
```

---

## Design Rationale

### Why Minimal Public API?

1. **Future Flexibility**: Smaller API surface means fewer breaking changes
2. **Clear Boundaries**: Users know exactly what's stable vs. internal
3. **CLI-First for Creation**: Database creation is better suited to CLI tools
4. **Reduced Complexity**: Users only see operations relevant to their needs
5. **Better Evolution**: Can refactor internals without breaking external code

### Why Exclude Database Creation?

1. **CLI is Better Interface**: Database creation naturally fits command-line tools
2. **Reduces Dependencies**: Library users typically work with existing databases
3. **Simplifies Public API**: Creation involves sudo context, file permissions, etc.
4. **Future-Proof**: Can enhance creation features without API breaks

---

## Performance Characteristics

**Memory Usage**: Fixed, independent of database size (with FinderStrategySimple)

**Build Time**: <10% increase from current baseline (target: <5% increase)

**Runtime Performance**: Zero overhead from re-export pattern (type aliases compile to zero cost)

**Thread Safety**: Unchanged from current implementation

---

## Summary

The minimal public API exposes only the core operations needed for working with existing frozenDB databases: opening, transactions, querying, and closing. Database creation and all file format details remain internal-only, with creation to be handled by CLI commands in future releases. This design provides maximum flexibility for internal evolution while maintaining a stable, focused public contract.
