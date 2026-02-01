# API Contract: Read-Mode File Updates

**Feature**: 039-read-mode-updates  
**Date**: 2026-02-01  
**Status**: Phase 1 Complete

## Overview

This document specifies the internal API changes to FileManager for read-mode file watching. **CRITICAL**: This feature introduces ZERO public API changes (TC-001). All modifications are internal to the FileManager implementation.

## Modified Interfaces

### DBFile Interface (No Changes)

**Location**: `internal/frozendb/file_manager.go:29-37`

**Current Interface** (unchanged):
```go
type DBFile interface {
    Read(start int64, size int32) ([]byte, error)
    Size() int64
    Close() error
    SetWriter(dataChan <-chan Data) error
    GetMode() string
    WriterClosed()
    Subscribe(callback func() error) (func() error, error)
}
```

**Behavior Changes by Mode**:

#### Size() - Read Mode Behavior
```go
func (fm *FileManager) Size() int64
```

**Current Behavior** (all modes):
- Returns `int64(fm.currentSize.Load())`
- Thread-safe atomic read

**New Behavior** (read mode only):
- Returns current atomic value (same as before)
- Value updated by file watcher goroutine when Write events detected
- Always returns consistent value (never partial update)

**Write Mode**: No behavior change

**Concurrency**: Thread-safe, can be called while file watcher is active

**Example Usage** (unchanged):
```go
db, _ := NewDBFile("/path/to/db.fdb", MODE_READ)
size := db.Size()  // Returns current size, updated by watcher
```

#### Close() - Read Mode Behavior
```go
func (fm *FileManager) Close() error
```

**Current Behavior** (all modes):
- Closes file handle
- Releases file lock (write mode)
- Idempotent (safe to call multiple times)

**New Behavior** (read mode only):
- Signals watcher goroutine to stop via `watcher.Close()` (closes channels)
- Blocks on `writerWg.Wait()` until goroutine detects closed channels and exits
- Closes file handle
- Returns nil (errors ignored for best-effort cleanup)

**Blocking Semantics**: FR-008 requires Close() to block until active update cycle completes

**Timeout**: No timeout enforced; Close() blocks until goroutine exits

**Write Mode**: No behavior change (existing write mode behavior preserved)

**Concurrency**: Safe to call from any goroutine; serializes internally

**Example Usage** (unchanged):
```go
db, _ := NewDBFile("/path/to/db.fdb", MODE_READ)
defer db.Close()  // Blocks until watcher stops
```

#### Subscribe() - Read Mode Behavior
```go
func (fm *FileManager) Subscribe(callback func() error) (func() error, error)
```

**Current Behavior** (write mode):
- Registers callback invoked after successful writes
- Returns unsubscribe function

**New Behavior** (read mode):
- Registers callback invoked after file size changes detected
- Callback invoked when another process writes to file
- Same signature, same error handling, same unsubscribe pattern

**Callback Invocation Timing**:
- Write mode: After `processWrite()` updates currentSize
- Read mode: After file watcher detects size change

**Callback Error Handling**: Unchanged - first error stops processing (FR-006)

**Concurrency**: Thread-safe; callbacks invoked synchronously in update cycle

**Example Usage** (unchanged):
```go
db, _ := NewDBFile("/path/to/db.fdb", MODE_READ)
unsubscribe, _ := db.Subscribe(func() error {
    fmt.Println("File updated by another process")
    return nil
})
defer unsubscribe()
```

## Internal Implementation Details

### FileManager Struct Modifications

**Location**: `internal/frozendb/file_manager.go:39-46`

**New Fields** (private, internal only):
```go
type FileManager struct {
    // Existing fields (unchanged)
    file         atomic.Value
    writeChannel atomic.Value
    writerWg     sync.WaitGroup
    currentSize  atomic.Uint64
    mode         string
    subscribers  *Subscriber[func() error]
    
    // NEW FIELD (read mode only, private)
    watcher *fsnotify.Watcher  // File system watcher (nil in write mode)
}
```

**Field Visibility**: All fields remain private (lowercase); no public API exposure

