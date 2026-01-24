# Research Report: Query Get Function Implementation

## JSON Unmarshaling Patterns

### Decision: Use direct json.Unmarshal with interface{} parameter
The Get() function will follow the exact pattern of json.Unmarshal by accepting `value any` as a parameter and using reflection to populate the destination.

**Rationale**: 
- Matches the user requirement "value works the same as how json.Unmarshal works"
- Provides maximum flexibility for different destination types
- Familiar pattern for Go developers
- Zero additional dependencies needed

**Implementation Approach**:
- Use input validation patterns from existing frozenDB methods
- Leverage Finder protocol for row location and transaction validation
- Apply standard json.Unmarshal for destination population
- Follow existing error wrapping patterns for structured error handling

**Alternatives considered**: 
- Generic type parameter `Get[T any](key UUID) (*T, error)` - More complex, doesn't match json.Unmarshal signature
- Reflection-based approach - Added complexity without benefit
- Pre-allocated struct pools - Optimization for later, not needed for initial implementation

## Error Handling Strategy

### Decision: Add InvalidDataError type and use existing error patterns
The Get() function will use a combination of existing error types and one new error type to handle all scenarios.

**Rationale**:
- Follows existing patterns in frozendb/errors.go
- Maintains compliance with docs/error_handling.md
- Provides clear, actionable error messages
- Enables proper error handling by callers

**New Error Type Required**:
```go
// InvalidDataError is returned for JSON data that cannot be unmarshaled
type InvalidDataError struct {
    FrozenDBError
}

func NewInvalidDataError(message string, err error) *InvalidDataError {
    return &InvalidDataError{
        FrozenDBError: FrozenDBError{
            Code:    "invalid_data",
            Message: message,
            Err:     err,
        },
    }
}
```

**Error Mapping**:
- **Key not found**: `NewKeyNotFoundError(fmt.Sprintf("key %s not found in committed data", key.String()), nil)`
- **Invalid destination**: `NewInvalidInputError("destination must be a pointer", nil)`
- **JSON unmarshal failure**: `NewInvalidDataError("failed to unmarshal JSON value", err)`
- **File I/O errors**: `NewReadError("failed to read row", err)`
- **Data corruption**: `NewCorruptDatabaseError("invalid row format", err)`

**Alternatives considered**: 
- Using generic error types - Loses specific error handling capabilities
- Creating multiple new error types - Unnecessary complexity
- Returning underlying errors directly - Doesn't follow frozenDB structured error pattern

## Transaction State Validation Algorithm

### Decision: Use Finder protocol with complete transaction boundary analysis
The Get() function will leverage the existing Finder interface to validate transaction state before returning any data.

**Rationale**:
- Reuses existing Finder protocol and implementations
- Maintains consistency with other frozenDB operations
- Handles all transaction states correctly (commit, rollback, partial rollback)
- Supports concurrent read operations safely

**Complete Algorithm**:

The logic to determine if a row should be returned follows these steps:

1. **Get transaction boundaries** using finder.GetTransactionStart() and finder.GetTransactionEnd()
2. **Read the transaction end row** to analyze end_control pattern
3. **Handle NullRow transactions** - these never contain user data and should return KeyNotFoundError
4. **Analyze end_control for DataRows**:
   - TC/SC: Committed transactions - all rows valid
   - R0/S0: Full rollback - all rows invalid  
   - R1-R9/S1-S9: Partial rollback - count savepoints to determine validity
5. **For partial rollbacks**: Count savepoints from transaction start to target row index and compare to rollback savepoint number
6. **Savepoint counting**: Iterate through rows to count savepoint-creating rows (SC/SE/S0-S9 patterns)

The algorithm leverages existing Finder interface methods and follows the transaction validation patterns established in the v1 file format specification.

**Alternatives considered**:
- Cache transaction state - Complexity vs benefit unclear
- Skip transaction validation for performance - Violates correctness requirements
- Use simple row-level flags - Insufficient for partial rollback scenarios

## Performance Considerations

### Decision: Prioritize correctness with optimization opportunities
Initial implementation will focus on correctness while identifying clear optimization paths for future work.

**Current Performance Characteristics**:
- **GetIndex()**: O(n) linear scan (worst case)
- **Transaction boundary methods**: O(k) where k ≤ 101 (transaction size limit)
- **JSON unmarshaling**: Depends on data size and complexity
- **Memory usage**: O(1) constant (aligns with constitutional requirements)

**Optimization Opportunities for Future**:
- Index caching for UUID lookup (O(log n) binary search)
- Transaction state caching to avoid repeated boundary analysis
- JSON decoder pooling for high-frequency operations
- Pre-validated type cache for common destination types

**Concurrent Safety**:
- Get() operations are read-only and safe during concurrent writes
- Finder implementations provide thread-safe Get* method access
- No locks required in Get() implementation

## Integration with Existing Code

### Decision: Add Get() method to existing FrozenDB struct
The Get() function will be implemented as a method on the main FrozenDB struct, leveraging existing components.

**Dependencies**:
- **Finder interface**: Already implemented in simple_finder.go
- **Row structures**: Already defined in row.go  
- **Error handling**: Will extend existing patterns in errors.go
- **File operations**: Existing DBFile interface for row reading

**Method Signature**:
```go
func (db *FrozenDB) Get(key uuid.UUID, value any) error
```

**File Location**: Add to frozendb/frozen.go alongside existing public methods

**Alternatives considered**:
- Separate Query struct - Unnecessary abstraction
- Standalone function - Loses access to internal state
- Interface-based approach - Over-engineering for current needs

## Compliance with Constitutional Principles

- **Immutability First**: Get() only reads existing data, no modifications
- **Data Integrity**: Validates transaction state before returning data
- **Correctness Over Performance**: Prioritizes accurate transaction validation
- **Chronological Ordering**: Leverages existing UUIDv7 timestamp handling
- **Concurrent Read-Write Safety**: Read-only operation safe during writes
- **Single-File Architecture**: Uses existing file format and operations

## Testing Strategy

Spec tests will be created in frozendb_spec_test.go following the naming convention `Test_S_XXX_FR_XXX_Description()` to validate each functional requirement from the specification.

## Conclusion

The research phase has identified clear patterns and approaches for implementing the Get() function. The implementation will:

1. Follow json.Unmarshal patterns for maximum compatibility
2. Add InvalidDataError to complete the error handling strategy  
3. Use the Finder protocol for robust transaction state validation
4. Prioritize correctness while maintaining performance characteristics
5. Integrate seamlessly with existing frozenDB architecture

## Implementation Approach

The Get function should follow this approach:

1. **Input validation** using existing frozenDB validation patterns
2. **Key location** using Finder interface GetIndex() method
3. **Transaction validation** using Finder GetTransactionStart/End methods
4. **Row visibility determination** based on transaction state and savepoint logic
5. **JSON unmarshaling** using standard json.Unmarshal with proper error wrapping
6. **Error handling** following existing frozenDB error patterns

This approach reuses established patterns in the codebase while adding the new InvalidDataError for JSON unmarshal failures.

All technical unknowns have been resolved and the implementation can proceed to Phase 1 design.