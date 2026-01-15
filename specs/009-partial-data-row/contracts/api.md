# PartialDataRow API Contract

**Feature**: 009-partial-data-row  
**Date**: 2026-01-14  
**Format**: OpenAPI-like specification for Go API

## Types and Enums

### PartialRowState

```go
type PartialRowState int

const (
    PartialDataRowWithStartControl PartialRowState = iota  // ROW_START + START_CONTROL only
    PartialDataRowWithPayload                               // With UUID + JSON + calculated padding  
    PartialDataRowWithSavepoint                              // With savepoint 'S' character
)
```

### InvalidActionError

```go
type InvalidActionError struct {
    FrozenDBError
}

func NewInvalidActionError(message string, err error) *InvalidActionError
func (e *InvalidActionError) Error() string
func (e *InvalidActionError) Unwrap() error
```

## Core Struct

### PartialDataRow

```go
type PartialDataRow struct {
    State PartialRowState
    d     DataRow  // Contained DataRow, not embedded
}
```

## Creation Pattern

### Direct Struct Creation

**Pattern**: Create PartialDataRow struct directly, then call `Validate()`:
```go
// Example usage
pdr := &PartialDataRow{
    State: PartialDataRowWithStartControl,
    d: DataRow{
        // Initialize DataRow with header and startControl
    },
}

if err := pdr.Validate(); err != nil {
    return err
}
```

**Requirements**:
- Must initialize contained DataRow with required header and startControl
- Must set initial state to PartialDataRowWithStartControl
- Must call Validate() after creation to ensure validity
- No constructor function needed - direct struct creation as shown in user input pattern

## State Transition Methods

### AddRow()

```go
func (p *PartialDataRow) AddRow(key uuid.UUID, json string) error
```

**Description**: Transitions from PartialDataRowWithStartControl to PartialDataRowWithPayload by adding UUIDv7 key and JSON payload  
**Parameters**:
- `key uuid.UUID` - UUIDv7 key (must be valid and Base64-encodable)
- `json string` - JSON payload (must be non-empty UTF-8 JSON string)

**Returns**:
- `error` - InvalidActionError if not in PartialDataRowWithStartControl, InvalidInputError for validation failures

**State Requirement**: Must be in PartialDataRowWithStartControl  
**State Change**: PartialDataRowWithStartControl → PartialDataRowWithPayload  
**Validation**: UUIDv7 validation, JSON structural validation, re-validation after transition

**Example**:
```go
key := uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8b")
err := pdr.AddRow(key, `{"name":"test","value":123}`)
if err != nil {
    return err
}
// pdr.State == PartialDataRowWithPayload
```

### Savepoint()

```go
func (p *PartialDataRow) Savepoint() error
```

**Description**: Transitions from PartialDataRowWithPayload to PartialDataRowWithSavepoint by setting savepoint intent  
**Parameters**: None  

**Returns**:
- `error` - InvalidActionError if not in PartialDataRowWithPayload

**State Requirement**: Must be in StateKeyData  
**State Change**: StateKeyData → StateSavepointIntent  
**Validation**: Verifies 'S' character set, re-validation after transition

**Example**:
```go
err := pdr.Savepoint()
if err != nil {
    return err
}
// pdr.State == PartialDataRowWithSavepoint
```

## Completion Methods

All completion methods return a complete DataRow and transition the PartialDataRow to a finished state. The returned DataRow is only valid if the error is nil.

### Commit()

```go
func (p *PartialDataRow) Commit() (*DataRow, error)
```

**Description**: Completes the PartialDataRow as a committed transaction  
**Parameters**: None  

**Returns**:
- `*DataRow` - Complete DataRow with commit end_control
- `error` - InvalidActionError if in PartialDataRowWithStartControl, validation error if DataRow validation fails

**State Requirement**: StateKeyData or StateSavepointIntent  
**End Control**: 
- From StateKeyData: "TC" (Transaction Commit)
- From StateSavepointIntent: "SC" (Savepoint + Commit)

**Example**:
```go
dataRow, err := pdr.Commit()
if err != nil {
    return err
}
// dataRow.EndControl == {'T','C'} or {'S','C'} depending on pdr.State
```

### Rollback()

```go
func (p *PartialDataRow) Rollback(savepointId int) (*DataRow, error)
```

**Description**: Completes the PartialDataRow with rollback to specified savepoint  
**Parameters**:
- `savepointId int` - Savepoint ID (0-9, where 0 = full rollback)

