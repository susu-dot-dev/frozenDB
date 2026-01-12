# API Contracts: Transaction Struct

## Core Transaction API

### Transaction Struct

```go
type Transaction struct {
    Rows []DataRow  // Single slice of DataRow objects (max 100)
}
```

**Purpose**: Represents a single database transaction with a slice of DataRow objects
**Fields**:
- `Rows`: Slice of DataRow objects representing the transaction (1-100 rows)
  - First row (index 0) must be transaction start (StartControl = 'T')
  - Last row is either transaction end or transaction is still open

**Usage**: Create Transaction struct directly, then call Validate() to ensure integrity:
```go
tx := &Transaction{Rows: rows}
if err := tx.Validate(); err != nil {
    // Handle validation error
}
```

### State Management Methods

```go
func (t *Transaction) IsCommitted() bool
```

**Purpose**: Determines if the transaction is fully completed (committed or rolled back)
**Returns**: `true` if transaction has proper termination, `false` if still open
**Edge Case**: Returns false if last row ends with continuation (RE/SE)

### Data Access Methods

```go
func (t *Transaction) GetCommittedRows() (func() (DataRow, bool), error)
```

**Purpose**: Returns an iterator function for rows that are actually committed according to v1 file format rollback logic
**Returns**: 
- `func() (DataRow, bool)`: Iterator function that returns the next committed row and a boolean indicating if more rows exist
- `error`: FrozenDBError if transaction state is invalid
**Behavior**: 
- Commit (TC/SC): Iterator returns all rows through commit row
- Full rollback (R0/S0): Iterator returns no rows (immediately returns false)
- Partial rollback (R1-R9/S1-S9): Iterator returns rows 1 through savepoint N
**Usage**: Call the returned function repeatedly until it returns false:
```go
iter, err := tx.GetCommittedRows()
if err != nil {
    // Handle error
}
for row, more := iter(); more; row, more = iter() {
    // Process row
}
```

```go
func (t *Transaction) IsRowCommitted(index int) (bool, error)
```

**Purpose**: Determines if a specific row at index is committed
**Parameters**:
- `index`: Index of row within transaction slice (0-based)
**Returns**:
- `bool`: true if row is committed, false if invalidated
- `error`: FrozenDBError if index out of bounds
**Behavior**: Applies transaction-wide rollback logic to individual row queries

### Savepoint Management Methods

```go
func (t *Transaction) GetSavepointIndices() []int
```

**Purpose**: Identifies all savepoint locations within the transaction
**Returns**: Slice of indices for savepoint rows within the slice
**Detection**: Uses EndControl patterns with 'S' as first character
**Ordering**: Returns indices in savepoint creation order

### Validation Methods

```go
func (t *Transaction) Validate() error
```

**Purpose**: Scans all rows in the slice to ensure transaction integrity and check for inconsistencies
**Returns**: CorruptDatabaseError or InvalidInputError for invalid transactions
**Validation Rules**:
- First row must have StartControl = 'T' (transaction start)
- Proper StartControl sequences (T followed by R's for subsequent rows)
- Savepoint consistency and rollback target validity
- Either exactly one transaction termination within range, or transaction is still open
- Maximum row count (100) and savepoint count (9) enforced

## Error Contracts

### Error Types

Transaction validation uses existing error types from the frozenDB error hierarchy:

```go
// CorruptDatabaseError: Used when parsing valid rows reveals corruption
// (rows validated at insert time, so structural issues indicate corruption)
type CorruptDatabaseError struct {
    FrozenDBError
}

// InvalidInputError: Used for logic/instruction errors in transaction construction
// (e.g., improper API usage, invalid transaction structure)
type InvalidInputError struct {
    FrozenDBError
}
```

### Error Codes

| Error Code | Description | Example |
|------------|-------------|---------|
| `corrupt_database` | Database corruption detected during transaction parsing | Invalid StartControl sequence, malformed EndControl |
| `invalid_input` | Logic/instruction errors in transaction construction | Too many rows (>100), invalid savepoint reference, calling Savepoint() before inserting row |

### Error Messages

- `"transaction must start with DataRow having StartControl = 'T'"`
- `"transaction cannot contain more than 100 data rows (got %d)"`
- `"rollback to savepoint %d but transaction only has %d savepoints"`
- `"transaction must contain exactly one transaction-ending command"`

## Usage Contracts

### Thread Safety

All Transaction methods are thread-safe for concurrent read operations due to:
- Immutable underlying DataRow slice
- No mutable state in Transaction struct
- Read-only operations on validated data

### Memory Management

- Transaction maintains references to provided DataRow slice
- Caller must ensure DataRow slice remains valid for Transaction lifetime
- Transaction does not copy DataRow objects (8-byte references only)
- Memory usage: O(n) where n ≤ 100 rows

### Validation Guarantees

- Validate() must be called after creating Transaction struct to ensure integrity
- Validate() can be called multiple times safely (idempotent)
- All methods assume valid transaction (Validate() was called successfully) or return appropriate errors
- Validation errors use CorruptDatabaseError (for corruption) or InvalidInputError (for logic errors)

## Integration Contracts

### DataRow Compatibility

Transaction works with any DataRow objects that:
- Pass DataRow.Validate() successfully
- Have proper StartControl/EndControl sequences
- Contain valid UUIDv7 keys and JSON values
- Follow v1_file_format.md specification

### File Format Compliance

Transaction enforces all v1 file format requirements:
- Transaction state machine compliance
- Savepoint numbering and creation order
- Rollback semantics as per specification
- Maximum constraints (100 rows, 9 savepoints)