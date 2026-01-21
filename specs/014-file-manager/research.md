# Research: FileManager Implementation

**Spec**: 014-file-manager  
**Date**: 2026-01-20  
**Status**: Complete  

## Overview

This research document outlines the technical decisions and patterns for implementing the FileManager struct in frozenDB. The FileManager serves as a thread-safe abstraction for concurrent file operations with exclusive write control.

## Key Technical Decisions

### Goroutine Lifecycle Management

**Decision**: Implement explicit goroutine lifecycle management with tombstone pattern  
**Rationale**: The user specifically emphasized avoiding hanging goroutines during process exit. The FileManager needs a tombstone flag to signal graceful shutdown of background writer goroutine. This prevents goroutine leaks and ensures clean process termination.  

**Implementation Pattern**:
```go
type FileManager struct {
    // ... other fields
    tombstone bool // Flag to signal goroutine shutdown
    // ... other fields
}
```

**Alternatives Considered**:
- context.Context cancellation: More complex than needed for simple shutdown signaling
- sync.WaitGroup: Good for waiting but doesn't provide the shutdown signal capability we need
- channel-based shutdown: Would require additional channels, tombstone flag is simpler

### Write Failure Persistence

**Decision**: Include tombstone flag in data model to persist write failures  
**Rationale**: The user requirement states "write failures persisting" - when a write operation fails, this state needs to be tracked so subsequent operations know about the failure. The tombstone flag serves dual purpose: goroutine lifecycle and failure state persistence.  

**Implementation Impact**: Failed writes will set the tombstone flag, preventing further write attempts until the FileManager is recreated or explicitly reset.

### Exclusive Write Control

**Decision**: Use atomic operations and mutex for writer exclusivity  
**Rationale**: The FileManager must enforce that only one caller can write at any given time. A simple atomic boolean or mutex check is sufficient for this requirement. The SetWriter method will return an error if a writer is already active.

**Implementation Pattern**:
```go
func (fm *FileManager) SetWriter(dataChan <-chan Data) error {
    fm.mutex.Lock()
    defer fm.mutex.Unlock()
    
    if fm.writeChannel != nil {
        return NewInvalidActionError("writer already active", nil)
    }
    fm.writeChannel = dataChan
    // Start writer goroutine
    go fm.writerLoop(dataChan)
    return nil
}
```

### Concurrent Read Safety

**Decision**: Use RWMutex for read-write coordination  
**Rationale**: Multiple readers should be able to access the file simultaneously while writes are exclusive. A sync.RWMutex allows multiple concurrent readers while ensuring writes are exclusive. Reads can access any byte ranges less than the current file end safely.

**Implementation Pattern**:
```go
func (fm *FileManager) Read(start int64, size int) ([]byte, error) {
    fm.mutex.RLock()
    defer fm.mutex.RUnlock()
    
    // Validate read bounds
    if start+int64(size) > fm.currentSize {
        return nil, NewInvalidInputError("read beyond file end", nil)
    }
    
    // Perform actual file read
    // ...
}
```

### File Size Tracking

**Decision**: Track current file size to enforce read boundaries  
**Rationale**: The FileManager needs to prevent reads beyond the current file end and ensure reads only access stable, written data. Tracking the current file size allows efficient boundary checking without constantly querying the file system.

### Channel-based Write Communication

**Decision**: Use <-chan Data with response channels for async writes  
**Rationale**: The specification requires Data payload with response channel for write completion notification. This pattern allows the writer goroutine to process writes sequentially while providing completion feedback to callers.

**Implementation Pattern**:
```go
type Data struct {
    Bytes    []byte
    Response chan error
}

func (fm *FileManager) writerLoop(dataChan <-chan Data) {
    defer func() {
        fm.mutex.Lock()
        fm.writeChannel = nil
        fm.mutex.Unlock()
    }()
    
    for data := range dataChan {
        err := fm.performWrite(data.Bytes)
        data.Response <- err
        
        if err != nil {
            fm.tombstone = true // Mark as failed
            return
        }
    }
}
```

## Performance Considerations

### Memory Management

**Decision**: Memory usage scales with read operations, not database size  
**Rationale**: Read operations return byte slices to callers, so memory usage depends on caller read patterns. The FileManager itself maintains minimal state (file pointer, size, mutex, channels) regardless of database file size.

### Read Performance

**Decision**: Direct file reads without caching  
**Rationale**: The FileManager is a low-level abstraction. Caching should be handled by higher-level components. Direct file reads ensure data consistency and avoid cache invalidation complexity.

### Write Performance

**Decision**: Sequential writes through single goroutine  
**Rationale**: Having a single writer goroutine ensures write ordering and eliminates write contention. The channel-based approach provides backpressure naturally when write operations are slower than production.

## Error Handling Strategy

**Decision**: Use structured errors following existing frozenDB patterns  
**Rationale**: The error_handling.md guide requires all errors to derive from FrozenDBError. FileManager will use appropriate error types:
- InvalidInputError for boundary violations
- InvalidActionError for concurrent write attempts and invalid state transitions
- CorruptionError for read integrity failures
- IOError for file system issues

## Integration with Existing Codebase

### Compatibility with frozenDB File Format

**Decision**: FileManager operates after header & checksum rows  
**Rationale**: The specification clearly states FileManager works "starting after the initial header & checksum row". This means FileManager doesn't need to understand the file format details, just provide raw byte access.

### Relationship with Transaction Layer

**Decision**: FileManager is a dependency, not a replacement for transactions  
**Rationale**: The FileManager provides the underlying file I/O that Transaction will use. Transaction handles the row formatting and file format details, while FileManager handles thread-safe I/O coordination.

## Testing Strategy

### Spec Test Requirements

**Decision**: Create file_manager_spec_test.go with all FR-XXX requirements  
**Rationale**: Following docs/spec_testing.md guidelines, each functional requirement needs corresponding spec tests. The tests will validate thread safety, exclusive write access, and proper error handling.

### Test Patterns

**Decision**: Use table-driven tests with goroutines for concurrency testing  
**Rationale**: Go's testing framework works well with table-driven tests. For concurrent read/write testing, we'll spawn multiple goroutines and use sync.WaitGroup to coordinate test completion.

## Conclusion

The FileManager implementation follows established Go patterns for concurrent file I/O while respecting frozenDB's architectural constraints. The design prioritizes data integrity and thread safety over performance optimizations, consistent with the frozenDB constitution. The tombstone pattern addresses both goroutine lifecycle management and write failure persistence requirements.

All technical decisions align with:
- Go 1.25.5 and standard library patterns
- frozenDB append-only immutable architecture
- User requirements for goroutine lifecycle management
- frozenDB structured error handling guidelines
- Spec testing requirements for comprehensive coverage