# DataRow Data Model

**Feature**: 005-data-row-handling  
**Date**: 2026-01-11  

## Core Entities

### DataRow

Represents a single key-value data row with UUIDv7 key and JSON string value.

**Fields**:
```go
type DataRow struct {
    baseRow[*DataRowPayload] // Embedded generic foundation
}
```

**Inherited from baseRow**:
- `Header *Header` - Reference for row_size and configuration
- `StartControl StartControl` - Control character ('T' or 'R' for DataRow)
- `EndControl EndControl` - End control sequence (TC/RE/SC/SE/R0-R9/S0-S9 for DataRow)
- `RowPayload *DataRowPayload` - Typed payload data

### DataRowPayload

Container for the actual key-value data.

**Fields**:
```go
type DataRowPayload struct {
    Key   uuid.UUID // UUIDv7 key for time ordering
    Value string    // JSON string value (no syntax validation at this layer)
}
```

### Control Characters

Validation constants for DataRow identification.

**Values**:
- **StartControl**: `'T'` (transaction begin) or `'R'` (row continuation)
- **EndControl**: Two-character sequences based on transaction state:
  - `TC` - Commit
  - `RE` - Continue
  - `SC` - Savepoint + Commit  
  - `SE` - Savepoint + Continue
  - `R0-R9` - Rollback to savepoint N
  - `S0-S9` - Savepoint + Rollback to savepoint N

## Entity Relationships

```
Header (shared)
  ↓
DataRow (container)
  ↓
DataRowPayload (key-value data)
  ├── Key: uuid.UUID (UUIDv7)
  └── Value: string (JSON)
```

## Validation Rules

### DataRow Level
1. **Header Validation**: Must not be nil
2. **Control Validation**: Valid individual controls ('T'/'R' for start, TC/RE/SC/SE/R0-R9/S0-S9 for end)
3. **Payload Validation**: Must not be nil
4. **Row Size**: Overall length must match Header.row_size
5. **Note**: Multi-row transaction state validation handled at higher layer

### DataRowPayload Level
1. **Key Validation**: UUIDv7 only (RFC 4122 variant, version 7)
2. **Value Validation**: Non-empty string (JSON syntax not validated)
3. **Null Input**: Reject nil/zero UUID and empty value

### UUIDv7 Validation
1. **Variant**: Must be RFC 4122 (reject Microsoft/NCS)
2. **Version**: Must be version 7 (reject v1, v4, v6)
3. **Format**: Proper UUID structure and encoding
4. **Base64**: 24-byte encoded format with "=" padding

## State Transitions

### DataRow Lifecycle

```
Input Validation → Manual Construction → Validate() → Serialization → UnmarshalText() → Validate()
     ↓               ↓                    ↓            ↓              ↓              ↓
   FR-011         Manual Creation       FR-002      FR-003         FR-004         FR-002
```

1. **Input Validation**: UUIDv7 check, nil checks (manual)
2. **Manual Construction**: Create DataRow struct with proper controls
3. **Validate()**: Complete structural and payload validation
4. **Serialization**: Marshal to bytes with padding and parity
5. **UnmarshalText()**: Unmarshal from bytes with basic validation
6. **Validate()**: Complete validation after unmarshaling

### Error States

```
Invalid UUIDv7 → InvalidInputError
Invalid Controls → InvalidInputError
Payload Corruption → CorruptDatabaseError
Size Mismatch → InvalidInputError
Nil Components → InvalidInputError
```

## Data Flow Patterns

### Creation Flow
```
Header + UUIDv7 + JSON → Manual DataRow Creation → Validate() → Ready for Use
```

### Serialization Flow
```
DataRow → Validate → MarshalText → Base64 UUID + JSON + Padding → Parity Calculation → Byte Array
```

### Deserialization Flow
```
Byte Array → Validate Sentinels → Parse Controls → Base64 UUID Decode → Extract JSON → Validate → DataRow
```

## Memory and Performance Characteristics

### Memory Usage
- **Fixed**: Constant regardless of JSON content size
- **Allocation**: Minimal - mainly for string copies during serialization
- **Pattern**: Follows baseRow memory management patterns

### Performance Profile
- **UUID Validation**: O(1) - constant time checks
- **Serialization**: O(row_size) - linear in row size for parity calculation
- **Validation**: O(1) - constant time for structural checks
- **Base64 Operations**: O(1) - fixed 16-byte UUID operations

## Integration Points

### With Header
- **Row Size**: Must match Header.row_size exactly
- **Configuration**: Uses Header settings for validation

### With baseRow
- **Shared Logic**: Inherits file format validation
- **Generic Types**: Uses `baseRow[*DataRowPayload]` for type safety
- **Parity Calculation**: Leverages existing LRC implementation

### With ChecksumRow
- **Pattern Consistency**: Same three-file architecture
- **Error Handling**: Same structured error types
- **Testing Patterns**: Same unit and spec test approach

## Constraints and Invariants

### Immutable Properties
- UUIDv7 key cannot be modified after creation
- JSON value content preserved exactly during round-trip
- Row structure fixed once created

### Validation Invariants
- All DataRows must have UUIDv7 keys
- All DataRows must have valid control characters
- All serialized rows must match exact byte layout

### Integrity Guarantees
- Round-trip serialization preserves all data exactly
- Parity bytes detect single-byte corruption
- Sentinel bytes detect truncated/corrupted rows