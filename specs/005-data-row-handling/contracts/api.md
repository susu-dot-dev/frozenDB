# DataRow API Contract

**Feature**: 005-data-row-handling  
**Version**: 1.0.0  
**Format**: OpenAPI 3.0 equivalent for Go package API

## Overview

This document defines the public API contract for the DataRow implementation in frozenDB. The DataRow provides key-value storage with UUIDv7 keys and JSON string values, following the append-only immutable architecture.

## Imports

```go
import (
    "github.com/anilmahadev/frozenDB/frozendb"
    "github.com/google/uuid"
)
```

## Public Types

### DataRow

Represents a single key-value data row with UUIDv7 key and JSON string value.

```go
type DataRow struct {
    baseRow[*DataRowPayload] // Embedded generic foundation
}
```

### DataRowPayload

Container for the actual key-value data.

```go
type DataRowPayload struct {
    Key   uuid.UUID // UUIDv7 key for time ordering
    Value json.RawMessage    // JSON string value (no syntax validation at this layer)
}
```

## Public API

### Constructor Pattern

DataRow uses direct struct initialization followed by manual validation. This enables both manual creation and UnmarshalText deserialization without requiring separate constructors.

```go
// Manual creation pattern
dataRow := &frozendb.DataRow{
    Header:       header,
    StartControl: frozendb.TRANSACTION_BEGIN, // or ROW_CONTINUATION
    EndControl:   frozendb.END_COMMIT,        // or other valid end controls
    RowPayload: &frozendb.DataRowPayload{
        Key:   uuidv7Key,
        Value: jsonString,
    },
}
err := dataRow.Validate()
if err != nil {
    return err
}

// Alternative: Set fields directly
dataRow := &frozendb.DataRow{}
dataRow.Header = header
dataRow.StartControl = frozendb.TRANSACTION_BEGIN
dataRow.EndControl = frozendb.END_COMMIT
dataRow.RowPayload = &frozendb.DataRowPayload{
    Key:   uuidv7Key,
    Value: jsonString,
}
err = dataRow.Validate()

// UnmarshalText pattern for database reads
var dataRow frozendb.DataRow
err := dataRow.UnmarshalText(serializedBytes)
if err != nil {
    return err
}
err = dataRow.Validate() // Complete validation after unmarshaling
```

### Serialization Methods

#### MarshalText

Serializes DataRow to byte array according to v1_file_format.md specification.

```go
func (dr *DataRow) MarshalText() ([]byte, error)
```

**Returns**:
- `[]byte`: Serialized byte array with proper structure, padding, and parity
- `error`: InvalidInputError for structural failures

**Format**:
```
[ROW_START][StartControl='T'/'R'][Base64 UUIDv7][JSON Value + NULL Padding][Parity][EndControl='TC'/'RE'/'SC'/'SE'/'R0-R9'/'S0-S9'][ROW_END]
```

#### UnmarshalText

Deserializes DataRow from byte array with comprehensive validation.

```go
func (dr *DataRow) UnmarshalText(data []byte) error
```

**Parameters**:
- `data []byte`: Serialized DataRow bytes

**Returns**:
- `error`: InvalidInputError for format errors, CorruptDatabaseError for integrity failures

**Validation**:
- Sentinel byte validation (ROW_START, ROW_END)
- Control character validation ('T'/'R' for start, TC/RE/SC/SE/R0-R9/S0-S9 for end)
- Base64 UUID decoding and version validation
- Parity byte verification
- Row size validation against header

### Validation Methods

#### Validate

Performs complete DataRow validation including controls and payload.

```go
func (dr *DataRow) Validate() error
```

**Returns**:
- `error`: InvalidInputError for validation failures

**Validation Steps**:
1. Header not nil
2. StartControl valid ('T' or 'R')
3. EndControl valid (TC/RE/SC/SE/R0-R9/S0-S9)
4. RowPayload not nil
5. Payload-specific validation (UUIDv7, non-empty value)
6. Note: Multi-row transaction sequence validation handled at higher layer

**Usage**: Call Validate() after manual struct creation or after UnmarshalText() for complete validation.

#### ValidateUUIDv7

Validates that a UUID is version 7 and RFC 4122 variant.

```go
func ValidateUUIDv7(u uuid.UUID) *InvalidInputError
```

**Parameters**:
- `u uuid.UUID`: UUID to validate

**Returns**:
- `*InvalidInputError`: nil if valid, error details if invalid

**Validation Criteria**:
- Variant = RFC 4122 (0x80xx)
- Version = 7 (time-ordered)

