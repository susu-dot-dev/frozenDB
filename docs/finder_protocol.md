# frozenDB Finder Protocol Specification

## 1. Introduction

This document defines the Finder protocol for frozenDB databases. The Finder provides mechanisms to locate rows by UUID key and determine transaction boundaries within frozenDB files. This protocol serves as the foundation for higher-level query and retrieval operations.

### 1.1. Design Philosophy

The Finder protocol operates on the fundamental principle that frozenDB files are append-only with fixed-width rows, enabling predictable indexing and deterministic transaction boundary detection.

**Key Design Principles:**
- **Index-Based Navigation**: All operations use zero-based indices relative to the first row after the header
- **Transaction Awareness**: All finder methods understand transaction semantics for proper boundary detection
- **State Consistency**: Finders maintain internal consistency between read operations and row addition notifications

### 1.2. Conformance and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

## 2. Index Definition and Structure

### 2.1. Row Indexing

A row index is a zero-based integer that identifies the position of a row within a frozenDB file. Indexing begins immediately after the 64-byte header, with index 0 being the first checksum row.

**Index Mapping Formula:**
```
row_offset = HEADER_SIZE + index * row_size
```

Where:
- `HEADER_SIZE` = 64 bytes (constant) - only the first 64 bytes of the header are excluded
- `index` = 0-based row index
- `row_size` = value from database header

**Byte Range Notation:**
For a given index, the complete row occupies bytes:
- **Start byte**: `64 + index * row_size`
- **End byte**: `64 + index * row_size + row_size - 1`
- **Range notation**: `[64 + index * row_size .. 64 + index * row_size + row_size - 1]`

**Index Examples:**
- **Index 0**: First checksum row (bytes [64 .. 64 + row_size - 1])
- **Index 1**: First data row or null row (bytes [64 + row_size .. 64 + 2 * row_size - 1])
- **Index 2**: Second data/null row (bytes [64 + 2 * row_size .. 64 + 3 * row_size - 1])
- **Index N**: Nth row after header (bytes [64 + N * row_size .. 64 + (N + 1) * row_size - 1])

### 2.2. Indexable Row Types

All complete rows in a frozenDB file have an index, including:

- **Checksum Rows**: Every 10,000th complete row (by definition)
- **Data Rows**: User data rows with UUID keys and JSON values
- **Null Rows**: Single-row transactions with uuid.Nil as key
- **Partial Data Rows**: The incomplete row at file end (if present)

**Index Properties:**
- Indices are contiguous - there are no gaps in the numbering
- Each index corresponds to exactly one row of the appropriate type
- PartialDataRows have an index but are excluded from GetIndex() searches

## 3. Row Location by UUID Key

### 3.1. GetIndex() Semantics

The `GetIndex(key UUID)` method finds the index of the first row containing the specified UUID key.

**Key Uniqueness Assumption:**
GetIndex() MAY assume that UUID keys are unique within the database. Since frozenDB enforces UUIDv7 uniqueness during insertion, there will be at most one row matching any given valid UUID key.

**Searchable Row Types:**
- **Data Rows**: Any complete data row with a matching UUIDv7 key

**Non-Searchable Row Types:**
- **Checksum Rows**: Do not contain UUID keys
- **Null Rows**: Use uuid.Nil as key, but GetIndex() must not find them (searching for UUID 0x0 is invalid)
- **Partial Data Rows**: Not considered for search (incomplete state)

### 3.2. Search Behavior

**Key Validation:**
- Valid UUIDv7 keys MAY be searched for and will match DataRows
- uuid.Nil (all zeros) MUST result in error (searching for UUID 0x0 is invalid)
- Empty or invalid UUIDs MUST result in error

**Search Algorithm:**
1. Skip checksum rows (no UUID key)
2. Skip NullRows (use uuid.Nil as key, but are not searchable)
3. Skip PartialDataRows (incomplete state)
4. Compare UUID keys for DataRows only
5. Return the matching index, or error if not found

**Concurrency Considerations:**
GetIndex() MAY be called within the context of Transaction.AddRow implementations. The method MUST NOT attempt to acquire transaction read or write locks, as this could cause deadlocks when called from within transaction operations. Implementations should use read-only locks appropriate for the finder's internal state management.

