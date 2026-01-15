# Research Document: PartialDataRow Implementation

**Date**: 2026-01-14  
**Feature**: 009-partial-data-row  
**Purpose**: Research existing patterns and resolve technical unknowns for PartialDataRow implementation

## Language and Framework Decisions

### Go Version and Dependencies
**Decision**: Go 1.25.5 + Go standard library only + github.com/google/uuid  
**Rationale**: Based on existing codebase analysis and AGENTS.md guidelines, frozenDB is a Go 1.25.5 project that exclusively uses the standard library plus the UUID library for UUIDv7 handling. No additional dependencies should be introduced.  
**Alternatives considered**: 
- Using only standard library (rejected - need UUIDv7 support)
- Adding additional dependencies (rejected - violates project principle)

### Project Type and Testing Framework
**Decision**: Single-project Go library with Go's built-in testing framework  
**Rationale**: frozenDB is a single-file key-value store library. The existing codebase uses Go's standard `go test` framework with table-driven tests and spec tests following the Test_S_XXX naming convention.  
**Alternatives considered**: 
- External test frameworks (rejected - overkill for this use case)
- Multiple packages (rejected - single cohesive library is better)

### Target Platform and Performance Goals
**Decision**: Linux server environment, fixed memory usage, O(1) row seeking  
**Rationale**: frozenDB is designed for server environments with fixed memory requirements regardless of database size. The append-only architecture with fixed-width rows enables O(1) seeking.  
**Constraints**: Memory must remain fixed regardless of database size, performance optimizations must not compromise correctness.

### Storage Architecture
**Decision**: Single-file append-only database (.fdb extension)  
**Rationale**: This is fundamental to frozenDB's architecture and specified in the constitution. The single-file design enables simple backup and recovery.  
**Alternatives considered**: Multi-file storage (rejected - violates architectural principles).

## Existing Codebase Patterns Analysis

### DataRow Validation and Composition Patterns

**Current Pattern**: DataRow uses generic baseRow composition
```go
type DataRow struct {
    baseRow[*DataRowPayload] // Embedded generic foundation
}
```

**Key Insights**:
- **Two-tier validation**: `baseRow.Validate()` + `DataRow.Validate()`
- **Validation delegation**: Specific validation delegated to sub-fields
- **PreDataRow + Validate() pattern**: Create struct first, then validate
- **MarshalText/UnmarshalText**: Delegation to baseRow with validation

### Error Hierarchy Analysis

**Existing Error Types**:
- `FrozenDBError` (base struct)
- `InvalidInputError` (for creation/validation errors)
- `CorruptDatabaseError` (for file corruption)
- `PathError`, `WriteError` (context-specific)

**Missing**: `InvalidActionError` (required by spec - need to implement)

### State Management Patterns

**Existing Enum Pattern**:
```go
type StartControl byte
const (
    START_TRANSACTION StartControl = 'T'
    ROW_CONTINUE      StartControl = 'R'
    CHECKSUM_ROW      StartControl = 'C'
)
```

**Recommended PartialDataRow State Pattern**:
```go
type PartialRowState int
const (
    State1 PartialRowState = iota  // ROW_START + START_CONTROL only
    State2                          // State1 + UUID + JSON + padding
    State3                          // State2 + 'S' character
)
```

## Technical Design Decisions

### Composition Strategy: Wrapper Pattern

**Decision**: Use composition with DataRow + state enum as suggested by user  
**Rationale**: 
- **DRY principle**: Reuses DataRow validation logic
- **State awareness**: Conditional validation based on current state  
- **Delegation**: Natural method forwarding through embedding
- **Testability**: Each component testable independently

**Implementation Pattern**:
```go
type PartialDataRow struct {
    State PartialRowState
    DataRow
}

func (p *PartialDataRow) Validate() error {
    return p.DataRow.validateFields(p.State)
}
```

### Validation Strategy

**Decision**: PartialDataRow implements state-aware validation directly  
**Rationale**: 
- **Proper dependency direction**: PartialDataRow depends on DataRow, not vice versa
- **No inappropriate coupling**: DataRow doesn't need to know about PartialDataRow states
- **Clear responsibility**: PartialDataRow validates its own state-dependent fields

**Implementation Strategy**:
```go
func (p *PartialDataRow) Validate() error {
    switch p.State {
    case PartialDataRowWithStartControl:
        return p.validateStartOnly()
    case PartialDataRowWithPayload:
        return p.validateKeyData()
    case PartialDataRowWithSavepoint:
        return p.validateSavepointIntent()
    }
}

func (p *PartialDataRow) validateStartOnly() error {
    // Validate ROW_START and START_CONTROL only
}

func (p *PartialDataRow) validateKeyData() error {
    // Validate all StartOnly + UUID + JSON + padding
}

func (p *PartialDataRow) validateSavepointIntent() error {
    // Validate all KeyData + 'S' character
}
```

