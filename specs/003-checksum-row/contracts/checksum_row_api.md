# frozenDB Checksum Row API Specification

## Overview

This document defines the API contracts for checksum row functionality in frozenDB. The specification covers data structures, interfaces, and methods required for implementing checksum rows per the v1 file format specification.

## Core Interfaces

### baseRow Generic Requirements

```go
// baseRow provides the generic foundation for all frozenDB row types
type baseRow[P RowPayload] struct {
    Header       *Header      // Header reference for row_size and configuration
    StartControl StartControl // Single byte control character (position 1)
    EndControl   EndControl   // Two-byte end control sequence (positions N-5,N-4)
    RowPayload   P            // Typed payload data, validated after structural checks
}
```

**Critical Validation Requirement**: When calling `UnmarshalText()`, the code MUST validate every bit of the row structure before attempting to unmarshal into the sub-types of the baseRow. This validation follows the v1_file_format specification:

#### 1. File Format Compliance

**Row Structure Validation** (per v1_file_format.md Section 5):
- ROW_START sentinel (0x1F) at position 0
- ROW_END sentinel (0x0A) at position row_size-1
- StartControl validation per valid enum values (Section 5.1, 8.2)
- EndControl validation per valid combinations (Section 5.1, 8.3)
- Parity bytes validation using LRC calculation (Section 5.2)

**Implementation Requirements**:
- MUST validate every structural element before any payload processing
- MUST reference v1_file_format.md for exact byte positions and values
- MUST return InvalidInputError for structural validation failures
- MUST use exact error messages with position and value information

#### 2. Validation Sequence

**Phase 1: Structural Integrity** (MUST pass first):
- Validate ROW_START and ROW_END sentinels
- Validate StartControl and EndControl enums
- Validate parity bytes with XOR calculation
- Return InvalidInputError immediately on any failure

**Phase 2: Payload Validation** (only after Phase 1 passes):
- Extract payload bytes (positions 2 through N-6)
- Handle NULL_BYTE padding per v1_file_format.md
- Call RowPayload.UnmarshalText() for type-specific validation
- Wrap payload errors in CorruptDatabaseError

#### 3. Error Handling Contract

**baseRow.validate() Requirements**:
```go
func (br *baseRow[P]) validate() error {
    // Phase 1: Structural validation per v1_file_format.md
    if err := br.validateStructure(); err != nil {
        return err // InvalidInputError with position details
    }
    
    // Phase 2: Payload validation (only if structure valid)
    if err := br.validatePayload(); err != nil {
        return NewCorruptDatabaseError("payload validation failed", err)
    }
    
    return nil
}
```

**UnmarshalText() Error Wrapping**:
- InvalidInputError: Returned directly for structural failures
- CorruptDatabaseError: Wraps any payload-specific validation errors
- No partial state modifications on validation failure

**Key Principles**:
- **Format First**: v1_file_format.md compliance is mandatory
- **Fail-Fast**: Immediate return on structural validation failure  
- **Layered Validation**: Structure before payload
- **Precise Errors**: Position-aware error reporting

### RowPayload Interface

```go
// RowPayload defines the interface for row-specific payload data
type RowPayload interface {
    // encoding.TextMarshaler provides text marshaling capability
    encoding.TextMarshaler
    
    // encoding.TextUnmarshaler provides text unmarshaling capability  
    encoding.TextUnmarshaler
}
```

**Methods**:

- `MarshalText() ([]byte, error)`: Converts CRC32 to Base64 text representation
- `UnmarshalText([]byte) error`: Populates CRC32 from Base64 text data

**Implementation Requirements**:
- All implementations must handle their own validation
- Marshal/Unmarshal must be reversible (round-trip compatible)
- Size() must return actual size after marshaling

### Internal Validation Functions

Row structs implement internal `validate() error` functions that perform comprehensive row validation. These functions are called automatically during UnmarshalText and constructor function execution.

**Validation Sequence**:
1. **baseRow Structural Validation** (must pass first):
   - Complete compliance with v1_file_format.md Section 5 (Row Structure)
   - Sentinel validation, control validation, parity calculation
   - InvalidInputError returned for any structural failure

2. **Type-Specific Validation** (only after baseRow passes):
   - ChecksumRow: StartControl='C' (CHECKSUM_ROW), EndControl=[C,S] (CHECKSUM_ROW_CONTROL) (v1_file_format.md Section 6.1)
   - CRC32 calculation per v1_file_format.md Section 6.2
   - Payload-specific format validation

**Critical Requirement**: Every bit MUST be validated by baseRow before any sub-type validation occurs. baseRow enforces v1_file_format.md structural requirements, while sub-types handle only type-specific validation defined in their respective sections.

## Enum Types

### StartControl Type and Constants

