# Quickstart: frozenDB Finder Interface

This guide demonstrates how to use the frozenDB Finder interface and SimpleFinder implementation for locating rows and determining transaction boundaries.

## Creating a Finder

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Open database file
    dbFile, err := frozendb.NewDBFile("example.fdb", "r")
    if err != nil {
        log.Fatal(err)
    }
    defer dbFile.Close()
    
    // Create SimpleFinder instance
    finder := frozendb.NewSimpleFinder(dbFile)
    
    // Use finder operations...
}
```

## Finding a Row by UUID Key

```go
// UUID key to search for (must be UUIDv7)
targetUUID := uuid.MustParse("018f1c7c-5b6a-7f8e-9d0a-1b2c3d4e5f6f")

// Find the row index containing this UUID
index, err := finder.GetIndex(targetUUID)
if err != nil {
    log.Printf("Error finding UUID: %v", err)
    return
}

fmt.Printf("Found UUID %s at index %d\n", targetUUID, index)
```

## Finding Transaction Boundaries

```go
// Given a row index, find its transaction boundaries
startIndex, err := finder.GetTransactionStart(index)
if err != nil {
    log.Printf("Error finding transaction start: %v", err)
    return
}

endIndex, err := finder.GetTransactionEnd(index)
if err != nil {
    log.Printf("Error finding transaction end: %v", err)
    return
}

fmt.Printf("Transaction spans indices %d to %d\n", startIndex, endIndex)

// Check if this is a single-row transaction
if startIndex == endIndex {
    fmt.Println("Single-row transaction")
} else {
    fmt.Printf("Multi-row transaction with %d rows\n", endIndex-startIndex+1)
}
```

## Handling Row Addition Notifications

```go
// This is typically handled automatically by Transaction
// Here's how you'd handle it manually if needed:

// When a new row is added at index 123
newRowIndex := int64(123)
newRow := &frozendb.RowUnion{
    // Row data would be populated by the transaction system
}

err = finder.OnRowAdded(newRowIndex, newRow)
if err != nil {
    log.Printf("Error handling row addition: %v", err)
    return
}

fmt.Printf("Successfully processed row addition at index %d\n", newRowIndex)
```

## Error Handling Patterns

```go
index, err := finder.GetIndex(targetUUID)
if err != nil {
    fmt.Printf("Error finding UUID: %v", err)
    return
}
```

## Complete Example: Transaction Analysis

```go
func analyzeTransaction(finder frozendb.Finder, rowUUID uuid.UUID) {
    // Find the row
    index, err := finder.GetIndex(rowUUID)
    if err != nil {
        fmt.Printf("Could not find UUID %s: %v\n", rowUUID, err)
        return
    }
    
    // Get transaction boundaries
    start, err := finder.GetTransactionStart(index)
    if err != nil {
        fmt.Printf("Error getting transaction start: %v\n", err)
        return
    }
    
    end, err := finder.GetTransactionEnd(index)
    if err != nil {
        fmt.Printf("Error getting transaction end: %v\n", err)
        return
    }
    
    // Analyze the transaction
    fmt.Printf("Transaction Analysis for UUID %s:\n", rowUUID)
    fmt.Printf("  Row index: %d\n", index)
    fmt.Printf("  Transaction span: %d to %d\n", start, end)
    fmt.Printf("  Transaction size: %d rows\n", end-start+1)
    
    if start == end {
        fmt.Println("  Type: Single-row transaction")
    } else {
        fmt.Printf("  Type: Multi-row transaction (%d data rows)\n", end-start+1)
    }
}
```

## Usage Notes

- **Index 0** always points to the first checksum row after the 64-byte header
- **UUID keys** must be valid UUIDv7 values; uuid.Nil is not searchable
- **Transaction boundaries** exclude checksum rows from the returned indices
- **Thread Safety**: Finder methods can be called concurrently from multiple goroutines
- **Performance**: SimpleFinder uses linear scanning, suitable for correctness validation and small databases

## Integration with Transactions

In typical usage, the Finder integrates with frozenDB's Transaction system, which automatically handles OnRowAdded() notifications during write operations:

```go
// Transaction usage (simplified)
tx, err := db.BeginTransaction()
if err != nil {
    log.Fatal(err)
}

// Transaction automatically notifies finder via OnRowAdded()
// when rows are successfully written
err = tx.AddRow(dataRow)
if err != nil {
    log.Fatal(err)
}

err = tx.Commit()
if err != nil {
    log.Fatal(err)
}

// Finder can now locate the newly added row
index, err := finder.GetIndex(dataRow.ID())
```
