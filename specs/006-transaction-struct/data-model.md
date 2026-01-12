# Data Model: Transaction Struct

## Entity Definitions

### Transaction

**Description**: High-level abstraction representing a single database transaction with maximum 100 DataRow objects in a single slice. The first row must be the transaction start (StartControl = 'T'), and the last row is either the end of the transaction or the transaction is still open. Provides methods for accessing committed data, managing savepoints, and determining row commit status.

**Fields**:
```go
type Transaction struct {
    rows []DataRow    // Single slice of DataRow objects (max 100)
    // First row (index 0) must be transaction start (StartControl = 'T')
    // Last row (index len(rows)-1) is either transaction end or transaction is still open
}
```

**Constraints**:
- Maximum 100 DataRow objects per transaction
- First row must be transaction start (StartControl = 'T')
- Last row is either transaction end or transaction is still open
- Underlying DataRow objects are immutable (thread-safe)

**Validation Rules**:
- Must contain at least one DataRow
- First DataRow must have StartControl = 'T' (verified by Validate())
- Subsequent DataRows must have StartControl = 'R'
- Validate() scans all rows in the slice to check for inconsistencies
- Either exactly one transaction-ending command required, or transaction is still open (no termination found)
- Maximum 9 savepoints allowed

## Relationships

### Transaction → DataRow (Composition)

**Relationship**: Transaction contains 1-100 DataRow objects
**Cardinality**: 1-to-many (1:100)
**Nature**: Strong ownership, immutable composition
**Lifecycle**: DataRows outlive Transaction (shared reference)

**Operations**:
- Transaction accesses DataRow properties via methods
- Transaction validates DataRow sequences
- Transaction determines DataRow commit status

### DataRow → Transaction State (Association)

**Relationship**: DataRow objects contribute to transaction state determination
**Cardinality**: Many-to-one ( DataRow : 1 Transaction )
**Nature**: Stateless association, derived relationship
**Lifecycle**: DataRow state derived from EndControl patterns

**State Determination**:
- Transaction committed if ends with TC/SC
- Transaction rolled back based on rollback commands
- Individual row commit status determined by transaction-wide logic

## Data Flow

### Transaction Creation
1. Transaction struct created directly with DataRow slice: `tx := &Transaction{Rows: rows}`
2. Validate() must be called to scan all rows and ensure proper transaction structure
3. First row must be transaction start (StartControl = 'T')
4. Savepoint indices calculated and cached
5. Transaction state determined from EndControl patterns (or detected as still open)

### Commit Status Determination
1. Parse transaction ending command from last DataRow
2. For commits: all rows through end are valid
3. For rollbacks: apply savepoint logic to determine valid rows
4. Cache commit status for efficient lookup

### Savepoint Management
1. Scan DataRows for EndControl patterns starting with 'S'
2. Number savepoints in creation order (1-9)
3. Store indices for savepoint locations within the slice
4. Support rollback target validation

## State Transitions

### Transaction States

**Validating**: Initial state during construction
- Validates DataRow sequence integrity
- Checks transaction constraints
- Transitions to Committed or RolledBack based on EndControl

**Committed**: Transaction ends with commit command
- All rows through commit row are valid
- IsCommitted() returns true
- GetCommittedRows() iterator returns all transaction rows

**RolledBack**: Transaction ends with rollback command
- Apply savepoint rollback logic
- IsCommitted() returns false
- GetCommittedRows() iterator returns subset based on savepoint

**Invalid**: Transaction validation fails
- Returns structured error with specific code
- No further operations permitted
- Must be recreated with valid DataRows

### Row Commit Status Transitions

**Pending**: Initial state during transaction parsing
- Row commit status not yet determined
- Depends on transaction ending command

**Committed**: Row determined to be part of final state
- Row included in GetCommittedRows() iterator
- IsRowCommitted(index) returns true
- Based on transaction-wide rollback logic

**Invalidated**: Row rolled back by transaction ending
- Row excluded from GetCommittedRows() iterator
- IsRowCommitted(index) returns false
- Still physically present in DataRow slice

## Integrity Constraints

### Structural Constraints
- **Row Count**: 1-100 DataRows per transaction
- **StartControl Sequence**: Must begin with 'T' (first row), continue with 'R' (subsequent rows)
- **EndControl Sequence**: Either exactly one ending command required, or transaction is still open
- **Savepoint Count**: Maximum 9 savepoints per transaction

### Semantic Constraints
- **Time Ordering**: UUIDv7 keys must maintain chronological order
- **Rollback Validity**: Rollback targets must exist within transaction
- **Savepoint Consistency**: Savepoint numbering must be sequential
- **Transaction Atomicity**: All-or-nothing commit/rollback semantics

### Referential Integrity
- **DataRow Immutability**: Underlying DataRow objects cannot be modified
- **Index Bounds**: All row indices must be within valid slice range
- **Savepoint References**: Savepoint indices must point to actual savepoint rows

### Concurrency
- **Read Safety**: Thread-safe due to immutable underlying data
- **Write Safety**: Not applicable (Transaction is read-only wrapper)
- **Memory Visibility**: No synchronization needed for concurrent reads
