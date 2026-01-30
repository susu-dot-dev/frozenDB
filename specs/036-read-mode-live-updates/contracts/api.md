# API Contracts: Read-Mode Live Updates

**Date**: Fri Jan 30 2026  
**Feature**: Read-Mode Live Updates for Finders  
**Branch**: 036-read-mode-live-updates

This document specifies the complete API for the read-mode live updates feature, including method signatures, behavior, and integration points.

---


## 1. Internal API (internal/frozendb)

### 1.1 FileWatcher Constructor (Internal to Finder)

**Purpose**: Creates and initializes a FileWatcher for monitoring database file changes using fsnotify.

**Visibility**: Internal to frozendb package. Called only by Finder implementations during construction.

```go
// NewFileWatcher creates a FileWatcher using fsnotify for file system events
// Launches background goroutine to process file change events from fsnotify channels
//
// Parameters:
//   - dbFilePath: Filesystem path to database file
//   - dbFile: Database file interface for reading
//   - onRowAdded: Callback function to notify parent Finder of new rows
//   - onError: Callback function to notify parent Finder of errors
//   - rowSize: Fixed row size from header
//   - initialSize: Initial file size at start of parent Finder's scan
//
// Returns:
//   - *FileWatcher: Fully initialized watcher with goroutine running
//   - error: InternalError if fsnotify initialization fails
//
// Thread Safety: Not thread-safe; called once during Finder construction
func NewFileWatcher(dbFilePath string, dbFile DBFile, onRowAdded func(int64, Row), 
                    onError func(error), rowSize int32, initialSize int64) (*FileWatcher, error)
```

**Error Conditions**:

| Error Type | Condition |
|------------|-----------|
| `InternalError` | fsnotify.NewWatcher() failed |
| `InternalError` | watcher.Add(dbFilePath) failed |

**Platform Support**: Linux only (fsnotify inotify backend)

**Behavior Notes**:
- Creates fsnotify.Watcher instance
- Adds watch for database file path
- Launches background goroutine to read from watcher.Events and watcher.Errors channels
- Sets lastProcessedSize to initialSize
- Returns error if fsnotify initialization or watch addition fails
- Uses callbacks instead of Finder interface for cleaner decoupling
- Capitalized despite being internal - won't be exported outside module due to /internal/ path

---

### 1.2 FileWatcher Cleanup

**Purpose**: Stops monitoring and releases resources.

```go
// Close releases file watcher resources and stops background goroutine
// Closes fsnotify.Watcher which closes event channels, causing goroutine to exit
//
// Returns:
//   - error: First error encountered during cleanup
//
// Thread Safety: Safe for concurrent calls; idempotent
func (fw *FileWatcher) Close() error
```

**Error Conditions**:

| Error Type | Condition |
|------------|-----------|
| `InternalError` | fsnotify watcher.Close() fails |

**Behavior Notes**:
- Calls watcher.Close() which closes Events and Errors channels
- Background goroutine exits when it detects closed channels
- Idempotent (safe to call multiple times)
- No need for separate stopChan - channel closure signals shutdown

---

### 1.3 FileWatcher Event Loop (Internal)

**Purpose**: Background goroutine that monitors fsnotify channels for file system events.

```go
// watchLoop is the background goroutine that monitors fsnotify event channels
// Runs until Close() is called (channels are closed)
//
// Internal implementation - not exposed
func (fw *FileWatcher) watchLoop()
```

**Behavior Notes**:
- Runs in background goroutine launched by NewFileWatcher()
- Uses Go select statement to read from watcher.Events and watcher.Errors
- On Write event: calls processBatch() to read and process new rows
- On error: calls onError callback and exits
- On channel close: exits cleanly (watcher.Close() was called)
- No need for Unix pipes, bridge goroutines, or stopChan coordination

---

### 2.1 Finder Interface Modifications

**Purpose**: Support for live updates and error handling during incremental updates.

```go
// OnError is called when an error occurs during live update processing
// Finder enters tombstone state and returns TombstonedError for subsequent operations
//
// Parameters:
//   - err: Error that occurred during batch processing
//
// Thread Safety: Must be safe for calls from FileWatcher goroutine
func (f Finder) OnError(err error)

// Close releases resources held by Finder
// Called by FrozenDB.Close() during database shutdown
//
// Returns:
//   - error: InternalError if cleanup fails; nil if successful
//
// Thread Safety: Called once during FrozenDB.Close()
func (f Finder) Close() error
```

**Error Conditions**:

| Error Type | Condition |
|------------|-----------|
| `TombstonedError` | Returned by GetIndex/GetTransactionStart/GetTransactionEnd after OnError() called |
| `InternalError` | Close() fails to release resources |

**Tombstone Behavior**:
- After OnError() called, Finder enters permanent error state
- GetIndex(), GetTransactionStart(), GetTransactionEnd() return TombstonedError
- MaxTimestamp() continues to work (no error return value)
- OnRowAdded() returns TombstonedError wrapping the original error
- Recovery requires closing and reopening database

---

### 2.2 Finder Constructor Updates

**Purpose**: Finder constructors now handle MODE parameter and create internal FileWatcher in read-mode.

```go
// NewInMemoryFinder creates an InMemoryFinder with optional file watching
//
// Parameters:
//   - dbFile: Database file interface
//   - dbFilePath: Filesystem path to database file (for file watching)
//   - rowSize: Fixed row size from header
//   - mode: MODE_READ or MODE_WRITE
//
// Returns:
//   - *InMemoryFinder: Fully initialized Finder with internal watcher (read-mode)
//   - error: CorruptDatabaseError, ReadError, InternalError (watcher init failure)
//
// Behavior:
//   - Captures initial file size once
//   - Scans database up to initial size
//   - In MODE_READ: Creates internal fileWatcher, handles init race condition
//   - In MODE_WRITE: No fileWatcher created
//
// Thread Safety: Not thread-safe; call once during FrozenDB initialization
func NewInMemoryFinder(dbFile DBFile, dbFilePath string, rowSize int32, 
                       mode OpenMode) (*InMemoryFinder, error)
```

