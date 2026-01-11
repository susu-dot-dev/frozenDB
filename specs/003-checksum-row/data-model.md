# Data Model: Checksum Row Implementation

## Core Data Types

### Exported Row Types

#### ChecksumRow
```go
type ChecksumRow struct {
    baseRow[Checksum] // Embedded with Checksum payload type
}
```

### baseRow (Generic - Internal)
```go
// Generic baseRow with type parameter P constrained to RowPayload
type baseRow[P RowPayload] struct {
    Header       *Header      // Header reference for row_size and configuration
    StartControl StartControl // Single byte control character
    EndControl   EndControl   // Two-byte end control sequence
    RowPayload   P            // Typed payload data, no interface needed
}
```

**Note**: `baseRow` is unexported and used internally by specific row types.

**Key Methods** (Internal):
- `MarshalText() ([]byte, error)` - Serialize row to exact byte format
- `UnmarshalText(text []byte) error` - Deserialize from bytes with automatic validation
- `validate() error` - Internal verification of row structure and integrity
- `GetParity() string` - Calculate XOR parity bytes dynamically
- `PaddingLength() int` - Calculate required null byte padding

**Note**: These methods are unexported and called by exported row type methods.

### StartControl (Type-safe Enum)
```go
type StartControl byte

const (
    START_TRANSACTION StartControl = 'T' // Transaction begin
    ROW_CONTINUE      StartControl = 'R' // Row continuation  
    CHECKSUM_ROW      StartControl = 'C' // Checksum row
)
```

**Methods**:
- `MarshalText() ([]byte, error)` - Convert to single byte
- `UnmarshalText(text []byte) error` - Parse and validate single byte

### EndControl (2-byte Array)
```go
type EndControl [2]byte // 2-byte array: [T,C], [R,E], [C,S], [S,E], [R,0-9], [S,0-9]
```

**Valid Patterns**:
- Checksum rows: [C,S] (checksum with savepoint)
- Transaction commit: [T,C] (transaction complete)
- Row continue: [R,E] (row end, transaction continues)
- Rollback: [R,0-9] (rollback to savepoint N)
- Savepoint rollback: [S,0-9] (rollback to savepoint N with savepoint)

**Methods**:
- `MarshalText() ([]byte, error)` - Convert 2-byte array to slice
- `UnmarshalText(text []byte) error` - Parse and validate 2-byte sequence into array
- `String() string` - Convert to string representation for display/debugging

### RowPayload Interface
```go
type RowPayload interface {
    encoding.TextMarshaler
    encoding.TextUnmarshaler
}
```

**Implementations**:
- `Checksum` - CRC32 checksum for checksum rows

### Checksum Type
```go
type Checksum uint32
```

**Methods**:
- `MarshalText() ([]byte, error)` - Convert to 8-character Base64 string
- `UnmarshalText(text []byte) error` - Parse 8-character Base64 to uint32

### ChecksumRow Methods

**Constructor**:
- `NewChecksumRow(header *Header, dataBytes []byte) (*ChecksumRow, error)` - Create from data block with automatic validation

**Accessors**:
- `GetChecksum() Checksum` - Direct access, no type assertion needed

**Internal Methods**:
- `validate() error` - Internal verification of checksum row specific requirements

**Note**: Serialization methods (MarshalText, UnmarshalText) are inherited from embedded baseRow.


## Function Signatures

### Core Serialization Functions
```go
// baseRow serialization (generic methods work for any P)
func (br *baseRow[P]) MarshalText() ([]byte, error)
func (br *baseRow[P]) UnmarshalText(text []byte) error
func (br *baseRow[P]) validate() error // Internal validation

// Checksum-specific operations
func NewChecksumRow(header *Header, dataBytes []byte) (*ChecksumRow, error)
func (cr *ChecksumRow) validate() error // Internal validation
func (cr *ChecksumRow) GetChecksum() Checksum // Direct return, no error

// Parity and padding calculations (generic methods)
func (br *baseRow[P]) GetParity() string
func (br *baseRow[P]) PaddingLength() int
```

## Key Validation Rules

### Row Structure Requirements
- ROW_START sentinel (0x1F) at position 0
- ROW_END sentinel (0x0A) at position row_size-1
- StartControl at position 1 must be valid enum value
- EndControl at positions row_size-5, row_size-4 must be valid combination

### ChecksumRow Specific Requirements
- StartControl must be 'C' (CHECKSUM_ROW constant)
- EndControl must be [C,S] (checksum-specific combination)
- CRC32 covers all bytes since previous checksum row
- Parity calculated on all bytes except last 2 (parity bytes themselves)
- No UUID in checksum rows

### File Format Compliance
- First checksum row at offset 64 (immediately after header)
- Maximum 10,000 data rows between checksum rows
- CRC32 uses IEEE polynomial 0xedb88320
- Checksum encoded as 8-character Base64 with "==" padding

## Generic Type Safety Benefits

The generic `baseRow[P RowPayload]` design provides several advantages:

### Type Safety Without Casting
```go
// Direct access, no casting needed
checksum := cr.GetChecksum() // Returns Checksum directly
```

### Method Specialization
Each row type can provide type-specific convenience methods:
```go
func (cr *ChecksumRow) GetChecksum() Checksum {
    return cr.RowPayload // Direct return, no error
}

```

## Data Relationships

```
Header (64 bytes)
    ↓
ChecksumRow (row_size bytes)
    ↓
DataRow (row_size bytes each) [0-9999]
    ↓
ChecksumRow (row_size bytes)
    ↓
DataRow (row_size bytes each) [10000-19999]
```

**Key Relationships**:
- Each ChecksumRow covers up to 10,000 preceding data rows
- First ChecksumRow is mandatory and immediately follows header
- Subsequent ChecksumRows appear before row 10,001, 20,001, etc.
- CRC32 calculation excludes header but includes all data rows in range