### Accessor Methods

#### GetKey

Retrieves the UUIDv7 key from the DataRow.

```go
func (dr *DataRow) GetKey() uuid.UUID
```

**Returns**:
- `uuid.UUID`: The stored UUIDv7 key

#### GetValue

Retrieves the JSON string value from the DataRow.

```go
func (dr *DataRow) GetValue() string
```

**Returns**:
- `string`: The stored JSON string value

## Error Contracts

### InvalidInputError

Used for validation failures and malformed input.

```go
type InvalidInputError struct {
    FrozenDBError
}
```

**Error Codes**:
- `invalid_input`: Base validation failure
- `invalid_uuid`: UUID validation failure
- `invalid_format`: Format specification violation

### CorruptDatabaseError

Used for integrity failures during deserialization.

```go
type CorruptDatabaseError struct {
    FrozenDBError
}
```

**Error Codes**:
- `corruption`: Data integrity violation
- `parity_mismatch`: Parity byte verification failure

## Constants

### Control Characters

```go
// DataRow Start Controls
const (
    TRANSACTION_BEGIN StartControl = 'T' // First row of transaction
    ROW_CONTINUATION StartControl = 'R' // Continuation row within transaction
)

// DataRow End Controls (transaction state dependent)
const (
    END_COMMIT            EndControl = "TC"  // Commit transaction
    END_CONTINUE           EndControl = "RE"  // Continue transaction
    END_SAVEPOINT_COMMIT  EndControl = "SC"  // Savepoint + commit
    END_SAVEPOINT_CONTINUE EndControl = "SE"  // Savepoint + continue
    END_ROLLBACK_BASE      EndControl = "R0"  // Full rollback (base)
    // R1-R9: Rollback to savepoint N
    END_SAVEPOINT_ROLLBACK_BASE EndControl = "S0"  // Savepoint + full rollback
    // S1-S9: Savepoint + rollback to savepoint N
)

// ChecksumRow Controls (for reference)
const (
    CHECKSUM_ROW         StartControl = 'C'  // ChecksumRow start control
    CHECKSUM_ROW_CONTROL EndControl   = "CS" // ChecksumRow end control
)
```

### File Format Constants

```go
const (
    ROW_START byte = 0x1F // Row start sentinel
    ROW_END   byte = 0x0A // Row end sentinel
    NULL_BYTE  byte = 0x00 // JSON value padding
)
```

## Implementation Requirements

### Thread Safety

- **Read Operations**: Safe for concurrent access
- **Write Operations**: Not thread-safe (require external synchronization)
- **Immutable State**: Once created, DataRow is immutable

### Memory Management

- **Fixed Allocation**: Memory usage constant regardless of JSON content size
- **No Leaks**: All allocations properly managed
- **Efficient Serialization**: Minimal allocations during marshal/unmarshal

### Performance Characteristics

- **Creation**: O(1) time complexity
- **Serialization**: O(row_size) for parity calculation
- **Deserialization**: O(row_size) for validation and parsing
- **Validation**: O(1) for structural checks

## Integration Contracts

### With Header

DataRow requires a valid Header instance for:
- Row size validation
- Configuration parameters
- Format specification compliance

### With baseRow

DataRow embeds baseRow[*DataRowPayload] for:
- Shared file format validation
- Parity calculation
- Sentinel byte handling
- Generic type safety

### With UUID Package

Integration with github.com/google/uuid for:
- UUIDv7 validation
- Base64 encoding/decoding
- Variant and version checking

## Testing Requirements

### Unit Tests

- Method-specific validation
- Error case coverage
- Edge case handling
- Round-trip serialization

### Specification Tests

- Functional requirement validation
- File format compliance
- Integration behavior
- Performance characteristics

## Version Compatibility

### Current Version: 1.0.0

- **File Format**: v1_file_format.md specification
- **UUID Version**: 7 only
- **Go Version**: 1.25.5+
- **Dependencies**: github.com/google/uuid, Go standard library

### Backward Compatibility

- File format is append-only and backward compatible
- API changes require semantic versioning
- Database files remain readable across versions

## Security Considerations

### Input Validation

- All inputs validated before processing
- UUIDv7 validation prevents timing attacks
- JSON values treated as untrusted strings

### Integrity Protection

- Parity bytes detect corruption
- Sentinel bytes prevent truncation
- Comprehensive validation prevents malformed data

### Memory Safety

- No unsafe operations
- Proper bounds checking
- Fixed memory allocation patterns