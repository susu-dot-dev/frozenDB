# Research: Project Structure Refactor

**Purpose**: Research findings that resolve technical unknowns from the specification  
**Created**: 2026-01-27  
**Updated**: 2026-01-27 (Detailed analysis added)
**Feature**: Project Structure Refactor

## Executive Summary

**User Problem Identified**: "Why is file.go, header.go, row.go, control.go, uuid.go all in the public pkg files? The file structure is an internal detail. Only things derivable from the current top-level frozendb.go struct should be visible."

**Key Finding**: After analyzing all 49 Go files in frozendb/, we discovered that while most file format internals CAN be hidden, only essential types (Header, DataRow, FinderStrategy) MUST remain public because they appear in public API signatures. This creates a constrained refactoring approach.

**Recommended Approach**: Hybrid model - move most internals to `internal/`, but keep essential types in public API with clear documentation about stability guarantees.

---

## Current Codebase Analysis

### Public API Determination

**Decision**: The public API consists of three primary interaction points plus supporting types:
1. **Database Lifecycle**: `NewFrozenDB()`, `Create()`, `FrozenDB.Close()`
2. **Query Operations**: `FrozenDB.Get()`, `FrozenDB.BeginTx()`, `FrozenDB.GetActiveTx()`
3. **Transaction Operations**: All Transaction methods (AddRow, Commit, Rollback, GetRows, etc.)
4. **Supporting Types**: DataRow, Header, FinderStrategy (required in API signatures)

**Rationale**: Detailed code analysis reveals that:
- `Transaction.GetRows() []DataRow` - returns DataRow slice, making DataRow public
- `Transaction.Header *Header` - exported struct field, making Header public
- `NewFrozenDB(..., strategy FinderStrategy)` - uses FinderStrategy constants, making FinderStrategy public
- Finder interface is internal - only strategy constants are exposed to users
- `Transaction.GetEmptyRow()` is an internal detail and should not be part of public API
- `Finder.OnRowAdded(row *RowUnion)` uses RowUnion, but Finder is internal, so RowUnion can be internal
- All error types must be public for `errors.As()` and `errors.Is()` checking

**Alternatives considered**: 
1. **Minimal API (only FrozenDB/Transaction methods)**: Cannot hide row types due to GetRows() return type
2. **Full API exposure (all file format details public)**: Current state, violates user's concern
3. **Wrapper types approach**: Create public wrappers delegating to internal types - adds complexity and allocations
4. **Interface-based approach**: Return interfaces instead of concrete types - breaking change, less idiomatic Go

### Detailed Public vs Internal Type Analysis

**Finding**: Of 49 Go files, we can categorize them into:

#### Must Remain Public (in pkg/frozendb/) - 6 core types
1. **frozendb.go** - `FrozenDB` struct (main database handle)
2. **transaction.go** - `Transaction` struct (transaction handle)
3. **create.go** - `CreateConfig` struct, `Create()` function
4. **errors.go** - All error types (for `errors.As()` checking)
5. **header.go** - `Header` struct (exposed via Transaction.Header field)
6. **data_row.go** - `DataRow` struct (returned by GetRows())

Additionally exposed (unavoidable):
- **row.go** - `StartControl`, `EndControl` types (fields in DataRow)
- **finder.go** - `FinderStrategy` type and constants (parameter to NewFrozenDB)

Note: Finder interface is internal - only FinderStrategy constants are public. Users select finder implementation via strategy constants, not by directly implementing Finder interface.

#### Can Move to Internal - 30+ files

