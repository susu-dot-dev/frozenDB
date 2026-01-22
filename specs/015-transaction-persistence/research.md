# Research: Transaction File Persistence

**Feature**: 015-transaction-persistence **Date**: 2026-01-21 **Spec**: spec.md

## Research Findings

### Decision 1: Row Serialization Strategy

**Decision**: Use existing MarshalText() methods on DataRow, PartialDataRow, and
NullRow

**Rationale**:

- All three row types (DataRow, PartialDataRow, NullRow) already implement
  `MarshalText() ([]byte, error)`
- These methods generate complete row bytes per v1_file_format.md specification
- Includes ROW_START (0x1F), start_control, payload, end_control, parity bytes,
  ROW_END (0x0A)
- No new serialization code needed - leverage existing implementation

**Alternatives Considered**:

- Custom serialization function: Rejected because existing MarshalText() is
  already well-tested and spec-compliant

### Decision 2: Synchronous Write Pattern

**Decision**: Use response channel pattern for synchronous writes

**Rationale**:

- FileManager Data struct includes `Response chan<- error` field
- Pattern: `responseChan := make(chan error, 1)`,
  `dataChan <- Data{Bytes: bytes, Response: responseChan}`,
  `err := <-responseChan`
- Blocks sender until FileManager processes the write and sends error or nil to
  Response channel
- Guarantees write completes before Begin()/AddRow()/Commit() returns
- Ensures FR-005 (synchronous writes) requirement is met

**Alternatives Considered**:

- Goroutine with sync.WaitGroup: Rejected because Response channel pattern is
  simpler and already established in FileManager

### Decision 3: Transaction Integration with FileManager

**Decision**: Add write channel field to Transaction struct, caller sets up
FileManager

**Rationale**:

- User input explicitly states: "the only extra data type needed for the
  Transaction struct is the Data chan to send write requests to"
- Caller interacts directly with FileManager (easier for testing - can mock
  channel handler)
- Transaction stores `writeChan chan<- Data` field
- Transaction methods create Data structs with row bytes and response channel
- Transaction sends to writeChan and waits for response

**Alternatives Considered**:

- Embed FileManager in Transaction: Rejected because user input specifies
  channel-based interaction only
- Store FileManager reference: Rejected because caller should manage FileManager
  lifecycle

### Decision 4: Error Handling for Write Failures

**Decision**: Tombstone transaction on write failure, return TombstonedError for
subsequent calls

**Rationale**:

- FR-006 requires: "If any write operation fails, the system MUST tombstone the
  transaction and return the write error. All subsequent public API calls on the
  tombstoned transaction MUST return TombstonedError"
- When write returns error, tombstone the transaction and return error to caller
- Write errors from FileManager already use NewWriteError or
  NewCorruptDatabaseError