**Memory Overhead** (read mode only):
- `watcher`: 8 bytes pointer (~4KB allocated by fsnotify)
- Total: ~4KB additional memory per read-mode DBFile

### NewDBFile Constructor Changes

**Location**: `internal/frozendb/file_manager.go:82-142`

**Signature**: No changes to signature
```go
func NewDBFile(path string, mode string) (DBFile, error)
```

**New Behavior** (read mode only):

**Initialization Sequence**:
```go
// 1. Existing file opening logic (unchanged)
file, err := os.OpenFile(path, os.O_RDONLY, 0)
// ... error handling

// 2. Existing file stat logic (unchanged)
fileInfo, err := file.Stat()
// ... capture initial size

// 3. NEW: Create and initialize watcher (read mode only)
if mode == MODE_READ {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        file.Close()
        return nil, NewWriteError("failed to create file watcher", err)
    }
    
    // Store watcher in FileManager
    fm.watcher = watcher
    
    // Add watch BEFORE starting goroutine (critical for FR-003)
    if err := watcher.Add(path); err != nil {
        watcher.Close()
        file.Close()
        return nil, NewWriteError("failed to add watch for database file", err)
    }
    
    // Start watcher goroutine
    fm.writerWg.Add(1)
    go fm.fileWatcherLoop()
}

// 4. Return initialized DBFile
return fm, nil
```

**Error Handling**:
- Watcher creation fails → Return WriteError, clean up file
- Watch add fails → Return WriteError, clean up watcher and file
- Any failure in read mode → Fail fast, no fallback (FR-007)

**Write Mode**: No changes to existing write mode initialization

### File Watcher Goroutine (New Internal Method)

**Location**: To be added in `internal/frozendb/file_manager.go`

**Signature** (private method):
```go
func (fm *FileManager) fileWatcherLoop()
```

**Purpose**: Event loop goroutine that processes file system events from fsnotify watcher

**Behavior**:
- Runs in dedicated goroutine for the lifetime of read-mode FileManager
- Processes fsnotify.Write events only (FR-001)
- Filters out non-Write events
- Triggers update cycle when Write event detected
- Handles fsnotify.ErrEventOverflow by triggering update cycle
- Exits when watcher channels are closed
- Ensures serialization: only one update cycle at a time (FR-004)

**Concurrency**: Single goroutine, no concurrent execution

**Shutdown**: Exits cleanly when watcher is closed

### File Update Processing (New Internal Method)

**Location**: To be added in `internal/frozendb/file_manager.go`

**Signature** (private method):
```go
func (fm *FileManager) processFileUpdate()
```

**Purpose**: Handles a single file update cycle

**Behavior**:
1. Reads current file size via os.Stat()
2. Updates currentSize atomically using Swap operation
3. Compares new size to old size
4. If size unchanged, returns immediately (FR-005)
5. If size changed, invokes subscriber callbacks in registration order (FR-006)
6. Stops on first callback error

**Size Check**: FR-005 requires skipping callbacks if size unchanged

**Callback Order**: Preserves registration order from subscribers.Snapshot()

**Error Handling**: FR-006 requires stopping on first callback error. Since this runs in background goroutine with no user action to propagate errors to, callbacks (especially Finders) must handle their own error state by tombstoning themselves before returning errors.

## Behavioral Contracts

### Serialization Guarantee (FR-004)

**Contract**: Only one update cycle executes at a time

**Mechanism**: Single goroutine processes events serially

**Verification**: Tests can trigger rapid Write events and instrument callbacks to track concurrent executions, asserting no overlap detected

### Race-Free Initialization (FR-002, FR-003)

**Contract**: Zero timing gaps where writes could be missed

**Mechanism**:
1. Capture initial size during DBFile creation
2. Start watcher with Add() before returning from NewDBFile
3. Goroutine starts after Add() completes
4. First update cycle compares current size to initial size

**Timing Sequence**:
- T0: Open file
- T1: Capture file size → store in currentSize
- T2: Create fsnotify watcher
- T3: Add watch to file path
- T4: Start fileWatcherLoop goroutine
- T5: Return from NewDBFile

