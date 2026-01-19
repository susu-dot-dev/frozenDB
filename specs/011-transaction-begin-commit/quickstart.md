# Transaction Begin and Commit Quickstart

## Overview

This quickstart guide demonstrates how to use the enhanced Transaction methods `Begin()` and `Commit()` to create empty transactions that result in a single NullRow. This is the foundational workflow for handling empty transactions in frozenDB.

## Prerequisites

- frozenDB library imported
- Understanding of basic frozenDB concepts (Header, DataRow, NullRow)
- Go development environment set up

## Basic Empty Transaction Workflow

### Step 1: Create a New Transaction

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create a new empty transaction
    tx := &frozendb.Transaction{}
    
    // Initialize transaction state (if not already done)
    // This depends on Transaction constructor implementation
    
    fmt.Println("Transaction created successfully")
}
```

### Step 2: Begin the Transaction

```go
// Begin the empty transaction
err := tx.Begin()
if err != nil {
    log.Fatalf("Failed to begin transaction: %v", err)
}

fmt.Println("Transaction begun - PartialDataRow created with start control")
// At this point:
// - tx.state == StateActive
// - tx.last != nil (PartialDataRowWithStartControl)
// - tx.rows is still empty
```

### Step 3: Commit the Transaction

```go
// Commit the empty transaction
err = tx.Commit()
if err != nil {
    log.Fatalf("Failed to commit transaction: %v", err)
}

fmt.Println("Transaction committed - NullRow created")
// At this point:
// - tx.state == StateCommitted
// - tx.last == nil
// - tx.rows contains exactly one NullRow
// - tx.empty != nil
```

### Complete Example

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create a new empty transaction
    tx := &frozendb.Transaction{}
    
    // Execute empty transaction workflow
    if err := executeEmptyTransaction(tx); err != nil {
        log.Fatalf("Empty transaction failed: %v", err)
    }
    
    // Verify results
    if err := verifyEmptyTransactionResult(tx); err != nil {
        log.Fatalf("Transaction verification failed: %v", err)
    }
    
    fmt.Println("Empty transaction completed successfully!")
}

func executeEmptyTransaction(tx *frozendb.Transaction) error {
    // Step 1: Begin transaction
    if err := tx.Begin(); err != nil {
        return fmt.Errorf("begin failed: %w", err)
    }
    
    // Step 2: Commit transaction (no intermediate steps needed for empty transaction)
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit failed: %w", err)
    }
    
    return nil
}

func verifyEmptyTransactionResult(tx *frozendb.Transaction) error {
    rows := tx.GetRows()
    
    // Should have exactly one row
    if len(rows) != 1 {
        return fmt.Errorf("expected 1 row, got %d", len(rows))
    }
    
    // The single row should be a NullRow
    row := rows[0]
    if _, isNullRow := row.(*frozendb.NullRow); !isNullRow {
        return fmt.Errorf("expected NullRow, got %T", row)
    }
    
    return nil
}
```

## Error Handling Patterns

### Invalid State Transitions

```go
func handleInvalidTransitions() {
    tx := &frozendb.Transaction{}
    
    // Try to commit without beginning - should fail
    err := tx.Commit()
    if err != nil {
        fmt.Printf("Expected error: %v\n", err)
        // Output: Commit() can only be called when transaction is active
    }
    
    // Begin the transaction
    if err := tx.Begin(); err != nil {
        log.Fatal(err)
    }
    
    // Try to begin again - should fail
    err = tx.Begin()
    if err != nil {
        fmt.Printf("Expected error: %v\n", err)
        // Output: Begin() cannot be called when transaction is already active
    }
}
```

### Comprehensive Error Handling

```go
func safeTransactionWorkflow() error {
    tx := &frozendb.Transaction{}
    
    // Begin with error handling
    if err := tx.Begin(); err != nil {
        switch e := err.(type) {
        case *frozendb.InvalidActionError:
            fmt.Printf("Invalid action: %s - %s\n", e.Action, e.Reason)
        default:
            return fmt.Errorf("unexpected error during begin: %w", err)
        }
        return err
    }
    
    // Commit with error handling
    if err := tx.Commit(); err != nil {
        switch e := err.(type) {
        case *frozendb.InvalidActionError:
            fmt.Printf("Invalid action: %s - %s\n", e.Action, e.Reason)
        default:
            return fmt.Errorf("unexpected error during commit: %w", err)
        }
        return err
    }
    
    return nil
}
```

## Integration with Existing Operations

### Transaction Lifecycle Management

