# Data Model: Transaction File Persistence

**Feature**: 015-transaction-persistence **Date**: 2026-01-21

## Entities

### Transaction

**Description**: Represents an in-progress database operation that can add rows
and must be committed. Handles persistence of transaction state via write
channel to FileManager.

**Fields**:

- `rows []DataRow` - Single slice of DataRow objects (max 100) - unexported for
  immutability
- `empty *NullRow` - Empty null row after successful commit
- `last *PartialDataRow` - Current partial data row being built
- `Header *Header` - Header reference for row creation
- `maxTimestamp int64` - Maximum timestamp seen in this transaction for UUID
  ordering
- `mu sync.RWMutex` - Mutex for thread safety
- `writeChan chan<- Data` - Write channel for sending Data structs to
  FileManager (NEW for this feature)
- `rowBytesWritten int` - Tracks how many bytes of current PartialDataRow have been
  written (NEW for this feature, internal field, NOT initialized by caller)
- `tombstone bool` - Tombstone flag set when write operation fails (NEW for this
  feature)

**Relationships**:

- Contains 0-100 DataRow objects in rows slice
- Contains 0-1 PartialDataRow in last field
- Contains 0-1 NullRow in empty field
- References Header for row creation
- Sends writes to FileManager via writeChan

**State Transitions**:

- **Inactive**: rows empty, empty nil, last nil (initial state)
- **Active**: last non-nil, empty nil (after Begin(), before Commit/Rollback)
- **Committed (Empty)**: empty non-nil, last nil (Begin() + Commit() with no
  AddRow())
- **Committed (Data)**: rows non-empty with transaction-ending control, last nil
  (Begin() + AddRow() + ... + Commit())
- **Tombstoned**: tombstone true (after any write operation fails); all
  subsequent public API calls return TombstonedError

**Validation Rules**:

- Maximum 100 data rows (both DataRows and PartialDataRows count)
- Maximum 9 savepoints
- Only one transaction-ending command (commit or rollback)
- Must have Begin() called before AddRow(), Commit(), Rollback()
- Write channel must be set before calling Begin(), AddRow(), Commit()
- Thread-safety: All state modifications are protected by mu (Write lock), reads
  are protected by mu (Read lock) per FR-010
- Tombstoned: Transaction is tombstoned on write failure; all subsequent public
  API calls return TombstonedError

**Operations**:

- `Begin() error` - Initializes transaction, writes PartialDataRow to disk via
  writeChan. Tomestones transaction on write failure.
- `AddRow(key uuid.UUID, value json.RawMessage) error` - Adds row, writes previous
  PartialDataRow (if exists) and new PartialDataRow to disk. Tombstones
  transaction on write failure.
- `Commit() error` - Finalizes transaction, writes NullRow or final DataRow to
  disk. Tombstones transaction on write failure.
- `Rollback(savepointId int) error` - Rolls back transaction. Returns
  TombstonedError if transaction is tombstoned.
- `Savepoint() error` - Creates savepoint. Returns TombstonedError if
  transaction is tombstoned.
- `IsTombstoned() bool` - Returns true if transaction is tombstoned

### PartialDataRow

**Description**: Represents an incomplete transaction row written to disk during
Begin() and AddRow() operations, indicates transaction is in-progress.

**Fields**:

- `state PartialDataRowState` - Current state: WithStartControl, WithPayload, or
  WithSavepoint
- `d DataRow` - Embedded DataRow with baseRow[*DataRowPayload]

**Relationships**:

- Embedded in Transaction.last field
- Becomes DataRow when finalized

**State Transitions**:

- **PartialDataRowWithStartControl**: ROW_START + START_TRANSACTION only
  (created by Begin())
- **PartialDataRowWithPayload**: ROW_START + START_TRANSACTION + UUID + JSON
  value (first AddRow())
- **PartialDataRowWithSavepoint**: ROW_START + START_TRANSACTION + UUID + JSON
  value + 'S' (after Savepoint())

**Validation Rules**:

- Must be in valid state before marshaling
- Payload fields (UUID, value) must be validated in WithPayload and
  WithSavepoint states
- Cannot be finalized as PartialDataRow - must become DataRow

**Operations**:

- `MarshalText() ([]byte, error)` - Serializes to bytes in current state
- `AddRow(key uuid.UUID, value json.RawMessage) error` - Adds key-value data
- `Savepoint() error` - Marks row as having savepoint intent
- `EndRow() (*DataRow, error)` - Finalizes with RE end_control (commit)
- `Commit() (*DataRow, error)` - Finalizes with TC or SC end_control (commit)
- `Rollback(savepointId int) (*DataRow, error)` - Finalizes with R0-R9 or S0-S9
  end_control (rollback)

### NullRow

**Description**: Represents an empty transaction (no rows added), written to
disk on Commit() when no data rows exist. The PartialDataRow created by Begin()
is finalized as a NullRow, resulting in exactly one row.

**Fields**:

- `baseRow[*NullRowPayload]` - Embedded baseRow with NullRowPayload

**Relationships**:

- Stored in Transaction.empty field after empty transaction commit
- Standalone transaction (no continuation rows possible)

**Validation Rules**:

- Start control must be 'T' (START_TRANSACTION)
- End control must be 'NR' (NULL_ROW_CONTROL)
- Key must be uuid.Nil (all zeros)
- No value field (padding starts immediately after UUID)

**Operations**:

- `MarshalText() ([]byte, error)` - Serializes complete row bytes

### Data

**Description**: Channel message type for sending write requests to FileManager.
Used by Transaction to send row bytes for persistence.

