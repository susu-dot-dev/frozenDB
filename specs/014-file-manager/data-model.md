# Data Model Design: FileManager Implementation

**Feature**: 014-file-manager\
**Date**: 2026-01-20\
**Repository**: github.com/susu-dot-dev/frozenDB

## 1. FileManager Struct

### 1.1 Core FileManager Definition

```go
// FileManager provides thread-safe file operations for frozenDB
// with concurrent read support and exclusive write control
type FileManager struct {
    // filePath is the path to the database file
    filePath string

    // file is the underlying file handle
    file *os.File

    // mutex protects all access to the FileManager state
    mutex sync.RWMutex

    // writeChannel tracks the active writer (nil when no writer is active)
    writeChannel <-chan Data

    // currentSize is the current file end position
    currentSize int64

    // tombstone indicates the FileManager has encountered an unrecoverable error
    tombstone bool

    // closed indicates the FileManager has been gracefully closed
    closed bool

    // wg waits for background goroutines to complete
    wg sync.WaitGroup
}
```

### 1.2 State Inference

FileManager writer state is inferred from the writeChannel field:

- **No active writer**: writeChannel is nil
- **Active writer**: writeChannel is non-nil

FileManager operational state is inferred from closed and tombstone fields:

- **Operational**: closed = false, tombstone = false
- **Closed**: closed = true (graceful shutdown)
- **Tombstoned**: tombstone = true (unrecoverable error)

No explicit writerActive or state machine field needed.

### 1.3 Data Payload Struct

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

## 2. State Management

### 2.1 Writer Activity Detection

Writer activity is determined by checking if writeChannel is nil:

```go
func (fm *FileManager) hasActiveWriter() bool {
    fm.mutex.RLock()
    defer fm.mutex.RUnlock()
    return fm.writeChannel != nil
}
```

### 2.2 Operational State Detection

Operational state is determined by checking closed and tombstone flags:

```go
func (fm *FileManager) isOperational() bool {
    fm.mutex.RLock()
    defer fm.mutex.RUnlock()
    return !fm.closed && !fm.tombstone
}
```

### 2.3 Writer Lifecycle

1. **Writer Acquisition**: SetWriter() sets writeChannel to provided channel
2. **Writer Active**: writeChannel is non-nil, goroutine processes writes
3. **Writer Release**: When channel closes normally, writeChannel set to nil
4. **Graceful Close**: Close() waits for in-flight writes, then sets closed flag
5. **Error Termination**: Write failures set tombstone flag and terminate writer

### 2.4 Close Behavior

Close() is designed to be safe and idempotent:

- **First Call**: Completes pending writes, shuts down gracefully
- **Subsequent Calls**: Returns nil immediately, no action taken
- **With Active Writer**: Waits for all pending Data payloads to be processed
- **Always Idempotent**: Multiple calls safe and return nil

## 3. Error Types

### 3.1 TombstonedError

```go
// TombstonedError is returned when operations are attempted on a tombstoned FileManager
type TombstonedError struct {
    FrozenDBError
}
```

## 4. Thread Safety Model

### 4.1 Read Operations

Multiple concurrent reads are allowed using RWMutex read locks:

- Read() operations can run concurrently
- Size() and IsTombstoned() use read locks
- No blocking between read operations

### 4.2 Write Operations

Write operations require exclusive access:

- SetWriter() uses exclusive write lock
- Only one writer can be active at any time
- Writer goroutine processes Data payloads sequentially

### 4.3 State Transitions

All state changes are atomic within mutex critical sections:

- Writer acquisition/release is atomic
- Tombstone flag setting is atomic
- Size tracking updates are atomic

## 5. Integration Points

### 5.1 FileManager Creation

```go
// NewFileManager creates a new FileManager instance
func NewFileManager(filePath string) (*FileManager, error) {
    fm := &FileManager{
        filePath:    filePath,
        currentSize: 0, // Set from file stat
        tombstone:   false,
        closed:      false,
    }
    
    // Initialize file handle and validate file
    // File size determined by os.Stat
    // ...
    
    return fm, nil
}
```

### 5.2 File Position Tracking

FileManager tracks current file end position to:

- Enforce read boundaries (start + size ≤ currentSize)
- Validate write operations append to correct position
- Ensure reads only access stable, written data

## 6. Performance Characteristics

### 6.1 Memory Usage

FileManager has fixed memory footprint regardless of database size:

- Struct fields: O(1) memory
- No scaling with file size or row count
- Memory usage scales with read operation size (caller responsibility)

### 6.2 Concurrency Model

- **Read Concurrency**: Unlimited concurrent readers
- **Write Exclusivity**: Single exclusive writer
- **Read-Write Coordination**: RWMutex ensures proper coordination

This data model provides the foundation for thread-safe file operations while
maintaining frozenDB's architectural principles of immutability and append-only
operations.
