# Research: NullRow Struct Implementation

**Feature**: 010-null-row-struct  
**Date**: 2025-01-18  
**Status**: Complete

## Research Findings

### Error Handling Patterns

**Decision**: Follow existing struct validation patterns established in frozenDB codebase

**Rationale**: 
- Consistency with existing codebase patterns
- InvalidInputError for validation failures matches the pattern used in struct_validation.go
- CorruptDatabaseError for unmarshal failures aligns with database corruption handling patterns
- Maintains error handling excellence per constitution requirements

**Alternatives Considered**:
- Custom error types specifically for NullRow - Rejected due to unnecessary complexity
- Generic Go errors - Rejected due to lack of structured information
- Wrapped errors with custom messages - Rejected due to inconsistency with existing patterns

### File Format Compliance

**Decision**: Implement NullRow according to v1 file format specification section 8.7

**Rationale**:
- NullRow structure must exactly match specification: start_control='T', end_control='NR', UUID with timestamp equal to max_timestamp, other fields zero
- Parity bytes calculated using LRC (Longitudinal Redundancy Check) per specification
- Fixed width row structure with proper padding
- Base64 encoding for UUID follows existing patterns in DataRow implementation

**Alternatives Considered**:
- Custom binary format - Rejected due to specification violation
- Variable width format - Rejected due to compatibility issues
- Different parity algorithm - Rejected due to LRC requirement in specification

### Go Implementation Patterns

**Decision**: Follow existing Go patterns in frozenDB codebase

**Rationale**:
- Marshal/Unmarshal methods follow existing patterns in other row types
- Validate() method matches struct validation pattern
- Error handling follows established patterns
- Package structure maintains single-file consistency with existing code

**Alternatives Considered**:
- Interface-based design - Rejected due to over-engineering for simple struct
- Builder pattern - Rejected due to unnecessary complexity
- Functional options pattern - Rejected due to fixed field requirements

### Performance Considerations

**Decision**: Focus on correctness with achievable performance targets

**Rationale**:
- <1ms target achievable with simple struct operations
- Fixed memory usage per constitution requirements
- No caching needed for simple operations
- O(1) operations for all NullRow methods

**Alternatives Considered**:
- Premature optimization - Rejected per constitution correctness principle
- Memory pooling - Rejected due to fixed field structure
- Lazy validation - Rejected due to data integrity requirements

## Conclusions

All research topics resolved with clear decisions aligned with frozenDB constitution and existing patterns. No NEEDS CLARIFICATION items remain. Implementation can proceed with confidence in technical approach.