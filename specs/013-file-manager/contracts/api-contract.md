# API Contracts: FileManager Implementation

**Feature**: 013-file-manager\
**Date**: 2026-01-20\
**Repository**: github.com/susu-dot-dev/frozenDB

## 1. Public API Contract

### 1.1 FileManager Constructor

```go
// NewFileManager creates a new FileManager instance
//
// Parameters:
//   - filePath: path to the frozenDB file
//
// Returns:
//   - *FileManager: configured FileManager instance
//   - error: error if initialization fails
//
// Preconditions:
//   - filePath must be a valid file path
//   - file must exist and be readable
//
// Postconditions:
//   - FileManager is ready for operations
//   - File handle is opened and file size is determined from os.Stat
//   - writeChannel is nil (no active writer)
//   - tombstone is false (operational state)
//
// Thread Safety: Returns a thread-safe FileManager instance
func NewFileManager(filePath string) (*FileManager, error)
```

### 1.2 Read Method Contract

```go
// Read reads raw bytes from the file starting at the specified position
//
// Parameters:
//   - start: byte offset from file beginning where reading should start
//   - size: number of bytes to read
//
// Returns:
//   - []byte: the requested byte data
//   - error: error if read operation fails (e.g., out of bounds, file corruption)
//
// Preconditions:
//   - start must be >= 0
//   - size must be > 0
//   - start + size must not exceed current file end
//   - FileManager must not be tombstoned or closed
//
// Postconditions:
//   - Returns requested byte range from stable file data
//   - No modification to file state
//   - Concurrent reads with overlapping ranges return consistent data
//
// Constraints:
//   - start + size must not exceed current file end
//   - reads can access any stable byte ranges (not in-flight writes)
//   - multiple Read operations can execute concurrently
//   - no artificial size limits imposed on read operations
//
// Thread Safety: Safe for concurrent use by multiple goroutines
func (fm *FileManager) Read(start int64, size int) ([]byte, error)
```

### 1.3 SetWriter Method Contract

```go
// SetWriter acquires exclusive write access and starts processing write operations
//
// Parameters:
//   - dataChan: read-only channel for receiving Data payloads to write
//
// Returns:
//   - error: error if writer cannot be acquired (e.g., writer already active)
//
// Preconditions:
//   - dataChan must be non-nil
//   - FileManager must not be tombstoned or closed
//   - No active writer (writeChannel must be nil)
//
// Postconditions:
//   - writeChannel is set to dataChan
//   - Background goroutine started to process writes
//   - Writer exclusivity acquired
//
// Behavior:
//   - Returns error if another writer is already active
//   - Starts a writer goroutine that processes Data payloads sequentially
//   - Each Data payload's Response channel is signaled when write completes
//   - Writer exclusivity is released when dataChan is closed
//   - Write failures set tombstone flag and terminate writer
//
// Thread Safety: Only one caller can successfully obtain writer access at a time
func (fm *FileManager) SetWriter(dataChan <-chan Data) error
```

### 1.4 Close Method Contract

```go
// Close gracefully shuts down the FileManager and releases resources
//
// Returns:
//   - error: error if shutdown fails (nil on success or if already closed)
//
// Preconditions:
//   - FileManager must be in valid state (tombstone or operational)
//
// Postconditions:
//   - All in-flight write operations are completed and their response channels signaled
//   - All background goroutines are terminated
//   - File handle is closed
//   - closed flag is set to true
//   - No further operations allowed (operations return TombstonedError)
//
// Behavior:
//   - If already closed, returns nil immediately (idempotent)
//   - If active writer exists, waits for all pending Data payloads to be processed
//   - Signals completion on all pending Response channels before shutting down
//   - Closes the writeChannel to terminate writer goroutine
//   - Waits for writer goroutine to complete
//   - Closes underlying file handle
//   - Sets closed flag to true
//   - Ensures all goroutines are properly terminated
//
// Thread Safety: Safe to call from any goroutine, idempotent
func (fm *FileManager) Close() error
```

### 1.5 Size Method Contract

```go
// Size returns the current file end position
//
// Returns:
//   - int64: current file size in bytes
//
// Preconditions:
//   - FileManager must be operational (not tombstoned, not closed)
//
// Postconditions:
//   - Returns current file end position
//   - No modification to file state
//
// Thread Safety: Safe for concurrent use
func (fm *FileManager) Size() int64
```

### 1.6 IsTombstoned Method Contract

```go
// IsTombstoned returns whether the FileManager has encountered an unrecoverable error
//
// Returns:
//   - bool: true if tombstone flag is set (no further operations allowed)
//
// Preconditions:
//   - None (can be called on any FileManager instance)
//
// Postconditions:
//   - Returns current tombstone state
//   - No modification to file state
//
// Thread Safety: Safe for concurrent use
func (fm *FileManager) IsTombstoned() bool
```

## 2. Error Contract

### 2.1 TombstonedError

```go
// TombstonedError is returned when operations are attempted on a tombstoned FileManager
type TombstonedError struct {
    FrozenDBError
}

// NewTombstonedError creates a new TombstonedError
func NewTombstonedError(message string, err error) *TombstonedError
```

**Usage Context**: Returned by any operation when tombstone or closed flag is
set

