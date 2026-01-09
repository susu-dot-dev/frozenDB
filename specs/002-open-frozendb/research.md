# Research: Open frozenDB Files

**Feature**: 002-open-frozendb  
**Date**: 2026-01-09  
**Purpose**: Research technical decisions for implementing database opening functionality

## Research Summary

This document consolidates research findings for implementing the NewFrozenDB function that opens existing frozenDB database files with proper concurrent access control, header validation, and resource management.

## Technical Decisions

### 1. File Locking Strategy

**Decision**: Use `syscall.Flock()` with OS-level advisory locking for concurrent access control.

**Rationale**: 
- `flock()` provides simpler API and more predictable behavior than `fcntl()`
- Native Go support through `syscall` package
- Automatic lock release on process exit
- Perfect for frozenDB's writer-only locking requirements (readers need no locks)
- Widely used in established databases (BoltDB, BadgerDB)

**Implementation**:
- **No locks** for readers - append-only files allow unlimited concurrent reads
- Exclusive locks (`LOCK_EX`) for writers - single access only to prevent concurrent appends  
- Non-blocking variants (`LOCK_NB`) for immediate failure on writer lock contention
- Proper lock release in cleanup paths

**Alternatives considered**:
- `fcntl()` locking: More complex, potential deadlock issues, byte-range overkill
- External lock files: Additional complexity, race conditions
- Library solutions (`gofrs/flock`): Good option but adds dependency

### 2. Header Validation Approach

**Decision**: Hybrid binary + text validation with manual JSON parsing for strict key ordering.

**Rationale**:
- Fixed positions (null terminator, newline) validated efficiently with binary operations
- JSON content parsed manually to enforce exact key ordering requirement
- Memory-efficient parsing in-place without allocations
- Detailed error reporting with position information

**Implementation**:
1. Validate fixed 64-byte size
2. Verify byte 63 is newline
3. Find null terminator position
4. Validate JSON structure with manual token-by-token parsing
5. Enforce exact key order: sig, ver, row_size, skew_ms
6. Validate field values against specification ranges

**Alternatives considered**:
- Full JSON unmarshal: Cannot enforce key ordering, more allocations
- Regex parsing: Complex for mixed format, harder error reporting
- Binary-only format: Would require breaking v1 format specification

### 3. Resource Management Pattern

**Decision**: Mutex-protected state management with comprehensive cleanup registration.

**Rationale**:
- Mutex for thread-safe, idempotent close operations
- Cleanup function registration pattern for guaranteed resource release on errors
- Fixed-size structures ensure memory usage remains constant
- Context-based cancellation for lock acquisition timeouts

**Implementation**:
- `closed` bool flag for idempotent Close() method (protected by mutex)
- Mutex for coordinating cleanup operations (multiple goroutines calling Close())
- File descriptor and file lock state managed through struct fields
- Cleanup function registration pattern for guaranteed resource release on errors
- Fixed buffers and structures regardless of database size

**Alternatives considered**:
- Manual state management: Race conditions possible
- Defer-only cleanup: Cannot handle complex error scenarios
- Reference counting: Overkill for simple open/close lifecycle

## Error Types Required

Based on spec requirements and existing error patterns:

**New Error Type**:
```go
type CorruptDatabaseError struct {
    FrozenDBError
}
```

**Reused Error Types**:
- `InvalidInputError`: Invalid path/mode parameters
- `PathError`: Filesystem access issues  
- `WriteError`: Lock acquisition failures

## Performance Considerations

### Memory Efficiency
- Fixed memory usage regardless of database size
- In-place header validation without allocations
- Stack-based structures only
- Streaming enumeration without loading entire database

### Performance Targets
- Database opening: <100ms for typical files
- Resource cleanup: <10ms regardless of database size
- Lock acquisition: Immediate failure for non-blocking mode

## Testing Strategy

### Resource Leak Detection
- File descriptor counting tests
- Goroutine leak detection with `goleak`
- Memory usage validation with runtime stats
- Concurrent access stress testing

### Header Validation Testing
- Comprehensive table-driven tests for all validation scenarios
- Edge cases: boundary values, malformed JSON, wrong key order
- Performance benchmarks for header parsing
- Error message validation for debugging

### Concurrency Testing
- Multiple concurrent readers validation (no locks needed)
- Writer exclusivity enforcement (exclusive lock only)
- Mixed read/write workload testing (readers operate freely)
- Lock timeout and cancellation testing (write mode only)

## Integration Points

### Existing Codebase
- Reuse `fsOperations` abstraction from create.go for filesystem operations
- Follow established error handling patterns from errors.go
- Maintain consistency with create.go's parameter validation approach
- Use existing constants and types where applicable

### New Components
- `frozendb.go`: Main FrozenDB struct and NewFrozenDB function (as requested)
- `open.go`: Header validation and file opening logic
- `open_test.go`: Unit tests for open functionality
- `open_spec_test.go`: Specification tests for functional requirements

## Dependencies

### Standard Library Only
- `os`: File operations
- `syscall`: File locking
- `encoding/json`: JSON validation  
- `sync`: Thread synchronization
- `context`: Cancellation support

### External Dependencies
None planned - maintain minimal dependency footprint per frozenDB principles.

## Security Considerations

### File Access Security
- OS-level permissions enforcement through standard file access
- No privilege escalation attempts
- Proper error handling without path/sensitive information leakage
- Append-only attribute preservation from create functionality

### Concurrent Access Safety
- Advisory locking prevents data corruption between processes
- Thread-safe state management within single process
- Mutex prevents race conditions
- Proper cleanup prevents resource contamination

## Future Extensibility

The design supports future enhancements:
- Additional lock types (upgrade locks, timeout locks)
- Enhanced header validation for future format versions
- Context-based operations throughout the API
- Metrics and observability hooks
- Additional filesystem implementations (Windows, macOS)

## Conclusion

The research supports a straightforward implementation using standard Go patterns that align with frozenDB's constitutional requirements. The approach prioritizes correctness and data integrity while maintaining performance characteristics and minimal resource usage.