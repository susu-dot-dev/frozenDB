# Quickstart: Transaction Savepoint and Rollback

This quickstart shows how to use transaction savepoints and rollback operations in frozenDB.

## Basic Savepoint Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Open database
    db, err := frozendb.Open("data.fdb")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Begin transaction
    tx := db.BeginTransaction()
    
    // Add some data
    tx.AddRow("user:1", `{"name": "Alice", "age": 30}`)
    tx.AddRow("user:2", `{"name": "Bob", "age": 25}`)
    
    // Create a savepoint
    err = tx.Savepoint()
    if err != nil {
        log.Fatal(err)
    }
    
    // Add more data
    tx.AddRow("user:3", `{"name": "Charlie", "age": 35}`)
    
    // Rollback to savepoint 1 (keeps first two users, removes Charlie)
    err = tx.Rollback(1)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Transaction rolled back successfully")
}
```

## Full Transaction Rollback

```go
func fullRollbackExample(db *frozendb.DB) error {
    tx := db.BeginTransaction()
    
    // Add data
    tx.AddRow("temp:1", `{"data": "temporary"}`)
    
    // Decide to rollback everything
    return tx.Rollback(0) // All data invalidated
}
```

## Multiple Savepoints

```go
func multipleSavepointsExample(db *frozendb.DB) error {
    tx := db.BeginTransaction()
    
    // First data
    tx.AddRow("step1", `{"completed": true}`)
    tx.Savepoint() // Savepoint 1
    
    // More data
    tx.AddRow("step2", `{"completed": true}`)
    tx.Savepoint() // Savepoint 2
    
    // Even more data
    tx.AddRow("step3", `{"completed": true}`)
    
    // Rollback to savepoint 1 (keeps step1, removes step2 and step3)
    return tx.Rollback(1)
}
```

## Error Handling

```go
func errorHandlingExample(db *frozendb.DB) error {
    tx := db.BeginTransaction()
    
    // Try to create savepoint before adding data (will fail)
    err := tx.Savepoint()
    if err != nil {
        // Handle InvalidActionError: cannot savepoint empty transaction
        fmt.Printf("Expected error: %v\n", err)
    }
    
    // Add data first
    tx.AddRow("data:1", `{"value": 1}`)
    
    // Now savepoint will work
    return tx.Savepoint()
}
```

## Key Points

- Savepoints can only be created after adding at least one data row
- Maximum 9 savepoints per transaction
- `Rollback(0)` performs a full rollback
- `Rollback(n)` where n>0 rolls back to that specific savepoint
- All operations are thread-safe
- Rollbacks follow append-only semantics (add new rows, don't modify existing)