**Return Values:**
- **Success**: Index of matching row (int64)
- **Not Found**: Error indicating key not present in database
- **Invalid Input**: Error for invalid UUID or corrupted data

### 3.3. Global Key Uniqueness

GetIndex() MUST locate rows regardless of their transaction state:
- Committed transaction rows are searchable
- Rolled back transaction rows are searchable  
- Savepoint-containing transaction rows are searchable
- The method does not validate transaction validity for the returned index

**Global Uniqueness Guarantee:**
Even if a transaction is rolled back, the UUID key cannot be re-inserted into the database. This ensures that each valid UUID key appears at most once in the entire database history, regardless of transaction outcome.

## 4. Transaction Boundary Detection

### 4.1. Transaction Boundary Fundamentals

Transaction boundaries are determined by analyzing the `start_control` and `end_control` bytes of rows as defined in [Section 9.2 of v1_file_format.md](v1_file_format.md#92-transaction-structure) and [Section 8.3 of v1_file_format.md](v1_file_format.md#83-end-control-rules).

For complete transaction boundary definitions, refer to:
- [Section 9.3 of v1_file_format.md](v1_file_format.md#93-transaction-validity-rules) for transaction validity rules
- [Section 9.4 of v1_file_format.md](v1_file_format.md#94-savepoint-tracking) for savepoint handling
- [Section 9.6 of v1_file_format.md](v1_file_format.md#96-invalid-sequences) for invalid transaction sequences

### 4.2. GetTransactionEnd() Mechanism

Given a row index, `GetTransactionEnd(index)` finds the index of the last row in the same transaction.

**Algorithm:**
1. **Input Validation**: Verify index is valid and points to a complete Data Row
2. **Current Row Analysis**: Examine the row at the given index, to see if the transaction ends with the current row
3. **Forward Scan**: Iterate forward through subsequent rows
4. **Termination Detection**: Find the first row with a transaction-ending end_control
5. **Boundary Return**: Return the index of the terminating row

**Failure Conditions:**
- **Invalid Index**: Negative index or index beyond file bounds
- **Checksum Row**: Index points to a checksum row, null row, or partial data row
- **Corrupted Data**: Row at index cannot be parsed or contains invalid control bytes
- **TransactionActiveError**: Transaction is still open (last transaction in file has no ending row)

**Boundary Handling:**
- **End Row**: If index points to a row that ends a transaction, return the same index (input and output are identical)

### 4.3. GetTransactionStart() Mechanism

Given a row index, `GetTransactionStart(index)` finds the index of the first row in the same transaction.

**Algorithm:**
1. **Input Validation**: Verify index is valid and does not point to a checksum row
2. **Current Row Analysis**: Examine the row at the given index
3. **Backward Scan**: Iterate backward through preceding rows
4. **Transaction Start Detection**: Find the first row with `start_control = 'T'` in the transaction chain
5. **Boundary Return**: Return the index of the transaction start row

**Failure Conditions:**
- **Invalid Index**: Negative index or index beyond file bounds
- **Checksum Row**: Index points to a checksum row
- **Corrupted Data**: Row at index cannot be parsed or contains invalid control bytes
- **Malformed Transaction**: No transaction start found (file corruption)

**Boundary Handling:**
- **Start Row**: If index points to a row that starts a transaction, return the same index (input and output are identical)

### 4.4. Special Row Type Handling

**NullRow Transactions:**
For NullRow transaction handling, see [Section 8.7 of v1_file_format.md](v1_file_format.md#87-null-row-t-nr)

**Checksum Row Exclusion:**
For checksum row structure and exclusion rules, see [Section 6 of v1_file_format.md](v1_file_format.md#6-checksum-row-ccs)

**PartialDataRow Considerations:**
For PartialDataRow handling and incomplete transaction scenarios, see [Section 8.6 of v1_file_format.md](v1_file_format.md#86-partial-data-row)

## 5. Row Addition Notifications

### 5.1. OnRowAdded() Timing and Guarantees

The `OnRowAdded(index int64, row *RowUnion)` method is called when new rows are successfully written to the database.

**Timing Guarantees:**
- **Write Completion**: Called only after the row write has succeeded
- **Disk Persistence**: The row is guaranteed to be found by subsequent disk reads (may not be flushed to physical disk)
- **Transaction Context**: Called within Transaction method boundaries, holding the transaction write lock
- **Sequential Ordering**: Calls are guaranteed to arrive in-order with no self-racing

**Concurrency Properties:**
- **No Self-Racing**: OnRowAdded() calls never race with each other
- **Blocking**: Callers block until the Finder finishes processing the callback
- **Exclusive Access**: Transaction write lock prevents concurrent modifications during notification

### 5.2. Callback Semantics

**Row Data:**
- `index`: The index of the newly added row (following the indexing scheme in Section 2.1)
- `row`: RowUnion containing the complete row data of the newly added row

**Finder Responsibilities:**
- Update internal state to include the new row for subsequent GetIndex() operations
- Update any cached indices or boundary information
- Maintain consistency between Get* methods and the added row data

**State Consistency:**
After OnRowAdded() returns, the Finder's internal state MUST reflect that:
- GetIndex() can locate the newly added row by its UUID key
- GetTransactionStart() and GetTransactionEnd() can handle the new index correctly
- The finder maintains correct transaction boundary information

## 6. Concurrency and State Management

### 6.1. Finder Concurrency Responsibility

Each Finder implementation is responsible for maintaining internal consistency between read operations (Get* methods) and write notifications (OnRowAdded).

**Required Synchronization:**
- **Read-Write Safety**: Finder must ensure Get* methods are safe during OnRowAdded() calls
- **Internal State**: Any internal caching or indexing must be protected against concurrent access
- **Method Isolation**: Each method must produce consistent results regardless of concurrent operations

**Permitted Synchronization Mechanisms:**
- Mutexes for protecting internal state
- Atomic operations for counters and indices
- Lock-free data structures where appropriate
- Any other concurrency control that ensures correctness

### 6.2. Timing Invariants

Finders must maintain these timing invariants:

1. **Initial State**: When a Finder is opened, it should track the fileSize of the database and all existing rows are initially in scope to be read. Finder methods MUST NOT include any extra data that may exist on disk but has not been confirmed via OnRowAdded() callback.

2. **Before OnRowAdded()**: Get* methods must not return information about rows written since the Finder was opened but not yet confirmed via OnRowAdded()

3. **During OnRowAdded()**: Get* methods may or may not see the newly added row until the callback completes

4. **After OnRowAdded()**: Finder MUST update its internal size pointer to read only as far as the last confirmed row. Get* methods must consistently return correct information about all confirmed rows.

**Alternative Size Access**: Callers can directly query the underlying DBFile for the latest file size when needed, since that implementation uses atomics and not locks.

**Thread Safety Requirements:**
- Multiple goroutines may call Get* methods concurrently
- One goroutine calls OnRowAdded() at a time (due to transaction write lock)
- Implementations must handle the concurrency between readers and the single writer

## 7. Error Handling

### 7.1. Error Types and Conditions

Finder methods must return appropriate errors for these conditions:

**Index Validation Errors:**
- Negative indices
- Indices beyond file bounds
- Indices pointing to checksum rows (for transaction boundary methods)

**Data Corruption Errors:**
- Rows that cannot be parsed
- Invalid control byte combinations
- Malformed transaction sequences

**Search Result Errors:**
- UUID key not found in database
- Empty or invalid UUID parameters
- TransactionActiveError: Transaction is still open when attempting to determine transaction boundaries

### 7.2. Error Propagation

Finders must propagate underlying database errors while maintaining finder-specific error context:
- Wrap file I/O errors with finder context
- Preserve original error information for debugging
- Provide clear error messages for invalid usage scenarios

## 8. Implementation Considerations

## 8. Implementation Considerations

### 8.1. Finder Implementation Strategies

Finder implementations may use various strategies while maintaining protocol compliance:

**Implementation Approaches:**
- Direct linear scanning of the file system
- Indexing structures for faster UUID lookup
- Cached transaction boundary information
- Memory-mapped file access patterns
- Concurrent scanning techniques

**Protocol Compliance Requirements:**
- Must maintain identical functional behavior across all implementations
- Must honor all timing and concurrency requirements
- Must return the same results for all valid inputs
- Must handle all specified error conditions correctly