**Fields**:

- `Bytes []byte` - Raw bytes to write to file
- `Response chan<- error` - Channel for FileManager to send back write
  completion/error

**Relationships**:

- Sent by Transaction to FileManager via writeChan
- Processed by FileManager.writerLoop()
- Response channel receives error or nil when write completes

**Validation Rules**:

- Bytes cannot be nil or empty (enforced by caller)
- Response channel must have capacity for at least 1 error (caller creates with
  buffer)

### FileManager

**Description**: Append-only file interface used by Transaction to write rows to
the database file. Not modified by this feature, but Transaction interacts with
it.

**Fields**:

- `filePath string` - Path to database file
- `file *os.File` - Underlying file handle
- `mutex sync.RWMutex` - Mutex for thread safety
- `writeChannel <-chan Data` - Write channel set by caller
- `currentSize int64` - Current file size (for append-only writes)
- `tombstone bool` - Tombstone flag (set on write error)
- `closed bool` - Closed flag

**Relationships**:

- Receives Data structs from Transaction via writeChannel
- Appends bytes to file (never modifies existing bytes)

**Operations**:

- `SetWriter(dataChan <-chan Data) error` - Sets write channel and starts
  writerLoop
- `Read(start int64, size int) ([]byte, error)` - Reads bytes from file
- `Size() int64` - Returns current file size
- `Close() error` - Closes file
- `IsTombstoned() bool` - Returns tombstone status

## Persistence Model

### Write Sequence for Transaction Operations

**Begin() Operation**:

1. Transaction creates PartialDataRow with state=PartialDataRowWithStartControl
2. Transaction calls PartialDataRow.MarshalText() → 2 bytes (ROW_START + 'T')
3. Transaction sends Data{Bytes: 2 bytes, Response: responseChan} to writeChan
4. Transaction waits: err := <-responseChan
5. If err != nil, Transaction returns error (no state change)
6. Otherwise, tx.rowBytesWritten = 2, tx.last = pdr (transaction is now Active)

**AddRow() Operation (first AddRow after Begin)**:

1. Transaction validates UUIDv7 and value
2. Transaction calls tx.last.AddRow(key, value) → advances to
   PartialDataRowWithPayload
3. Transaction calls PartialDataRow.MarshalText() → rowSize-5 bytes (complete up
   to padding)
4. Transaction slices off first 2 bytes: newBytes = bytes[2:] → rowSize-7 bytes
5. Transaction sends new bytes to writeChan
6. Transaction waits for response
7. If err != nil, Transaction returns error (no state change)
8. Otherwise, tx.rowBytesWritten = rowSize-5, AddRow completes (partial row now has
   payload)

**AddRow() Operation (subsequent AddRow calls)**:

1. Transaction finalizes previous PartialDataRow: a. Calculates parity bytes for
   row positions 0 to rowSize-6 b. Sends 5 bytes: RE + parity + ROW_END to
   writeChan c. Waits for response d. If err != nil, returns error (no state
   change)
2. Transaction appends finalized DataRow to tx.rows
3. Transaction creates new PartialDataRow with StartControl=ROW_CONTINUE
4. Transaction calls newPartial.AddRow(key, value)
5. Transaction calls newPartial.MarshalText() → rowSize-5 bytes (NEW row, start
   fresh)
6. Transaction sends all rowSize-5 bytes to writeChan
7. Transaction waits for response
8. If err != nil, Transaction returns error (no state change)
9. Otherwise, tx.rowBytesWritten = rowSize-5, tx.last = newPartial (ready for next
   AddRow or Commit)

**Commit() Operation (empty transaction)**:

1. Transaction creates NullRow with StartControl='T', EndControl='NR',
   Key=uuid.Nil
2. Transaction calls NullRow.MarshalText() → rowSize bytes
3. Transaction slices off first 2 bytes: newBytes = bytes[2:] → rowSize-2 bytes
4. Transaction sends new bytes to writeChan
5. Transaction waits for response
6. If err != nil, Transaction returns error (no state change)
7. Otherwise, tx.rowBytesWritten = 0, tx.empty = nullRow, tx.last = nil
   (transaction Committed)

**Commit() Operation (data transaction)**:

1. Transaction calls tx.last.Commit() → finalized DataRow with TC or SC
2. Transaction calls DataRow.MarshalText() → rowSize bytes (complete row)
3. Transaction slices off rowBytesWritten bytes: newBytes = bytes[tx.rowBytesWritten:]
4. Transaction sends new bytes to writeChan
5. Transaction waits for response
6. If err != nil, Transaction returns error (no state change)
7. Otherwise, tx.rows.append(finalizedRow), tx.rowBytesWritten = 0, tx.last = nil
   (transaction Committed)

### Error Handling Model

- All write operations create responseChan with buffer size 1
- All write operations wait for response before returning (synchronous)
- If write returns error, Transaction is tombstoned and the error is returned
- Once tombstoned, all subsequent public API calls return TombstonedError
- Write errors are wrapped and returned as-is from FileManager (WriteError,
  CorruptDatabaseError)
- Partial writes are impossible because FileManager is append-only (atomic file
  writes)
- If FileManager becomes tombstoned, subsequent writes return error immediately

### Data Integrity Guarantees

- Append-only semantics preserved (writes only add bytes, never modify)
- Row bytes include sentinel bytes (ROW_START, ROW_END) for corruption detection
- Parity bytes included in each row for LRC validation
- Write failures prevent state changes (atomic operations)
- No checksum rows written (assumes < 10,000 rows per spec)