### State Transition Implementation

**Decision**: State-based validation in transition methods  
**Rationale**: 
- Ensures only valid transitions are allowed
- Provides clear error messages for invalid attempts
- Maintains state immutability (no reverting to lower states)

**Transition Methods**:
```go
func (p *PartialDataRow) AddRow(key uuid.UUID, json string) error {
    if p.State != State1 {
        return NewInvalidActionError("AddRow() can only be called from State1")
    }
    // Update DataRow fields, transition to State2
    p.State = State2
    return p.Validate()
}

func (p *PartialDataRow) Savepoint() error {
    if p.State != State2 {
        return NewInvalidActionError("Savepoint() can only be called from State2")
    }
    // Set 'S' character, transition to State3
    p.State = State3
    return p.Validate()
}
```

### Serialization Strategy

**Decision**: State-aware MarshalText() with calculated padding  
**Rationale**: 
- Different states have different byte lengths
- State2 and State3 use full DataRow padding calculation
- State1 has minimal structure (no padding)

**Implementation Approach**:
```go
func (p *PartialDataRow) MarshalText() ([]byte, error) {
    switch p.State {
    case State1:
        return p.marshalState1()
    case State2:
        return p.marshalState2()
    case State3:
        return p.marshalState3()
    }
}
```

### Error Handling Strategy

**Decision**: Follow existing error hierarchy with new InvalidActionError  
**Rationale**: 
- Maintains consistency with existing patterns
- Provides appropriate error types for different failure modes
- Enables proper error wrapping as specified

**Error Usage**:
- `InvalidActionError`: Invalid state transitions
- `InvalidInputError`: Validation errors during object creation
- `CorruptDatabaseError`: Wraps validation errors from UnmarshalText()

## File Format Integration

### PartialDataRow vs DataRow Distinction

**Key Findings**:
- PartialDataRows are variable-length (stop at state boundary)
- DataRows are fixed-width (full row_size)
- PartialDataRows can ONLY exist as last row in file
- Both use same validation rules for present fields

### Byte Layout Analysis

**State 1**: 2 bytes (ROW_START + start_control)  
**State 2**: 27+ bytes (State1 + 24-byte UUID + JSON + calculated padding)  
**State 3**: State2 + 1 byte ('S' character)

**Padding Calculation**: Use full DataRow formula for States 2+3:
```
padding_bytes = row_size - len(json_payload) - 31
```

## Dependencies and Integration

### No Additional Dependencies Required

**Decision**: Use only existing dependencies  
**Rationale**: 
- All required functionality exists in codebase
- UUIDv7 validation already implemented
- JSON handling uses standard library
- Error handling follows existing patterns

### Integration Points

**Key Integration Points**:
- `ValidateUUIDv7()` function (already implemented)
- Base64 encoding/decoding (standard library)
- Parity calculation (existing baseRow implementation)
- Header structure (existing Header type)

## Performance Considerations

### Fixed Memory Constraint

**Approach**: Follow existing memory patterns  
- Use fixed-size buffers for serialization
- Avoid dynamic memory allocation in hot paths
- Leverage existing baseRow patterns

### Validation Performance

**Strategy**: Efficient state-based validation  
- Skip expensive validation for fields not present in current state
- Cache validation results where appropriate
- Use efficient string/byte operations

## Testing Strategy

### Spec Testing Requirements

**Approach**: Follow existing Test_S_XXX pattern  
- Each functional requirement gets corresponding spec test
- Tests placed in `[filename]_spec_test.go` files
- Table-driven tests for multiple scenarios

### Unit Testing Strategy

**Coverage Areas**:
- State transition validation
- MarshalText/UnmarshalText for each state
- Error handling for invalid transitions
- Edge cases (invalid UUID, malformed JSON, etc.)

## Implementation Plan Summary

### Phase 1: Core Structure
1. Implement InvalidActionError type
2. Create PartialRowState enum
3. Add validateField() method to DataRow
4. Implement PartialDataRow wrapper struct

### Phase 2: State Management
1. Implement state transition methods (AddRow, Savepoint)
2. Add validation for each state
3. Implement completion methods (Commit, Rollback, EndRow)

### Phase 3: Serialization
1. Implement state-aware MarshalText()
2. Implement UnmarshalText() with error wrapping
3. Add padding calculation for States 2+3

### Phase 4: Testing
1. Create spec tests for all functional requirements
2. Add comprehensive unit tests
3. Verify integration with existing codebase

This research resolves all technical unknowns and provides a clear path forward for implementing PartialDataRow following frozenDB's established patterns while meeting all specification requirements.