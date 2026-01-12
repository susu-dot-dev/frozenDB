# DataRow Quickstart Guide

**Feature**: 005-data-row-handling  
**Date**: 2026-01-11  

## Overview

This guide provides quick examples for using the DataRow implementation in frozenDB. DataRow enables storage of key-value pairs with UUIDv7 keys and JSON string values, following the immutable append-only architecture.

## Basic Usage

### Creating a DataRow

```go
package main

import (
    "fmt"
    "github.com/anilmahadev/frozenDB/frozendb"
    "github.com/google/uuid"
)

func main() {
    // Create a header (or use existing one)
    header := &frozendb.Header{
        RowSize: 512, // Example row size
    }
    
    // Generate a UUIDv7 key
    key, err := uuid.NewV7()
    if err != nil {
        panic(err)
    }
    
    // JSON string value (no syntax validation at this layer)
    value := `{"name":"John Doe","age":30,"city":"New York"}`
    
    // Create DataRow with manual initialization
    dataRow := &frozendb.DataRow{
        Header:       header,
        StartControl: frozendb.TRANSACTION_BEGIN, // 'T' for first row
        EndControl:   frozendb.END_COMMIT,        // 'TC' for single row commit
        RowPayload: &frozendb.DataRowPayload{
            Key:   key,
            Value: value,
        },
    }
    
    // Validate the constructed DataRow
    if err := dataRow.Validate(); err != nil {
        panic(err)
    }
    
    fmt.Printf("DataRow created with key: %s\n", dataRow.GetKey())
}
```

### Serializing a DataRow

```go
// Serialize to bytes
bytes, err := dataRow.MarshalText()
if err != nil {
    panic(err)
}

fmt.Printf("Serialized size: %d bytes\n", len(bytes))
```

### Deserializing a DataRow

```go
// Create empty DataRow for unmarshaling
var emptyRow frozendb.DataRow

// Unmarshal from bytes
err = emptyRow.UnmarshalText(bytes)
if err != nil {
    panic(err)
}

fmt.Printf("Key: %s\n", emptyRow.GetKey())
fmt.Printf("Value: %s\n", emptyRow.GetValue())
```

## Validation Examples

### DataRow Validation

```go
// Validate an existing DataRow
err = dataRow.Validate()
if err != nil {
    panic(fmt.Sprintf("DataRow validation failed: %v", err))
}
```

## Error Handling Patterns

### Handling Invalid UUID

```go
// Try to create with invalid UUID (v4 instead of v7)
invalidUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000") // v4
invalidDataRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   invalidUUID,
        Value: value,
    },
}
err := invalidDataRow.Validate()

if err != nil {
    var invalidErr *frozendb.InvalidInputError
    if errors.As(err, &invalidErr) {
        fmt.Printf("Expected validation error: %s\n", invalidErr.Message)
    }
}
```

### Handling Corrupted Data

```go
// Try to unmarshal corrupted bytes
corruptedBytes := []byte("corrupted data")
var corruptedRow frozendb.DataRow
err = corruptedRow.UnmarshalText(corruptedBytes)

if err != nil {
    var corruptErr *frozendb.CorruptDatabaseError
    if errors.As(err, &corruptErr) {
        fmt.Printf("Data corruption detected: %s\n", corruptErr.Message)
    }
}
```

## Round-Trip Example

```go
func roundTripExample() {
    // Original data
    originalKey, _ := uuid.NewV7()
    originalValue := `{"product":"widget","price":19.99}`
    
    // Create DataRow with manual initialization
    dataRow := &frozendb.DataRow{
        Header:       header,
        StartControl: frozendb.TRANSACTION_BEGIN,
        EndControl:   frozendb.END_COMMIT,
        RowPayload: &frozendb.DataRowPayload{
            Key:   originalKey,
            Value: originalValue,
        },
    }
    if err := dataRow.Validate(); err != nil {
        panic(err)
    }
    
    // Serialize
    serialized, err := dataRow.MarshalText()
    if err != nil {
        panic(err)
    }
    
    // Deserialize
    var restoredRow frozendb.DataRow
    err = restoredRow.UnmarshalText(serialized)
    if err != nil {
        panic(err)
    }
    
    // Verify round-trip
    if restoredRow.GetKey() != originalKey {
        panic("Key mismatch!")
    }
    
    if restoredRow.GetValue() != originalValue {
        panic("Value mismatch!")
    }
    
    fmt.Println("Round-trip successful!")
}
```

## Common Pitfalls

### 1. Using Non-UUIDv7 Keys

```go
// WRONG: Using UUIDv4
v4UUID := uuid.Must(uuid.NewV4())
wrongRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   v4UUID,
        Value: value,
    },
}
err := wrongRow.Validate() // Will fail!

// RIGHT: Using UUIDv7
v7UUID, err := uuid.NewV7() // Correct!
rightRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   v7UUID,
        Value: value,
    },
}
err = rightRow.Validate()
```

### 2. Empty Values

```go
// WRONG: Empty value
emptyRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   key,
        Value: "",
    },
}
err := emptyRow.Validate() // Will fail!

// RIGHT: Non-empty value
validRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   key,
        Value: "{}",
    },
}
err = validRow.Validate()
```

### 3. Nil Header

```go
// WRONG: Nil header
nilHeaderRow := &frozendb.DataRow{
    Header:       nil, // This will fail validation!
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   key,
        Value: value,
    },
}
err := nilHeaderRow.Validate() // Will fail!

// RIGHT: Valid header
header := &frozendb.Header{RowSize: 512}
validRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN,
    EndControl:   frozendb.END_COMMIT,
    RowPayload: &frozendb.DataRowPayload{
        Key:   key,
        Value: value,
    },
}
err = validRow.Validate()
```

## Next Steps

1. **Integration**: Learn how DataRows work within the broader frozenDB database
2. **Performance**: Review performance characteristics and optimization strategies
3. **Advanced Usage**: Explore transaction handling and batch operations
4. **Testing**: Understand spec testing requirements and patterns

## Reference

- **File Format**: See `docs/v1_file_format.md` for complete specification
- **Error Handling**: Review `AGENTS.md` for error handling patterns
- **Testing**: See `docs/spec_testing.md` for spec testing guidelines