**Behavior Notes**:
- Finder manages FileWatcher lifecycle internally
- No initialSize return value needed
- FileWatcher created via internal newFileWatcher() call
- Similar signatures for NewBinarySearchFinder and NewSimpleFinder
- SimpleFinder does not create FileWatcher (uses on-demand scanning)

---

## 3. Integration Points

### 3.1 FrozenDB.Open Sequence

**Updated Flow**:

```
Old: Open → Validate → NewFinder → Return DB
New: Open → Validate → NewFinder(mode) → Return DB
                              ↓
                    (Finder creates internal fileWatcher if MODE_READ)
```

**Behavior Notes**:
- Read-mode: Finder constructor creates and starts internal fileWatcher
- Write-mode: Finder constructor skips fileWatcher creation
- FrozenDB is unaware of FileWatcher existence
- No initialSize passing required
- Finder handles initialization race internally

---

### 3.2 FrozenDB.Close Sequence

**Updated Flow**:

```
Old: Close File → Return
New: Close Finder → Close File → Return
         ↓
   (Finder closes internal fileWatcher if exists)
```

**Behavior Notes**:
- Finder.Close() stops internal fileWatcher and releases resources
- FrozenDB doesn't directly interact with FileWatcher
- Watcher goroutine exits before file is closed (prevents dangling reads)
- Simpler shutdown sequence - FrozenDB just calls Finder.Close()

---

### 3.3 External Write Detection (Read-Mode)

**Flow**:

```
[External Writer Process]
    ↓ Writes row to file
[OS File System]
    ↓ Generates inotify event (Linux)
[fsnotify library]
    ↓ Sends event to Events channel
[FileWatcher goroutine]
    ↓ Receives from channel, calls processBatch()
[Finder]
    ↓ Updates internal state (maps, timestamps) via onRowAdded callback
[User Get() Call]
    ↓ Returns newly added key
```

---

## 4. Error Handling Summary

### 4.1 Initialization Errors (Fail Fast)

| API | Error Type | Condition | User Action |
|-----|------------|-----------|-------------|
| `NewFinder` | `InternalError` | NewFileWatcher() initialization fails (fsnotify.NewWatcher or watcher.Add error) | Check file permissions, verify path exists, retry, or report bug |
| `NewInMemoryFinder` | `ReadError` | File I/O failure during scan | Check file permissions and disk health |
| `NewInMemoryFinder` | `CorruptDatabaseError` | Invalid row format during scan | Restore from backup |
| `NewFrozenDB` | `PathError` | File does not exist | Check path |

---

### 4.2 Runtime Errors (Tombstone State)

| Operation | Error Type | Condition | Behavior |
|-----------|------------|-----------|--------|
| FileWatcher batch processing | `ReadError` | Disk I/O failure | Call `finder.OnError(err)`, enter tombstone |
| FileWatcher batch processing | `CorruptDatabaseError` | Invalid row format | Call `finder.OnError(err)`, enter tombstone |
| Finder API calls (after error) | `TombstonedError` | Any GetIndex/GetTransactionStart/GetTransactionEnd | Return error wrapping original cause |

**Tombstone Behavior**:

Once `finder.OnError()` is called:
- ✅ `GetIndex()` returns `TombstonedError`
- ✅ `GetTransactionStart()` returns `TombstonedError`
- ✅ `GetTransactionEnd()` returns `TombstonedError`
- ✅ `MaxTimestamp()` continues to work (no error return value)
- ✅ `OnRowAdded()` returns `TombstonedError` wrapping the original error

**Recovery**: User must close and reopen database to recover from tombstone state.

---

## 5. Performance Characteristics

### 5.1 Memory

| Component | Overhead |
|-----------|----------|
| FileWatcher struct | ~128 bytes per database |
| fsnotify.Watcher internal state | ~1-2 KB (platform-specific) |
| Finder overhead | No additional per-row overhead (existing Finder memory usage unchanged) |

**Total**: Minimal memory impact for live updates feature.

---

### 5.2 CPU

| Scenario | Overhead |
|----------|----------|
| Idle (no writes) | 0% (goroutine blocked on channel) |
| Active writes | ~1-5% (batch processing overhead) |
| Initialization | Typically <10ms additional overhead |

---

### 5.3 Latency

| Metric | Target | Requirement |
|--------|--------|-------------|
| Write → Notification | <100ms typical | <2 seconds (SC-001) ✅ |
| Batch Processing | 10-50µs per row | - |

**Conclusion**: All performance requirements met.

---


### 7.3 Spec Tests

All functional requirements must have corresponding spec tests:

- `Test_S_036_FR_001_ReadModeEnablesFileWatching`
- `Test_S_036_FR_002_WriteModeDisablesFileWatching`
- `Test_S_036_FR_003_NewKeysDetected`
- `Test_S_036_FR_004_InitializationRacePrevention`
- `Test_S_036_FR_005_DataCorruptionPrevention`
- `Test_S_036_FR_006_PartialRowHandling`
- `Test_S_036_FR_007_WatcherFailureHandling`
- `Test_S_036_FR_008_RapidWriteHandling`

---

This API contract document specifies all public and internal APIs for the read-mode live updates feature.
