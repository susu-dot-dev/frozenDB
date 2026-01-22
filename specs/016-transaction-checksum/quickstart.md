# Quickstart: Transaction Checksum Row Insertion

## Overview

This quickstart demonstrates how frozenDB automatically inserts checksum rows at 10,000-row intervals. The system calculates row count from file size and inserts checksum rows transparently - whether at transaction boundaries or in the middle of a transaction.

## Example 1: Basic Transaction with Automatic Checksum

```go
package main

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/anilcode/frozendb/frozendb"
)

func main() {
    // Create new database
    err := frozendb.Create(&frozendb.CreateConfig{
        Path:    "/path/to/database.fdb",
        RowSize: 128,
        SkewMs:  0,
    })
    if err != nil {
        panic(err)
    }

    // Open database for writing
    db, err := frozendb.NewFrozenDB("/path/to/database.fdb", frozendb.MODE_WRITE)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Get header for transaction creation
    // Note: FrozenDB.header is private; NewTransaction will need access to header
    // Implementation detail: either add GetHeader() method to FrozenDB or pass header differently
    header := &frozendb.Header{} // Placeholder - actual implementation will need real header

    // Create a new transaction (NewTransaction is new API for this feature)
    // Takes DBFile interface, header reference, and write channel
    tx, err := frozendb.NewTransaction(db, header, nil)
    if err != nil {
        panic(err)
    }

    // Begin transaction
    err = tx.Begin()
    if err != nil {
        panic(err)
    }

    // Add rows to transaction
    // If this makes row count reach 10,000, checksum row is inserted immediately
    for i := 0; i < 10; i++ {
        key, _ := uuid.NewV7()
        err := tx.AddRow(key, fmt.Sprintf(`{"value": %d}`, i))
        if err != nil {
            panic(err)
        }
    }

    // Commit transaction
    err = tx.Commit()
    if err != nil {
        panic(err)
    }

    fmt.Println("Transaction committed successfully")
}
```

**Key Points:**
- Checksum rows are inserted automatically - no explicit API needed
- System calculates row count from file size after each row write
- Checksum insertion happens immediately after 10,000th row is written

---

## Example 2: Transaction Crossing Checksum Boundary

```go
package main

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/anilcode/frozendb/frozendb"
)

func main() {
    // Open database with 9,995 existing complete rows
    db, err := frozendb.NewFrozenDB("/path/to/database.fdb", frozendb.MODE_WRITE)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    header := db.header

    // Begin new transaction (current row count: 9,995)
    tx, err := frozendb.NewTransaction(db, header, nil)
    if err != nil {
        panic(err)
    }

    err = tx.Begin()
    if err != nil {
        panic(err)
    }

    // Add 10 rows to transaction
    // Row 5 of this transaction will be the 10,000th complete row overall
    // Checksum row is inserted immediately after row 5 is written
    for i := 0; i < 10; i++ {
        key, _ := uuid.NewV7()
        value := fmt.Sprintf(`{"data": "row-%d"}`, 9_996+i)
        err := tx.AddRow(key, value)
        if err != nil {
            panic(err)
        }
        fmt.Printf("Added row %d\n", 9_996+i)
    }

    // Commit transaction
    // All 10 rows are committed, checksum row already inserted after row 5
    err = tx.Commit()
    if err != nil {
        panic(err)
    }

    fmt.Println("All 10 rows committed with checksum row inserted after row 10,000")
}
```

**Key Points:**
- Checksum row is inserted in the middle of this transaction (after row 5)
- Transaction logic continues normally after checksum row insertion
- All 10 rows are part of the same transaction

---

## Example 3: Multiple Checksums Across Transactions

```go
package main

import (
    "fmt"
    "github.com/google/uuid"
    "github.com/anilcode/frozendb/frozendb"
)

func main() {
    // Create new database
    err := frozendb.Create(&frozendb.CreateConfig{
        Path:    "/path/to/new.fdb",
        RowSize: 128,
        SkewMs:  0,
    })
    if err != nil {
        panic(err)
    }

    // Open database for writing
    db, err := frozendb.NewFrozenDB("/path/to/new.fdb", frozendb.MODE_WRITE)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    header := db.header

    // Insert 30,000 rows across multiple transactions
    // This will trigger 3 checksum insertions at 10,000, 20,000, and 30,000
    rowsPerTx := 100 // Max rows per transaction
    totalRows := 30000

    for row := 0; row < totalRows; row += rowsPerTx {
        tx, err := frozendb.NewTransaction(db, header, nil)
        if err != nil {
            panic(err)
        }

        err = tx.Begin()
        if err != nil {
            panic(err)
        }

        // Add up to 100 rows per transaction
        for i := 0; i < rowsPerTx && (row+i) < totalRows; i++ {
            key, _ := uuid.NewV7()
            value := fmt.Sprintf(`{"row": %d}`, row+i)
            err := tx.AddRow(key, value)
            if err != nil {
                panic(err)
            }
        }

        err = tx.Commit()
        if err != nil {
            panic(err)
        }

        // Check if checksum was just inserted (after row write completed)
        currentRow := row + rowsPerTx
        if currentRow > 0 && currentRow%10000 == 0 {
            fmt.Printf("Checkpoint reached: %d rows written (checksum row inserted)\n", currentRow)
        }
    }

    fmt.Printf("Successfully inserted %d rows with 3 checksum rows\n", totalRows)
}
```

**Key Points:**
- Multiple checksum rows automatically inserted at 10,000-row intervals
- Checksum rows inserted immediately after each 10,000th row write
- System maintains data integrity automatically