```go
// StartControl represents single-byte control characters at row position [1]
type StartControl byte

// StartControl constants represent valid control characters
const (
    // START_TRANSACTION marks the beginning of a new transaction
    START_TRANSACTION StartControl = 'T'
    
    // ROW_CONTINUE marks the continuation of an existing transaction
    ROW_CONTINUE StartControl = 'R'
    
    // CHECKSUM_ROW marks a checksum integrity row
    CHECKSUM_ROW StartControl = 'C'
)

// MarshalText converts StartControl to single byte
func (sc StartControl) MarshalText() ([]byte, error) {
    return []byte{byte(sc)}, nil
}

// UnmarshalText parses single byte and validates StartControl
func (sc *StartControl) UnmarshalText(text []byte) error {
    if len(text) != 1 {
        return fmt.Errorf("start_control must be exactly 1 byte, got %d", len(text))
    }
    
    switch text[0] {
    case byte(START_TRANSACTION), byte(ROW_CONTINUE), byte(CHECKSUM_ROW):
        *sc = StartControl(text[0])
        return nil
    default:
        return fmt.Errorf("invalid start_control: %c (0x%02X)", text[0], text[0])
    }
}

```

**Values**:
- `START_TRANSACTION` ('T'): Transaction begin
- `ROW_CONTINUE` ('R'): Row continuation  
- `CHECKSUM_ROW` ('C'): Checksum row

**Validation**:
- Only valid constants accepted (T, R, C)
- Direct byte type makes ASCII character mapping trivial
- Case-sensitive (uppercase only)
- No parsing functions needed - use direct byte constants
- For text parsing, use standard MarshalText/UnmarshalText patterns

### EndControl Struct

```go
// EndControl represents two-byte control sequence at row positions [N-5:N-4]
type EndControl [2]byte // 2-byte array: [T,C], [R,E], [C,S], [S,E], [R,0-9], [S,0-9]

// Constants for common control sequences as byte arrays
var (
    // Data row end controls
    TRANSACTION_COMMIT  = EndControl{'T', 'C'} // Transaction commit, no savepoint
    ROW_END             = EndControl{'R', 'E'} // Transaction continue, no savepoint
    SAVEPOINT_COMMIT    = EndControl{'S', 'C'} // Transaction commit with savepoint
    SAVEPOINT_CONTINUE  = EndControl{'S', 'E'} // Transaction continue with savepoint
    FULL_ROLLBACK       = EndControl{'R', '0'} // Full rollback to savepoint 0
    
    // Checksum row end controls
    CHECKSUM_ROW_CONTROL = EndControl{'C', 'S'} // Checksum-specific end control
)

// MarshalText converts EndControl 2-byte array to slice
func (ec EndControl) MarshalText() ([]byte, error) {
    return ec[:], nil
}

// UnmarshalText parses 2-byte sequence into EndControl array with validation
func (ec *EndControl) UnmarshalText(text []byte) error {
    if len(text) != 2 {
        return fmt.Errorf("end_control must be exactly 2 bytes, got %d", len(text))
    }
    
    // Convert to EndControl for comparison
    inputEnd := EndControl{text[0], text[1]}
    
    // Check against valid constants
    if inputEnd == TRANSACTION_COMMIT || inputEnd == ROW_END || 
       inputEnd == CHECKSUM_ROW_CONTROL || inputEnd == SAVEPOINT_CONTINUE ||
       inputEnd == SAVEPOINT_COMMIT || inputEnd == FULL_ROLLBACK {
        copy(ec[:], text)
        return nil
    }
    
    // Check R1-9 patterns (rollback to savepoint)
    if inputEnd[0] == 'R' && inputEnd[1] >= '1' && inputEnd[1] <= '9' {
        copy(ec[:], text)
        return nil
    }
    
    // Check S1-9 patterns (savepoint + rollback to savepoint)
    if inputEnd[0] == 'S' && inputEnd[1] >= '1' && inputEnd[1] <= '9' {
        copy(ec[:], text)
        return nil
    }
    
    return fmt.Errorf("invalid end_control: [%c,%c], must be one of [T,C], [R,E], [C,S], [S,E], [S,C], [R,0-9], or [S,0-9]", 
        inputEnd[0], inputEnd[1])
}

// String converts EndControl to string representation for display/debugging
func (ec EndControl) String() string {
    return string(ec[:])
}
```