- Begin()/AddRow()/Commit() tombstone transaction on write failure (FR-006: "no
  partial data is persisted")
- Ensures atomic behavior - either write succeeds or transaction is tombstoned
- Follows FileManager pattern for consistency

**Alternatives Considered**:

- Return error, no state change: Rejected because spec requires tombstoning
- Retry on failure: Rejected because spec says "throw an error upon failure"
- Continue with warning: Rejected because spec requires error and FR-006 says
  "tombstone the transaction"

### Decision 5: PartialDataRow Incremental Write Strategy

**Decision**: Transaction tracks bytesWritten for current PartialDataRow and
slices MarshalText() output to append only new bytes

**Rationale**:

- PartialDataRow state progression is cumulative (bytes accumulate, never
  overwritten):
  - State 1 (PartialDataRowWithStartControl): ROW_START + start_control = 2
    bytes
  - State 2 (PartialDataRowWithPayload): State 1 + UUID + JSON + padding =
    rowSize-5 bytes
  - State 3 (PartialDataRowWithSavepoint): State 2 + 'S' = rowSize-4 bytes
- Calling MarshalText() at each state would rewrite all bytes, violating
  append-only semantics
- Solution: Add `bytesWritten int` field to Transaction to track how many bytes
  of current PartialDataRow have been written
- When writing, call MarshalText() and slice off beginning:
  `newBytes := allBytes[tx.bytesWritten:]`

**Alternatives Considered**:

- PartialDataRow tracks bytesWritten internally: Rejected because would require
  modifying PartialDataRow struct and MarshalText() behavior; Transaction is
  better place for bookkeeping
- Read database to see what exists: Rejected because requires file I/O on every
  write, inefficient and complex
- Stateful MarshalText(): Rejected because would break existing interface
  (MarshalText() should return complete row bytes)

### Decision 6: PartialDataRow Finalization on Subsequent AddRow()

**Decision**: Finalize previous PartialDataRow with RE end_control when adding
new row

**Rationale**:

- Per v1_file_format.md, PartialDataRow must be completed as DataRow before new
  row can begin
- When AddRow() is called and current PartialDataRow has payload:
  - Complete PartialDataRow with RE end_control (row continuation)
  - Write RE end_control + parity bytes + ROW_END (3 bytes for RE + 2 for
    parity + 1 for ROW_END = 6 bytes)
  - This "closes" the previous row without committing
  - New PartialDataRow created with ROW_CONTINUE start_control
- This matches the pattern from AddRow() in existing transaction.go (calls
  tx.last.EndRow())

**Alternatives Considered**:

- Continue writing same PartialDataRow: Rejected because PartialDataRow
  represents a single row, not multiple rows

### Decision 7: Thread-Safety for Concurrent Access

**Decision**: Use existing sync.RWMutex in Transaction struct to protect all
state modifications

**Rationale**:

- FR-010 requires Transaction methods be thread-safe for concurrent goroutine
  access
- Transaction.mu (sync.RWMutex) is already present in the struct
- All state modifications (fields: rows, empty, last, maxTimestamp,
  bytesWritten, writeChan) must be protected by mu
- Use Write lock for modifications (Begin, AddRow, Commit, Rollback, Savepoint)
- Use Read lock for reads (GetRows, GetEmptyRow, GetMaxTimestamp, IsCommitted,
  etc.)
- Write operations to writeChan are already serialized by FileManager's RWMutex,
  but Transaction state must also be protected

**Alternatives Considered**:

- Use sync.Mutex instead of RWMutex: Rejected because existing code uses RWMutex
  and there are read operations that can benefit from concurrent reads
- Remove locking and document single-goroutine requirement: Rejected because Go
  is concurrent and FR-010 requires thread-safety

### Decision 8: Checksum Row Handling

**Decision**: Do not write checksum rows (per spec assumption < 10,000 rows)

**Rationale**:

- FR-009 explicitly states: "Transaction MUST NOT write checksum rows (assumes
  database < 10,000 rows)"
- PartialDataRows are excluded from 10,000-row count per v1_file_format.md
  section 2.3
- Only complete DataRows and NullRows count toward checksum interval
- Transaction should not interfere with existing checksum logic (handled
  elsewhere)

**Alternatives Considered**:

- Write checksums after every row: Rejected because FR-009 explicitly forbids it
- Delegate checksum responsibility: Chosen - Transaction does not write
  checksums, caller/other code handles them

## Implementation Approach

### Transaction Struct Changes

```go
type Transaction struct {
    rows         []DataRow
    empty        *NullRow
    last         *PartialDataRow
    Header       *Header
    maxTimestamp int64
    mu           sync.RWMutex
    writeChan    chan<- Data   // NEW: channel for write requests
    bytesWritten int          // NEW: tracks bytes written for current PartialDataRow
}
```

### Write Helper Function

```go
func (tx *Transaction) writeBytes(bytes []byte) error {
    responseChan := make(chan error, 1)
    data := Data{
        Bytes:    bytes,
        Response: responseChan,
    }
    tx.writeChan <- data
    err := <-responseChan
    return err
}

// writePartialRow writes incremental bytes from PartialDataRow
func (tx *Transaction) writePartialRow(pdr *PartialDataRow) error {
    allBytes, err := pdr.MarshalText()
    if err != nil {
        return err
    }

    // Slice off bytes already written
    newBytes := allBytes[tx.bytesWritten:]

    // Track bytes written
    tx.bytesWritten = len(allBytes)

    return tx.writeBytes(newBytes)
}
```

