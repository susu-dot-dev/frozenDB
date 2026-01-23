# API Contracts: DBFile Read/Write Modes and File Locking

## Core API Contract

### NewDBFile Constructor

```go
// NewDBFile creates a new DBFile instance with specified access mode
// Returns DBFile interface configured for read or write operations with appropriate locking
func NewDBFile(path string, mode string) (DBFile, error)
```

**Parameters**:
- `path` (string, required): File system path to frozenDB database file
- `mode` (string, required): Access mode - either "read" or "write"

**Returns**:
- `DBFile`: Interface implementation configured with mode-specific behavior
- `error`: Error if mode invalid, file inaccessible, or lock acquisition fails

**Error Types**:
- `InvalidInputError`: Invalid mode parameter or path validation failure
- `PathError`: File not found (read mode) or path invalid
- `WriteError`: Unable to acquire exclusive lock (write mode) or file operation failures

### DBFile Interface Methods

```go
type DBFile interface {
    Read(start int64, size int32) ([]byte, error)
    Size() int64
    Close() error
    SetWriter(dataChan <-chan Data) error
    GetMode() string  // NEW: Returns the access mode
}
```

**Method Behavior by Mode**:

| Method | Read Mode | Write Mode |
|--------|-----------|------------|
| `Read()` | ✅ Normal operation | ✅ Normal operation |
| `Size()` | ✅ Normal operation | ✅ Normal operation |
| `Close()` | ✅ Normal operation | ✅ Releases lock + closes |
| `SetWriter()` | ❌ InvalidActionError | ✅ Normal operation |
| `GetMode()` | ✅ Returns "read" | ✅ Returns "write" |


## Performance Contracts

### Resource Limits
- Memory usage: Fixed overhead per DBFile instance
- Concurrent readers: No limit (subject to OS file handle limits)
- File locks: One exclusive lock per file maximum

## Integration Contracts

### Backward Compatibility
- Existing DBFile method signatures unchanged
- Existing Transaction behavior preserved
- Error types extend current error hierarchy

### Migration Path
1. Phase 1: New DBFile constructor alongside existing FileManager
2. Phase 2: Refactor open.go functions to use DBFile
3. Phase 3: Deprecate direct os.File operations in favor of DBFile
