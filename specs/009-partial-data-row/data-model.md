# Data Model: PartialDataRow Implementation

**Date**: 2026-01-14  
**Feature**: 009-partial-data-row  
**Purpose**: Define entities, validation rules, and state transitions for PartialDataRow

## Core Entities

### 1. PartialRowState Enum

```go
type PartialRowState int

const (
    PartialDataRowWithStartControl PartialRowState = iota  // ROW_START + START_CONTROL only
    PartialDataRowWithPayload                               // PartialDataRowWithStartControl + UUID + JSON + calculated padding  
    PartialDataRowWithSavepoint                              // PartialDataRowWithPayload + 'S' character (savepoint intent)
)
```

**Purpose**: Represents the current construction state of a PartialDataRow  
**Transitions**: PartialDataRowWithStartControl → PartialDataRowWithPayload → PartialDataRowWithSavepoint (forward only, no reversal)  
**Validation**: Each state has different validation requirements

### 2. PartialDataRow Struct

```go
type PartialDataRow struct {
    State PartialRowState
    d     DataRow  // Contained DataRow, not embedded
}
```

**Purpose**: Incomplete data row that can only exist as the last row in a frozenDB file  
**Composition Pattern**: Contains a DataRow as a field for state-aware validation and construction  
**Key Properties**: 
- Variable length (stops at state boundary)
- State immutability (cannot revert to lower states)
- Reuses DataRow validation logic through helper methods
- NOT a DataRow itself - methods like Parity() cannot be called directly
- Can convert contained DataRow to complete DataRow when finished

### 3. InvalidActionError (New Error Type)

```go
type InvalidActionError struct {
    FrozenDBError
}

func NewInvalidActionError(message string, err error) *InvalidActionError {
    return &InvalidActionError{
        FrozenDBError: FrozenDBError{
            Code:    "invalid_action", 
            Message: message,
            Err:     err,
        },
    }
}
```

**Purpose**: Invalid state transitions and actions not permitted in current state  
**Usage**: Returned when AddRow/Savepoint/completion methods called from invalid states

## State Definitions and Validation Rules

### PartialDataRowWithStartControl: Row Start Only

**Byte Layout**: `[ROW_START][START_CONTROL]` (2 bytes total)  
**Valid Fields**: start_control only  
**Validation Requirements**:
- ROW_START must be byte 0x1F
- START_CONTROL must be valid uppercase alphanumeric (T or R for data rows)
- No UUID, JSON, or end_control present
- MarshalText() must output exactly 2 bytes

### PartialDataRowWithPayload: Complete Key-Value Data

**Byte Layout**: PartialDataRowWithStartControl + UUID_base64 + JSON_payload + calculated padding  
**Valid Fields**: start_control, UUID_base64, JSON_payload, padding (no end_control)  
**Validation Requirements**:
- All PartialDataRowWithStartControl validation + UUIDv7 validation + JSON structural validation
- UUID_base64 must be 24-byte valid Base64 encoding of UUIDv7
- JSON_payload must be valid UTF-8 string (non-empty); structural JSON syntax validation is out of scope at this layer (same as DataRow)
- Padding must be correct NULL_BYTE count immediately following JSON string value

**Padding Calculation**:
```go
padding_bytes = row_size - len(json_payload) - 31
// where 31 = 1(ROW_START) + 1(start_control) + 24(UUID) + 2(end_control) + 2(parity) + 1(ROW_END)
// Padding comes immediately after JSON string value, up to row_size - 2 bytes
```

### PartialDataRowWithSavepoint: Savepoint Intent

**Byte Layout**: PartialDataRowWithStartControl + UUID_base64 + JSON_payload + padding + 'S' character (first byte of END_CONTROL only)  
**Valid Fields**: All PartialDataRowWithPayload fields + single 'S' character  
**Validation Requirements**:
- All PartialDataRowWithPayload validation + 'S' character verification
- 'S' must be single character indicating savepoint intent
- No second END_CONTROL byte present
- Padding comes immediately after JSON string value (same as PartialDataRowWithPayload)
- 'S' character comes after padding, leaving space for final end_control byte
- MarshalText() must output: PartialDataRowWithStartControl + UUID_base64 + JSON_payload + padding + 'S'

## Error Handling Summary

| Error Type | Usage | Context |
|------------|-------|---------|
| InvalidActionError | State transition failures | AddRow/Savepoint from wrong state, completion from State1 |
| InvalidInputError | Creation validation | Invalid UUID, empty JSON, invalid savepointId |
| CorruptDatabaseError | UnmarshalText failures | Wraps all UnmarshalText validation errors |
| FrozenDBError | Base error | All errors inherit from this base |

## Integration Points

### Existing Dependencies Used

- `ValidateUUIDv7()` function from existing codebase
- `DataRow` struct and `baseRow` generic foundation  
- `StartControl`, `EndControl` enum patterns
- `InvalidInputError`, `CorruptDatabaseError` types
- Header structure and row_size calculations

### New Components Required

- `PartialRowState` enum
- `InvalidActionError` type  
- `validateField()` method on DataRow
- State-aware MarshalText/UnmarshalText implementations
- Completion method implementations

This data model provides a complete foundation for implementing PartialDataRow while maintaining consistency with existing frozenDB patterns and meeting all functional requirements from the specification.