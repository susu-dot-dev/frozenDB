# API Contract: Transaction Checksum Row Insertion

## Overview

This contract defines the API for automatic checksum row insertion in frozenDB
transactions. Checksum rows are inserted transparently at 10,000-row intervals
without requiring explicit user code.

## New Public APIs

### DBFile Interface

**File**: `frozendb/file_manager.go`

```go
// DBFile defines interface for file operations, enabling mock implementations
type DBFile interface {
    Read(start int64, size int32) ([]byte, error)
    Size() int64
    Close() error
    SetWriter(dataChan <-chan Data) error
}
```

**Description**: Interface for file operations used by Transaction to read rows
and calculate checksums.

**Methods**:

- `Read(start int64, size int32) ([]byte, error)`: Reads `size` bytes from file
  at offset `start`
- `Size() int64`: Returns current file size in bytes
- `Close() error`: Closes the file
- `SetWriter(dataChan <-chan Data) error`: Sets write channel for appending data

**Errors**:

- `PathError`: File access issues
- `TombstonedError`: File manager is closed
- `InvalidInputError`: Invalid offset or size parameters

---

### NewTransaction Function

**File**: `frozendb/transaction.go`

```go
// NewTransaction creates a new transaction with automatic checksum row insertion
func NewTransaction(db DBFile, header *Header, writeChan chan<- Data) (*Transaction, error)
```

**Parameters**:

- `db DBFile`: File manager interface for row reading and checksum calculation
- `header *Header`: Validated header reference containing row_size and
  configuration
- `writeChan chan<- Data`: Write channel for sending Data structs to FileManager

**Returns**:

- `*Transaction`: New transaction instance
- `error`: Error if setup fails

**Behavior**:

1. Calls `db.SetWriter(writeChan)` to configure write channel
2. Stores both `db` (interface) and `writeChan` in transaction object

**Errors**:

- `InvalidActionError`: Writer already active on FileManager
- `InvalidInputError`: Invalid parameters (nil header, nil writeChan)

---

## Modified Public APIs

### Transaction.AddRow()

**File**: `frozendb/transaction.go`

```go
// AddRow adds a new key-value pair to the transaction
func (tx *Transaction) AddRow(key uuid.UUID, value json.RawMessage) error
```

**New Behavior**:

- After successful DataRow write:
  1. Calculates row count from file size:
     `(fileSize - HEADER_SIZE - checksumRows * rowSize) / rowSize`
  2. Checks if checksum row needed: `rowCount > 0 && rowCount % 10000 == 0`
  3. If checksum needed: a. Reads 10,000 rows since last checksum via
     `DBFile.Read()` b. Validates parity of all rows c. Calculates CRC32
     checksum d. Creates ChecksumRow via `NewChecksumRow()` e. Writes
     ChecksumRow to file

**No changes to existing behavior or errors.**

---

### Transaction.Commit()

**File**: `frozendb/transaction.go`

```go
// Commit finalizes the transaction
func (tx *Transaction) Commit() error
```

**New Behavior**:

- After successful commit of DataRow or NullRow:
  1. Calculates row count from file size:
     `(fileSize - HEADER_SIZE - checksumRows * rowSize) / rowSize`
  2. Checks if checksum row needed: `rowCount > 0 && rowCount % 10000 == 0`
  3. If checksum needed: a. Reads 10,000 rows since last checksum via
     `DBFile.Read()` b. Validates parity of all rows c. Calculates CRC32
     checksum d. Creates ChecksumRow via `NewChecksumRow()` e. Writes
     ChecksumRow to file

**No changes to existing behavior or errors.**

---

### Transaction.Rollback()

**File**: `frozendb/transaction.go`

```go
// Rollback rolls back the transaction to a specified savepoint or fully closes it
func (tx *Transaction) Rollback(savepointId int) error
```

**New Behavior**:

- After successful rollback DataRow or NullRow write:
  1. Calculates row count from file size:
     `(fileSize - HEADER_SIZE - checksumRows * rowSize) / rowSize`
  2. Checks if checksum row needed: `rowCount > 0 && rowCount % 10000 == 0`
  3. If checksum needed: a. Reads 10,000 rows since last checksum via
     `DBFile.Read()` b. Validates parity of all rows c. Calculates CRC32
     checksum d. Creates ChecksumRow via `NewChecksumRow()` e. Writes
     ChecksumRow to file

**No changes to existing behavior or errors.**

---

## Transaction.Begin()

**File**: `frozendb/transaction.go`

```go
// Begin initializes an empty transaction
func (tx *Transaction) Begin() error
```

**No changes to existing behavior or errors.**

---

## Internal APIs (Not Public)

### RowUnion (New Type)

```go
// RowUnion holds pointers to all possible row types
// Exactly one pointer will be non-nil after unmarshaling
type RowUnion struct {
    DataRow     *DataRow
    NullRow     *NullRow
    ChecksumRow *ChecksumRow
}

// UnmarshalText unmarshals a row by examining control bytes first
// Reads start_control at position [1] and end_control at [rowSize-5:rowSize-4]
// Unmarshals directly into the correct type - no trial and error
func (ru *RowUnion) UnmarshalText(rowBytes []byte) error
```

**Behavior:**
- Reads start_control byte at position [1]
- Reads end_control bytes at positions [rowSize-5:rowSize-4]
- Determines row type from control bytes:
  - `start_control='C'` and `end_control='CS'` → ChecksumRow
  - `start_control='T'` and `end_control='NR'` → NullRow
  - `start_control='T'` or `'R'` → DataRow
- Unmarshals directly into the correct row type
- Automatically validates parity via each row type's `UnmarshalText()`
- Returns RowUnion with exactly one non-nil pointer

**Errors:**
- `CorruptDatabaseError`: Parity mismatch or invalid row structure

### insertChecksumRow

```go
// insertChecksumRow writes a checksum row after 10,000 complete rows
func (tx *Transaction) insertChecksumRow() error
```

---

---

## Data Types

### ChecksumRow (Existing)

**File**: `frozendb/checksum.go`

```go
// ChecksumRow represents a checksum integrity row
type ChecksumRow struct {
    baseRow[*Checksum]
}

// NewChecksumRow creates checksum row from header and data bytes
func NewChecksumRow(header *Header, dataBytes []byte) (*ChecksumRow, error)
```

**Properties**:

- `start_control`: 'C'
- `end_control`: 'CS'
- Contains CRC32 checksum of data bytes
- Fixed width: rowSize bytes

**Errors**:

- `InvalidInputError`: Invalid header or empty dataBytes

---

## Error Handling

All errors follow frozenDB's structured error pattern from `errors.go`:

### NewError Scenarios

1. **Parity Validation Failure**
   - Type: `CorruptDatabaseError`
   - Code: `corrupt_database`
   - Trigger: Row parity mismatch during checksum calculation
   - Message: "parity mismatch: expected XX, got YY"

2. **Checksum Write Failure**
   - Type: `WriteError`
   - Code: `write_error`
   - Trigger: Failed to write checksum row to file
   - Message: "failed to write checksum row"

3. **Invalid File Manager State**
   - Type: `TombstonedError`
   - Code: `tombstoned`
   - Trigger: Operation on closed FileManager
   - Message: "file manager is closed"

---