### 2.2 Error Hierarchy

```go
// FrozenDBError is the base error type for all frozenDB errors
type FrozenDBError interface {
    error
    
    // GetCode returns the error code for programmatic handling
    GetCode() string
    
    // GetMessage returns the human-readable error message
    GetMessage() string
    
    // GetUnderlying returns the underlying error if any
    GetUnderlying() error
}

// Specific error types implement FrozenDBError interface:
// - InvalidActionError    (writer already active, invalid state transitions)
// - TombstonedError       (operations on failed FileManager)
// - InvalidInputError     (invalid parameters, boundaries)
// - IOError               (file system operations)
```

## 3. Data Contract

### 3.1 Data Payload

```go
// Data represents a write operation payload with completion notification
type Data struct {
    // Bytes contains the raw byte data to be appended to the file
    Bytes []byte

    // Response is a channel for signaling write completion status.
    // The channel will receive an error (nil for success, error for failure)
    // when the write operation completes. Channel must be buffered (at least 1).
    Response chan<- error
}
```

**Validation Requirements**:

- Bytes cannot be nil (can be empty)
- Response must be non-nil and buffered
- Response channel must be ready to receive error without blocking

## 4. Performance Contract

### 4.1 Time Complexity

| Operation      | Complexity | Description                           |
| -------------- | ---------- | ------------------------------------- |
| NewFileManager | O(1)       | File opening and initialization       |
| Read           | O(n)       | n = size parameter (direct file read) |
| SetWriter      | O(1)       | State check and goroutine start       |
| Close          | O(1)       | Resource cleanup                      |
| Size           | O(1)       | Direct field access                   |
| IsTombstoned   | O(1)       | Direct field access                   |

### 4.2 Space Complexity

| Operation            | Memory | Description                              |
| -------------------- | ------ | ---------------------------------------- |
| FileManager creation | O(1)   | Fixed size struct                        |
| Read operation       | O(n)   | n = size parameter (returned byte slice) |
| Write processing     | O(1)   | Per Data payload (sequential)            |

### 4.3 Concurrency Guarantees

```go
// Thread Safety Contract:
//
// Read Operations:
//   - Read(), Size(), IsTombstoned() use read locks
//   - Multiple concurrent reads allowed
//   - No blocking between read operations
//
// Write Operations:
//   - SetWriter(), Close() use exclusive write lock
//   - Only one writer can be active at any time
//   - Writer operations are mutually exclusive
//
// Consistency:
//   - All state changes are atomic within mutex critical sections
//   - No partial visibility to concurrent operations
//   - RWMutex ensures memory ordering guarantees
```

## 5. Integration Contract

### 5.1 FrozenDB Integration

FileManager is designed to work with higher-level frozenDB components:

```go
// Example integration pattern:
type FrozenDB struct {
    fileManager *FileManager
    // other fields...
}

func (db *FrozenDB) beginTransaction() (*Transaction, error) {
    // FileManager provides raw I/O capabilities
    // Transaction layer handles row formatting and semantics
}
```

### 5.2 File Format Independence

FileManager operates after header & checksum rows:

- No knowledge of frozenDB row format required
- Treats file as append-only byte stream
- Higher layers handle row boundaries and validation

## 6. Testing Contract

### 6.1 Spec Test Requirements

Each functional requirement FR-XXX must have corresponding spec test:

```go
// Naming convention: Test_S_013_FR_XXX_Description
//
// Location: module/file_manager_spec_test.go
//
// Requirements:
//   - Test exactly as specified in requirement
//   - No modifications after implementation without user permission
//   - Distinct from unit tests, focus on functional validation
```

### 6.2 Unit Test Coverage

```go
// Unit test requirements:
//
// Method Coverage:
//   - All public methods with success and error paths
//   - All private methods critical for functionality
//
// Edge Cases:
//   - Boundary conditions (file start/end)
//   - Concurrent access patterns
//   - Error propagation paths
//
// Concurrency:
//   - Thread safety under concurrent access
//   - Race condition detection
//   - Deadlock prevention
```

## 7. Validation Contract

### 7.1 Input Validation Matrix

| Input                         | Validation                                    | Error Type                   | Reference      |
| ----------------------------- | --------------------------------------------- | ---------------------------- | -------------- |
| SetWriter dataChan            | Must be non-nil, no active writer, not closed | InvalidActionError           | FR-003, FR-004 |
| Read start/size               | Boundary checking, not tombstoned, not closed | InvalidInputError            | FR-001         |
| NewFileManager filePath       | Valid file path, file exists and readable     | InvalidInputError, PathError | Initialization |
| All operations except Close() | Tombstone or closed state check               | TombstonedError              | FR-012         |

### 7.2 State Transitions

```go
// State transitions:
//   Initialized → WriterActive (SetWriter succeeds)
//   WriterActive → Ready (channel closes)
//   Ready → WriterActive (new SetWriter)
//   Any state → Tombstoned (unrecoverable error)
//   Any state → Closed (Close called, idempotent)
//   Closed → Closed (subsequent Close() calls, returns nil)
```

This contract document provides the complete API specification for implementing
FileManager functionality while maintaining frozenDB's constitutional principles
and ensuring thread safety, data integrity, and performance requirements.
