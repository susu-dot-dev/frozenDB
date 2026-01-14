# frozenDB Header API Contract

## Header Struct

```go
type Header struct {
    Signature string `json:"sig"`     // Always "fDB"
    Version   int    `json:"ver"`     // Always 1 for v1 format
    RowSize   int    `json:"row_size"` // Size of each data row in bytes (128-65536)
    SkewMs    int    `json:"skew_ms"`  // Time skew window in milliseconds (0-86400000)
}
```

## Public API Methods

### Getter Methods

```go
func (h *Header) GetSignature() string
func (h *Header) GetVersion() int
func (h *Header) GetRowSize() int
func (h *Header) GetSkewMs() int
```

**Description**: Return header field values  
**Precondition**: Header struct is properly initialized  
**Postcondition**: Returns field values as stored in struct

### Serialization Methods

```go
func (h *Header) MarshalText() ([]byte, error)
func (h *Header) UnmarshalText([]byte) error
```

**MarshalText Description**: Converts Header struct to 64-byte header format  
**Precondition**: Header fields contain valid values  
**Postcondition**: Returns exactly 64 bytes: JSON + NULL padding + newline  
**Error Conditions**: Invalid header fields, content too long

**UnmarshalText Description**: Parses 64-byte buffer into Header struct  
**Precondition**: Input buffer is exactly 64 bytes  
**Postcondition**: Header struct populated with parsed values, Validate() called  
**Error Conditions**: Invalid buffer length, malformed JSON, invalid field values

### Validation Method

```go
func (h *Header) Validate() error
```

**Description**: Validates Header field values  
**Precondition**: Header struct fields are set  
**Postcondition**: Returns nil if valid, error with details if invalid  
**Validation Rules**:
- signature must equal "fDB"
- version must equal 1
- rowSize must be between 128 and 65536
- skewMs must be between 0 and 86400000

## Usage Patterns

### Header Creation (New Pattern)

```go
// Direct struct initialization + validation + marshaling
header := &Header{
    signature: "fDB",
    version:   1,
    rowSize:   1024,
    skewMs:    5000,
}

// Optional validation (MarshalText will validate automatically)
if err := header.Validate(); err != nil {
    return err
}

// Get 64-byte header representation
headerBytes, err := header.MarshalText()
if err != nil {
    return err
}
```

### Header Reading (Unchanged)

```go
// Read from file or buffer
headerBytes := make([]byte, 64)
n, err := file.Read(headerBytes)
if err != nil {
    return err
}

// Parse into Header struct (includes validation)
header := &Header{}
if err := header.UnmarshalText(headerBytes); err != nil {
    return err
}

// Access fields
rowSize := header.GetRowSize()
skewMs := header.GetSkewMs()
```

## Error Types

All methods return structured errors deriving from `frozendb.FrozenDBError`:

- `InvalidInputError`: Invalid parameter values, malformed input
- `CorruptDatabaseError`: Invalid header format during unmarshaling

## Constants

```go
const (
    HEADER_SIZE      = 64       // Fixed header size in bytes
    HEADER_SIGNATURE = "fDB"    // Signature string for format identification
    MIN_ROW_SIZE     = 128      // Minimum allowed row size
    MAX_ROW_SIZE     = 65536    // Maximum allowed row size
    MAX_SKEW_MS      = 86400000 // Maximum time skew (24 hours)
    PADDING_CHAR     = '\x00'   // Null character for header padding
    HEADER_NEWLINE   = '\n'     // Byte 63 must be newline
)
```

## Format Specification

### JSON Structure

```json
{
    "sig": "fDB",
    "ver": 1,
    "row_size": 1024,
    "skew_ms": 5000
}
```

### 64-Byte Layout

```
[JSON Content: 49-58 bytes][NULL Padding: variable][Newline: 1 byte]
Total: 64 bytes
```

**Example**:
```
{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":5000}\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\n
```

## Migration from generateHeader()

**Before**:
```go
headerBytes, err := generateHeader(rowSize, skewMs)
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1,
    rowSize:   rowSize,
    skewMs:    skewMs,
}
```

**After**:
```go
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1,
    rowSize:   rowSize,
    skewMs:    skewMs,
}
headerBytes, err := header.MarshalText()
```

## Backward Compatibility

- All existing API methods preserved
- MarshalText() output identical to generateHeader()
- UnmarshalText() behavior unchanged
- Error handling patterns preserved
- 64-byte format maintained exactly