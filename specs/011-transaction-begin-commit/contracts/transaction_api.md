# Transaction API Contract

## Overview

This document defines the API contract for enhanced Transaction methods `Begin()` and `Commit()` that support empty transaction workflows in frozenDB.

## Transaction Struct Enhancement

```go
type Transaction struct {
    rows []DataRow           // Complete data rows (existing)
    empty *NullRow          // Empty null row after commit (new)
    last  *PartialDataRow   // Current partial data row (new)
    mu    sync.RWMutex      // Thread safety mutex (new)
}
```

## Public API Methods

### Begin() Method

**Signature:**
```go
func (tx *Transaction) Begin() error
```

**Purpose:** 
Initialize an empty transaction by creating a PartialDataRow in `PartialDataRowWithStartControl` state.

**Preconditions:**
- Transaction must be inactive (all fields empty/nil)
- `rows` slice must be empty
- `empty` field must be nil
- `last` field must be nil

**Postconditions:**
- Transaction becomes active (inferred from field values)
- `last` field points to new PartialDataRow with start control
- All other fields remain unchanged

**Error Conditions:**
```go
// When transaction is already active
NewInvalidActionError("Begin() cannot be called when transaction is already active")

// When rows exist
NewInvalidActionError("Begin() cannot be called when rows exist")

// When empty row exists
NewInvalidActionError("Begin() cannot be called when empty row exists")

// When partial row exists
NewInvalidActionError("Begin() cannot be called when partial row exists")
```

**Example Usage:**
```go
tx := &Transaction{state: StateInactive}
err := tx.Begin()
if err != nil {
    return fmt.Errorf("failed to begin transaction: %w", err)
}
// tx.state == StateActive
// tx.last != nil
```

### Commit() Method

**Signature:**
```go
func (tx *Transaction) Commit() error
```

**Purpose:**
Complete an empty transaction by converting the PartialDataRow to a NullRow with null payload.

**Preconditions:**
- Transaction must be active (partial data row exists, empty is nil, rows empty)
- `last` field must be non-nil
- `last.GetState()` must equal `PartialDataRowWithStartControl`

**Postconditions:**
- Transaction becomes committed (inferred from field values)
- `rows` slice contains exactly one NullRow
- `empty` field points to created NullRow
- `last` field is set to nil

**Error Conditions:**
```go
// When transaction is not active
NewInvalidActionError("Commit() can only be called when transaction is active")

// When no partial data row exists
NewInvalidActionError("Commit() requires a partial data row")

// When partial data row is in wrong state
NewInvalidActionError("Commit() requires PartialDataRowWithStartControl state")
```

**Example Usage:**
```go
err := tx.Commit()
if err != nil {
    return fmt.Errorf("failed to commit transaction: %w", err)
}
// tx.state == StateCommitted
// len(tx.rows) == 1
// tx.empty != nil
// tx.last == nil
```

## State Management Contract

### State Transitions

```
StateInactive --Begin()--> StateActive --Commit()--> StateCommitted
     |                        |                           |
     |                        v                           v
   InvalidActionError  InvalidActionError           InvalidActionError
```

### Thread Safety Contract

All public methods must be thread-safe:
- Methods acquire write lock (`t.mu.Lock()`) before state changes
- Methods acquire read lock (`t.mu.RLock()`) for read operations
- Locks are released using `defer` to prevent deadlocks
- All state validation occurs under lock protection

### Memory Safety Contract

- No memory leaks: pointer fields are properly managed
- Constant memory usage: struct size doesn't grow with database size
- Atomic operations: state changes are indivisible
- No data races: all shared state access is mutex-protected

## Integration Contract

### Existing Method Compatibility

Existing Transaction methods remain unchanged in behavior:
- `GetRows()` returns the `rows` slice (read-only access)
- `IsCommitted()` checks if transaction has proper termination
- `GetCommittedRows()` returns iterator for committed rows
- `Validate()` performs comprehensive validation

### Error Handling Integration

- All errors use existing `InvalidActionError` type
- Error messages follow frozenDB error handling guidelines
- Error wrapping preserves underlying error context
- Error types support proper error unwrapping and comparison

### Spec Testing Integration

- All functional requirements have corresponding spec tests
- Tests follow naming convention: `Test_S_XXX_FR_XXX_Description()`
- Tests are co-located with implementation in `transaction_spec_test.go`
- Tests are not modified after implementation without explicit user permission

## Performance Contract

### Timing Requirements

- `Begin()` operation completes in under 1ms
- `Commit()` operation completes in under 1ms
- Full empty transaction workflow (Begin→Commit) under 10ms per SC-001

### Memory Requirements

- Transaction struct size remains constant
- No dynamic memory allocation beyond fixed struct fields
- Memory usage does not scale with database size
- Pointer fields enable efficient optional data representation

### Concurrency Requirements

- Multiple goroutines can safely call methods concurrently
- No data races or corruption under concurrent access
- Read operations can proceed without blocking other reads
- Write operations are serialized to maintain consistency

## Validation Contract

### Pre-operation Validation

All methods validate conditions before making state changes:
- State transitions are validated before execution
- Field constraints are checked before mutation
- Error conditions return early without changing state

### Post-operation Validation

All methods ensure consistency after execution:
- State transitions are atomic and complete
- Field states match the expected post-conditions
- Invariant constraints are maintained

### Comprehensive Validation

The `Validate()` method continues to provide complete validation:
- Transaction structure integrity
- Row sequence correctness
- Control character validity
- Savepoint consistency
- Transaction termination rules