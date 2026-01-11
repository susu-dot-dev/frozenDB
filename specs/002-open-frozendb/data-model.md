# Data Model: Open frozenDB Files

**Feature**: 002-open-frozendb  
**Date**: 2026-01-09  
**Purpose**: Entity definitions and data structures for database opening functionality

## Primary Entities

### FrozenDB

The main database instance representing an open connection to a frozenDB file.

```go
type FrozenDB struct {
    // Core file resources
    file     *os.File           // Open file descriptor
    fileLock *syscall.Flock     // File lock handle (nil for read mode)
    
    // Database metadata from header
    header   Header             // Parsed header information
    mode     AccessMode         // Read or write access mode
    
    // State management
    mu       sync.Mutex         // Thread synchronization for cleanup
    closed   bool              // Closed flag (protected by mutex)
}
```

**Validation Rules**:
- Must be created through NewFrozenDB function only
- File descriptor must remain open until Close() called
- Lock must be acquired before any write operations (fileLock is nil for read mode)
- Header must be validated per v1_file_format.md specification
- Mode determines available operations (read vs write)

**State Transitions**:
1. **Opening**: File descriptor acquired → Header validated → Lock acquired if write mode → Instance ready
2. **Operating**: File operations allowed based on access mode
3. **Closing**: Lock released if present → File descriptor closed → Resources cleaned up

### AccessMode

Enumeration of supported database access modes.

```go
type AccessMode string

const (
    MODE_READ  AccessMode = "read"  // Read-only access, no lock needed
    MODE_WRITE AccessMode = "write" // Read-write access with exclusive lock
)
```

**Validation Rules**:
- Must be exactly "read" or "write"
- Mode determines lock type: read=shared, write=exclusive
- Mode cannot be changed after database open

### Header

Parsed representation of frozenDB v1 file header.

```go
type Header struct {
    Signature string // Must be "fDB"
    Version   int    // Must be 1
    RowSize   int    // 128-65536 bytes per row
    SkewMs    int    // 0-86400000 milliseconds time skew
}
```

**Validation Rules**:
- Signature must be exactly "fDB"
- Version must be exactly 1
- RowSize must be between 128 and 65536 inclusive
- SkewMs must be between 0 and 86400000 inclusive
- Header must be 64 bytes with JSON content + null padding + newline
- JSON format: `{"sig":"fDB","ver":1,"row_size":NNNN,"skew_ms":NNNN}`
- Maximum JSON content size: 58 bytes (leaving 6 bytes padding + newline)

## Supporting Entities

(removed unnecessary LockState enum - lock presence can be inferred from fileLock field)

(ValidationResult not needed for current implementation - errors are returned directly)

## Data Flow

### Opening Process

```
Input: path string, mode AccessMode
  ↓
1. Validate inputs (path format, mode value)
  ↓
2. Open file descriptor
  ↓
3. Read first 64 bytes
  ↓
4. Validate header format and content
  ↓
5. Acquire exclusive lock (write mode only)
  ↓
6. Create FrozenDB instance
  ↓
Output: *FrozenDB, error
```

### Closing Process

```
Input: FrozenDB instance
  ↓
1. Check if already closed (with mutex)
  ↓
2. Acquire mutex lock for cleanup coordination
  ↓
3. Release file lock (write mode only, if present)
  ↓
4. Close file descriptor
  ↓
5. Mark as closed (under mutex)
  ↓
Output: error (if any cleanup failed)
```

**Thread Safety Note**: Close() is designed to be thread-safe and can be called concurrently from multiple goroutines. This is important for cleanup scenarios where defer statements or signal handlers might invoke Close() simultaneously. The mutex ensures cleanup operations are coordinated and prevents double-close. Lock presence is inferred from fileLock field being nil (read mode) or non-nil (write mode).

## Relationships

### FrozenDB Composition

```
FrozenDB
├── file (os.File) - managed resource
├── fileLock (*syscall.Flock) - managed resource (nil for read mode)
├── header (Header) - parsed metadata
├── mode (AccessMode) - access restriction
├── mu (sync.RWMutex) - synchronization
└── closed (int32) - state flag
```

### Header to File Format

```
File bytes [0-64] → Header struct
├── Signature → "fDB"
├── Version → 1
├── RowSize → int (128-65536)
└── SkewMs → int (0-86400000)
```

## Memory Layout

### Fixed Memory Usage

The FrozenDB struct maintains constant memory usage regardless of database file size:

- Base struct: ~200 bytes (including mutex overhead)
- Header struct: 32 bytes
- File handles: OS-managed, not counted in Go memory
- No dynamic allocations after initialization

### Buffer Management

No large buffers are allocated:
- Header reading uses fixed 64-byte buffer
- File operations use OS-level buffering
- Enumeration processes one record at a time

## Concurrency Model

### Reader Concurrency

Multiple FrozenDB instances can open the same file in read mode:
- No file locks needed (append-only file is safe for concurrent reads)
- Independent file descriptors
- Internal mutex for instance coordination (Close() thread safety)
- No interference between readers

### Writer Exclusivity

Only one FrozenDB instance can open a file in write mode:
- Exclusive file lock prevents concurrent writers (append coordination)
- Readers operate freely without file lock coordination
- Lock acquisition fails immediately if contested
- Mutex-protected state changes prevent race conditions
- Internal mutex for instance coordination (Close() thread safety)

## Error Scenarios

### Validation Errors

- **InvalidInputError**: Path format issues, invalid mode values
- **PathError**: File not found, permission denied, parent directory issues
- **CorruptDatabaseError**: Header validation failures, malformed JSON, invalid field values

### Runtime Errors

- **WriteError**: Lock acquisition failures, file descriptor issues
- (General operation failures should use specific error types like WriteError or create new types as needed)

### Cleanup Errors

- Multiple Close() calls: Safe and idempotent
- Operations on closed database: Prevented with mutex-protected checks
- Resource cleanup failures: Reported but don't cause panics

## Integration Points

### Existing Create Functionality

- Reuses Header struct from create.go
- Follows same validation patterns
- Compatible with files created by Create() function
- Maintains append-only and immutability properties

### Future Read Operations

- Header fields provide metadata for future read implementations
- RowSize determines data record boundaries
- SkewMs used for UUIDv7 time-based lookups
- Access mode determines available future operations