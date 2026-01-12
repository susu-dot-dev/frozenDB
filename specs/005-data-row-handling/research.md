# DataRow Implementation Research

**Feature**: 005-data-row-handling  
**Date**: 2026-01-11  
**Research Focus**: ChecksumRow patterns, UUIDv7 validation, and implementation approach

## Research Summary

This document captures research findings for implementing DataRow handling in frozenDB, building on the established ChecksumRow architecture while adding UUIDv7 key validation and JSON payload handling.

## Key Research Areas

### 1. ChecksumRow Architecture Analysis

**Decision**: Adopt the three-file architecture and generic baseRow patterns from ChecksumRow

**Rationale**: ChecksumRow provides a proven, tested foundation that maintains consistency across frozenDB codebase. The generic `baseRow[T]` approach enables type safety while sharing file format validation logic.

**Key Patterns to Replicate**:
- Three-file organization: `data_row.go`, `data_row_test.go`, `data_row_spec_test.go`
- Generic `baseRow[T]` embedding for shared validation
- Hierarchical validation: `baseRow.Validate()` → `DataRow.Validate()`
- Structured error handling with `InvalidInputError` and `CorruptDatabaseError`
- Table-driven unit tests with comprehensive edge cases
- Spec tests following `Test_S_005_FR_XXX_Description()` naming convention
- Correct control characters per v1_file_format.md: T/R for start, TC/RE/SC/SE/R0-R9/S0-S9 for end

### 2. UUIDv7 Validation Strategy

**Decision**: Use github.com/google/uuid package with comprehensive validation

**Rationale**: The github.com/google/uuid package is the de facto standard for UUID handling in Go, providing robust parsing, validation, and version checking capabilities. It integrates well with frozenDB's existing error handling patterns.

**Transaction State Handling**: DataRow validates that start and end controls are valid for a single row. Multi-row transaction validation (like ensuring proper T/R sequences within a transaction) is handled at a higher layer (transaction manager).

**Implementation Approach**:
```go
func ValidateUUIDv7(u uuid.UUID) *InvalidInputError {
    if u.Variant() != uuid.RFC4122 {
        return NewInvalidInputError("UUID must be RFC 4122 variant", nil)
    }
    if u.Version() != 7 {
        return NewInvalidInputError("UUID must be version 7", nil)
    }
    return nil
}
```

**Key Validation Points**:
- RFC 4122 variant requirement (reject Microsoft/NCS variants)
- Version 7 requirement (reject v1, v4, v6, etc.)
- Base64 encoding/decoding with proper 24-byte format and "=" padding
- Early length and format validation for performance

### 3. UUID Variant vs Version Understanding

**Decision**: Validate both Variant (RFC 4122) and Version (7) for strict compliance

**Rationale**: frozenDB requires time-ordered keys for optimal performance, which necessitates UUIDv7 specifically. RFC 4122 variant ensures standard encoding and behavior.

**Key Differences**:
- **Variant**: Layout/encoding scheme (RFC 4122, Microsoft, NCS, Future)
- **Version**: Generation algorithm within RFC 4122 (v1=time+MAC, v4=random, v7=time-ordered)

### 4. JSON Payload Handling Strategy

**Decision**: Accept JSON string values without syntax validation at DataRow layer

**Rationale**: Feature specification clarifies that DataRow should expect JSON strings but not validate syntax - caller responsible for validation. This maintains separation of concerns and performance.

**Implementation Considerations**:
- Store as raw string with NULL_BYTE padding
- No JSON parsing or validation in DataRow
- Preserves original formatting and whitespace
- Enables caller to handle JSON deserialization as needed

### 5. Base64 Encoding Integration

**Decision**: Use standard library encoding/base64 for UUID key serialization

**Rationale**: v1_file_format.md specifies 24-byte Base64 with "=" padding for UUID keys. Standard library provides reliable, efficient implementation.

**Integration Points**:
- UUID to Base64: `base64.StdEncoding.EncodeToString(uuid[:])` (24 bytes)
- Base64 to UUID: `base64.StdEncoding.DecodeString()` with length validation
- Input validation for 24-character length with proper padding

### 6. Error Handling Patterns

**Decision**: Follow existing frozenDB error patterns with structured errors

**Rationale**: Consistency with ChecksumRow and existing codebase enables predictable error handling and debugging.

**Error Types to Use**:
- `InvalidInputError`: For validation failures (wrong UUID version, invalid controls)
- `CorruptDatabaseError`: For integrity failures (parity mismatch, sentinels)
- All errors embed base `FrozenDBError` with descriptive messages

## Technology Choices

### Dependencies
- **Primary**: github.com/google/uuid (UUIDv7 validation)
- **Secondary**: Go standard library only (encoding/base64, encoding/json)

### Performance Considerations
- Fixed memory usage regardless of database size
- Early validation checks to minimize processing
- Efficient Base64 encoding/decoding for 16-byte UUIDs
- LRC parity calculation using XOR algorithm

## Integration with Existing Architecture

### baseRow Integration
DataRow will embed `baseRow[*DataRowPayload]` following ChecksumRow patterns:
- `baseRow` handles file format structure validation
- `DataRow` handles context-specific validation
- Shared sentinel bytes and parity calculation

### File Format Compliance
Follow v1_file_format.md exactly:
- ROW_START (0x1F) and ROW_END (0x0A) sentinels
- DataRow StartControl: 'T' (transaction begin) or 'R' (row continuation)
- DataRow EndControl: TC/RE/SC/SE/R0-R9/S0-S9 (transaction state dependent)
- ChecksumRow StartControl='C' and EndControl='CS'
- Base64-encoded UUIDv7 keys (24 bytes)
- JSON string payload with NULL_BYTE padding
- LRC parity bytes for integrity

### Testing Strategy
- Unit tests: Method-specific validation and edge cases
- Spec tests: Functional requirement validation with `Test_S_005_FR_XXX_Description()` naming
- Round-trip testing: Marshal/Unmarshal preservation
- Error case testing: Comprehensive invalid input handling

## Alternatives Considered

### UUID Libraries
- **Alternative**: Custom UUID parsing
- **Rejected**: Increased complexity and maintenance burden
- **Chosen**: github.com/google/uuid for standard compliance and reliability

### JSON Validation
- **Alternative**: Validate JSON syntax in DataRow
- **Rejected**: Specification indicates caller responsibility
- **Chosen**: Store raw JSON strings without validation

### Error Handling
- **Alternative**: Simple string errors
- **Rejected**: Inconsistent with existing codebase
- **Chosen**: Structured errors with FrozenDBError base

## Implementation Risks and Mitigations

### Risk 1: UUID Validation Complexity
**Mitigation**: Use proven github.com/google/uuid library with comprehensive testing

### Risk 2: File Format Compliance
**Mitigation**: Follow ChecksumRow patterns exactly and test against v1_file_format.md

### Risk 3: Performance Impact
**Mitigation**: Early validation checks and fixed memory usage patterns

## Next Steps

1. Create DataRow struct following ChecksumRow patterns
2. Implement UUIDv7 validation with github.com/google/uuid
3. Add JSON payload handling with NULL_BYTE padding
4. Implement serialization according to v1_file_format.md
5. Create comprehensive unit and spec tests
6. Validate integration with existing frozenDB architecture

This research provides a solid foundation for implementing DataRow that maintains consistency with existing patterns while meeting all functional requirements.