# Quickstart: Transaction Struct

## Overview

The Transaction struct provides a high-level abstraction for working with frozenDB transactions as logical units. It wraps collections of DataRow objects and provides methods to access committed data, manage savepoints, and validate transaction integrity.

## Basic Usage

### Creating a Transaction

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/susu-dot-dev/frozenDB/frozendb"
    "github.com/google/uuid"
)

func main() {
    // Create some DataRows (normally loaded from file)
    rows := createSampleTransaction()
    
    // Create Transaction struct and validate
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        log.Fatalf("Failed to validate transaction: %v", err)
    }
    
    fmt.Printf("Transaction committed: %v\n", tx.IsCommitted())
}
```

### Accessing Committed Data

```go
func analyzeTransaction(tx *frozendb.Transaction) {
    // Get iterator for committed rows
    iter, err := tx.GetCommittedRows()
    if err != nil {
        log.Printf("Error getting committed rows: %v", err)
        return
    }
    
    count := 0
    for row, more := iter(); more; row, more = iter() {
        fmt.Printf("Row %d: Key=%s, Value=%s\n", 
            count, row.GetKey().String(), row.GetValue())
        count++
    }
    fmt.Printf("Committed rows: %d\n", count)
}
```

### Checking Individual Row Status

```go
func checkRowStatus(tx *frozendb.Transaction) {
    for i := 0; i < 10; i++ { // Check first 10 rows max
        committed, err := tx.IsRowCommitted(i)
        if err != nil {
            if i >= 10 { // Expected error for out of bounds
                break
            }
            log.Printf("Error checking row %d: %v", i, err)
            continue
        }
        
        status := "invalidated"
        if committed {
            status = "committed"
        }
        fmt.Printf("Row %d: %s\n", i, status)
    }
}
```

### Working with Savepoints

```go
func analyzeSavepoints(tx *frozendb.Transaction) {
    savepointIndices := tx.GetSavepointIndices()
    fmt.Printf("Savepoints found at indices: %v\n", savepointIndices)
    
    if len(savepointIndices) > 0 {
        fmt.Printf("First savepoint at index %d\n", savepointIndices[0])
        
        // Check if row at first savepoint is committed
        committed, err := tx.IsRowCommitted(savepointIndices[0])
        if err != nil {
            log.Printf("Error checking savepoint row: %v", err)
        } else {
            fmt.Printf("Savepoint row committed: %v\n", committed)
        }
    }
}
```

## Transaction Scenarios

### Scenario 1: Clean Commit

```go
func createCleanCommitTransaction() []frozendb.DataRow {
    header := &frozendb.Header{} // Assume properly configured header
    
    // Row 1: Transaction start + continue
    row1 := &frozendb.DataRow{}
    row1.Header = header
    row1.StartControl = frozendb.START_TRANSACTION // 'T'
    row1.EndControl = frozendb.ROW_END_CONTROL    // 'RE'
    row1.RowPayload = &frozendb.DataRowPayload{
        Key:   uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8b"),
        Value: `{"data": "first"}`,
    }
    
    // Row 2: Transaction continuation + commit
    row2 := &frozendb.DataRow{}
    row2.Header = header
    row2.StartControl = frozendb.ROW_CONTINUE     // 'R'
    row2.EndControl = frozendb.TRANSACTION_COMMIT // 'TC'
    row2.RowPayload = &frozendb.DataRowPayload{
        Key:   uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8c"),
        Value: `{"data": "second"}`,
    }
    
    return []frozendb.DataRow{*row1, *row2}
}
```

### Scenario 2: Partial Rollback

```go
func createPartialRollbackTransaction() []frozendb.DataRow {
    header := &frozendb.Header{}
    
    // Row 1: Transaction start + savepoint + continue
    row1 := &frozendb.DataRow{}
    row1.Header = header
    row1.StartControl = frozendb.START_TRANSACTION
    row1.EndControl = frozendb.SAVEPOINT_CONTINUE // 'SE' (savepoint 1)
    row1.RowPayload = &frozendb.DataRowPayload{
        Key:   uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8b"),
        Value: `{"data": "saved"}`,
    }
    
    // Row 2: Transaction continuation
    row2 := &frozendb.DataRow{}
    row2.Header = header
    row2.StartControl = frozendb.ROW_CONTINUE
    row2.EndControl = frozendb.ROW_END_CONTROL // 'RE'
    row2.RowPayload = &frozendb.DataRowPayload{
        Key:   uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8c"),
        Value: `{"data": "will_be_rolled_back"}`,
    }
    
    // Row 3: Transaction continuation + rollback to savepoint 1
    row3 := &frozendb.DataRow{}
    row3.Header = header
    row3.StartControl = frozendb.ROW_CONTINUE
    row3.EndControl[0] = 'R' // R1 - rollback to savepoint 1
    row3.EndControl[1] = '1'
    row3.RowPayload = &frozendb.DataRowPayload{
        Key:   uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8d"),
        Value: `{"data": "rollback_command"}`,
    }
    
    return []frozendb.DataRow{*row1, *row2, *row3}
}
```

## Error Handling

### Validation Errors

```go
func handleValidationErrors() {
    // Create invalid transaction (starts with R instead of T)
    rows := createInvalidTransaction()
    
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        // Handle specific error types
        switch err := err.(type) {
        case *frozendb.CorruptDatabaseError:
            fmt.Printf("Database corruption detected: %s\n", err.Message)
        case *frozendb.InvalidInputError:
            fmt.Printf("Invalid instruction/logic error: %s\n", err.Message)
        default:
            fmt.Printf("Generic error: %v\n", err)
        }
        return
    }
    
    // Transaction is valid, proceed with operations
    processValidTransaction(tx)
}
```

### Runtime Errors

```go
func handleRuntimeErrors(tx *frozendb.Transaction) {
    // Check out-of-bounds access
    committed, err := tx.IsRowCommitted(999)
    if err != nil {
        fmt.Printf("Error checking row 999: %v\n", err)
    } else {
        fmt.Printf("Row 999 committed: %v\n", committed)
    }
    
    // Get committed rows iterator (handles invalid state)
    iter, err := tx.GetCommittedRows()
    if err != nil {
        fmt.Printf("Error getting committed rows: %v\n", err)
    } else {
        count := 0
        for _, more := iter(); more; _, more = iter() {
            count++
        }
        fmt.Printf("Got %d committed rows\n", count)
    }
}
```

## Best Practices

### 1. Always Validate Transactions

```go
func safeTransactionCreation(rows []frozendb.DataRow) (*frozendb.Transaction, error) {
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        return nil, fmt.Errorf("transaction validation failed: %w", err)
    }
    
    return tx, nil
}
```

### 2. Handle Concurrent Access

```go
func processTransactionConcurrently(tx *frozendb.Transaction) {
    // Safe for concurrent read access
    var wg sync.WaitGroup
    
    // Multiple goroutines can read from the same Transaction
    for i := 0; i < 4; i++ {
        wg.Add(1)
        go func(goroutineID int) {
            defer wg.Done()
            
            committed, _ := tx.IsRowCommitted(0)
            fmt.Printf("Goroutine %d: Row 0 committed = %v\n", goroutineID, committed)
        }(i)
    }
    
    wg.Wait()
}
```

### 3. Memory Management

```go
func efficientTransactionProcessing(rows []frozendb.DataRow) {
    // Transaction holds references, not copies
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        return
    }
    
    // Process committed data efficiently using iterator
    iter, _ := tx.GetCommittedRows()
    for row, more := iter(); more; row, more = iter() {
        // Process row without copying
        processDataRow(row)
    }
    
    // No need to explicitly clean up - garbage collector handles it
}
```

## Integration Tips

### Loading DataRows from Files

```go
func loadTransactionFromFile(filePath string) (*frozendb.Transaction, error) {
    // Use existing frozenDB file reading functionality
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    // Parse file to get DataRows (implementation depends on your file reader)
    rows, err := parseDataRowsFromFile(file)
    if err != nil {
        return nil, err
    }
    
    // Create and validate transaction
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        return nil, err
    }
    return tx, nil
}
```

### Working with Databases

```go
func extractTransactionFromDB(db *frozendb.DB, startIndex int) (*frozendb.Transaction, error) {
    // Extract transaction rows from database (implementation depends on DB API)
    rows, err := db.GetTransactionRows(startIndex)
    if err != nil {
        return nil, err
    }
    
    tx := &frozendb.Transaction{Rows: rows}
    if err := tx.Validate(); err != nil {
        return nil, err
    }
    return tx, nil
}
```