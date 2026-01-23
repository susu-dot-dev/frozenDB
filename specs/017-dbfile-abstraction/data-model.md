# Data Model: DBFile Read/Write Modes and File Locking

**Date**: 2026-01-23  
**Feature**: 017-dbfile-abstraction  
**Status**: Draft

## Core Entities

### DBFile Interface (Enhanced)

**Description**: Extended interface for file operations with mode-specific locking support

**Fields/Methods**:
- `Read(start int64, size int32) ([]byte, error)` - Random access read operation
- `Size() int64` - File size in bytes  
- `Close() error` - Close file and release resources
- `SetWriter(dataChan <-chan Data) error` - Configure write channel
- `GetMode() string` - Get file access mode (NEW)

**Validation Rules**:
- Mode must be either "read" or "write"
- Read mode operations must reject write attempts
- Write mode operations must acquire exclusive lock before success

### FileManager (Implementation)

**Description**: Concrete implementation of DBFile interface with file locking capabilities

**Internal Fields**:
- `file *os.File` - Underlying file handle
- `mode string` - Access mode ("read" or "write")
- `path string` - File system path


### FileMode Enumeration

**Description**: Constants defining file access modes

**Values**:
- `MODE_READ = "read"` - Read-only access, no locking
- `MODE_WRITE = "write"` - Read-write access with exclusive locking

**Validation Rules**:
- Must be predefined constant values
- Case-sensitive matching required

## Key Relationships

### DBFile Interface Hierarchy
```
DBFile (interface)
├── FileManager (production implementation)
└── mockDBFile (test implementation)
```

### Usage Flow
```
Application → NewDBFile(path, mode) → DBFile interface → Transaction operations
```

### Locking Behavior Matrix

| Mode | Lock Type | Concurrent Access | Blocking |
|------|-----------|-------------------|----------|
| read | None | Multiple readers allowed | Never blocks |
| write | Exclusive (LOCK_EX) | Single writer only | Non-blocking (fast fail) |

## Data Flow

### Read Mode Path
1. `NewDBFile(path, "read")` → Validate mode → Open file O_RDONLY
2. Transaction operations → Direct file access through DBFile interface
3. No file locking applied → Multiple concurrent readers allowed

### Write Mode Path  
1. `NewDBFile(path, "write")` → Validate mode → Open file O_RDWR
2. Attempt exclusive lock acquisition → Success or fail fast
3. Transaction operations → File access through DBFile interface
4. Lock automatically released on Close()

## Error Conditions

### Constructor Errors
- Invalid mode string → InvalidInputError
- File not found (read mode) → PathError  
- Permission denied → PathError
- Lock acquisition failure → WriteError

### Operation Errors
- Write attempt on read-mode DBFile → InvalidActionError
- Read after file closed → TombstonedError
- Lock operations after close → TombstonedError

## Transaction Integration

### Read-Only Transactions
- Use DBFile opened in read mode
- Cannot call SetWriter() (returns InvalidActionError)
- All read operations work normally

### Write Transactions  
- Use DBFile opened in write mode
- Must acquire exclusive lock before SetWriter() success
- All operations supported (read and write)

## File System Integration

### Path Handling
- Relative and absolute paths supported
- File extension validation (.fdb) preserved
- Path security checks maintained

### Lock Implementation
- Uses OS-level flocks (syscall.Flock)
- Lock automatically released on process termination
- Non-blocking for write mode (matches current behavior)

## Configuration Constants

### Timeout Values
- Lock acquisition timeout: 0ms (non-blocking, fail fast)
- File operation timeouts: OS defaults
- Concurrent reader limit: Unlimited

### Buffer Sizes
- Read buffer size: Unchanged from current implementation
- Write channel buffer: Unchanged from current implementation

## Performance Characteristics

### Memory Usage
- Fixed memory footprint regardless of database size
- Additional per-DBFile overhead: minimal (mode string, lock bool)

### Concurrency Performance  
- Multiple readers: No blocking between readers
- Single writer: Exclusive access guarantee
- Mode switching: Not supported (immutable after creation)
