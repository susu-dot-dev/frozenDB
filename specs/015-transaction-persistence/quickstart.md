# Quickstart: Transaction File Persistence

**Feature**: 015-transaction-persistence **Date**: 2026-01-21

## Basic Usage Pattern

### Example 1: Simple Transaction with Persistence

```go
package main

import (
    "log"

    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Open existing database file
    fm, err := frozendb.NewFileManager("data.fdb")
    if err != nil {
        log.Fatal(err)
    }
    defer fm.Close()

    // Create write channel and set up FileManager
    dataChan := make(chan frozendb.Data, 10)
    if err := fm.SetWriter(dataChan); err != nil {
        log.Fatal(err)
    }

    // Get header from database (already loaded)
    header := frozendb.Header{} // Loaded from file in real usage

    // Create transaction with write channel
    tx := &frozendb.Transaction{
        Header:    &header,
        writeChan: dataChan,
    }

    // Begin transaction - writes PartialDataRow to disk
    if err := tx.Begin(); err != nil {
        log.Fatal(err)
    }

    // Add row - writes previous partial and new partial to disk
    if err := tx.AddRow(uuid.New(), `{"name": "Alice"}`); err != nil {
        log.Fatal(err)
    }

    // Commit transaction - writes final DataRow to disk
    if err := tx.Commit(); err != nil {
        log.Fatal(err)
    }

    log.Println("Transaction committed successfully")
}
```

### Example 2: Empty Transaction (No Data Rows)

```go
// Empty transaction - Begin() followed immediately by Commit()
tx := &frozendb.Transaction{
    Header:    &header,
    writeChan: dataChan,
}

// Begin writes PartialDataRow to disk
if err := tx.Begin(); err != nil {
    log.Fatal(err)
}

// Commit writes NullRow to disk (no data rows added)
if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

## Key Points

- **Write Channel**: Always set `writeChan` field before calling Begin(),
  AddRow(), or Commit(), this can be enforced by Transaction.Validate()
- **FileManager Setup**: Caller creates write channel and sets up FileManager
  with `SetWriter()`
- **Synchronous Writes**: All operations wait for write completion before
  returning
- **Error Handling**: Check errors on every operation - write failures tombstone
  the transaction and subsequent calls return TombstonedError
  errors without modifying state
- **Append-Only**: All writes only append new bytes to the file

## Testing with Mock Write Channel

```go
// Custom channel handler for testing
dataChan := make(chan frozendb.Data, 10)
writtenRows := [][]byte{}

go func() {
    for data := range dataChan {
        writtenRows = append(writtenRows, data.Bytes)
        data.Response <- nil // Simulate success
    }
}()

tx := &frozendb.Transaction{
    Header:    &header,
    writeChan: dataChan,
}

// Test operations without real file system
tx.Begin()
tx.AddRow(uuid.New(), `{"test": "data"}`)
tx.Commit()

// Verify written rows
log.Printf("Wrote %d rows", len(writtenRows))
```
