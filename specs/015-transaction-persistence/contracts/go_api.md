# API Contract: Transaction File Persistence

**Feature**: 015-transaction-persistence
**API Type**: Go Library
**Date**: 2026-01-21

## Transaction API

**Description**: Represents a database transaction with file persistence capability. All transaction operations (Begin, AddRow, Commit) write rows to disk via write channel.

For complete data structure details, see `data-model.md`.

**Thread-Safety (FR-010)**: All Transaction methods (Begin, AddRow, Commit, Rollback, Savepoint) are thread-safe. Multiple goroutines can call these methods concurrently on the same Transaction instance without data races or corruption.

**Tombstoning (FR-006)**: If any write operation fails, the Transaction is tombstoned. Once tombstoned, all subsequent public API calls on the Transaction MUST return TombstonedError. Use IsTombstoned() to check if a transaction has been tombstoned.

### Transaction Constructor Pattern

```go
// Caller creates write channel and sets up FileManager
dataChan := make(chan Data, 10)
err := fileManager.SetWriter(dataChan)

// Transaction is created with write channel reference
tx := &Transaction{
    Header:    header,
    writeChan: dataChan,  // Channel for write requests
    // other fields initialized
}
```

### When Begin() is Called

**Writes that happen**:

1. A PartialDataRow is created with state=PartialDataRowWithStartControl
2. The PartialDataRow is marshaled to bytes (2 bytes: ROW_START + 'T')
3. These bytes are sent to the write channel
4. The method waits synchronously for write completion before returning

**State when writes fail**:

- Transaction is tombstoned if write fails
- Transaction remains in inactive state (no partial data persisted)
- FR-006 is satisfied: transaction is tombstoned and subsequent calls return TombstonedError

---

### When AddRow() is Called

**Writes that happen**:

**First AddRow after Begin()**:

1. The PartialDataRow advances to PartialDataRowWithPayload state
2. The PartialDataRow is marshaled to bytes (rowSize-5 bytes)
3. Only incremental bytes (positions 2 to rowSize-6) are written to channel (first 2 bytes already written by Begin())
4. The method waits synchronously for write completion before returning

**Subsequent AddRow() calls**:

1. The previous PartialDataRow is finalized with RE end_control (5 bytes: RE + parity + ROW_END)
2. These 5 bytes are sent to the write channel
3. A new PartialDataRow is created with ROW_CONTINUE
4. The new PartialDataRow advances to PartialDataRowWithPayload state
5. The new PartialDataRow is marshaled to bytes (rowSize-5 bytes)
6. All rowSize-5 bytes are sent to the write channel (this is a new row, all bytes written)
7. The method waits synchronously for all writes to complete before returning

**State when writes fail**:

- Transaction is tombstoned if any write fails
- Transaction state remains exactly as before AddRow() call (no partial data persisted)
- FR-006 is satisfied: transaction is tombstoned and subsequent calls return TombstonedError

---

### When Commit() is Called

**Writes that happen**:

**Empty transaction** (no AddRow calls):

1. A NullRow is created with StartControl='T', EndControl='NR', Key with timestamp equal to max_timestamp, other fields zero
2. The NullRow is marshaled to bytes (rowSize bytes)
3. Only incremental bytes (positions 2 to rowSize-1) are written to channel (first 2 bytes already written by Begin())

**Data transaction** (one or more AddRow calls):

1. The final PartialDataRow is finalized with TC or SC end_control
2. The finalized DataRow is marshaled to bytes (rowSize bytes)
3. Only incremental bytes (positions rowBytesWritten to rowSize-1) are written to channel

4. The method waits synchronously for write completion before returning

**State when writes fail**:

- Transaction is tombstoned if write fails
- Transaction remains in active state (no partial data persisted)
- FR-006 is satisfied: transaction is tombstoned and subsequent calls return TombstonedError

---

## Write Channel Data Flow

### Data Struct

```go
type Data struct {
    Bytes    []byte        // Row bytes from MarshalText()
    Response chan<- error   // Channel for write completion/error
}
```

### Write Pattern

The Transaction uses an incremental write pattern for PartialDataRow to preserve append-only semantics:

1. Serialize row to bytes using MarshalText() (returns complete row bytes)
2. For PartialDataRow writes: Use internal tracking to slice off already-written bytes, then write only new incremental bytes
3. For complete rows (DataRow, NullRow): Write complete row bytes
4. Create response channel (buffered for synchronous wait)
5. Send Data struct to write channel
6. Wait for completion (synchronous)

Note: The internal tracking of already-written bytes is an implementation detail. The key behavior is that only new bytes are appended to maintain append-only semantics.

---

## Error Handling Guarantees

### FR-006: Write Failure Semantics

- All write operations are synchronous (wait for response before returning)
- Write failure tombstones the transaction and returns the write error to caller
- Once tombstoned, all subsequent public API calls on the transaction MUST return TombstonedError
- No partial data is persisted on error (atomic operations)
- Write errors are wrapped and returned as-is from FileManager

### FR-010: Concurrent Access Thread-Safety

- All Transaction methods (Begin, AddRow, Commit, Rollback, Savepoint) are thread-safe
- Multiple goroutines can call methods concurrently on same Transaction instance
- Internal state modifications are protected by synchronization primitives
- No data races or corruption when called concurrently

---

## Backward Compatibility

### API Changes

- **Breaking Change**: Transaction struct adds `writeChan chan<- Data` field
- Caller must provide write channel before calling Begin(), AddRow(), Commit()
- Existing Transaction methods unchanged in signature (same return types)
- New behavior: All operations now write to disk via write channel

### Migration Path

1. Caller creates write channel and sets up FileManager
2. Caller initializes Transaction with write channel field set
3. Existing Begin(), AddRow(), Commit() calls work as before
4. All operations now persist to disk (previously in-memory only)

---