```go
func transactionLifecycleExample() {
    tx := &frozendb.Transaction{Header: header}
    
    // Check if transaction is committed (initially false)
    fmt.Printf("Is committed: %v\n", tx.IsCommitted()) // false
    
    // Begin transaction
    if err := tx.Begin(); err != nil {
        log.Fatal(err)
    }
    
    // Check if transaction is committed (still false - not yet committed)
    fmt.Printf("Is committed: %v\n", tx.IsCommitted()) // false
    
    // Commit transaction
    if err := tx.Commit(); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Is committed: %v\n", tx.IsCommitted()) // true
    fmt.Printf("Empty row exists: %v\n", tx.GetEmptyRow() != nil) // true
}
```

### Working with Existing Methods

```go
func integrationWithExistingMethods() error {
    tx := &frozendb.Transaction{}
    
    // Execute empty transaction
    if err := tx.Begin(); err != nil {
        return err
    }
    if err := tx.Commit(); err != nil {
        return err
    }
    
    // Use existing methods to inspect results
    rows := tx.GetRows()
    fmt.Printf("Transaction has %d rows\n", len(rows))
    
    // Get committed rows iterator
    iterator, err := tx.GetCommittedRows()
    if err != nil {
        return err
    }
    
    // Iterate through committed rows
    for {
        row, hasMore := iterator()
        if !hasMore {
            break
        }
        fmt.Printf("Row type: %T\n", row)
    }
    
    return nil
}
```

## Testing Your Implementation

### Unit Test Example

```go
func TestEmptyTransactionWorkflow(t *testing.T) {
    tests := []struct {
        name    string
        testFn  func(*testing.T, *frozendb.Transaction)
    }{
        {
            name: "successful_empty_transaction",
            testFn: func(t *testing.T, tx *frozendb.Transaction) {
                // Begin
                if err := tx.Begin(); err != nil {
                    t.Fatalf("Begin() failed: %v", err)
                }
                
                // Commit
                if err := tx.Commit(); err != nil {
                    t.Fatalf("Commit() failed: %v", err)
                }
                
                // Verify result
                rows := tx.GetRows()
                if len(rows) != 1 {
                    t.Errorf("expected 1 row, got %d", len(rows))
                }
            },
        },
        {
            name: "begin_twice_should_fail",
            testFn: func(t *testing.T, tx *frozendb.Transaction) {
                if err := tx.Begin(); err != nil {
                    t.Fatalf("First Begin() failed: %v", err)
                }
                
                if err := tx.Begin(); err == nil {
                    t.Error("Second Begin() should have failed")
                }
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tx := &frozendb.Transaction{}
            tt.testFn(t, tx)
        })
    }
}
```

## Performance Considerations

### Benchmark Example

```go
func BenchmarkEmptyTransactionWorkflow(b *testing.B) {
    for i := 0; i < b.N; i++ {
        tx := &frozendb.Transaction{}
        
        if err := tx.Begin(); err != nil {
            b.Fatal(err)
        }
        
        if err := tx.Commit(); err != nil {
            b.Fatal(err)
        }
    }
}
```

### Memory Usage Monitoring

```go
func monitorMemoryUsage() {
    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)
    
    // Execute many empty transactions
    for i := 0; i < 10000; i++ {
        tx := &frozendb.Transaction{}
        tx.Begin()
        tx.Commit()
    }
    
    runtime.GC()
    runtime.ReadMemStats(&m2)
    
    fmt.Printf("Memory used: %d bytes\n", m2.Alloc-m1.Alloc)
    // Should be minimal due to fixed memory usage
}
```

## Troubleshooting

### Common Issues

1. **"transaction is already active"** - Call `Begin()` only once per transaction lifecycle
2. **"no partial data row"** - Ensure `Begin()` was called successfully before `Commit()`
3. **"wrong partial state"** - Don't modify `PartialDataRow` between `Begin()` and `Commit()`
4. **Data races** - Ensure proper synchronization in concurrent environments

### Debug Mode

```go
func debugTransactionState(tx *frozendb.Transaction) {
    fmt.Printf("Rows: %d\n", len(tx.GetRows()))
    fmt.Printf("Empty row: %v\n", tx.GetEmptyRow() != nil)
    fmt.Printf("Is committed: %v\n", tx.IsCommitted())
}
```

## Next Steps

- Review the full API contract in `contracts/transaction_api.md`
- Examine the detailed data model in `data-model.md`
- Check spec tests for complete functional requirements
- Integrate with your existing frozenDB application code