**Coverage**:
- Window T0-T3: Writes visible in initial size captured at T1
- Window T3-T5: Writes generate events, queued in watcher
- Window T5+: Goroutine processes queued events

**Result**: No writes missed

### Close() Blocking Guarantee (FR-008)

**Contract**: Close() blocks until active update cycle completes

**Mechanism**:
- Close() calls watcher.Close() which closes Events and Errors channels
- Goroutine detects closed channels and exits
- Close() calls writerWg.Wait() to block until goroutine completes
- After Wait() returns, Close() proceeds with cleanup

**Channel Closure Detection**:
Goroutine must check the `ok` boolean when receiving from channels to detect closure

**Timing**:
- Typical case: <100ms (SC-010)
- Worst case: Unbounded (depends on callback duration)
- No timeout: Intentional design choice

**Idempotency**: Multiple Close() calls safe; first call does work, others no-op

## Error Contracts

### Watcher Initialization Errors (FR-007)

**Contract**: Watcher initialization failure fails NewDBFile entirely

**Error Types**:
- `NewWriteError("failed to create file watcher", err)` - watcher creation
- `NewWriteError("failed to add watch for database file", err)` - Add() failure

**No Fallback**: Read mode requires working watcher; no static-size fallback

**Example**:
```go
db, err := NewDBFile("/path/to/db.fdb", MODE_READ)
if err != nil {
    // err is WriteError indicating watcher failure
    // Database NOT opened, no cleanup needed
}
```

### Callback Error Handling (FR-006)

**Contract**: First callback error stops processing chain; callbacks MUST tombstone themselves

