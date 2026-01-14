# Header Refactor Quickstart Guide

## Overview

This guide demonstrates the updated Header creation patterns in frozenDB after the header refactor. The refactor eliminates the dual creation pattern and provides a single, consistent API for Header management.

## Before vs After

### Before: Dual Creation Pattern

```go
// Old way - dual creation
rowSize := 1024
skewMs := 5000

// 1. Generate bytes separately
headerBytes, err := generateHeader(rowSize, skewMs)
if err != nil {
    return err
}

// 2. Create Header struct separately
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1,
    rowSize:   rowSize,
    skewMs:    skewMs,
}
```

### After: Single Creation Pattern

```go
// New way - single creation
rowSize := 1024
skewMs := 5000

// 1. Create Header struct directly
header := &frozendb.Header{
    Signature: "fDB",
    Version:   1,
    RowSize:   rowSize,
    SkewMs:    skewMs,
}

// 2. Get bytes when needed (validates automatically)
headerBytes, err := header.MarshalText()
if err != nil {
    return err
}
```

## Basic Usage

### Creating a Header

```go
package main

import (
    "fmt"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
// Create Header with direct initialization
header := &frozendb.Header{
    Signature: "fDB",
    Version:   1,
    RowSize:   1024,
    SkewMs:    5000,
}
    
    // Optional: Validate explicitly (MarshalText will validate)
    if err := header.Validate(); err != nil {
        panic(err)
    }
    
    fmt.Printf("Header created successfully: rowSize=%d, skewMs=%d\n", 
        header.GetRowSize(), header.GetSkewMs())
}
```

### Serializing Header to Bytes

```go
// Get 64-byte header representation
headerBytes, err := header.MarshalText()
if err != nil {
    return err
}

fmt.Printf("Header bytes length: %d (should be 64)\n", len(headerBytes))
```

### Parsing Header from Bytes

```go
// Simulate reading header from file
headerBytes := []byte(`{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":5000}` + 
    strings.Repeat("\x00", 20) + "\n")

// Parse into Header struct (includes validation)
header := &frozendb.Header{}
if err := header.UnmarshalText(headerBytes); err != nil {
    return err
}

fmt.Printf("Parsed header: rowSize=%d, skewMs=%d\n", 
    header.GetRowSize(), header.GetSkewMs())
```

## Database Creation Example

```go
package main

