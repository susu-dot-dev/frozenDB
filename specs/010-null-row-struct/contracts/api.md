# NullRow API Contract

**Feature**: 010-null-row-struct  
**Date**: 2025-01-18  
**Version**: 1.0.0

## Overview

NullRow provides validation, marshaling, and unmarshaling capabilities for null operation rows in frozenDB. The API follows Go conventions and frozenDB error handling patterns.

## Payload Type

### NullRowPayload

Represents the UUID-only payload for NullRows following the RowPayload interface.

```go
type NullRowPayload struct {
    Key uuid.UUID // UUIDv7 with timestamp = max_timestamp, other fields zero
}
```

**Methods:**
- `MarshalText() ([]byte, error)`: Returns Base64-encoded UUIDv7 with timestamp = max_timestamp, other fields zero
- `UnmarshalText([]byte) error`: Validates UUID timestamp equals max_timestamp and other fields are zero

## Usage Pattern

### NullRow Creation

NullRows are created directly following existing row patterns:

```go
// Create null row with required controls
nullRow := &frozendb.NullRow{
    baseRow: frozendb.baseRow[*frozendb.NullRowPayload]{
        Header:       header,
        StartControl: frozendb.START_TRANSACTION,
        EndControl:   frozendb.EndControl{'N', 'R'},
        RowPayload:   &frozendb.NullRowPayload{Key: uuid.Nil},
    },
}
```

**Requirements:**
- Must set StartControl to START_TRANSACTION ('T')
- Must set EndControl to EndControl{'N', 'R'} (null row)
- Must create NullRowPayload with Key having timestamp = max_timestamp, other fields zero
- baseRow handles row_size validation automatically

## Validation Methods

### Validate

Verifies that the NullRow meets all file format specification requirements.

```go
func (nr *NullRow) Validate() error
```

**Returns:**
- `error`: 
  - `nil` if validation passes
  - `InvalidInputError` if any field violates specification (FR-009, FR-010, FR-011)

**Validation Rules:**
- start_control must equal 'T'
- uuid_base64 must be a UUIDv7 with timestamp equal to max_timestamp, all other fields zero
- end_control must equal []byte{'N', 'R'}

## Serialization Methods

### MarshalText

Converts NullRow to binary format using baseRow foundation matching v1 file format specification.

```go
func (nr *NullRow) MarshalText() ([]byte, error)
```

**Returns:**
- `[]byte`: Binary representation ready for file storage
- `error`: 
  - `nil` on success
  - `InvalidInputError` if row structure is invalid (FR-012)

**Binary Format:**
```
Position:  [0]    [1]    [2..25]         [26..N-6]        [N-5..N-4]    [N-3..N-2]   [N-1]
           ├──────┼──────┼───────────────┼─────────────────┼─────────────┼────────────┼──────┤
           │ROW_  │ start │ uuid_base64   │   padding       │    end      │   parity   │ROW_  │
           │START │ ctrl  │ (24 bytes)    │  (NULL_BYTE)    │  control    │   bytes    │END   │
           └──────┴──────┴───────────────┴─────────────────┴─────────────┴────────────┴──────┘
```

Note: baseRow automatically handles:
- ROW_START (0x1F) and ROW_END (0x0A) sentinels
- StartControl and EndControl placement
- Payload marshaling (UUIDv7 with timestamp = max_timestamp, other fields zero, Base64-encoded for NullRow)
- Padding calculation and application
- Parity byte calculation

### UnmarshalText

Deserializes NullRow from binary data using baseRow foundation.

```go
func (nr *NullRow) UnmarshalText(text []byte) error
```

**Parameters:**
- `text` ([]byte): Binary data from file

**Returns:**
- `error`: 
  - `nil` on success
  - `CorruptDatabaseError` wrapping validation errors if data format is invalid (FR-013)

**Validation:**
- baseRow.UnmarshalText() automatically verifies:
  - ROW_START (0x1F) and ROW_END (0x0A) sentinels
  - StartControl and EndControl values
  - Payload structure (UUID timestamp must equal max_timestamp, other fields must be zero for NullRow)
  - Parity byte calculations
- Additional NullRow-specific validation performed afterward

## Error Types

### InvalidInputError

Used for validation failures where input data violates specification.

```go
type InvalidInputError struct {
    FrozenDBError
    // Specific validation failure details
}
```

### CorruptDatabaseError

Used for unmarshal failures where file data appears corrupted.

```go
type CorruptDatabaseError struct {
    FrozenDBError
    // Corruption details and wrapped validation error
}
```

## Performance Requirements

- **Validation**: < 0.5ms for typical row sizes
- **Marshaling**: < 0.5ms for typical row sizes
- **Unmarshaling**: < 0.5ms for typical row sizes
- **Memory**: Fixed usage proportional to row_size

## Compliance

- Follows frozenDB v1 file format specification
- Adheres to constitution principles (immutability, correctness, data integrity)
- Maintains error handling excellence
- Provides comprehensive spec test coverage