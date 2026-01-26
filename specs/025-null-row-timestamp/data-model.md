# Data Model: NullRow Timestamp Modification

**Purpose**: New data entities, validation rules, and state changes introduced by feature.

## Modified Entity: NullRow

### Overview
The NullRow entity is modified to use timestamp-aware UUIDs instead of uuid.Nil, while maintaining all other structural characteristics from the v1 file format specification.

### Entity Definition

```go
type NullRow struct {
    baseRow[*NullRowPayload]
}

type NullRowPayload struct {
    Key uuid.UUID  // Previously uuid.Nil, now UUIDv7 with maxTimestamp
}
```

### Validation Rules

**Parameter Validation:**
- `rowSize` must be between 128-65536 bytes (file format constraint)
- `maxTimestamp` must be non-negative (timestamp constraint)

**Structural Validation:**
- StartControl must be `START_TRANSACTION` ('T')
- EndControl must be `NULL_ROW_CONTROL` ('NR')
- UUID must be valid UUIDv7 format
- UUID timestamp component must equal provided `maxTimestamp`
- UUID random components must be zeroed (deterministic NullRows)
- Base64 encoding must be exactly 24 bytes
- Parity bytes must calculate correctly for row integrity

### State Changes

**Before Modification:**
- NullRow.Key = uuid.Nil (all zeros)
- Base64 encoding = "AAAAAAAAAAAAAAAAAAAAAA=="
- Timestamp extraction returns 0

**After Modification:**
- NullRow.Key = UUIDv7(maxTimestamp, 0, 0, 0, 0)
- Base64 encoding varies based on maxTimestamp
- Timestamp extraction returns maxTimestamp
- Maintains all other NullRow properties

## New Entity: UUID Helpers

### Overview
Centralized UUID utility functions to replace scattered functionality across multiple files.

### Entity Definition

```go
// UUID creation and validation functions
func ValidateUUIDv7(u uuid.UUID) *InvalidInputError
func ExtractUUIDv7Timestamp(u uuid.UUID) int64
func NewUUIDv7() (uuid.UUID, error)
func MustNewUUIDv7() uuid.UUID

// NullRow-specific UUID functions
func CreateNullRowUUID(maxTimestamp int64) uuid.UUID
func ValidateNullRowUUID(u uuid.UUID, maxTimestamp int64) *InvalidInputError
func IsNullRowUUID(u uuid.UUID) bool

// UUID Base64 encoding utilities
func EncodeUUIDBase64(u uuid.UUID) ([]byte, error)
func DecodeUUIDBase64(data []byte) (uuid.UUID, error)
func ValidateBase64UUIDLength(data []byte) *InvalidInputError
```

### Validation Rules

**UUIDv7 Validation:**
- UUID cannot be zero/Nil
- Variant must be RFC 4122
- Version must be 7

**NullRow UUID Validation:**
- Must be valid UUIDv7
- Timestamp component must equal specified maxTimestamp
- Random components must be zeroed

**Base64 Validation:**
- Input must be exactly 24 bytes
- Must decode to valid UUIDv7

## Relationships and Data Flow

### NullRow Creation Flow

1. **Input Parameters**: `rowSize` (int), `maxTimestamp` (int64)
2. **Parameter Validation**: Check rowSize range and maxTimestamp non-negative
3. **UUID Generation**: Create UUIDv7 with maxTimestamp and zeroed random components
4. **Struct Construction**: Initialize NullRow with proper control characters
5. **Final Validation**: Validate complete NullRow structure
6. **Return**: Initialized NullRow or error

### Integration with Existing Components

**Finder Integration:**
- SimpleFinder/InMemoryFinder receive NullRows via `OnRowAdded()`
- `MaxTimestamp()` method continues to work with new UUID format
- No changes needed to finder implementations

**Transaction Integration:**
- NullRow UUID validation uses new centralized functions
- Timestamp ordering validation unchanged (already uses `extractUUIDv7Timestamp`)
- Transaction atomicity preserved

**File Format Integration:**
- Marshal/Unmarshal operations work with new UUID values
- Base64 encoding/decoding uses centralized utilities
- Row size calculations unchanged

## Error Conditions

**Constructor Errors:**
- `InvalidInputError` for invalid rowSize (<128 or >65536)
- `InvalidInputError` for negative maxTimestamp
- `InvalidInputError` for UUID generation failures
- `InvalidInputError` for validation failures

**UUID Validation Errors:**
- `InvalidInputError` for non-UUIDv7 format
- `InvalidInputError` for timestamp mismatches
- `InvalidInputError` for Base64 encoding/decoding failures

**State Transition Errors:**
- Validation failures during NullRow construction
- UUID format violations during serialization
- Parity calculation failures during marshaling

## Constraint Implications

**Memory Usage:**
- No additional memory usage introduced
- UUID field size unchanged (16 bytes)
- Constructor uses stack allocation only

**Performance Impact:**
- UUID generation is O(1) bit manipulation
- No additional database lookup required
- Finder performance unchanged (O(1) maxTimestamp)

**File Format Compliance:**
- Maintains fixed-width row structure
- Preserves control character requirements
- Follows v1_file_format.md specification

## Migration Requirements

**Code Changes:**
- Move existing UUID functions to `uuid_helpers.go`
- Update NullRow to use new constructor
- Remove duplicate UUID validation/encoding logic
- Update import statements across codebase

**Test Updates:**
- Tests use `NewNullRow()` constructor instead of manual creation
- UUID helper functions replace scattered implementations
- Validation tests cover new error conditions

This data model maintains compatibility with existing frozenDB architecture while implementing the required timestamp-aware NullRow functionality.