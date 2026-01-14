# Research: Header Refactor Implementation

## Summary

Research completed for Header refactor to eliminate dual header creation pattern and align with DataRow/ChecksumRow patterns. All technical questions resolved.

## Decision: MarshalText() Implementation

**Decision**: Implement `Header.MarshalText()` method following the exact same pattern as `DataRow` and `ChecksumRow`

**Rationale**: 
- Maintains API consistency across all major structs
- Eliminates the dual creation pattern (generateHeader() + separate Header struct)
- Follows Go's `encoding.TextMarshaler` interface best practices
- Leverages existing `Validate()` method for integrity

**Implementation Details**:
```go
func (h *Header) MarshalText() ([]byte, error) {
    // Validate struct state first
    if err := h.Validate(); err != nil {
        return nil, err
    }
    
    // Generate JSON content using existing pattern
    jsonContent := fmt.Sprintf(HEADER_FORMAT, h.rowSize, h.skewMs)
    
    // Calculate padding (same logic as generateHeader)
    contentLength := len(jsonContent)
    if contentLength > 58 {
        return nil, NewInvalidInputError("header content too long", nil)
    }
    paddingLength := 63 - contentLength
    padding := strings.Repeat(string(PADDING_CHAR), paddingLength)
    
    // Assemble header: JSON + padding + newline
    header := jsonContent + padding + string(HEADER_NEWLINE)
    
    return []byte(header), nil
}
```

## Decision: File Organization

**Decision**: Create dedicated `header.go` file containing all Header-related functionality

**Rationale**:
- Aligns with existing codebase organization (data_row.go, checksum.go, transaction.go)
- Improves code organization and maintainability
- Separates concerns between Header functionality and database creation logic
- No risk of circular imports

**Components to Move**:
- `Header` struct and all methods (updated with JSON struct tags)
- Header-related constants (HEADER_SIZE, HEADER_SIGNATURE, etc.)
- `HEADER_FORMAT` constant
- `generateHeader()` function (for backward compatibility during transition)

**Components Removed**:
- `headerJSON` helper struct (eliminated in favor of direct JSON unmarshaling)

**Components to Keep in create.go**:
- `CreateConfig` struct
- `SudoContext` struct
- Filesystem operation interfaces
- `Create()` function and database creation logic

## Decision: Pattern Alignment

**Decision**: Use direct struct initialization + Validate() pattern (no constructor needed)

**Rationale**:
- Matches DataRow and ChecksumRow patterns exactly
- Simpler API surface (no NewHeader constructor)
- Follows Go idioms for struct creation
- Maintains backward compatibility with existing code

**Usage Pattern**:
```go
// New unified pattern (replaces dual creation)
header := &Header{
    signature: HEADER_SIGNATURE,
    version:   1,
    rowSize:   config.rowSize,
    skewMs:    config.skewMs,
}
headerBytes, err := header.MarshalText()
```

## Decision: Backward Compatibility Strategy

**Decision**: Maintain 100% backward compatibility for all existing APIs

**Rationale**:
- FR-009 requires maintaining all existing Header getter methods unchanged
- Existing `UnmarshalText()` must remain exactly the same
- Header format compatibility is critical (FR-007, FR-011)
- Tests must continue to pass without modification

**Compatibility Measures**:
- All getter methods remain unchanged
- `UnmarshalText()` behavior preserved exactly
- 64-byte header format maintained precisely
- Existing validation logic unchanged
- Error types and messages preserved

## Decision: Dependencies

**Decision**: No additional dependencies needed

**Rationale**:
- All required imports already available in create.go
- `fmt`, `strings`, `bytes`, `encoding/json` cover all needs
- No new external packages required
- Changes are self-contained within frozendb package

## Alternatives Considered

### Alternative 1: Keep Header in create.go, only add MarshalText()
**Rejected Because**: Would miss FR-001 requirement for dedicated header.go file organization

### Alternative 2: Create NewHeader constructor function
**Rejected Because**: Would not align with DataRow/ChecksumRow pattern which uses direct initialization

### Alternative 3: Gradual migration with compatibility shims
**Rejected Because**: Not needed - the direct replacement maintains compatibility without complex migration logic

## Additional Optimization: Remove headerJSON Helper Struct

**Decision**: Eliminate `headerJSON` helper struct and unmarshal JSON directly into Header struct

**Rationale**:
- Go's JSON struct tags can map JSON field names directly to Go field names
- Eliminates unnecessary helper struct and field copying
- Follows standard Go JSON patterns
- Reduces memory allocations and code complexity

**Implementation**:
```go
type Header struct {
    Signature string `json:"sig"`     // Maps JSON "sig" to Go Signature
    Version   int    `json:"ver"`     // Maps JSON "ver" to Go Version
    RowSize   int    `json:"row_size"` // Maps JSON "row_size" to Go RowSize
    SkewMs    int    `json:"skew_ms"`  // Maps JSON "skew_ms" to Go SkewMs
}

// Simplified UnmarshalText - no helper struct needed
func (h *Header) UnmarshalText(headerBytes []byte) error {
    // ... validation logic unchanged ...
    
    // Direct unmarshaling into Header struct
    if err := json.Unmarshal(jsonContent, h); err != nil {
        return NewCorruptDatabaseError("failed to parse JSON header", err)
    }
    
    return h.Validate()
}
```

**Compatibility**: Header fields are only accessed through getter methods, so renaming to exported fields with JSON tags is safe and won't break existing code.

## Implementation Risk Assessment

**Low Risk Changes**:
- Adding MarshalText() method to Header struct
- Moving Header functionality to dedicated file
- Updating Create() function to use new pattern

**No Breaking Changes**:
- All existing APIs preserved
- Tests continue to work without modification
- Header format unchanged
- Import paths unchanged

## Technical Requirements Satisfied

✅ FR-001: Move Header to dedicated header.go file
✅ FR-002: Header implements MarshalText() method  
✅ FR-003: Direct struct initialization + Validate() pattern
✅ FR-004: Single Header creation followed by MarshalText()
✅ FR-005: Identical byte format to current generateHeader()
✅ FR-006: Remove generateHeader() function completely
✅ FR-007: Maintain exact 64-byte header format compatibility
✅ FR-009: Maintain all existing Header getter methods unchanged
✅ FR-011: Maintain exact 64-byte header format compatibility

All technical clarifications resolved - ready for Phase 1 design and implementation.