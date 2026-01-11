# Research: Struct Validation and Immutability Patterns

## Summary

This research consolidates findings about all structs in the frozenDB codebase to implement the 004 struct validation and immutability feature. The analysis identified which structs need validation patterns, which can have invalid states, and what field immutability changes are required.

## Decision: Implement Validation Pattern for Core Structs

**Rationale**: The codebase has partial validation implementation but lacks systematic consistency across all structs that can have invalid states. Core database structs like Header, FrozenDB, and row processing components need standardized validation to ensure data integrity.

**Alternatives considered**: 
- Keep existing scattered validation (rejected: inconsistent error handling)
- Use external validation library (rejected: adds dependencies, Go-only project)

## Current State Analysis

### Structs Requiring 004 Validation Pattern

| Struct | Current State | Validation Needed | Field Changes |
|--------|---------------|-------------------|---------------|
| **Header** | Validated via `parseHeader()` function | ✅ Validate() method + constructor integration | ❌ Exported → unexported + getters |
| **FrozenDB** | No validation method | ✅ Validate() for internal consistency | ✅ Already unexported |
| **CreateConfig** | ✅ Has Validate() + constructor calls it | ⚡ Standardize method name consistency | ⚡ Add getters for external access |
| **SudoContext** | No validation method | ✅ Validate() method + constructor integration | ❌ Exported → unexported + getters |
| **StartControl** | ✅ UnmarshalText() validation only | ✅ Validate() method + constructor integration | ✅ Already unexported |
| **EndControl** | ✅ UnmarshalText() validation only | ✅ Validate() method + constructor integration | ✅ Already unexported |
| **Checksum** | ✅ UnmarshalText() validation only | ✅ Validate() method + constructor integration | ✅ Already unexported |

### Structs with Existing Validation (Standardization Needed)

| Struct | Current State | Required Change |
|--------|---------------|-----------------|
| **baseRow[T]** | Private `validate()` method | Export to `Validate()` |
| **ChecksumRow** | Private `validate()` method | Export to `Validate()` |

### Structs Needing No Changes

| Struct | Reason |
|--------|--------|
| **FrozenDBError** and variants | Error wrappers, always valid |
| **fsOperations** | Interface implementation |
| **headerJSON** | Internal parsing helper |

## Technical Implementation Details

### Constructor Pattern Integration

All constructors must call Validate() before returning:

```go
// Pattern 1: Constructor function
func NewHeader(data []byte) (*Header, error) {
    header := &Header{...parsed fields...}
    return header, header.Validate()
}

// Pattern 2: UnmarshalText integration
func (sc *StartControl) UnmarshalText(text []byte) error {
    // ... existing parsing logic ...
    *sc = parsedValue
    return sc.Validate()
}

// Pattern 3: Direct initialization requirement
header := &Header{signature: "...", version: 1, ...}
if err := header.Validate(); err != nil {
    return err
}
```

### Field Immutability Pattern

Exported fields → unexported + getter functions:

```go
// Before
type Header struct {
    Signature string
    Version   int
    RowSize   int
    SkewMs    int
}

// After
type Header struct {
    signature string
    version   int
    rowSize   int
    skewMs    int
}

func (h *Header) GetSignature() string { return h.signature }
func (h *Header) GetVersion() int { return h.version }
func (h *Header) GetRowSize() int { return h.rowSize }
func (h *Header) GetSkewMs() int { return h.skewMs }
```

### Parent-Child Validation Responsibilities

- **Child structs** validate their own fields during construction
- **Parent structs** assume child validity, check contextual requirements
- **Example**: ChecksumRow requires StartControl='C' (parent contextual validation)

## Architecture Compliance

### frozenDB Constitution Alignment

✅ **Immutability First**: Field immutability prevents post-construction modification  
✅ **Data Integrity**: Struct validation ensures only valid data enters database  
✅ **Correctness Over Performance**: Validation prioritizes correctness over speed  
✅ **Spec Test Compliance**: All FR requirements will have corresponding spec tests  

### File Format Compliance

Based on v1_file_format.md analysis:
- Header validation must enforce signature, version, row_size, skew_ms constraints
- Row control validation must enforce transaction state machine rules
- Checksum validation must maintain CRC32 integrity requirements

## Integration Points

### Existing Validation Logic

The following existing validation will be consolidated into Validate() methods:

1. **Header**: `validateHeaderFields()` function → `Header.Validate()`
2. **CreateConfig**: Existing `Validate()` method (already compliant)
3. **Row controls**: UnmarshalText() validation → separate Validate() methods
4. **SudoContext**: No existing validation → new Validate() method

### Spec Test Requirements

Per docs/spec_testing.md, each FR-XXX requirement needs spec tests in `*_spec_test.go` files:

- `header_spec_test.go` for Header validation
- `frozendb_spec_test.go` for FrozenDB validation  
- `create_spec_test.go` for CreateConfig validation
- `row_spec_test.go` for row control validation

## Performance Considerations

### Validation Cost Analysis

- **Header validation**: One-time cost during file opening
- **Row validation**: Per-row cost during database operations
- **Getter functions**: Zero allocation, direct field access

### Memory Impact

- **Validation methods**: No additional memory allocations
- **Getter functions**: Inline-friendly, no heap allocation
- **Field immutability**: Reduces need for repeated validation checks

## Implementation Priority

### Phase 1: Core Database Validation
1. **Header** (P1) - Critical for all database operations
2. **CreateConfig** (P1) - Already mostly compliant, minimal changes
3. **SudoContext** (P1) - Security-critical for privileged operations

### Phase 2: Row Processing Validation
4. **StartControl/EndControl/Checksum** (P2) - Fundamental row components
5. **baseRow[T]/ChecksumRow** (P2) - Standardize existing validation

### Phase 3: Field Immutability
6. **Field visibility changes** (P3) - Convert exported fields to unexported + getters

## Risk Assessment

### Low Risk
- CreateConfig (already has validation)
- baseRow[T]/ChecksumRow (just method name changes)
- Row controls (simple validation logic)

### Medium Risk  
- Header (critical, validation logic exists but needs refactoring)
- SudoContext (new validation, but simple field checks)

### Mitigation Strategies
- Comprehensive spec test coverage for all FR requirements
- Gradual implementation with testing at each step
- Preserve all existing validation behavior during refactoring