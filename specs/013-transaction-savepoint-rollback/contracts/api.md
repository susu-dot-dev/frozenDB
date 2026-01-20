# API Contract: Transaction Savepoint and Rollback

## Overview

This contract defines the public API for transaction savepoint and rollback operations in frozenDB.

## Methods

### Transaction.Savepoint()

Creates a savepoint at the current position in the transaction.

**Signature:**
```go
func (t *Transaction) Savepoint() error
```

**Parameters:** None

**Returns:**
- `error`: 
  - `nil` on success
  - `*InvalidActionError` if called on empty transaction
  - `*InvalidActionError` if called on inactive transaction
  - `*InvalidActionError` if more than 9 savepoints would be created

**Preconditions:**
- Transaction must be active
- Transaction must contain at least one data row
- Savepoint count (derived from existing savepoint rows) must be less than 9

**Postconditions:**
- Current row is marked as a savepoint
- Transaction state transitions to PartialDataRowWithSavepoint
- Savepoint count is derived from analyzing completed rows with savepoint flags

**Example:**
```go
tx := db.BeginTransaction()
tx.AddRow("key1", "value1")
err := tx.Savepoint() // Creates savepoint 1
if err != nil {
    // Handle error
}
```

### Transaction.Rollback(savepointId int)

Rolls back the transaction to a specified savepoint or fully closes it.

**Signature:**
```go
func (t *Transaction) Rollback(savepointId int) error
```

**Parameters:**
- `savepointId`: Target savepoint number (0-9)
  - `0`: Full rollback (invalidate all rows)
  - `1-9`: Partial rollback to specified savepoint

**Returns:**
- `error`:
  - `nil` on success
  - `*InvalidActionError` if called on inactive transaction
  - `*InvalidInputError` if savepointId > current savepoint count
  - `*InvalidInputError` if savepointId < 0 or > 9

**Preconditions:**
- Transaction must be active
- savepointId must be valid (0 to current savepoint count derived from existing savepoint rows)

**Postconditions:**
- For savepointId = 0: All rows invalidated, NullRow created if transaction empty
- For savepointId > 0: Rows up to savepoint committed, subsequent rows invalidated
- Transaction is closed and no longer active
- Appropriate end control encoding applied (R0-R9, S0-S9)

**Examples:**
```go
// Full rollback
tx := db.BeginTransaction()
tx.AddRow("key1", "value1")
tx.AddRow("key2", "value2")
err := tx.Rollback(0) // All rows invalidated

// Partial rollback
tx := db.BeginTransaction()
tx.AddRow("key1", "value1")
tx.Savepoint() // Creates savepoint 1
tx.AddRow("key2", "value2")
tx.Savepoint() // Creates savepoint 2
tx.AddRow("key3", "value3")
err := tx.Rollback(1) // key1 committed, key2 and key3 invalidated
```

## Error Types

### InvalidActionError

Used for operations that are invalid due to transaction state.

**When returned:**
- Savepoint() called on empty transaction
- Savepoint() called on inactive transaction
- Savepoint() would exceed 9 savepoint limit
- Rollback() called on inactive transaction

### InvalidInputError

Used for invalid parameter values.

**When returned:**
- Rollback() with savepointId > current savepoint count
- Rollback() with savepointId < 0 or > 9

## Transaction States

### State Transitions

```
Begin → PartialDataRowWithStartControl
AddRow → PartialDataRowWithPayload
Savepoint → PartialDataRowWithSavepoint
Rollback/Commit → Transaction Closed
```

### Valid Operations by State

| State | Savepoint() | Rollback() | AddRow() |
|-------|-------------|------------|----------|
| PartialDataRowWithStartControl | ❌ | ✅ | ✅ |
| PartialDataRowWithPayload | ✅ | ✅ | ✅ |
| PartialDataRowWithSavepoint | ✅ | ✅ | ✅ |
| Closed | ❌ | ❌ | ❌ |

**State Validation**: Savepoint limits enforced by counting existing rows with `EndControl[0] == 'S'`

## Constraints

### Savepoint Limits
- Maximum 9 savepoints per transaction (enforced by counting savepoint rows)
- Savepoints numbered 1-9 by counting rows with savepoint flags in creation order
- At least one data row required before first savepoint
- No explicit counter needed - count derived from row analysis

### Transaction Limits
- Maximum 100 data rows per transaction
- No nested transactions
- Exactly one transaction-ending command per transaction

### End Control Encoding

The rollback operations use specific end control encodings:

| Operation | End Control | Description |
|-----------|-------------|-------------|
| Full rollback (no savepoint) | `R0` | Rollback to savepoint 0 |
| Full rollback (with savepoint) | `S0` | Savepoint + rollback to 0 |
| Partial rollback (no savepoint) | `R1-R9` | Rollback to savepoint N |
| Partial rollback (with savepoint) | `S1-S9` | Savepoint + rollback to N |

## Implementation Notes

### Thread Safety
All transaction methods are thread-safe through internal mutex usage.

### Append-Only Semantics
Rollback operations do not modify existing rows. Instead, they append new rows with special end control encoding that invalidates the appropriate rows according to the v1_file_format specification.

### Memory Usage
Transaction operations maintain fixed memory usage regardless of database size, consistent with frozenDB's architecture principles.