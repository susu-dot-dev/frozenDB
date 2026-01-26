# API Specification: NullRow Constructor and UUID Helpers

**Purpose**: Complete API specification for NullRow constructor and UUID utility functions.

## Constructor Functions

### NewNullRow

Creates a new NullRow with timestamp-aware UUID for the specified maxTimestamp.

```go
func NewNullRow(rowSize int, maxTimestamp int64) (*NullRow, error)
```

**Parameters:**
- `rowSize` (int): The fixed row size from database header (must be 128-65536)
- `maxTimestamp` (int64): The maximum timestamp of database at insertion time (must be non-negative)

**Returns:**
- `*NullRow`: A fully initialized NullRow ready for serialization
- `error`: InvalidInputError for invalid parameters

**Success Behavior:**
- Creates UUIDv7 with timestamp=maxTimestamp and zeroed random components
- Initializes NullRow with proper control characters (T/NR)
- Validates complete NullRow structure
- Returns ready-to-use NullRow

**Error Conditions:**
- Returns InvalidInputError if rowSize < 128 or > 65536
- Returns InvalidInputError if maxTimestamp < 0
- Returns InvalidInputError if UUID generation fails
- Returns InvalidInputError if validation fails after construction

**Thread Safety:** Safe (no shared state modification)
**Performance:** O(1) construction time, constant memory usage

## UUID Utility Functions

### Core UUID Functions

#### ValidateUUIDv7
```go
func ValidateUUIDv7(u uuid.UUID) *InvalidInputError
```
Validates that UUID conforms to UUIDv7 specification.

**Parameters:**
- `u` (uuid.UUID): UUID to validate

**Returns:**
- `*InvalidInputError`: nil if valid, error with details if invalid

**Validation Rules:**
- UUID cannot be zero/Nil
- Variant must be RFC 4122
- Version must be 7

#### ExtractUUIDv7Timestamp
```go
func ExtractUUIDv7Timestamp(u uuid.UUID) int64
```
Extracts timestamp component from UUIDv7.

**Parameters:**
- `u` (uuid.UUID): UUIDv7 to extract timestamp from

**Returns:**
- `int64`: Timestamp in milliseconds (first 48 bits of UUID)

**Thread Safety:** Safe (read-only operation)
**Performance:** O(1) bit manipulation

#### NewUUIDv7
```go
func NewUUIDv7() (uuid.UUID, error)
```
Creates new UUIDv7 with current timestamp.

**Returns:**
- `uuid.UUID`: New UUIDv7
- `error`: Error if UUID generation fails

**Thread Safety:** Safe (uses underlying library)
**Performance:** O(1) generation time

#### MustNewUUIDv7
```go
func MustNewUUIDv7() uuid.UUID
```
Creates new UUIDv7 with current timestamp, panics on failure.

**Returns:**
- `uuid.UUID`: New UUIDv7

**Usage:** For tests and initialization where failure is not acceptable

### NullRow-Specific UUID Functions

#### CreateNullRowUUID
```go
func CreateNullRowUUID(maxTimestamp int64) uuid.UUID
```
Creates UUIDv7 with specified timestamp and zeroed random components for NullRows.

**Parameters:**
- `maxTimestamp` (int64): Timestamp to embed in UUID

**Returns:**
- `uuid.UUID`: UUIDv7 with timestamp=maxTimestamp, random components=0

**Implementation Notes:**
- Sets first 6 bytes to timestamp (big-endian)
- Sets version bits to 7
- Sets variant bits to RFC 4122
- Zeroes remaining random bits

**Thread Safety:** Safe (no shared state)
**Performance:** O(1) bit manipulation

#### ValidateNullRowUUID
```go
func ValidateNullRowUUID(u uuid.UUID, maxTimestamp int64) *InvalidInputError
```
Validates UUID for NullRow usage with expected timestamp.

**Parameters:**
- `u` (uuid.UUID): UUID to validate
- `maxTimestamp` (int64): Expected timestamp in UUID

**Returns:**
- `*InvalidInputError`: nil if valid, error with details if invalid

**Validation Rules:**
- UUID must be valid UUIDv7
- Timestamp component must equal maxTimestamp
- Random components must be zeroed

#### IsNullRowUUID
```go
func IsNullRowUUID(u uuid.UUID) bool
```
Checks if UUID follows NullRow pattern (timestamp present, random components zeroed).

**Parameters:**
- `u` (uuid.UUID): UUID to check

**Returns:**
- `bool`: true if UUID matches NullRow pattern

**Usage:** For testing and identification purposes

### UUID Base64 Utilities

#### EncodeUUIDBase64
```go
func EncodeUUIDBase64(u uuid.UUID) ([]byte, error)
```
Encodes UUID to Base64 with validation.

**Parameters:**
- `u` (uuid.UUID): UUID to encode

**Returns:**
- `[]byte`: Base64 encoding (exactly 24 bytes)
- `error`: InvalidInputError if encoding fails

**Validation:**
- Input UUID must be valid UUIDv7
- Output must be exactly 24 bytes

#### DecodeUUIDBase64
```go
func DecodeUUIDBase64(data []byte) (uuid.UUID, error)
```
Decodes UUID from Base64 with validation.

**Parameters:**
- `data` ([]byte): Base64 encoded UUID (must be exactly 24 bytes)

**Returns:**
- `uuid.UUID`: Decoded UUID
- `error`: InvalidInputError if decoding fails

**Validation:**
- Input must be exactly 24 bytes
- Decoded UUID must be valid UUIDv7

#### ValidateBase64UUIDLength
```go
func ValidateBase64UUIDLength(data []byte) *InvalidInputError
```
Validates Base64 UUID encoding length.

**Parameters:**
- `data` ([]byte): Base64 data to validate

**Returns:**
- `*InvalidInputError`: nil if length is 24, error otherwise

**Thread Safety:** Safe (read-only operation)

## Integration Notes

### Usage Pattern
```go
// Get current maxTimestamp from finder
maxTimestamp := finder.MaxTimestamp()

// Create new NullRow with timestamp
nullRow, err := NewNullRow(rowSize, maxTimestamp)
if err != nil {
    return err
}

// Use NullRow for serialization or database operations
data, err := nullRow.MarshalText()
```

### Migration Pattern
Replace manual NullRow creation:
```go
// Old way (to be replaced)
nr := &NullRow{
    baseRow: baseRow[*NullRowPayload]{...},
}

// New way (to be used)
nr, err := NewNullRow(rowSize, maxTimestamp)
if err != nil {
    return err
}
```

### Error Handling
All functions use structured error handling:
- `InvalidInputError` for parameter validation failures
- Error messages are descriptive and actionable
- Error wrapping preserves context

### Performance Characteristics
- All operations are O(1) time complexity
- No additional memory allocations beyond normal UUID processing
- Thread-safe by design (no shared mutable state)
- Compatible with existing frozenDB performance constraints

This API provides a clean, centralized interface for UUID operations and NullRow construction while maintaining compatibility with existing frozenDB architecture.