**internal/fields/** (row field structures):
- `row.go` - baseRow[T], RowPayload interface, Validator interface, control constants
- `data_row.go` - DataRowPayload (internal to DataRow)
- `null_row.go` - NullRowPayload (internal to NullRow)
- `checksum.go` - ChecksumRow, Checksum type (only used internally)
- `partial_data_row.go` - PartialDataRow, PartialRowState (internal transaction state)
- `uuid_helpers.go` - UUIDv7 validation functions

**internal/finder/** (query implementations):
- `finder.go` - Finder interface (internal abstraction)
- `simple_finder.go` - SimpleFinder implementation
- `inmemory_finder.go` - InMemoryFinder implementation
- `binary_search_finder.go` - BinarySearchFinder implementation
- `fuzzy_binary_search.go` - FuzzyBinarySearch helper function

**internal/fileio/** (file I/O management):
- `file_manager.go` - FileManager implementation, DBFile interface, Data struct
- `open.go` - Database validation helpers (validateDatabaseFile, etc.)

**Key Insight**: While we cannot hide DataRow/Header completely, we CAN hide:
- NullRow, ChecksumRow, PartialDataRow, RowUnion (internal implementation details)
- Internal payload types (DataRowPayload, NullRowPayload)
- Generic base type (baseRow[T])
- All format constants (ROW_START, ROW_END, control byte constants)
- Finder interface and all finder implementations (only FinderStrategy constants stay public)
- All file I/O internals

**Decision**: Follow standard Go project layout with clear separation between `pkg/` (public) and `internal/` (private).

**Rationale**: 
- `pkg/` provides stable API that external developers can depend on
- `internal/` allows free refactoring of implementation details
- Go compiler enforces these boundaries preventing accidental internal usage

**Alternatives considered**:
- Single package with visibility rules: Insufficient for clear API boundaries
- Multiple public packages: Over-engineering for current needs

### File Organization Approach

**Decision**: Organize by functional responsibility rather than technical layers.

**Rationale**: 
- Groups related functionality together (e.g., all transaction code)
- Makes navigation intuitive for both external and internal developers
- Supports future feature additions without major restructuring

**Alternatives considered**:
- Technical layering (api/, service/, repository/): Overly complex for database library
- Domain-driven organization: Current scope doesn't warrant domain complexity

## Existing Code Assessment

### Current frozendb/ Directory Analysis

**Files identified as Public API (25 total)**:
- `frozendb.go` - Main database struct and core operations
- `errors.go` - All error types and constructors
- `transaction.go` - Transaction management
- `finder.go` - Finder interface and strategies
- `file.go` - DBFile interface
- `header.go` - Database metadata operations
- `row.go` - Row data structures
- `uuid_helpers.go` - UUID utilities
- Plus supporting files for controls, constants, and public helpers

**Files identified as Internal Implementation (15 total)**:
- `file_manager.go` - File handling implementation
- `simple_finder.go` - Simple search implementation
- `inmemory_finder.go` - In-memory search implementation
- `binary_search_finder.go` - Binary search implementation
- `fuzzy_binary_search.go` - Fuzzy search implementation
- `create.go` - File system operations (non-public parts)
- `open.go` - Database validation logic
- `checksum.go` - Checksum calculations
- Plus various internal helpers and utilities

### Test Migration Strategy

**Decision**: Move tests with their corresponding files, update import paths only.

**Rationale**: 
- Maintains test coverage during transition
- Tests belong with the code they're testing
- Import path updates are mechanical changes

**Alternatives considered**:
- Separate test directory: Would break Go conventions and tooling
- Keep all tests in root: Would create confusion about what's being tested

## Import Path Impact Analysis

### External Developer Impact

**Decision**: Maintain existing import path `github.com/susu-dot-dev/frozenDB` for backward compatibility.

**Rationale**: 
- Breaking changes would impact existing users
- Go modules support this pattern through package redirection
- Internal restructure shouldn't affect external consumers

**Implementation**: Use go.mod package aliasing or package redirection to maintain compatibility.

### Internal Import Updates

**Decision**: Update all internal imports to use new package structure.

**Impact**: ~40 files require import path updates
- Public API files: Import internal packages
- Internal files: Import other internal packages
- Test files: Update both public and internal imports

### Build System Updates

**Decision**: Update Makefile to support new directory structure while maintaining existing targets.

**Key changes needed**:
- Add build targets for `cmd/frozendb`
- Add build targets for `examples/`
- Update test targets to work with new structure
- Ensure CI validates all components build successfully

## CLI and Examples Requirements

### CLI Tool Requirements

**Decision**: Create minimal CLI in `cmd/frozendb/` with hello-world functionality.

**Rationale**: 
- Satisfies immediate requirement for working CLI
- Provides foundation for future command additions
- Follows standard Go CLI patterns

**Implementation approach**:
- Simple main.go that imports public API
- Hello world output for validation
- Build target in Makefile for distribution

### Example Applications

**Decision**: Create basic examples demonstrating core frozenDB usage.

**Rationale**: 
- Reduces learning curve for new developers
- Demonstrates proper API usage patterns
- Provides testable validation of public API design

**Initial examples planned**:
- Basic database creation and data insertion
- Simple transaction usage
- Read operations with different finder strategies

## Performance and Compatibility Considerations

### Performance Impact

**Decision**: Refactor should have zero performance impact.

**Rationale**: 
- Only moving code between packages, no algorithm changes
- Go compiler optimizations unaffected by package structure
- Import overhead negligible compared to database operations

### Backward Compatibility

**Decision**: Maintain 100% backward compatibility for existing API.

**Rationale**: 
- Breaking changes would require major version bump
- Current user base relies on existing API
- Migration burden would be significant

**Implementation strategy**:
- Keep all exported identifiers unchanged
- Maintain existing function signatures
- Preserve error types and behaviors

## Risk Assessment and Mitigation

### High-Risk Areas

1. **Import Path Updates**: Mechanical but extensive changes
   - **Mitigation**: Use automated refactoring tools, comprehensive testing
   
2. **Test Migration**: Risk of breaking existing test coverage
   - **Mitigation**: Move tests with their implementation, validate all tests pass

3. **Build System Integration**: Complex CI updates
   - **Mitigation**: Incremental updates, test each component separately

### Low-Risk Areas

1. **Public API Design**: Based on existing proven API
2. **Internal Structure**: Follows established Go conventions
3. **CLI/Examples**: Simple, isolated components

## Implementation Confidence

**Confidence Level**: High

**Justification**: 
- Well-established Go project patterns
- Extensive existing codebase analysis
- Clear separation of concerns
- Comprehensive test coverage to validate changes

**Next Steps**: Proceed with data model and contracts definition based on this research.