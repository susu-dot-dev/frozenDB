# Data Model: Row Finder Interface and Implementation

## New Interface Definition

### Finder Interface

**New interface defining row location and transaction boundary detection:**

```go
type Finder interface {
    GetIndex(key uuid.UUID) (int64, error)
    GetTransactionEnd(index int64) (int64, error)
    GetTransactionStart(index int64) (int64, error)
    OnRowAdded(index int64, row *RowUnion) error
}
```

**Behavioral Characteristics:**
- Thread-safe for concurrent read operations
- Handles row addition notifications sequentially
- Maintains consistency between read operations and state updates

## New Implementation Types

### SimpleFinder Struct

**New reference implementation using direct disk scanning:**

```go
type SimpleFinder struct {
    dbFile  frozendb.DBFile
    rowSize int32
    size    int64  // atomic
}
```

**State Management:**
- `size` is atomic int64 for confirmed file size
- No mutex needed - OnRowAdded() sequential, Get* methods read-only
- Size only increases via OnRowAdded() validation

## New Error Types

### KeyNotFoundError

**Used when GetIndex() cannot find specified UUID key:**
```go
type KeyNotFoundError struct {
    FrozenDBError
    key uuid.UUID
}

func NewKeyNotFoundError(key uuid.UUID) *KeyNotFoundError
```

**Thrown by:**
- `GetIndex()` when UUID not found in database



### TransactionActiveError

**Used when transaction is still open (no ending row found):**
```go
type TransactionActiveError struct {
    FrozenDBError
    index int64
}

func NewTransactionActiveError(index int64) *TransactionActiveError
```

**Thrown by:**
- `GetTransactionEnd()` when no transaction end found in forward scan



### ReadError

**Used when disk read operations fail:**
```go
type ReadError struct {
    FrozenDBError
}

func NewReadError(message string, err error) *ReadError
```

**Thrown by:**
- `GetIndex()` when file I/O errors occur during row scanning
- `GetTransactionEnd()` when file I/O errors occur during forward scanning
- `GetTransactionStart()` when file I/O errors occur during backward scanning

## Error Mapping by Method

### GetIndex(key uuid.UUID) (int64, error)

**Error Types Thrown:**
- `NewInvalidInputError()` - UUID is nil, uuid.Nil, or malformed
- `NewKeyNotFoundError(key)` - No DataRow contains the specified UUID
- `NewCorruptDatabaseError()` - Database corruption prevents row parsing
- `NewReadError()` - File I/O errors during disk read operations
- `NewInvalidInputError()` - File I/O errors wrapped with finder context

### GetTransactionEnd(index int64) (int64, error)

**Error Types Thrown:**
- `NewInvalidInputError()` - Index negative, out of bounds, or points to checksum row
- `NewCorruptDatabaseError()` - Row parsing fails or control bytes invalid
- `NewTransactionActiveError(index)` - No transaction end found (still active)
- `NewCorruptDatabaseError()` - Invalid transaction sequence
- `NewReadError()` - File I/O errors during forward scanning

### GetTransactionStart(index int64) (int64, error)

**Error Types Thrown:**
- `NewInvalidInputError()` - Index negative, out of bounds, or points to checksum row
- `NewCorruptDatabaseError()` - Row parsing fails or control bytes invalid
- `NewCorruptDatabaseError()` - No transaction start found
- `NewReadError()` - File I/O errors during backward scanning

### OnRowAdded(index int64, row *RowUnion) error

**Error Types Thrown:**
- `NewInvalidInputError()` - Index does not match expected next position
- `NewInvalidInputError()` - Index skips positions (gaps in sequential ordering)
- `NewCorruptDatabaseError()` - Row data cannot be parsed or is invalid

## Constructor Functions

### SimpleFinder Constructor

**Factory function to create SimpleFinder instances:**
```go
func NewSimpleFinder(dbFile frozendb.DBFile) (*SimpleFinder, error)
```

**Initialization:**
- Reads current database header to get row_size
- Initializes atomic size to current DBFile.Size()
- Validates file format and accessibility
