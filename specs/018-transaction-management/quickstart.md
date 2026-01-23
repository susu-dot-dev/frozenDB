# frozenDB Transaction Management Quickstart

## Getting Started

This quickstart shows the essential patterns for managing transactions in frozenDB, including automatic recovery and safe transaction creation.

## Basic Usage

### Open Database and Check Transaction State

```go
package main

import (
    "fmt"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Open database - automatically recovers incomplete transactions
    db, err := frozendb.NewFrozenDB("mydb.fdb")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Check if there's an active transaction
    tx := db.GetActiveTx()
    if tx != nil {
        fmt.Printf("Resumed active transaction with %d rows\n", len(tx.GetRows()))
        
        // Complete or rollback the existing transaction
        err = tx.Rollback(0) // or tx.Commit()
        if err != nil {
            panic(err)
        }
    } else {
        fmt.Println("No active transaction")
    }
}
```

### Create New Transaction

```go
// Create a new transaction (fails if one is already active)
tx, err := db.BeginTx()
if err != nil {
    return fmt.Errorf("cannot create transaction: %w", err)
}

// Add data to the transaction
err = tx.AddRow(uuid.New(), `{"name": "Alice", "age": 30}`)
if err != nil {
    tx.Rollback(0) // Clean up on error
    return fmt.Errorf("failed to add row: %w", err)
}

// Commit the transaction
err = tx.Commit()
if err != nil {
    return fmt.Errorf("failed to commit: %w", err)
}
```

### Safe Transaction Pattern

```go
func AddUser(db *frozendb.FrozenDB, user User) error {
    // Check for existing transaction first
    if db.GetActiveTx() != nil {
        return fmt.Errorf("cannot add user: transaction already active")
    }

    // Create new transaction
    tx, err := db.BeginTx()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    // Ensure cleanup on any error
    defer func() {
        if err != nil {
            tx.Rollback(0)
        }
    }()

    // Add user data
    err = tx.AddRow(user.ID, user.ToJSON())
    if err != nil {
        return fmt.Errorf("failed to add user row: %w", err)
    }

    // Commit transaction
    err = tx.Commit()
    if err != nil {
        return fmt.Errorf("failed to commit user: %w", err)
    }

    return nil
}
```

### Recovery After Crash

```go
func ResumeTransaction(db *frozendb.FrozenDB) error {
    tx := db.GetActiveTx()
    if tx == nil {
        fmt.Println("No transaction to resume")
        return nil
    }

    fmt.Printf("Found incomplete transaction with %d rows\n", len(tx.GetRows()))

    // Inspect existing data
    for i, row := range tx.GetRows() {
        fmt.Printf("Row %d: Key=%s, Value=%s\n", i, row.UUID, row.JSONPayload)
    }

    // Choose to commit or rollback based on application logic
    if shouldCompleteTransaction(tx) {
        return tx.Commit()
    } else {
        return tx.Rollback(0)
    }
}

func shouldCompleteTransaction(tx *frozendb.Transaction) bool {
    // Your business logic here
    return len(tx.GetRows()) > 0
}
```

## Key Points

- **Automatic Recovery**: FrozenDB automatically detects and restores incomplete transactions when opening files
- **Conflict Prevention**: `BeginTx()` fails if an active transaction already exists
- **Thread Safety**: All transaction methods are thread-safe using proper locking
- **Direct Access**: `GetActiveTx()` returns the actual Transaction object for direct manipulation

## Error Handling

Always handle transaction errors appropriately:

```go
tx, err := db.BeginTx()
if err != nil {
    switch err.(type) {
    case *frozendb.InvalidActionError:
        // Transaction already active
        return handleExistingTransaction()
    case *frozendb.WriteError:
        // File system error
        return fmt.Errorf("write failed: %w", err)
    default:
        return fmt.Errorf("unexpected error: %w", err)
    }
}
```