**Critical Insight**: File watcher runs in background goroutine with no user action to return errors to. When a callback fails:
1. The watcher stops invoking remaining callbacks
2. **The callback that failed MUST tombstone itself** (Finder's responsibility)
3. The watcher cannot fix the callback's state - this is the callback's responsibility

**Behavior**:
If callbacks are [cb1, cb2, cb3]:
- cb1() returns nil → continue
- cb2() returns error → STOPS HERE (cb2 must have tombstoned itself)
- cb3() is not called

**Finder Tombstoning Requirements**:
All Finder implementations must:
1. Add tombstoned error state field (protected by existing mutex)
2. Set tombstoned state BEFORE returning any error from refresh callback
3. Check tombstoned state FIRST in all public methods
4. Return TombstonedError if tombstoned, preventing access to stale data

**Why This Pattern is Essential**:
1. Background goroutines (file watcher, write mode processWrite) can't return errors to users
2. If Finder has stale data but continues returning results → data inconsistency
3. Tombstoning ensures errors are propagated to the NEXT user action (Get call)
4. Once tombstoned, Finder stays tombstoned (permanent error state)

**Mode Independence**: This pattern applies to BOTH read mode (file watcher) and write mode (processWrite callback invocation). The error handling is identical regardless of what triggered the update.

## Integration Points

### Finder Integration

**Component**: `internal/frozendb/finder.go` (SimpleFinder, BinaryFinder, InMemoryFinder)

**Integration Pattern**:
Finders subscribe to file updates during initialization. The refresh callback is invoked by:
- Read mode: File watcher goroutine when file size changes
- Write mode: processWrite goroutine after successful writes

The same callback is used for both modes with identical error handling.

**Finder.Refresh Signature**:
Refresh callbacks follow the pattern: `func() error`

**Refresh Behavior Requirements**:
1. Re-read current file size from DBFile.Size()
2. Attempt to update internal Finder state
3. If ANY error occurs, set tombstoned state BEFORE returning error
4. Return error (which stops callback chain per FR-006)

**Tombstoning Contract**:
All Finder implementations must:
- Add tombstoned error state field (protected by existing mutex)
- Set tombstoned state on ANY refresh error BEFORE returning
- Check tombstoned state FIRST in all public methods (Get, All, etc.)
- Return TombstonedError if tombstoned, preventing stale data access
- Tombstoned state is permanent (no recovery mechanism)

**Affected Finders**:
- SimpleFinder: Uses OnRowAdded callback, protects size and maxTimestamp
- BinarySearchFinder: Uses onRowAdded callback, protects size, maxTimestamp, skewMs
- InMemoryFinder: Uses onRowAdded callback, protects size, maxTimestamp, and in-memory indexes

**Why Tombstoning is Required**:
1. Refresh() is called from background goroutines (file watcher or processWrite)
2. These goroutines have no user action to return errors to
3. If Refresh() fails but Finder continues operating → stale data returned to users
4. Tombstoning ensures: error in background → error on next user action
5. This maintains data consistency even when errors occur asynchronously

**Mode Independence**: This pattern is IDENTICAL in both read mode and write mode. The Finder doesn't know or care which mode triggered the refresh - it just maintains correctness.

### Transaction Integration

**Component**: `internal/frozendb/transaction.go`

**Integration**: None - transactions don't interact with file watcher

**Rationale**:
- Transactions only exist in write mode
- File watcher only active in read mode
- Never both in same FileManager instance

## Performance Characteristics

### Latency

**File Size Update**: O(1) - single atomic Swap operation

**Callback Invocation**: O(n) where n = number of subscribers (typically 1-2)

**Total Update Cycle**: Dominated by callback duration (typically <100ms)

**Get() Latency**: <1 second to see new keys after write (SC-001)

### Memory

**Per-DBFile Overhead** (read mode):
- fsnotify.Watcher: ~4KB
- Goroutine stack: ~4KB minimum
- Total: ~8KB per read-mode instance

**Fixed Memory**: Does not scale with database size (constitutional requirement)

### Thread Safety

**Atomic Operations**: currentSize.Swap(), currentSize.Load()

**Concurrency**:
- Size() callable from any goroutine (atomic read)
- Subscribe() callable from any goroutine (thread-safe)
- Close() callable from any goroutine (serializes internally)
- Callbacks invoked on watcher goroutine (single-threaded)

**No Locks Required**: Atomics and single-goroutine processing eliminate lock contention

## Compatibility

### Backward Compatibility

**Public API**: Zero changes (TC-001) - all existing code works unchanged

**File Format**: No changes - watcher only reads, never writes

**Existing Tests**: All existing tests continue to pass (no behavior change in write mode)

### Forward Compatibility

**Mode Detection**: Clients detect read vs write mode via `GetMode()` (existing method)

**Feature Detection**: No explicit feature detection needed (transparent to callers)

## Testing Contracts

### Spec Test Requirements

**Location**: `internal/frozendb/file_manager_spec_test.go`

**Naming Convention**: `Test_S_039_FR_XXX_Description()`

**Coverage**:
- FR-001: File watcher created only in read mode, listens for Write events only
- FR-002: Initial file size captured before watcher starts
- FR-003: Zero timing gaps during initialization
- FR-004: Only one update cycle active at a time
- FR-005: Size update completes before callbacks; no callbacks if size unchanged
- FR-006: Callbacks in registration order; stop on first error; Finder tombstones
- FR-007: Watcher failure fails NewDBFile creation
- FR-008: Close() blocks until update cycle completes

**Example Test Approach for FR-003**:
Test should start a writer process, open read-mode database during active writes, wait for watcher to catch up, then verify all keys written are retrievable via Get() operations.

### Unit Test Requirements

**Location**: `internal/frozendb/file_manager_test.go`

**Coverage**:
- Watcher goroutine starts and stops cleanly
- processFileUpdate detects size changes correctly
- Callback invocation order preserved
- Error handling for fsnotify errors
- Idempotent Close() behavior
- Memory leak detection (goroutine counting)

## Open Questions

None - all clarifications resolved during specification phase. See spec.md Clarifications section for resolved questions.

## Summary

This feature adds internal file watching to FileManager for read-mode instances with:
- **Zero public API changes** (TC-001 compliance)
- **Race-free initialization** (FR-002, FR-003)
- **Serialized updates** (FR-004)
- **Fail-fast on errors** (FR-007)
- **Blocking shutdown** (FR-008)
- **Fixed memory overhead** (~8KB per read-mode instance)
- **<1s Get() latency** for newly written keys (SC-001)

All behavior changes are internal and transparent to existing clients.
