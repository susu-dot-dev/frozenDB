# Research Findings: Row Finder Interface and Implementation

## Research Summary

This document consolidates research findings for implementing the Finder interface and SimpleFinder in frozenDB. All technical unknowns have been resolved through comprehensive codebase analysis.

## Technical Decisions

### Decision: Use existing frozendb package structure
**Rationale**: The frozendb package already contains all necessary row types, file I/O interfaces, and error handling patterns. Adding Finder functionality to the existing package maintains consistency and leverages existing infrastructure.

### Decision: Leverage existing DBFile interface for file operations
**Rationale**: The DBFile interface (`/home/anil/code/frozenDB/frozendb/file_manager.go`) provides thread-safe read operations with `Read(start int64, size int32) ([]byte, error)` and `Size() int64` methods. SimpleFinder can use this interface directly for disk-based row reading.

### Decision: Use existing RowUnion.UnmarshalText() for row type detection
**Rationale**: The RowUnion type (`/home/anil/code/frozenDB/frozendb/row_union.go`) already implements discriminatored union parsing with automatic type detection based on control bytes. This eliminates the need for custom row parsing logic.

### Decision: Follow established error handling patterns
**Rationale**: The frozendb package has a structured error system with `FrozenDBError` base type and specific error constructors. SimpleFinder should use existing error types like `NewInvalidInputError()`, `NewCorruptDatabaseError()`, etc.

## Implementation Details Resolved

### File Format and Indexing
- **Header Size**: 64 bytes (constant `HEADER_SIZE`)
- **Row Indexing Formula**: `row_offset = HEADER_SIZE + index * row_size`
- **Index 0**: First checksum row after header
- **Index 1+**: Data rows, null rows, etc.

### Row Types and Control Bytes
- **DataRow**: Start control `'T'` or `'R'`, end controls `TC`, `RE`, `SC`, `SE`, `R0-R9`, `S0-S9`
- **NullRow**: Start control `'T'`, end control `'NR'`, UUID is always `uuid.Nil`
- **ChecksumRow**: Start control `'C'`, end control `'CS'`, never in transactions
- **PartialDataRow**: Only at file end, progressive states

### Transaction Boundary Detection
- **Transaction Start**: First row with `start_control = 'T'` in transaction chain
- **Transaction End**: Row with transaction-ending end control (`TC`, `RE`, `SC`, `SE`, `R0-R9`, `S0-S9`, `NR`)
- **Maximum Transaction Size**: 100 data rows + checksum rows

### Error Handling Strategy
- **Key Not Found**: Use existing `NewInvalidInputError()` with descriptive message
- **Invalid Index**: Use `NewInvalidInputError()` for out-of-bounds or checksum row indices
- **Corruption**: Use `NewCorruptDatabaseError()` for unparseable rows
- **Malformed Transaction**: Use `NewInvalidInputError()` for boundary detection failures

## File Structure Implementation

### Core Files
- **`frozendb/finder.go`**: Finder interface definition
- **`frozendb/simple_finder.go`**: SimpleFinder implementation  
- **`frozendb/simple_finder_spec_test.go`**: Spec tests for functional requirements
- **`frozendb/simple_finder_test.go`**: Unit tests for implementation details

### Dependencies and Imports
- `"github.com/google/uuid"` - UUID handling
- `"sync"` - Mutex for thread safety
- Existing frozendb package types and interfaces

## Performance Characteristics Confirmed

### SimpleFinder Constraints
- **Memory Usage**: O(row_size) - constant regardless of database size
- **Time Complexity**: O(n) linear scan for GetIndex() operations
- **Disk I/O**: One DBFile.Read() call per row examined
- **Concurrency**: Thread-safe via mutex protection

### Optimization Opportunities (Future)
- Indexing structures for O(log n) UUID lookup
- Cached transaction boundary information
- Memory-mapped file access patterns

## Testing Strategy Aligned

### Spec Testing Requirements
- **Test Naming**: `Test_S_XXX_FR_XXX_Description()`
- **Location**: `frozendb/simple_finder_spec_test.go`
- **Coverage**: All functional requirements (FR-001 through FR-010)
- **Validation**: Both success and error paths

### Unit Testing Structure  
- **Location**: `frozendb/simple_finder_test.go`
- **Focus**: Implementation details, edge cases, performance characteristics
- **Mocking**: Use mock DBFile for isolated testing

## Integration Considerations

### Database File Access
- Use `NewDBFile()` for creating file handles
- Leverage atomic file size operations for concurrent access
- Respect file locking mechanisms (write mode exclusivity)

### Row Addition Callbacks
- `OnRowAdded()` called within transaction write lock context
- Sequential ordering guaranteed (no self-racing)
- Size tracking must align with DBFile.Size() atomic operations

### Concurrency Model
- Multiple goroutines can call Get* methods concurrently
- Single goroutine calls OnRowAdded() at a time (transaction write lock)
- Internal state protection via mutex or atomic operations

## Alternatives Considered

### Alternative: Separate finder package
**Rejected Because**: Would duplicate existing row types, file I/O, and error handling infrastructure. Integration with existing frozendb types would require complex dependency management.

### Alternative: In-memory caching for SimpleFinder
**Rejected Because**: Violates the "reference implementation" principle. SimpleFinder should demonstrate simplest correct implementation without optimizations.

### Alternative: Custom row parsing logic
**Rejected Because**: RowUnion.UnmarshalText() already provides robust row type detection with control byte parsing. Reimplementing would duplicate functionality and increase maintenance burden.

## Conclusion

All technical unknowns have been resolved through comprehensive analysis of the existing frozendb codebase. The implementation can proceed with clear understanding of:

1. File format and indexing scheme
2. Row types and transaction semantics
3. Error handling patterns and testing conventions
4. Concurrency model and thread safety requirements
5. Integration points with existing infrastructure

The SimpleFinder implementation will serve as a correct reference baseline while maintaining compatibility with existing frozendb architecture and design principles.