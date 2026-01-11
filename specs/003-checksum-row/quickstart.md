# Checksum Row Implementation Quickstart

This guide provides step-by-step instructions for implementing and using checksum row functionality in frozenDB.

## Overview

The checksum row implementation provides:
- **ChecksumRow**: Specific implementation for integrity checking
- **DataRow**: For key-value data rows with JSON payloads
- **BinaryRow**: For key-value data rows with binary payloads
- **Control Enums**: Type-safe control byte handling
- **Parity Calculation**: LRC checksum for per-row integrity
- **CRC32 Integration**: IEEE standard for block integrity

**Note**: Internal infrastructure (baseRow) is unexported and not directly accessible.

## Quick Start

### 1. Basic ChecksumRow Creation

```go
package main

import (
    "fmt"
    "github.com/susu-dot-dev/frozenDB/frozendb"
)

func main() {
    // Create a header with row size
    header := &frozendb.Header{
        RowSize: 1024,
        SkewMs:  5000,
    }
    
    // Sample data block (single byte array, not slice of rows)
    dataBlock := []byte("sample data block for checksum calculation")
    
    // Create checksum row using constructor
    checksumRow, err := frozendb.NewChecksumRow(header, dataBlock)
    if err != nil {
        panic(err)
    }
    
    // Note: Validation is done automatically within CreateChecksumRow
    
    // Serialize to bytes
    rowBytes, err := checksumRow.MarshalText()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Created checksum row (%d bytes): %x\n", len(rowBytes), rowBytes[:20])
}
```

### 2. Parsing Existing Checksum Row

```go
func parseChecksumRow(rowBytes []byte) error {
    var checksumRow frozendb.ChecksumRow
    
    // Parse from bytes (validation done automatically)
    if err := checksumRow.UnmarshalText(rowBytes); err != nil {
        return err
    }
    
    // Get checksum value (no type assertion needed)
    checksum := checksumRow.GetChecksum() // Returns Checksum directly
    checksumText, err := checksum.MarshalText()
    if err != nil {
        return err
    }
    
    payload := frozendb.NewChecksumPayload()
    crc32, err := payload.Unmarshal(string(checksumText))
    if err != nil {
        return err
    }
    
    fmt.Printf("Parsed checksum: 0x%08X\n", crc32)
    return nil
}
```

## Core Components

### ChecksumRow Usage

```go
// Create checksum row using constructor function
header := &frozendb.Header{RowSize: 1024, SkewMs: 5000}
dataBlock := []byte("sample data for checksum")

checksumRow, err := frozendb.NewChecksumRow(header, dataBlock)
if err != nil {
    panic(err)
}
// Note: Validation is done automatically within NewChecksumRow

// Access checksum via typed method (no type assertion)
checksum := checksumRow.GetChecksum() // Returns Checksum directly
fmt.Printf("CRC32: %08X\n", checksum)

// Serialize to bytes
rowBytes, err := checksumRow.MarshalText()
if err != nil {
    panic(err)
}
fmt.Printf("Row bytes: %x\n", rowBytes[:20])
```

### DataRow Usage

```go
// Create data row using constructor function
header := &frozendb.Header{RowSize: 1024, SkewMs: 5000}
key := uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8b")
value := map[string]interface{}{"name": "test", "value": 42}

dataRow, err := frozendb.NewDataRow(header, key, value)
if err != nil {
    panic(err)
}

// Access data via typed methods
rowKey := dataRow.GetKey() // Returns uuid.UUID
payload := dataRow.GetValue() // Returns JSONPayload

// Serialize to bytes
rowBytes, err := dataRow.MarshalText()
if err != nil {
    panic(err)
}
```

### Checksum Operations

```go
// Create checksum value
var checksum frozendb.Checksum = 0xEDB88320

// Marshal CRC32 to Base64 via MarshalText
checksumText, err := checksum.MarshalText()
if err != nil {
    return err
}
fmt.Printf("CRC32 %08X → %s\n", checksum, string(checksumText))

// Unmarshal Base64 back to CRC32 via UnmarshalText
if err := checksum.UnmarshalText([]byte("7bCHIA==")); err != nil {
    return err
}
fmt.Printf("Base64 CRC32 %08X\n", checksum)
```

### Spec Testing Pattern

```go
func Test_S_FR_001_ChecksumRowFormat(t *testing.T) {
    // Test exact byte format compliance per v1_file_format.md
    checksumRow := createTestChecksumRow()
    
    rowBytes, err := checksumRow.MarshalText()
    if err != nil {
        t.Fatalf("Failed to marshal: %v", err)
    }
    
    // Verify row structure
    if rowBytes[0] != frozendb.ROW_START {
        t.Errorf("ROW_START mismatch: expected 0x%02X, got 0x%02X", 
            frozendb.ROW_START, rowBytes[0])
    }
    
    if rowBytes[1] != byte(frozendb.CHECKSUM_ROW) {
        t.Errorf("Start control mismatch: expected 'C', got '%c'", rowBytes[1])
    }
    
    if rowBytes[len(rowBytes)-1] != frozendb.ROW_END {
        t.Errorf("ROW_END mismatch: expected 0x%02X, got 0x%02X", 
            frozendb.ROW_END, rowBytes[len(rowBytes)-1])
    }
    
    // Verify checksum format
    checksumText := string(rowBytes[2:10]) // Base64 checksum at positions 2-9
    if len(checksumText) != 8 || checksumText[6:] != "==" {
        t.Errorf("Invalid checksum format: %s", checksumText)
    }
}
```

This quickstart provides the essential patterns for implementing and using checksum row functionality in frozenDB. For detailed API information, see the contracts/checksum_row_api.md specification.