**Returns**:
- `*DataRow` - Complete DataRow with rollback end_control
- `error` - InvalidActionError if in PartialDataRowWithStartControl or savepointId invalid, validation error if DataRow validation fails

**State Requirement**: StateKeyData or StateSavepointIntent  
**End Control**: 
- From StateKeyData: "R0"-"R9" (Rollback to savepoint N)
- From StateSavepointIntent: "S0"-"S9" (Savepoint + Rollback to savepoint N)

**Validation**: savepointId must be between 0-9 inclusive

**Example**:
```go
dataRow, err := pdr.Rollback(1)
if err != nil {
    return err
}
// dataRow.EndControl == {'R','1'} or {'S','1'} depending on pdr.State
```

### EndRow()

```go
func (p *PartialDataRow) EndRow() (*DataRow, error)
```

**Description**: Completes the PartialDataRow as a continuing row (transaction continues)  
**Parameters**: None  

**Returns**:
- `*DataRow` - Complete DataRow with continue end_control
- `error` - InvalidActionError if in PartialDataRowWithStartControl, validation error if DataRow validation fails

**State Requirement**: StateKeyData or StateSavepointIntent  
**End Control**: 
- From StateKeyData: "RE" (Row End - continue)
- From StateSavepointIntent: "SE" (Savepoint + End - continue)

**Example**:
```go
dataRow, err := pdr.EndRow()
if err != nil {
    return err
}
// dataRow.EndControl == {'R','E'} or {'S','E'} depending on pdr.State
```

## Validation Methods

### Validate()

```go
func (p *PartialDataRow) Validate() error
```

**Description**: Validates PartialDataRow according to its current state  
**Parameters**: None  

**Returns**:
- `error` - InvalidInputError with specific validation details

### validateField() (Private Helper)

```go
func (p *PartialDataRow) validateField(field string) error
```

**Description**: Private helper method that validates specific fields based on current state  
**Parameters**:
- `field string` - Field name to validate ("row_start", "start_control", "uuid", "json", "end_control_first", "padding")

**Returns**:
- `error` - InvalidInputError if field validation fails, nil if valid

**Field Validation Rules**:
- `"row_start"`: Validates ROW_START byte (0x1F) - all states
- `"start_control"`: Validates START_CONTROL character - all states  
- `"uuid"`: Validates UUIDv7 using existing ValidateUUIDv7() - PartialDataRowWithPayload+
- `"json"`: Validates JSON payload (non-empty, UTF-8 string) - PartialDataRowWithPayload+
- `"end_control_first"`: Validates 'S' character for savepoint intent - PartialDataRowWithSavepoint only
- `"padding"`: Validates NULL_BYTE padding count - PartialDataRowWithPayload+

**Note on JSON Validation**: The v1_file_format.md specification requires JSON_payload to be valid UTF-8 JSON. However, like DataRow, structural JSON syntax validation is intentionally out of scope at this layer. The Value field is stored as a string and validated only for non-emptiness. Full JSON syntax validation should be performed by application-level code that interprets the data.

**Usage in Validate()**:
```go
func (p *PartialDataRow) Validate() error {
    fields := []string{"row_start", "start_control"}
    
    switch p.State {
    case PartialDataRowWithPayload:
        fields = append(fields, "uuid", "json", "padding")
    case PartialDataRowWithSavepoint:
        fields = append(fields, "uuid", "json", "padding", "end_control_first")
    }
    
    for _, field := range fields {
        if err := p.validateField(field); err != nil {
            return err
        }
    }
    return nil
}
```

**State-Specific Validation**:
- **PartialDataRowWithStartControl**: ROW_START and START_CONTROL validation
- **PartialDataRowWithPayload**: PartialDataRowWithStartControl + UUIDv7 + JSON (non-empty UTF-8 string) + padding validation
- **PartialDataRowWithSavepoint**: PartialDataRowWithPayload + 'S' character validation

**Note**: JSON structural syntax validation is intentionally out of scope at this layer, consistent with DataRow behavior. The JSON payload is validated for non-emptiness and UTF-8 encoding only.

**Fields Validated**:
- ROW_START (must be 0x1F)
- START_CONTROL (must be valid uppercase alphanumeric)
- UUID_base64 (24-byte valid Base64 UUIDv7) - PartialDataRowWithPayload+
- JSON_payload (valid UTF-8 string, non-empty) - PartialDataRowWithPayload+ (structural JSON syntax validation is out of scope - same as DataRow)
- 'S' character (must be 'S') - PartialDataRowWithSavepoint+
- Padding (correct NULL_BYTE count) - PartialDataRowWithPayload+