import (
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func createDatabase() error {
    // Configuration for new database
    config := frozendb.CreateConfig{
        path:    "/tmp/example.fdb",
        rowSize: 1024,
        skewMs:  5000,
    }
    
    // Create database (uses new Header pattern internally)
    return frozendb.Create(config)
}
```

## File Organization

### New File Structure

```
frozendb/
├── header.go              # NEW: All Header functionality
├── create.go              # MODIFIED: Header creation removed
├── data_row.go            # Existing: DataRow functionality
├── checksum.go            # Existing: ChecksumRow functionality
├── transaction.go         # Existing: Transaction functionality
└── errors.go              # Existing: Error definitions
```

### What Moved to header.go?

- `Header` struct and all methods
- `headerJSON` helper struct
- Header-related constants (`HEADER_SIZE`, `HEADER_SIGNATURE`, etc.)
- `HEADER_FORMAT` constant
- `generateHeader()` function (for reference during transition)

### What Stayed in create.go?

- `CreateConfig` struct
- `SudoContext` struct
- Filesystem operation interfaces
- `Create()` function (updated to use new pattern)

## Migration Guide

### For Code Using Header Directly

**No changes required** - all existing Header APIs preserved:

```go
// This code continues to work unchanged
header := &frozendb.Header{}
if err := header.UnmarshalText(headerBytes); err != nil {
    return err
}

rowSize := header.GetRowSize()
skewMs := header.GetSkewMs()
```

### For Code Using generateHeader()

**Update required** - replace generateHeader() calls:

```go
// Before
headerBytes, err := generateHeader(rowSize, skewMs)

// After
header := &frozendb.Header{
    Signature: frozendb.HEADER_SIGNATURE,
    Version:   1,
    RowSize:   rowSize,
    SkewMs:    skewMs,
}
headerBytes, err := header.MarshalText()
```

### For Test Code

**Update required** - test files using generateHeader():

```go
// Before
headerBytes, _ := generateHeader(1024, 5000)

// After
header := &frozendb.Header{
    Signature: frozendb.HEADER_SIGNATURE,
    Version:   1,
    RowSize:   1024,
    SkewMs:    5000,
}
headerBytes, _ := header.MarshalText()
```

## Error Handling

### Validation Errors

```go
header := &frozendb.Header{
    Signature: "invalid",  // Wrong signature
    Version:   1,
    RowSize:   1024,
    SkewMs:    5000,
}

if err := header.Validate(); err != nil {
    // Returns frozendb.CorruptDatabaseError
    fmt.Printf("Validation failed: %v\n", err)
}
```

### Marshal Errors

```go
header := &frozendb.Header{
    Signature: "fDB",
    Version:   1,
    RowSize:   100000,  // Invalid: exceeds MAX_ROW_SIZE
    SkewMs:    5000,
}

headerBytes, err := header.MarshalText()
if err != nil {
    // MarshalText calls Validate() automatically
    fmt.Printf("Marshal failed: %v\n", err)
}
```

## Best Practices

### 1. Always Validate or Marshal

```go
header := &frozendb.Header{...}

// Option 1: Explicit validation
if err := header.Validate(); err != nil {
    return err
}

// Option 2: MarshalText validates automatically
headerBytes, err := header.MarshalText()
```

### 2. Use Constants for Values

```go
// Good: Use constants
header := &frozendb.Header{
    Signature: frozendb.HEADER_SIGNATURE,
    Version:   1,  // Will always be 1 for v1 format
    RowSize:   1024,
    SkewMs:    5000,
}

// Avoid: Hardcoded values
header := &frozendb.Header{
    Signature: "fDB",  // Use frozendb.HEADER_SIGNATURE instead
    Version:   1,
    RowSize:   1024,
    SkewMs:    5000,
}
```

### 3. Handle Errors Gracefully

```go
headerBytes, err := header.MarshalText()
if err != nil {
    // Handle specific error types
    switch err.(type) {
    case *frozendb.InvalidInputError:
        fmt.Printf("Invalid header parameters: %v\n", err)
    case *frozendb.CorruptDatabaseError:
        fmt.Printf("Header validation failed: %v\n", err)
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
    return err
}
```

## Testing the Changes

### Running Tests

```bash
# Run all tests
go test ./...

# Run Header-related spec tests
go test -v ./... -run "^Test_S_.*FR_00[0-9]"

# Run specific Header test
go test -v ./frozendb -run Test_S_Header_Refactor_FR_002
```

### Verifying Compatibility

```go
func testHeaderCompatibility() {
    // Test that new pattern produces same output as old pattern
    rowSize, skewMs := 1024, 5000
    
    // New way
    header := &frozendb.Header{
        Signature: frozendb.HEADER_SIGNATURE,
        Version:   1,
        RowSize:   rowSize,
        SkewMs:    skewMs,
    }
    newBytes, _ := header.MarshalText()
    
    // Old way (if generateHeader still available)
    oldBytes, _ := generateHeader(rowSize, skewMs)
    
    // Should be identical
    if !bytes.Equal(newBytes, oldBytes) {
        panic("Header format compatibility broken")
    }
}
```

## Summary

The Header refactor provides:

✅ **Simplified API**: Single Header creation instead of dual pattern  
✅ **Pattern Consistency**: Same constructor/validation/marshaling as DataRow/ChecksumRow  
✅ **Better Organization**: Header functionality in dedicated header.go file  
✅ **Full Compatibility**: All existing APIs preserved unchanged  
✅ **Cleaner Code**: Eliminates generateHeader() function redundancy  

The migration is straightforward and maintains full backward compatibility while providing a cleaner, more consistent API.