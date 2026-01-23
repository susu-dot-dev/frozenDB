# Research: DBFile Read/Write Modes and File Locking

**Date**: 2026-01-23  
**Feature**: 017-dbfile-abstraction  
**Status**: Research Complete

## Breaking Change Analysis

### Critical Breaking Changes Identified

The implementation of FR-001 (NewDBFile constructor) and FR-006 (open.go refactoring) will require **significant breaking changes** to the frozenDB codebase:

1. **DBFile Interface Enhancement**: The minimal 4-method interface must be extended to support mode-specific operations
2. **Constructor Signature Changes**: `NewFileManager()` requires mode parameter addition
3. **Function Signature Changes**: Core functions in open.go need parameter/return type changes
4. **File Descriptor Abstraction**: Moving `syscall.Flock()` calls into DBFile interface

## Current Architecture Analysis

### DBFile Interface Scope
- **Location**: `frozendb/file_manager.go:23-28`
- **Current Methods**: `Read(start, size)`, `Size()`, `Close()`, `SetWriter(dataChan)`
- **Implementations**: Only `FileManager` in production, `mockDBFile` in tests
- **Usage**: Primarily through `NewTransaction(db DBFile, header)` in `transaction.go`

### Dual File System Problem
Current codebase maintains **two separate file abstractions**:
1. **Direct `os.File` operations** in `open.go` (for locking, validation, metadata)
2. **DBFile interface operations** in `file_manager.go` (for transaction data access)

### File Locking Architecture Gap
- **Current Location**: `open.go` functions `acquireFileLock()`/`releaseFileLock()`
- **Implementation**: Direct `syscall.Flock()` on `os.File.Fd()`
- **Problem**: DBFile interface doesn't expose file descriptors needed for locking

## Technical Decisions

### Decision: Extend DBFile Interface vs Create New Interface
**Chosen**: Extend existing DBFile interface
**Rationale**: 
- DBFile is already well-encapsulated with focused scope
- Only `FileManager` implements it in production (single implementation)
- All usage is through dependency injection via `NewTransaction()`
- Mock implementations are isolated to test files
- Alternative new interface would duplicate functionality and increase complexity

### Decision: Mode Parameter in Constructor vs Separate Factory Methods
**Chosen**: Add mode parameter to existing `NewFileManager(path, mode)` constructor
**Rationale**:
- Follows FR-001 specification exactly
- Consistent with existing `openDatabaseFile(path, mode)` pattern
- Simpler API surface than multiple factory methods
- Mode is fundamental property, not a behavioral variant

### Decision: File Locking Implementation Strategy
**Chosen**: Move locking logic into DBFile interface/FileManager implementation
**Rationale**:
- Consolidates dual file system approach
- Provides single interface for all file operations
- Better testability through interface mocking
- Cleaner separation of concerns

## Interface Enhancement Requirements

### New Constructor (FR-001, FR-002)
```go
func NewDBFile(path string, mode string) (DBFile, error)
```

### Mode Constants (Reuse Existing)
```go
const MODE_READ = "read"
const MODE_WRITE = "write" 
```

### Locking Behavior Integration
- **Read Mode**: No file locks (allow concurrent readers)
- **Write Mode**: Exclusive lock (`syscall.LOCK_EX | syscall.LOCK_NB`)
- **Lock Release**: Automatic on `Close()` or process termination

## Refactoring Scope (FR-006)

### Functions Requiring Changes
1. `openDatabaseFile(path, mode)` - Return type: `*os.File` → `DBFile`
2. `validateDatabaseFile(file)` - Parameter: `*os.File` → `DBFile`
3. `acquireFileLock(file, mode, blocking)` - Move into DBFile implementation
4. `releaseFileLock(file)` - Move into DBFile implementation

### Struct Changes Required
- `FrozenDB.file` field: `*os.File` → `DBFile`
- `NewFrozenDB()` function initialization changes

## Testing Impact Analysis

### Files Requiring Updates
- `transaction_spec_test.go` - Mock DBFile implementation
- `transaction_test.go` - 30+ test cases using mock DBFile
- Any new spec tests for FR-001 through FR-010

### Mock Enhancement Required
The existing `mockDBFile` in `transaction_spec_test.go` needs enhancement to support:
- Mode-aware behavior
- Locking behavior simulation
- Error scenarios for mode validation

## Performance Considerations

### Memory Usage
- **No additional memory pressure** - interface changes don't affect allocation patterns
- **Fixed memory requirement maintained** - consistent with Constitution Principle V

### Concurrency Performance
- **Multiple readers**: No locking overhead (current behavior preserved)
- **Single writer**: Exclusive locking with fast fail (current behavior preserved)
- **No deadlocks**: Current non-blocking locking strategy maintained

## Implementation Strategy

### Phase 1: Interface Enhancement
1. Extend `FileManager` to support mode parameter
2. Implement file locking within `FileManager`
3. Update `NewDBFile` constructor

### Phase 2: Refactoring Consolidation
1. Update `open.go` functions to use DBFile
2. Update `FrozenDB` struct and initialization
3. Ensure all tests pass without modification

### Phase 3: Spec Testing
1. Create spec tests for FR-001 through FR-010
2. Validate all functional requirements
3. Ensure backward compatibility

## Risk Mitigation

### Breaking Change Risks
- **Backward Compatibility**: Maintain exact API behavior during refactoring
- **Test Coverage**: Comprehensive test suite ensures no regression
- **Incremental Rollout**: Interface changes can be tested independently

### File Locking Risks
- **OS Dependency**: Rely on OS automatic lock release for crash scenarios
- **Non-blocking Behavior**: Maintain current fast-fail strategy for write conflicts

## Constitution Compliance

### All Principles Maintained
- **Immutability First**: No changes to append-only data operations
- **Data Integrity**: File locking enhances integrity guarantees
- **Correctness Over Performance**: Mode validation prioritizes correctness
- **Chronological Ordering**: No impact on key ordering
- **Concurrent Safety**: Enhanced through proper locking semantics
- **Single-File Architecture**: No impact on file structure
- **Spec Test Compliance**: All FR requirements will have corresponding spec tests

## Conclusion

The research confirms that while this feature requires significant breaking changes to internal interfaces, the changes are well-contained within the file operations layer. The DBFile interface has a focused scope that makes enhancement manageable, and the existing architecture provides good patterns to leverage for the consolidation.

The implementation will successfully eliminate the dual file system approach while maintaining all current functionality and performance characteristics.