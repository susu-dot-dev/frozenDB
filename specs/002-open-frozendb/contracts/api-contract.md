# API Contract: frozenDB Open Operations

**Feature**: 002-open-frozendb  
**Date**: 2026-01-09  
**Purpose**: Public API specification for opening frozenDB database files

## Public API

### NewFrozenDB

Opens an existing frozenDB database file with specified access mode.

```go
func NewFrozenDB(path string, mode string) (*FrozenDB, error)
```

**Parameters**:
- `path` (string): Filesystem path to existing frozenDB database file (.fdb extension)
- `mode` (string): Access mode - "read" for read-only access, "write" for read-write access

**Returns**:
- `*FrozenDB`: Database instance for successful operations
- `error`: Error information for failed operations

**Error Types**:
- `InvalidInputError`: Invalid path format, empty path, invalid mode value
- `PathError`: File not found, permission denied, parent directory issues
- `CorruptDatabaseError`: Header validation failures, invalid file format
- `WriteError`: Lock acquisition failures, file descriptor issues

**Behavior**:
- Validates input parameters before filesystem operations
- Opens file descriptor and reads first 64 bytes for header validation
- Validates header per v1_file_format.md specification
- Acquires exclusive lock (write mode only) - readers need no locks
- Returns ready-to-use database instance on success
- Ensures all resources are cleaned up on any error

### Close

Closes the database connection and releases all resources.

```go
func (db *FrozenDB) Close() error
```

**Parameters**:
- None (method receiver)

**Returns**:
- `error`: Error information for cleanup failures (nil on success)

**Behavior**:
- Thread-safe and idempotent (multiple calls safe)
- Uses mutex to coordinate cleanup when called from multiple goroutines
- Releases file lock (write mode only)
- Closes file descriptor
- Marks instance as closed under mutex protection
- Returns nil if already closed or cleanup successful
- Reports cleanup errors without panicking

## Constants

### Access Modes

```go
const (
    ModeRead  string = "read"  // Read-only access, no lock needed
    ModeWrite string = "write" // Read-write access with exclusive lock
)
```

## Data Structures

### FrozenDB (Internal)

Database instance structure (internal implementation detail, not exposed in API).

```go
type FrozenDB struct {
    // Implementation details hidden from API users
}
```

## Usage Examples

### Basic Read Access

```go
// Open database for reading
    db, err := NewFrozenDB("/data/mydb.fdb", ModeRead)
if err != nil {
    log.Fatal("Failed to open database:", err)
}
defer db.Close()

// Database is ready for read operations
// Future read methods would be called here
```

### Basic Write Access

```go
// Open database for writing
    db, err := NewFrozenDB("/data/mydb.fdb", ModeWrite)
if err != nil {
    log.Fatal("Failed to open database for writing:", err)
}
defer db.Close()

// Database is ready for write operations
// Future write methods would be called here
```

### Error Handling

```go
    db, err := NewFrozenDB("/data/mydb.fdb", ModeRead)
if err != nil {
    switch e := err.(type) {
    case *InvalidInputError:
        log.Printf("Invalid input: %s", e.Message)
    case *PathError:
        log.Printf("Filesystem error: %s", e.Message)
    case *CorruptDatabaseError:
        log.Printf("Database corruption detected: %s", e.Message)
    case *WriteError:
        log.Printf("Lock/operation error: %s", e.Message)
    default:
        log.Printf("Unknown error: %v", err)
    }
    return
}
defer db.Close()
```

### Concurrent Access Patterns

```go
// Multiple readers are safe
for i := 0; i < 5; i++ {
    go func() {
        db, err := NewFrozenDB("/data/mydb.fdb", ModeRead)
        if err != nil {
            log.Printf("Reader %d failed: %v", i, err)
            return
        }
        defer db.Close()
        
        // Perform read operations
        log.Printf("Reader %d operating", i)
    }()
}

// Only one writer succeeds
    db, err := NewFrozenDB("/data/mydb.fdb", ModeWrite)
if err != nil {
    if errors.Is(err, &WriteError{}) {
        log.Println("Another writer has the database locked")
    }
    return
}
defer db.Close()
```

## Functional Requirements Mapping

| API Function | FR Requirement | Behavior |
|--------------|----------------|----------|
| NewFrozenDB | FR-001 | Provides NewFrozenDB(path string, mode string) (*FrozenDB, error) |
| Constants | FR-002 | Defines ModeRead = "read" and ModeWrite = "write" |
| NewFrozenDB | FR-003 | Validates mode parameter and uses spec 001 file path semantics |
| NewFrozenDB | FR-004 | Opens file descriptor, then validates frozenDB v1 header |
| NewFrozenDB | FR-005 | Acquires exclusive lock only after valid header AND mode is ModeWrite |
| NewFrozenDB | FR-006 | Maintains open file descriptor and lock until Close() is called |
| Close | FR-007 | Provides idempotent Close() method that flushes, closes fd, and releases locks |
| NewFrozenDB | FR-008 | Allows multiple readers (no locks) and at most one writer (exclusive lock) to open concurrently |
| NewFrozenDB | FR-009 | Returns WriteError immediately when trying to open in write mode while another writer holds lock |
| NewFrozenDB | FR-010 | Ensures operations on different database files do not interfere |
| NewFrozenDB | FR-011 | Uses fixed memory regardless of database file size |
| NewFrozenDB | FR-012 | Closes file descriptors and releases any acquired locks for ALL error conditions |
| NewFrozenDB | FR-013 | Returns CorruptDatabaseError for header validation failures |
| NewFrozenDB | FR-014 | Returns WriteError for lock acquisition failures |
| NewFrozenDB | FR-015 | Returns InvalidInputError for invalid path/mode parameters |
| NewFrozenDB | FR-016 | Returns PathError for filesystem access issues |

## Concurrency Guarantees

### Thread Safety

- `NewFrozenDB`: Thread-safe for concurrent calls on different files
- `Close`: **Thread-safe and idempotent** for the same instance - multiple goroutines can safely call Close() concurrently
- Instance methods: Not thread-safe across goroutines (use one instance per goroutine)

**Thread Safety Rationale**:
- `Close()` must be thread-safe because it's a cleanup method that may be called from defer statements in different goroutines or from signal handlers
- Mutex to prevent double-free/close scenarios
- Other operations require one instance per goroutine to avoid complex synchronization overhead

### File System Coordination

- Multiple readers: Allowed concurrent access with no locks (append-only safe)
- Multiple writers: Only one at a time with exclusive locks
- Mixed readers/writers: Readers operate freely, writers use exclusive locks
- Different files: No interference between separate database files

## Performance Characteristics

### Resource Usage

- Memory: Fixed constant usage regardless of database size
- File descriptors: One per open instance
- Locks: One per open instance
- CPU: Header validation is O(1) constant time

### Latency

- Database opening: <100ms for typical files
- Lock acquisition: Immediate failure if contested (non-blocking, write mode only)
- Resource cleanup: <10ms regardless of database size

## Backwards Compatibility

### Format Compatibility

- Opens files created by Create() function from spec 001
- Follows v1_file_format.md specification exactly
- Maintains append-only immutability from create functionality
- Preserves append-only file attributes

### API Stability

- Public API signatures are stable
- Error types follow existing patterns
- Constants are exported for user code
- Internal implementation details are hidden
