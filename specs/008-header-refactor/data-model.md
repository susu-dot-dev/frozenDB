# Data Model: Header Refactor

## Entity Analysis

The Header refactor involves modifying existing entities and their relationships, without introducing new entity types.

## Existing Entities

### Header

**Location**: Moving from `create.go` to dedicated `header.go`

**Fields**:
- `Signature string json:"sig"` - Always "fDB" (format identifier)
- `Version int json:"ver"` - Always 1 (v1 format version)  
- `RowSize int json:"row_size"` - Size of each data row in bytes (128-65536)
- `SkewMs int json:"skew_ms"` - Time skew window in milliseconds (0-86400000)

**Field Naming Change**: Fields renamed from lowercase (signature, version, rowSize, skewMs) to exported (Signature, Version, RowSize, SkewMs) to enable direct JSON unmarshaling with struct tags. Existing getter methods remain unchanged for backward compatibility.

**Validation Rules**:
- `signature` must equal "fDB" (`HEADER_SIGNATURE`)
- `version` must equal 1
- `rowSize` must be between `MIN_ROW_SIZE` (128) and `MAX_ROW_SIZE` (65536)
- `skewMs` must be between 0 and `MAX_SKEW_MS` (86400000)

**State Transitions**: Not applicable (immutable after creation)

### Eliminated: headerJSON Helper Struct

**Status**: REMOVED - Header struct now uses JSON struct tags for direct unmarshaling

**Previously**: Internal helper struct for mapping JSON field names to Go field names
**Now**: Header struct uses `json:"..."` struct tags to map JSON fields directly

**Simplified UnmarshalText**: 
```go
// Before (with helper)
var hdr headerJSON
json.Unmarshal(jsonContent, &hdr)
h.signature = hdr.Sig
// ... field copying for all fields ...

// After (direct)
json.Unmarshal(jsonContent, h)
// No helper struct needed, no field copying
```

## Relationships

### Header ↔ CreateConfig

**Relationship**: CreateConfig uses Header values during database creation

**Before Refactor**:
```go
// Dual creation pattern
headerBytes, err := generateHeader(config.rowSize, config.skewMs)
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1, 
    rowSize:   config.rowSize,
    skewMs:    config.skewMs,
}
```

**After Refactor**:
```go
// Single creation pattern
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1,
    rowSize:   config.rowSize, 
    skewMs:    config.skewMs,
}
headerBytes, err := header.MarshalText()
```

### Header ↔ ChecksumRow

**Relationship**: Unchanged - ChecksumRow still requires Header for checksum calculation

**Usage Pattern**:
```go
checksumRow, err := NewChecksumRow(header, headerBytes)
```

## Validation Flow

### Header Creation and Validation

1. **Direct Initialization**: Create Header struct with field values
2. **Validate()**: Call Validate() method to ensure field validity
3. **MarshalText()**: Convert to 64-byte format (automatically validates)

### Header Reading and Validation

1. **UnmarshalText()**: Parse 64-byte buffer into Header struct using direct JSON unmarshaling
2. **Automatic Validation**: UnmarshalText() calls Validate() internally
3. **Access Methods**: Use getter methods to access field values (unchanged for backward compatibility)

## Data Format

### 64-Byte Header Structure

**Format**: JSON content + NULL padding + newline

**Example**:
```
{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":5000}\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\n
```

**Breakdown**:
- **Bytes 0-48**: JSON content (49 bytes in example)
- **Bytes 49-62**: NULL padding (14 bytes)
- **Byte 63**: Newline character

**Constraints**:
- JSON content must be 49-58 bytes
- NULL padding fills remaining bytes up to byte 62
- Total must be exactly 64 bytes

## API Methods

### Header Methods (Preserved)

**Getter Methods** (unchanged):
- `GetSignature() string`
- `GetVersion() int`
- `GetRowSize() int` 
- `GetSkewMs() int`

**Serialization Methods**:
- `MarshalText() []byte, error` - NEW: Replaces generateHeader()
- `UnmarshalText([]byte) error` - EXISTING: Preserved unchanged
- `Validate() error` - EXISTING: Preserved unchanged

## Constants and Helpers

**Header Constants** (moving to header.go):
- `HEADER_SIZE = 64`
- `HEADER_SIGNATURE = "fDB"`
- `MIN_ROW_SIZE = 128`
- `MAX_ROW_SIZE = 65536` 
- `MAX_SKEW_MS = 86400000`
- `PADDING_CHAR = '\x00'`
- `HEADER_NEWLINE = '\n'`
- `HEADER_FORMAT = {"sig":"fDB","ver":1,"row_size":%d,"skew_ms":%d}`

**Removed Function**:
- `generateHeader(rowSize, skewMs int) ([]byte, error)` - Replaced by Header.MarshalText()

## Impact Analysis

### No Entity Changes

- No new entity types introduced
- No field modifications to existing entities  
- No relationship structure changes
- Only implementation patterns and file organization affected

### API Compatibility

- All existing Header methods preserved
- Header struct fields unchanged
- Public API surface unchanged
- Backward compatibility maintained

### File Organization Changes

- Header struct and methods moved from create.go to header.go
- Header constants consolidated in header.go
- Helper struct headerJSON moved to header.go
- No import changes or circular dependencies