### Operation Flow

**Begin()**:

1. Create PartialDataRow with start_control='T',
   state=PartialDataRowWithStartControl
2. MarshalText() → 2 bytes (ROW_START + 'T')
3. writeBytes(2 bytes)
4. bytesWritten = 2
5. Return error if write fails

**AddRow() (first AddRow after Begin)**:

1. Validate UUIDv7 and value
2. Call tx.last.AddRow(key, value) → advances to PartialDataRowWithPayload
3. MarshalText() → rowSize-5 bytes (ROW_START + 'T' + UUID + JSON + padding)
4. Slice off first 2 bytes: newBytes = bytes[2:] → rowSize-7 bytes
5. writeBytes(newBytes)
6. bytesWritten = rowSize-5
7. Tombstone transaction if write fails

**AddRow() (subsequent AddRow calls)**:

1. Finalize previous PartialDataRow with RE end_control:
   - Calculate parity bytes for the row (positions 0 to rowSize-6)
   - Write RE + parity bytes + ROW_END (2 + 2 + 1 = 5 bytes)
   - Append finalized DataRow to tx.rows
2. Create new PartialDataRow with start_control='R',
   state=PartialDataRowWithStartControl
3. Call newPartial.AddRow(key, value) → advances to PartialDataRowWithPayload
4. MarshalText() → rowSize-5 bytes (ROW_START + 'R' + UUID + JSON + padding)
5. writeBytes(all rowSize-5 bytes) → this is a NEW row, start fresh
6. bytesWritten = rowSize-5
7. Tombstone transaction if write fails

**Commit() (empty transaction)**:

1. The PartialDataRow from Begin() has only 2 bytes written (ROW_START + 'T')
2. Need to complete as NullRow with 'NR' end_control:
   - MarshalText() on NullRow → rowSize bytes
   - Write rowSize-2 bytes (skip ROW_START + 'T' = 2 bytes, add NR + parity +
     ROW_END)
   - Or: writeBytes(rowSize - 2) for remaining bytes
3. Set tx.empty = nullRow, tx.last = nil, bytesWritten = 0
4. Return error if write fails

**Commit() (data transaction)**:

1. Finalize PartialDataRow with TC or SC end_control
2. MarshalText() on finalized DataRow → rowSize bytes (complete row)
3. Slice off bytesWritten bytes: newBytes = allBytes[tx.bytesWritten:]
4. writeBytes(newBytes)
5. Append finalized DataRow to tx.rows
6. Set tx.last = nil, bytesWritten = 0
7. Tombstone transaction if write fails

## Key Constraints

- All writes must be synchronous (wait for response before returning)
- Write failure tombstones transaction and returns error; subsequent calls
  return TombstonedError
- bytesWritten tracks bytes for current PartialDataRow, reset to 0 when row
  finalized or transaction committed
- All state modifications must be protected by Transaction.mu (sync.RWMutex) for
  thread-safety (FR-010)
- No checksum rows written (assumes < 10,000 rows)
- Caller responsible for setting up FileManager with write channel
- Append-only semantics preserved (writes only add new bytes via slicing)

## Critical Insight

The key insight is that PartialDataRow.MarshalText() returns cumulative bytes at
each state:

- State 1: 2 bytes (positions 0-1)
- State 2: rowSize-5 bytes (positions 0 to rowSize-6)
- State 3: rowSize-4 bytes (positions 0 to rowSize-6 plus 'S')

When we transition from State 1 → State 2, we've already written positions 0-1,
so we only write positions 2 to rowSize-6. When we finalize to complete row, we
write end_control second byte, parity bytes, and ROW_END (positions rowSize-5 to
rowSize-1).

This preserves append-only semantics while maintaining correct row structure.