```go
// ChecksumRow represents a checksum integrity row in frozenDB
type ChecksumRow struct {
    baseRow[Checksum] // Embedded with typed Checksum payload
}

// NewChecksumRow creates a new checksum row from header and data bytes
func NewChecksumRow(header *Header, dataBytes []byte) (*ChecksumRow, error) {
    checksumValue := calculateCRC32(dataBytes) // Calculate CRC32 for data block
    row := &ChecksumRow{
        baseRow[Checksum]{
            Header:       header,
            StartControl: CHECKSUM_ROW,      // byte 'C'
            EndControl:   CHECKSUM_ROW_CONTROL, // [C,S]
            RowPayload:   Checksum(checksumValue), // Typed payload
        },
    }
    
    // Validate the created row
    if err := row.validate(); err != nil {
        return nil, err
    }
    
    return row, nil
}

// GetChecksum extracts the CRC32 checksum value (no type assertion needed)
func (cr *ChecksumRow) GetChecksum() Checksum {
    return cr.RowPayload // Direct access, type-safe
}
```

## Error Contracts

### Validation Error Hierarchy

**baseRow.validate() Error Handling**:
- **InvalidInputError**: Returned for any structural validation failure
  - All failures related to v1_file_format.md Section 5 compliance
  - Returned directly without wrapping for clarity
- **CorruptDatabaseError**: Wraps payload-specific validation failures
  - Only occurs after successful structural validation
  - Preserves original payload error context

**UnmarshalText() Error Wrapping**:
- InvalidInputError: Returned directly for structural failures
- CorruptDatabaseError: Wraps any payload validation failures
- No new error types: Use existing frozendb error hierarchy

**Constructor Functions**:
- Call baseRow.validate() and return validation errors directly
- Preserve original error context without additional wrapping

**Key Principle**: Structural compliance errors (v1_file_format.md) are InvalidInputError; payload corruption errors are CorruptDatabaseError.
## Usage Examples

### Creating a Checksum Row

```go
// Create checksum row from header and data
header := &Header{RowSize: 1024}
// dataBytes should contain all bytes covered since previous checksum row
// For first checksum: all data rows from offset 64 to current position
// For subsequent checksums: all data rows since last checksum row
dataBytes := concatenateRows(row1, row2, row3, ...)

checksumRow, err := NewChecksumRow(header, dataBytes)
if err != nil {
    return err
}
// Note: Validation is performed automatically within NewChecksumRow

// Serialize to bytes
rowBytes, err := checksumRow.MarshalText()
if err != nil {
    return err
}
```

### Parsing a Checksum Row

```go
// Parse row from disk
rowBytes := readRowFromFile()

var checksumRow ChecksumRow
if err := checksumRow.UnmarshalText(rowBytes); err != nil {
    // Validation follows v1_file_format.md compliance:
    // - InvalidInputError for structural issues (sentinels, controls, parity)
    // - CorruptDatabaseError for payload-specific issues (CRC32 format)
    return err
}
// Note: Comprehensive validation performed automatically per v1_file_format.md:
// 1. Row Structure validation (Section 5)
// 2. Checksum Row specific validation (Section 6) 
// 3. Payload validation (type-specific)

// Extract CRC32 value
crc32Value := checksumRow.GetChecksum()
```

### Validation Error Categories

```go
// Structural compliance errors (v1_file_format.md Section 5)
err := checksumRow.UnmarshalText(structurallyInvalidBytes)
// Returns: InvalidInputError with position-specific details

// Payload corruption errors (post-structural validation)  
err := checksumRow.UnmarshalText(structurallyValidBytes)
// Returns: CorruptDatabaseError wrapping payload-specific error
```

### Using Control Enums

```go
// Parse control bytes from raw row data
var startCtrl StartControl
if err := startCtrl.UnmarshalText(rowBytes[1:2]); err != nil {
    return err
}

var endCtrl EndControl
if err := endCtrl.UnmarshalText(rowBytes[rowSize-5 : rowSize-3]); err != nil {
    return err
}

// Check transaction state
if endCtrl[1] == 'C' {
    // Transaction committed
} else if endCtrl[1] >= '0' && endCtrl[1] <= '9' {
    // Rollback to savepoint
    target := int(endCtrl[1] - '0')
    // Rolled back to savepoint target
}
```

## Implementation Requirements

### Performance Requirements

- **Memory**: Fixed memory usage regardless of database size
- **CPU**: Efficient XOR calculation for parity (O(n) where n = row_size)
- **I/O**: Support for direct row seeking (O(1) by row index)

### Compatibility Requirements

- **Go Version**: Compatible with Go 1.25.5 standard library
- **Dependencies**: Standard library only (encoding/base64, encoding/json, hash/crc32)
- **Platform**: Cross-platform compatible (Linux, Windows, macOS)

### Testing Requirements

- **Unit Tests**: 100% coverage for all methods
- **Spec Tests**: Compliance with v1_file_format.md specification
- **Integration Tests**: Compatibility with existing frozendb code
- **Benchmarks**: Performance validation for large datasets

This API specification provides the complete contract for implementing checksum row functionality in frozenDB while maintaining exact compliance with the file format specification and ensuring clean integration with existing code patterns.
