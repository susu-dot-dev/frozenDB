# frozenDB Finder Interface API Specification

## Overview

This document defines the API contracts for the frozenDB Finder interface, which provides row location and transaction boundary detection capabilities.

## Interface Definition

```go
// Finder defines methods for locating rows and transaction boundaries in frozenDB files
type Finder interface {
    // GetIndex returns the index of the first row containing the specified UUID key
    // Returns error if key is not found or input is invalid
    GetIndex(key uuid.UUID) (int64, error)
    
    // GetTransactionEnd returns the index of the last row in the transaction containing the specified index
    // Returns error if index is invalid, points to checksum row, or transaction boundaries cannot be determined
    GetTransactionEnd(index int64) (int64, error)
    
    // GetTransactionStart returns the index of the first row in the transaction containing the specified index  
    // Returns error if index is invalid, points to checksum row, or transaction boundaries cannot be determined
    GetTransactionStart(index int64) (int64, error)
    
    // OnRowAdded is called when a new row is successfully written to the database
    // Updates finder internal state to include the new row for subsequent operations
    OnRowAdded(index int64, row *RowUnion) error
}
```

## Method Contracts

### GetIndex(key uuid.UUID) (int64, error)

**Purpose**: Locate the index of the first row containing a specific UUID key.

**Parameters**:
- `key` (uuid.UUID): The UUIDv7 key to search for. Must not be uuid.Nil.

**Return Values**:
- `index` (int64): Zero-based index of the matching DataRow
- `error`: Error if key not found, invalid UUID, or data corruption

**Preconditions**:
- Finder is properly initialized with valid database file
- Database file is accessible and not corrupted
- Input UUID is valid (not nil, not uuid.Nil)

**Postconditions**:
- Returns correct index of first matching DataRow if found
- All row types except complete DataRows are skipped during search
- Returned index is within confirmed file size bounds

**Error Conditions**:
- `InvalidInputError`: UUID is nil, uuid.Nil, or malformed
- `KeyNotFoundError`: No DataRow contains the specified UUID key
- `CorruptDatabaseError`: Database corruption prevents row parsing
- `ReadError`: Disk read operations fail

**Concurrency**: Thread-safe, may be called concurrently with other Get* methods

### GetTransactionEnd(index int64) (int64, error)

**Purpose**: Find the index of the last row in the transaction containing the specified index.

**Parameters**:
- `index` (int64): Index of a row within the transaction

**Return Values**:
- `endIndex` (int64): Index of the last row in the same transaction
- `error`: Error if index invalid, transaction boundaries indeterminable

**Preconditions**:
- Input index is valid (non-negative, within file bounds)
- Input index does not point to a checksum row
- Database file contains valid transaction structure

**Postconditions**:
- Returns index of row with transaction-ending end_control
- If input row itself ends transaction, returns same index
- Returned index is within same transaction as input index

**Error Conditions**:
- `InvalidInputError`: Index is negative, out of bounds, or points to checksum row
- `CorruptDatabaseError`: Row parsing fails or control bytes are invalid
- `TransactionActiveError`: Transaction has no ending row (still active)
- `CorruptDatabaseError`: No transaction start found in backward scan
- `ReadError`: Disk read operations fail during forward scanning

**Concurrency**: Thread-safe, may be called concurrently with other Get* methods

### GetTransactionStart(index int64) (int64, error)

**Purpose**: Find the index of the first row in the transaction containing the specified index.

**Parameters**:
- `index` (int64): Index of a row within the transaction

**Return Values**:
- `startIndex` (int64): Index of the first row in the same transaction
- `error`: Error if index invalid, transaction boundaries indeterminable

**Preconditions**:
- Input index is valid (non-negative, within file bounds)
- Input index does not point to a checksum row
- Database file contains valid transaction structure

**Postconditions**:
- Returns index of row with start_control = 'T' in transaction chain
- If input row itself starts transaction, returns same index
- Returned index is within same transaction as input index

**Error Conditions**:
- `InvalidInputError`: Index is negative, out of bounds, or points to checksum row
- `CorruptDatabaseError`: Row parsing fails or control bytes are invalid
- `CorruptDatabaseError`: No transaction start found in backward scan
- `ReadError`: Disk read operations fail during backward scanning

**Concurrency**: Thread-safe, may be called concurrently with other Get* methods

### OnRowAdded(index int64, row *RowUnion) error

**Purpose**: Update finder state when a new row is successfully written to the database.

**Parameters**:
- `index` (int64): Index of the newly added row
- `row` (RowUnion): Complete row data of the newly added row

**Return Values**:
- `error`: Error if index validation fails or state update encounters issues

**Preconditions**:
- Called within transaction write lock context
- Row data is successfully written and persistent on disk
- Index follows zero-based scheme and sequential ordering

**Postconditions**:
- Finder internal state includes the new row for subsequent operations
- GetIndex() can locate the new row by its UUID key
- Transaction boundary methods handle the new index correctly
- Confirmed file size updated to include new row

**Error Conditions**:
- `InvalidInputError`: Index does not match expected next position
- `InvalidInputError`: Index skips positions (gaps in sequential ordering)
- `CorruptDatabaseError`: Row data cannot be parsed or is invalid

**Concurrency**: Called sequentially (no self-racing), blocks until completion

## Index Scheme

**Formula**: `row_offset = HEADER_SIZE + index * row_size`

**Index Examples**:
- Index 0: First checksum row (bytes [64 .. 64 + row_size - 1])
- Index 1: First data/null row (bytes [64 + row_size .. 64 + 2 * row_size - 1])
- Index N: Nth row after header (bytes [64 + N * row_size .. 64 + (N + 1) * row_size - 1])

## Transaction Control Recognition

**Start Controls**:
- `'T'`: Transaction begin
- `'R'`: Transaction continuation
- `'C'`: Checksum row (not in transactions)

**End Controls**:
- Transaction ending: `TC`, `RE`, `SC`, `SE`, `R0-R9`, `S0-S9`, `NR`
- Non-transaction ending: Other values

## Performance Characteristics

**GetIndex()**:
- Time: O(n) linear scan where n = number of rows
- Space: O(1) constant memory
- Disk I/O: One read per row examined

**GetTransactionStart()/GetTransactionEnd()**:
- Time: O(k) where k = distance to boundary (max ~101)
- Space: O(1) constant memory  
- Disk I/O: Up to 101 reads in worst case

**OnRowAdded()**:
- Time: O(1) constant time
- Space: O(1) memory update
- Disk I/O: None (state update only)