**Example**:
```go
if err := pdr.Validate(); err != nil {
    return fmt.Errorf("validation failed: %w", err)
}
```

## Serialization Methods

### MarshalText()

```go
func (p *PartialDataRow) MarshalText() ([]byte, error)
```

**Description**: Serializes the PartialDataRow to its byte representation according to current state  
**Parameters**: None  

**Returns**:
- `[]byte` - Serialized byte sequence for current state
- `error` - InvalidInputError if state is invalid

**State-Specific Output**:
- **PartialDataRowWithStartControl**: 2 bytes `[ROW_START][START_CONTROL]`
- **PartialDataRowWithPayload**: PartialDataRowWithStartControl + UUID_base64 + JSON + calculated padding
- **PartialDataRowWithSavepoint**: PartialDataRowWithStartControl + UUID_base64 + JSON + calculated padding + 'S' character

**Padding Calculation**: Uses full DataRow formula for PartialDataRowWithPayload+PartialDataRowWithSavepoint:
```
padding_bytes = row_size - len(json_payload) - 31
```

**Example**:
```go
bytes, err := pdr.MarshalText()
if err != nil {
    return err
}
// len(bytes) varies by state: 2 for PartialDataRowWithStartControl, variable for PartialDataRowWithPayload, row_size-1 for PartialDataRowWithSavepoint
```

### UnmarshalText()

```go
func (p *PartialDataRow) UnmarshalText(text []byte) error
```

**Description**: Deserializes byte sequence into PartialDataRow with proper state detection  
**Parameters**:
- `text []byte` - Byte sequence to deserialize

**Returns**:
- `error` - CorruptDatabaseError wrapping any validation errors

**State Detection**: Automatically determines state based on length and content:
- Length 2 → PartialDataRowWithStartControl
- Contains 'S' character after UUID position → PartialDataRowWithSavepoint
- Otherwise → PartialDataRowWithPayload

**Error Handling**: Always wraps validation errors in CorruptDatabaseError as required by specification

**Example**:
```go
pdr := &PartialDataRow{}
err := pdr.UnmarshalText(bytes)
if err != nil {
    return fmt.Errorf("corrupt data: %w", err)
}
// pdr.State automatically detected and set
```

## State Query Methods

### GetState()

```go
func (p *PartialDataRow) GetState() PartialRowState
```

**Description**: Returns the current state of the PartialDataRow  
**Parameters**: None  

**Returns**: `PartialRowState` - Current state (PartialDataRowWithStartControl, PartialDataRowWithPayload, or PartialDataRowWithSavepoint)

**Example**:
```go
state := pdr.GetState()
switch state {
case PartialDataRowWithStartControl:
    fmt.Println("Transaction start only")
case PartialDataRowWithPayload:
    fmt.Println("Key-value data present")
case PartialDataRowWithSavepoint:
    fmt.Println("Savepoint intent set")
}
```

## Error Conditions Summary

### InvalidActionError Scenarios

| Method | Invalid State | Error Message |
|--------|---------------|---------------|
| AddRow() | PartialDataRowWithPayload, PartialDataRowWithSavepoint | "AddRow() can only be called from PartialDataRowWithStartControl" |
| Savepoint() | PartialDataRowWithStartControl, PartialDataRowWithSavepoint | "Savepoint() can only be called from PartialDataRowWithPayload" |
| Commit() | PartialDataRowWithStartControl | "Commit() cannot be called from PartialDataRowWithStartControl" |
| Rollback() | PartialDataRowWithStartControl | "Rollback() cannot be called from PartialDataRowWithStartControl" |
| Rollback() | Any | "savepointId must be between 0-9" |
| EndRow() | PartialDataRowWithStartControl | "EndRow() cannot be called from PartialDataRowWithStartControl" |

### InvalidInputError Scenarios

- Constructor: nil header, invalid startControl
- AddRow(): invalid UUIDv7, empty JSON
- Validate(): field validation failures per state
- MarshalText(): invalid state

### CorruptDatabaseError Scenarios

- UnmarshalText(): all validation failures wrapped in CorruptDatabaseError
- Invalid byte sequences, corrupted data, malformed fields

This API contract defines the complete interface for PartialDataRow implementation, ensuring consistent behavior and proper error handling across all state transitions and operations.