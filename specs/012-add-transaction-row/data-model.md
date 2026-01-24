# Data Model Design: AddRow Transaction Implementation

**Feature**: 012-add-transaction-row  
**Date**: 2026-01-19  
**Repository**: github.com/susu-dot-dev/frozenDB

## 1. Transaction Changes

### 1.1 Updated Transaction Struct

The Transaction struct is modified to add timestamp ordering support while maintaining existing fields:

```go
// Transaction represents a frozenDB transaction with AddRow capability
type Transaction struct {
    // Core transaction fields (existing from current implementation)
    rows   []DataRow       // Completed data rows
    last   *PartialDataRow // Current partial row being built  
    empty  *NullRow        // Empty transaction row (if applicable)
    Header *Header         // Header reference for row creation
    
    // UUID ordering management (only new field needed)
    maxTimestamp int64    // Maximum timestamp seen in this transaction
    
    // Thread safety (existing from current implementation)
    mu sync.RWMutex       // Mutex for thread-safe operations
}
```

### 1.2 State Inference

Transaction state is inferred from existing field combinations:
- **Inactive**: rows empty, empty nil, last nil
- **Active**: last non-nil, empty nil, rows may contain completed rows  
- **Committed**: empty non-nil, last nil

No explicit TransactionState field needed.

### 1.3 New Error Type

```go
// KeyOrderingError represents UUID timestamp ordering violation
type KeyOrderingError struct {
    FrozenDBError
}
```

## 2. AddRow Operation Data Flow

### 2.1 State Changes

When AddRow() is called:
1. Previous PartialDataRow (if any) is finalized and moved to rows[]
2. New PartialDataRow created with provided key-value data
3. maxTimestamp updated if new timestamp > current maxTimestamp
4. Thread safety maintained via mutex

### 2.2 UUIDv7 Ordering Logic

- Extract 48-bit timestamp from UUIDv7 using direct byte manipulation
- Validate: new_timestamp + skew_ms > max_timestamp
- Update maxTimestamp when valid ordering confirmed

### 2.3 Validation Matrix

| Requirement | Validation | Error Type |
|-------------|-------------|------------|
| FR-001 | Transaction must be active (last non-nil, empty nil) | InvalidActionError |
| FR-006 | UUID must be valid UUIDv7 | InvalidInputError |
| FR-007 | JSON value must be non-empty | InvalidInputError |
| FR-010 | Row count must be < 100 | InvalidInputError |
| FR-013 | Timestamp ordering: new_ts + skew_ms > max_ts | KeyOrderingError |
| FR-011 | Transaction must not be committed (empty nil) | InvalidActionError |

## 3. Integration Points

### 3.1 Transaction Initialization

Callers create Transaction struct directly, initializing maxTimestamp from database state:

```go
tx := &Transaction{
    rows:         make([]DataRow, 0, 100),
    maxTimestamp: initialMaxTimestamp,
    Header:       header,
}
```

### 3.2 New Methods

```go
// AddRow adds a new key-value pair to the transaction
func (tx *Transaction) AddRow(key uuid.UUID, value json.RawMessage) error

// GetMaxTimestamp returns the current maximum timestamp in the transaction
func (tx *Transaction) GetMaxTimestamp() int64
```

**Note**: Savepoint() and Rollback() methods are not part of this implementation scope.

### 3.3 Thread Safety Model

- Read operations: Use RLock for concurrent access
- Write operations: Use Lock for exclusive access
- State transitions: Atomic within critical sections

This data model provides the minimal changes needed for AddRow functionality while preserving frozenDB's architectural principles.