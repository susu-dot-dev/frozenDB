# Data Model: NullRow Implementation

**Feature**: 010-null-row-struct  
**Date**: 2025-01-18  
**Status**: Complete

## Entity: NullRow

### Purpose
Represents a null operation row in frozenDB file format. NullRows are single-row transactions that represent empty operations and maintain proper file structure integrity. Uses the baseRow generic foundation following established patterns.

### Structure

```go
// NullRowPayload contains the payload data for a NullRow.
// NullRows have UUID with timestamp equal to max_timestamp, other fields zero, but no user value data.
type NullRowPayload struct {
    Key uuid.UUID // UUIDv7 with timestamp = max_timestamp, other fields zero
}

// NullRow represents a null operation row using baseRow for common functionality.
type NullRow struct {
    baseRow[*NullRowPayload] // Embedded generic foundation
}
```

### BaseRow Composition

The NullRow uses the baseRow[T] generic foundation with these components:
- **Header**: Reference to database Header for row_size and configuration
- **StartControl**: Always START_TRANSACTION ('T') for null rows
- **EndControl**: Always EndControl{'N', 'R'} for null row termination
- **RowPayload**: NullRowPayload instance (no data content)

### NullRowPayload Behavior

Since NullRows represent null operations with UUID only:
- **MarshalText()**: Returns Base64-encoded UUIDv7 with timestamp = max_timestamp, other fields zero
- **UnmarshalText()**: Validates UUID timestamp equals max_timestamp and other fields are zero
- **Validation**: Key timestamp must equal max_timestamp, all other UUID fields must be zero

### Relationships

- **BaseRow Foundation**: Inherits all baseRow functionality (validation, marshaling, parity calculation)
- **File Context**: Exists within frozenDB file structure alongside ChecksumRow and DataRow
- **Transaction Context**: Represents a complete single-row transaction

### Validation Rules

1. **Start Control Validation**: Must be START_TRANSACTION ('T')
2. **End Control Validation**: Must be EndControl{'N', 'R'} (null row)
3. **Payload Validation**: NullRowPayload.Key must have timestamp equal to max_timestamp, with all other UUID fields set to zero
4. **Parity Validation**: LRC calculated automatically by baseRow
5. **Row Size Validation**: Handled automatically by baseRow with proper padding

### State Transitions

NullRow has no state transitions - it's an immutable structure once created. The lifecycle is:
1. **Creation**: Direct struct initialization with NullRowPayload and required controls
2. **Validation**: Verify StartControl='T', EndControl='NR', payload is empty
3. **Marshaling**: baseRow.MarshalText() handles binary format conversion
4. **Unmarshaling**: baseRow.UnmarshalText() handles deserialization

### Constraints

- **Immutable**: baseRow foundation ensures immutability
- **Fixed Width**: baseRow handles exact row_size matching
- **Single Transaction**: Each NullRow represents one complete transaction
- **Position**: Can only appear where new transactions are allowed
- **Timestamp UUID Payload**: NullRowPayload contains UUIDv7 with timestamp = max_timestamp, other fields zero, but no value data

### API Methods

```go
// NullRow represents a null operation row using baseRow for common functionality.
type NullRow struct {
    baseRow[*NullRowPayload] // Embedded generic foundation
}

// Validate performs NullRow-specific validation
func (nr *NullRow) Validate() error

// MarshalText serializes to binary format using baseRow
func (nr *NullRow) MarshalText() ([]byte, error)

// UnmarshalText deserializes from binary using baseRow
func (nr *NullRow) UnmarshalText([]byte) error
```

### Error Handling

- **InvalidInputError**: Returned for validation failures (FR-009, FR-010, FR-011)
- **InvalidInputError**: Returned for marshaling failures (FR-012)
- **CorruptDatabaseError**: Returned for unmarshal failures wrapping validation errors (FR-013)