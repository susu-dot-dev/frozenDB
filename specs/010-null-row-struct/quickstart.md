# NullRow Implementation Quickstart

**Feature**: 010-null-row-struct  
**Date**: 2025-01-18  
**Target Audience**: frozenDB developers

## Overview

NullRow provides a complete implementation for null operation rows in frozenDB. This quickstart guide covers the essential usage patterns for creating, validating, marshaling, and unmarshaling NullRows.

## Basic Usage

### Creating a NullRow

```go
package main

import (
    "fmt"
    "log"
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create database header (typically from existing file)
    header := &frozendb.Header{
        // Header fields populated from file or defaults
    }
    
    // Create NullRow directly following existing patterns
    nullRow := &frozendb.NullRow{
        baseRow: frozendb.baseRow[*frozendb.NullRowPayload]{
            Header:       header,
            StartControl: frozendb.START_TRANSACTION,
            EndControl:   frozendb.EndControl{'N', 'R'},
            RowPayload:   &frozendb.NullRowPayload{Key: uuid.Nil},
        },
    }
    
    fmt.Printf("Created NullRow with baseRow foundation\n")
}
```

### Validating a NullRow

```go
// Validate the NullRow meets specification requirements
err := nullRow.Validate()
if err != nil {
    // Handle InvalidInputError for validation failures
    log.Printf("Validation failed: %v", err)
    return
}

fmt.Println("NullRow validation passed")
```

### Marshaling to Binary

```go
// Convert NullRow to binary format using baseRow foundation
data, err := nullRow.MarshalText()
if err != nil {
    // Handle InvalidInputError for marshaling failures
    log.Printf("Marshaling failed: %v", err)
    return
}

fmt.Printf("Marshaled to %d bytes using baseRow\n", len(data))
```

### Unmarshaling from Binary

```go
// Create new NullRow instance and unmarshal from binary data
nullRow2 := NullRow{}
err := nullRow2.UnmarshalText(&data)
if err != nil {
    // Handle CorruptDatabaseError for unmarshaling failures
    log.Printf("Unmarshaling failed: %v", err)
    return
}

fmt.Printf("Unmarshaled NullRow using baseRow foundation\n")
```

## Complete Round-trip Example

```go
package main

import (
    "fmt"
    "log"
    "github.com/google/uuid"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // 1. Create NullRow
    original := &frozendb.NullRow{
        baseRow: frozendb.baseRow[*frozendb.NullRowPayload]{
            Header:       header,
            StartControl: frozendb.START_TRANSACTION,
            EndControl:   frozendb.EndControl{'N', 'R'},
            RowPayload:   &frozendb.NullRowPayload{Key: uuid.Nil},
        },
    }
    
    // 2. Validate
    if err := original.Validate(); err != nil {
        log.Fatalf("Validation failed: %v", err)
    }
    
    // 3. Marshal
    data, err := original.MarshalText()
    if err != nil {
        log.Fatalf("Marshaling failed: %v", err)
    }
    
    // 4. Unmarshal
    restored := &frozendb.NullRow{}
    err = restored.UnmarshalText(data)
    if err != nil {
        log.Fatalf("Unmarshaling failed: %v", err)
    }
    
    // 5. Verify round-trip
    if err := restored.Validate(); err != nil {
        log.Fatalf("Restored validation failed: %v", err)
    }
    
    fmt.Println("NullRow round-trip successful!")
}
```

## Error Handling Patterns

### Validation Errors

```go
if err := nullRow.Validate(); err != nil {
    var invalidInput *frozendb.InvalidInputError
    if errors.As(err, &invalidInput) {
        fmt.Printf("Invalid input: %s\n", invalidInput.Message)
        // Handle specific validation failure
    }
}
```

### Corruption Errors

```go
nullRow, err := frozendb.UnmarshalNullRow(data)
if err != nil {
    var corruptDb *frozendb.CorruptDatabaseError
    if errors.As(err, &corruptDb) {
        fmt.Printf("Database corruption detected: %s\n", corruptDb.Message)
        // Handle corruption scenario
    }
}
```

## Testing with Spec Tests

Run the comprehensive spec test suite:

```bash
cd frozendb
go test -v -run Test_S_010
```

This will run all spec tests validating functional requirements FR-001 through FR-013.

## Integration Notes

- NullRows are single-row transactions by definition
- They count toward checksum intervals but don't appear in query results
- Use uuid.Nil for UUID field - cannot use regular UUIDv7 keys
- Position rules apply - only appear where new transactions are allowed

## Performance Tips

- NullRow operations are designed to be <1ms
- Memory usage is fixed and scales with row_size
- No caching needed for simple operations
- Validation is optimized for known field values

## File Format Reference

The binary format follows frozenDB v1 specification:
- ROW_START: 0x1F
- Start Control: 'T'
- UUID Base64: "AAAAAAAAAAAAAAAAAAAAAA=="
- End Control: 'NR'
- ROW_END: 0x0A
- Parity: LRC calculated over all bytes except parity bytes

For complete specification details, see `docs/v1_file_format.md`.
