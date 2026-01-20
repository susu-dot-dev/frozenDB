# Quick Start Guide: AddRow Transaction Implementation

**Feature**: 012-add-transaction-row  
**Date**: 2026-01-19  
**Repository**: github.com/susu-dot-dev/frozenDB

## Overview

Basic pattern for adding key-value pairs to frozenDB transactions with UUID timestamp ordering validation.

## Example 1: Basic Transaction Pattern

```go
package main

import (
    "log"
    
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    db, err := frozendb.Open("example.fdb")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    tx, err := db.BeginTransaction()
    if err != nil {
        log.Fatal(err)
    }
    
    key := uuid.Must(uuid.NewV7())
    err = tx.AddRow(key, `{"user": "alice", "action": "login"}`)
    if err != nil {
        log.Fatal(err)
    }
    
    err = tx.Commit()
    if err != nil {
        log.Fatal(err)
    }
}
```

## Example 2: Multiple Rows in Transaction

```go
tx, err := db.BeginTransaction()
if err != nil {
    return err
}

for i := 0; i < 5; i++ {
    key, _ := uuid.NewV7()
    value := fmt.Sprintf(`{"record": %d}`, i)
    
    err = tx.AddRow(key, value)
    if err != nil {
        return err
    }
}

return tx.Commit()
```

## Example 3: Error Handling

```go
err := tx.AddRow(key, value)
if err != nil {
    switch e := err.(type) {
    case *frozendb.KeyOrderingError:
        fmt.Printf("Timestamp ordering violation: %v\n", e)
    case *frozendb.InvalidActionError:
        fmt.Printf("Invalid action: %s\n", e.GetMessage())
    case *frozendb.InvalidInputError:
        fmt.Printf("Invalid input: %s\n", e.GetMessage())
    default:
        fmt.Printf("Error: %v\n", err)
    }
}
```
