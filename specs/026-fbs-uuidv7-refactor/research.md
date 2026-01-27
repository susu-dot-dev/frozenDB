# Phase 0 Research: FBS UUIDv7 Refactor

## Research Summary

This document contains the research findings for refactoring the FuzzyBinarySearch algorithm to work with UUIDv7 keys instead of int64 timestamps.

## Technical Decisions

### Decision: Use Existing UUIDv7 Validation Function

**Rationale**: The codebase already provides `ValidateUUIDv7(u uuid.UUID)` in `uuid_helpers.go` which performs comprehensive validation including:
- Zero UUID check (uuid.Nil rejection)
- RFC 4122 variant validation  
- UUID version 7 validation

**Alternatives considered**: Creating a new validation function vs. reusing existing one. The existing function is comprehensive, tested, and follows the established error handling patterns.

### Decision: Maintain Current Algorithm Structure

**Rationale**: The current FuzzyBinarySearch algorithm structure is optimal for the frozenDB use case:
- Three-way partitioned binary search (<, ==, >)
- Linear scan for indeterminate range within skew window
- O(log n) + k complexity preserved

**Alternatives considered**: Complete rewrite vs. adaptation. Adaptation preserves the proven algorithm structure while changing only the key type and comparison logic.

### Decision: Use ExtractUUIDv7Timestamp for Comparison

**Rationale**: The existing `ExtractUUIDv7Timestamp()` function efficiently extracts the 48-bit timestamp using optimized bit operations:
```go
func ExtractUUIDv7Timestamp(u uuid.UUID) int64 {
    return int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
        int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
}
```

**Alternatives considered**: String parsing vs. direct byte manipulation. Direct byte manipulation is significantly more performant and matches existing patterns.

### Decision: Preserve Current Test Structure

**Rationale**: The existing test structure with 50+ comprehensive test functions provides excellent coverage:
- Unit tests in `fuzzy_binary_search_test.go`
- Spec tests in `fuzzy_binary_search_spec_test.go`
- Performance benchmarks included

**Alternatives considered**: Rewriting tests vs. adapting existing tests. Adapting preserves the proven test patterns while ensuring UUIDv7-specific scenarios are covered.

## Integration Analysis

### Current FuzzyBinarySearch Integration Status

**Finding**: FuzzyBinarySearch is currently a standalone utility and not integrated with the main finder implementations.

**Current State**:
- `SimpleFinder` uses linear scanning (`GetIndex` method)
- `InMemoryFinder` uses UUID-to-index map for O(1) lookups
- Neither finder currently uses `FuzzyBinarySearch`

**Implications**: The refactor will make FuzzyBinarySearch directly usable by the finder interfaces without requiring integration work.

### Performance Characteristics

**Current Performance**: The `ValidateUUIDv7()` function performs three O(1) checks:
1. Zero UUID comparison
2. Variant check using `u.Variant()`
3. Version check using `u.Version()`

**Memory Efficiency**: No allocations during validation or timestamp extraction, preserving the O(1) space complexity requirement.

### Error Handling Patterns

**Consistent with Codebase**: All UUID validation uses the structured error system:
```go
return NewInvalidInputError("UUID version must be 7, got %d", u.Version())
```

This matches the patterns established in the error handling guide.

## File Format Compliance

The refactored algorithm will be fully compliant with the v1 file format specification:

### UUIDv7 Requirements
- All keys must be UUIDv7 for proper time ordering ✓
- Timestamp ordering algorithm uses extracted timestamps ✓
- Skew window handling preserved ✓

### Algorithm Requirements
- Binary search uses timestamp portion for comparison ✓
- Linear scan uses full UUID equality for exact matching ✓
- Handles multiple UUIDs with identical timestamps ✓

## Implementation Strategy

### Phase 1 Approach
1. Refactor function signature to accept UUIDv7 parameters
2. Modify binary search comparison to use extracted timestamps
3. Update linear scan to use full UUID equality
4. Preserve all existing performance characteristics

### Testing Strategy
1. Adapt existing unit tests to work with UUIDv7 test data
2. Create new spec tests for all 5 functional requirements
3. Ensure performance benchmarks remain valid
4. Add UUIDv7-specific edge case tests

### Integration Benefits
The refactored function will be immediately usable by existing finder interfaces, potentially enabling a new `FuzzyFinder` implementation that combines the benefits of binary search performance with UUIDv7 key support.

## Conclusion

The research confirms that the refactoring is straightforward and leverages existing, well-tested infrastructure. The existing UUID validation, timestamp extraction, and error handling patterns provide a solid foundation for the implementation. The algorithm structure can be preserved while changing only the key types and comparison logic.

**All technical unknowns have been resolved through this research phase.**