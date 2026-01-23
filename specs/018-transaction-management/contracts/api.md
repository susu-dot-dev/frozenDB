# frozenDB Transaction Management API

## Overview

The frozenDB transaction management API provides methods for detecting active transactions during database loading, querying active transaction state, and creating new transactions with proper conflict prevention.

## API Methods

### GetActiveTx() *Transaction

Returns the current active transaction or nil if no transaction is active.

**Signature:**
```go
func (db *FrozenDB) GetActiveTx() *Transaction
```

**Returns:**
- `*Transaction`: Active transaction instance if one exists
- `nil`: If no active transaction (committed, rolled back, or never started)

**Thread Safety:**
- Thread-safe using read lock on FrozenDB.txMu
- Returns reference to actual Transaction object (not copy)

**Example:**
```go
tx := db.GetActiveTx()
if tx != nil {
    fmt.Printf("Active transaction with %d rows\n", len(tx.GetRows()))
} else {
    fmt.Println("No active transaction")
}
```

### BeginTx() (*Transaction, error)

Creates a new transaction if no active transaction exists.

**Signature:**
```go
func (db *FrozenDB) BeginTx() (*Transaction, error)
```

**Returns:**
- `*Transaction`: New active transaction instance
- `error`: Error if transaction creation fails or conflicts with existing active transaction

**Error Conditions:**
- Returns `InvalidActionError` if an active transaction already exists
- Returns `WriteError` if transaction initialization fails

**Thread Safety:**
- Thread-safe using write lock on FrozenDB.txMu
- Atomic check-and-create operation prevents race conditions

**Example:**
```go
tx, err := db.BeginTx()
if err != nil {
    return fmt.Errorf("cannot create transaction: %w", err)
}
defer tx.Commit() // or tx.Rollback(0)

err = tx.AddRow(uuid.New(), "example value")
if err != nil {
    return fmt.Errorf("failed to add row: %w", err)
}
```

## Transaction State Detection

### Automatic Recovery

When a frozenDB file is opened, the system automatically:

1. Scans the last row to determine transaction state
2. Creates an active Transaction if an incomplete transaction is detected
3. Makes the recovered transaction available via `GetActiveTx()`

### Transaction State Mapping

| Last Row State | End Control | GetActiveTx() Result |
|----------------|-------------|---------------------|
| Open Transaction | RE, SE | Active Transaction |
| Committed Transaction | TC, SC | nil |
| Rolled Back Transaction | R0-R9, S0-S9 | nil |
| Partial Data Row | Any State | Active Transaction |
| Null Row | NR | nil |

## Usage Patterns

### Check Active Transaction Before Starting New One

```go
if db.GetActiveTx() != nil {
    return fmt.Errorf("cannot begin transaction: one is already active")
}

tx, err := db.BeginTx()
if err != nil {
    return fmt.Errorf("failed to begin transaction: %w", err)
}
```

### Resume Incomplete Transaction After Crash

```go
db := frozendb.NewFrozenDB("path/to/database.fdb")

tx := db.GetActiveTx()
if tx != nil {
    fmt.Printf("Resumed transaction with %d existing rows\n", len(tx.GetRows()))
    
    // Complete the transaction
    if shouldCommit {
        err = tx.Commit()
    } else {
        err = tx.Rollback(0)
    }
    if err != nil {
        return fmt.Errorf("failed to complete transaction: %w", err)
    }
}
```

### Transaction Workflow Integration

```go
func AddUser(db *frozendb.FrozenDB, user User) error {
    // Attempt to start transaction
    tx, err := db.BeginTx()
    if err != nil {
        return fmt.Errorf("transaction conflict: %w", err)
    }

    // Add user data
    err = tx.AddRow(user.ID, user.ToJSON())
    if err != nil {
        tx.Rollback(0) // Clean up on error
        return fmt.Errorf("failed to add user: %w", err)
    }

    // Commit transaction
    err = tx.Commit()
    if err != nil {
        return fmt.Errorf("failed to commit user: %w", err)
    }

    return nil
}
```

## Error Handling

### Error Types

- **InvalidActionError**: Transaction conflicts (BeginTx when active transaction exists)
- **CorruptDatabaseError**: Invalid transaction state detected during recovery
- **WriteError**: File operation failures during transaction creation

### Recovery Errors

Files with corrupted transaction state are rejected during loading:
- Invalid end control sequences
- Invalid PartialDataRow structure
- Multiple apparent transaction endings
- Rows with invalid structure

## Performance Considerations

- **GetActiveTx()**: < 5ms response time target
- **BeginTx()**: O(1) operation with minimal overhead
- **Transaction Recovery**: One-time cost during database opening
- **Memory Usage**: Constant regardless of database size