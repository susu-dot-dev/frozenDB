# Data Model: Read-Mode File Updates

**Feature**: 039-read-mode-updates  
**Date**: 2026-02-01  
**Status**: Phase 1 Complete

## Overview

This document defines new data entities, state transitions, and validation rules introduced by the read-mode file updates feature. The feature adds internal file watching capabilities to FileManager without modifying the external API or file format.

## New Data Entities

### File Watcher (Internal Component)

**Purpose**: Monitors file system events to detect when another process modifies the database file

**Lifecycle**: Created during DBFile initialization (read mode only), destroyed during Close()

**Attributes**:
- `watcher`: `*fsnotify.Watcher` - fsnotify library instance

**Ownership**: Embedded within FileManager struct; not exposed through public API

**Validation Rules**:
- Watcher MUST be created only when `mode == MODE_READ` (FR-001)
- Watcher MUST successfully initialize or DBFile creation fails (FR-007)
- Watcher MUST be listening before initial file size is captured (FR-003)
- Watcher MUST be cleanly destroyed during Close() with no goroutine leaks (FR-008)

### Update Cycle (Event Processing Unit)

**Purpose**: Represents a single serialized sequence of file size update and subscriber notifications

**Lifecycle**: Triggered by fsnotify.Write event, completes when last callback returns

**Sequence**:
1. File modification event detected (fsnotify.Write)
2. Current file size read from OS
3. New size compared to previous size
4. If changed: subscriber callbacks invoked in order
5. Cycle completes (next event can begin)

**State Transitions**:
```
Idle → Event Received → Size Checking → Callbacks Invoked → Idle
       ^                |                                     |
       |                |--(no size change)-----------------→ |
       |                |--(watcher closed)------------------→ Stopped
       |                                                       |
       |<---(error from callback)-----------------------------'
```

**Validation Rules**:
- Only ONE update cycle MUST be active at any time (FR-004)
- File size update MUST complete before callbacks invoked (FR-005)
- If size unchanged, callbacks MUST NOT be invoked (FR-005)
- If callback returns error, cycle MUST stop immediately (FR-006)

**Timing Requirements**:
- Cycle must complete within reasonable time (AS-004 assumes sub-second callbacks)
- Close() blocks until active cycle completes (FR-008)
- No timeout enforced (caller responsibility to ensure fast callbacks)

### File Modification Event (External Input)

**Purpose**: Notification from fsnotify indicating database file has been modified

**Source**: Linux inotify system via fsnotify library

**Attributes**:
- `event.Name`: File path that changed
- `event.Op`: Operation type (fsnotify.Write, etc.)

**Filtering**:
- MUST only process `fsnotify.Write` events (FR-001)
- MUST filter by database file path (ignore other files in directory)
- MUST handle `fsnotify.ErrEventOverflow` by triggering update cycle

**Validation Rules**:
- Events MUST be processed serially (one at a time)
- Events received after watcher closed MUST be ignored (goroutine exits)
- Invalid events (nil, closed channel) MUST terminate watcher goroutine

## Modified Data Entities

### FileManager Struct (internal/frozendb/file_manager.go)

**New Fields Added**:
```go
type FileManager struct {
    // Existing fields...
    file         atomic.Value
    writeChannel atomic.Value
    writerWg     sync.WaitGroup
    currentSize  atomic.Uint64
    mode         string
    subscribers  *Subscriber[func() error]
    
    // NEW FIELD (read mode only)
    watcher *fsnotify.Watcher  // nil in write mode and after Close()
}
```

**Field Semantics**:
- `watcher`: Non-nil only in read mode; nil in write mode and after Close()

**Initialization State (Read Mode)**:
```
Before NewDBFile returns:
1. watcher != nil (initialized)
2. watcher.Add(path) succeeded
3. goroutine started (writerWg counter = 1)
4. currentSize set to initial file size
```

**Closed State**:
```
After Close() returns:
1. watcher closed (Events and Errors channels closed)
2. writerWg counter = 0 (goroutine exited)
3. No events being processed
```

### currentSize Field Semantics (Read Mode)

**Existing Behavior (Write Mode)**:
- Updated by `processWrite()` after successful write
- Incremented atomically: `fm.currentSize.Add(appendSize)`

**New Behavior (Read Mode)**:
- Updated by file watcher goroutine when Write event fires
- Replaced atomically: `oldSize := fm.currentSize.Swap(newSize)`
- Read for comparison to detect size changes

**Consistency Guarantee**:
- Size() always returns current atomic value
- Readers see either old size or new size, never partial update
- Update cycle ensures size update visible before callbacks invoked

## State Transitions

### DBFile Lifecycle (Read Mode)

```
┌──────────────┐
│  Not Created │
└──────┬───────┘
       │ NewDBFile(path, MODE_READ)
       │
       ├─→ Open file
       ├─→ Capture initial size
       ├─→ Create watcher
       ├─→ Add watch
       ├─→ Start goroutine
       │
       ▼
┌──────────────┐         fsnotify.Write event
│   Active     │◄────────────────────┐
│  (watching)  │                     │
└──────┬───────┘                     │
       │                             │
       │ Update Cycle ───────────────┘
       │
       │ Close()
       │
       ├─→ Close watcher (closes channels)
       ├─→ Wait for goroutine (writerWg)
       ├─→ Close file
       │
       ▼
┌──────────────┐
│    Closed    │
└──────────────┘
```

### Update Cycle State Machine

```
                     ┌─────────────┐
              ┌─────►│    IDLE     │◄────┐
              │      └─────┬───────┘     │
              │            │             │
              │            │ Write event │
              │            ▼             │
              │      ┌─────────────┐    │
              │      │   READING   │    │
              │      │  FILE SIZE  │    │
              │      └─────┬───────┘    │
              │            │             │
              │            ▼             │
              │      ┌─────────────┐    │
              │      │  COMPARING  │    │
no change     │      │    SIZES    │    │ changed
              │      └─────┬───────┘    │
              │            │             │
              │            ├─────────────┤
              │            │             │
              │            ▼             │
              │      ┌─────────────┐    │
              │      │  INVOKING   │    │
              │      │  CALLBACKS  │    │
              │      └─────┬───────┘    │
              │            │             │
              │            │ all succeed │
              └────────────┴─────────────┘
                           │
                           │ callback error
                           ▼
                     ┌─────────────┐
                     │   STOPPED   │
                     └─────────────┘
```

**State Definitions**:

- **IDLE**: Waiting for next event; no cycle active; writerWg blocked on receive
- **READING FILE SIZE**: Executing `os.Stat()` syscall to get current file size
- **COMPARING SIZES**: Comparing new size to previous size stored in currentSize
- **INVOKING CALLBACKS**: Iterating through subscriber snapshot, calling each callback
- **STOPPED**: Goroutine terminated due to error or shutdown signal

**Transitions**:
- IDLE → READING: fsnotify.Write event received
- READING → COMPARING: Stat() returns successfully
- COMPARING → IDLE: Sizes equal (no change)
- COMPARING → INVOKING: Sizes differ (file grew)
- INVOKING → IDLE: All callbacks succeed
- INVOKING → STOPPED: Any callback returns error
- Any state → STOPPED: Watcher closed (Events/Errors channels closed)

## Validation Rules

### File Watcher Creation (FR-007)

**Rule**: Watcher MUST successfully initialize or NewDBFile MUST fail

**Validation Points**:
1. `fsnotify.NewWatcher()` returns error → Fail with WriteError
2. `watcher.Add(path)` returns error → Close watcher, fail with WriteError
3. Goroutine start succeeds (no validation, assumed safe)

**Error Messages**:
- "failed to create file watcher" (watcher creation)
- "failed to add watch for database file" (Add failure)

### Event Processing (FR-001, FR-004)

**Rule**: Only fsnotify.Write events processed, serially

**Validation Points**:
1. Event type: MUST have `fsnotify.Write` flag set
2. Event path: MUST match database file path
3. Concurrency: Only one goroutine processes events (serialization)
4. Channel closure: Detect with `event, ok := <-fm.watcher.Events`
5. Error handling: ErrEventOverflow triggers update cycle anyway

### Size Change Detection (FR-005)

**Rule**: Callbacks invoked ONLY if size actually changed

**Validation Logic**:
```go
newSize := getCurrentFileSize()
oldSize := fm.currentSize.Swap(newSize)  // Atomic read-modify-write

if newSize == oldSize {
    return  // No callbacks
}

// Size changed, invoke callbacks...
```

**Edge Cases**:
- Write event but size unchanged (metadata only) → No callbacks
- Multiple writes coalesced into one event → Callbacks invoked once
- Size exactly same after separate writes → No callbacks

### Callback Error Handling (FR-006, FR-009, FR-010, FR-011)

**Rule**: Stop on first callback error; callbacks MUST handle their own error state

**Critical Insight**: Update cycles run in background goroutines with no user action to propagate errors to. Callbacks MUST be self-sufficient in error handling.

**Validation Logic**:
```go
snapshot := fm.subscribers.Snapshot()
for _, callback := range snapshot {
    if err := callback(); err != nil {
        // Stop immediately, no further callbacks
        // The callback that returned error MUST have handled its own state
        // (e.g., Finder tombstoned itself)
        return
    }
}
```

## Finder Tombstoning Pattern (FR-010, FR-011)

All Finder implementations MUST support tombstoning to handle asynchronous refresh errors from background update cycles.

### Tombstoned State Requirements

**State Attribute**:
All Finders must track a tombstoned error state (protected by existing mutex) that is set when OnRowAdded() fails.

**State Transitions**:
```
Normal Operation → OnRowAdded Error → Tombstoned (permanent)
```

**Behavior Requirements**:
1. When Finder OnRowAdded() encounters ANY error, the Finder MUST set tombstonedErr field BEFORE returning the error
2. After tombstoning, ALL public Finder methods MUST check tombstonedErr FIRST
3. Tombstoned methods MUST return the tombstonedErr without accessing potentially stale data
4. Tombstoned state is permanent for the Finder's lifetime (no recovery)

### Affected Finder Implementations

**SimpleFinder** (internal/frozendb/simple_finder.go):
- Refresh callback: OnRowAdded (public method)
- Must tombstone on index mismatch or timestamp extraction errors
- Protected state: size, maxTimestamp, tombstonedErr (all protected by mu sync.Mutex)

**BinarySearchFinder** (internal/frozendb/binary_search_finder.go):
- Refresh callback: onRowAdded (private method)
- Must tombstone on index mismatch or timestamp extraction errors  
- Protected state: size, maxTimestamp, skewMs, tombstonedErr (all protected by mu sync.Mutex)

**InMemoryFinder** (internal/frozendb/inmemory_finder.go):
- Refresh callback: onRowAdded (private method)
- Must tombstone on index mismatch or map update errors
- Protected state: size, maxTimestamp, uuidIndex, transactionStart, transactionEnd maps, tombstonedErr (all protected by mu sync.RWMutex)
- **Critical**: If onRowAdded fails, in-memory indexes are inconsistent; tombstoning prevents access

### Concurrency Considerations

All Finders already have mutexes protecting their mutable state:
- SimpleFinder: sync.Mutex
- BinarySearchFinder: sync.Mutex
- InMemoryFinder: sync.RWMutex

The tombstonedErr field must be protected by the existing mutex. Public methods must acquire the appropriate lock, check tombstonedErr first, then proceed with normal logic.

### Why This Pattern is Required

1. **OnRowAdded callbacks** run in background goroutines (file watcher in read mode, processWrite in write mode)
2. **No user action to propagate error to** - these goroutines have nowhere to return errors
3. **If Finder continues with stale/corrupt state** → data inconsistency, wrong query results
4. **Tombstoning ensures**: async error → error surfaced on next user action (GetIndex call)
5. **Permanent state**: once tombstoned, always tombstoned (prevents any chance of serving stale data)

### Mode Independence

This pattern is IDENTICAL in both read mode and write mode. Finder callbacks run in background goroutines in both modes, so error handling must be the same regardless of which goroutine invoked the callback.

### Shutdown Validation (FR-008)

**Rule**: Close() blocks until active update cycle completes

**Validation Points**:
1. `fm.watcher.Close()` closes Events and Errors channels
2. Goroutine detects closed channels via `ok == false`
3. Goroutine exits, `writerWg.Done()` called
4. `writerWg.Wait()` in Close() unblocks
5. Close() returns

**Timing**:
- Typical: <100ms (SC-010)
- Maximum: Unbounded (depends on callback execution time)
- No timeout enforced (blocking is intentional)

## Data Flow Diagram

```
┌─────────────────────┐
│  Another Process    │
│  (Write Mode)       │
└──────────┬──────────┘
           │
           │ Appends data to file
           │
           ▼
┌─────────────────────┐
│   Database File     │
│   (filesystem)      │
└──────────┬──────────┘
           │
           │ OS inotify notification
           │
           ▼
┌─────────────────────┐
│  fsnotify.Watcher   │
│  (library)          │
└──────────┬──────────┘
           │
           │ Write event
           │
           ▼
┌─────────────────────┐
│  FileManager        │
│  Watcher Goroutine  │
└──────────┬──────────┘
           │
           ├─→ os.Stat(path)
           │   (read file size)
           │
           ├─→ currentSize.Swap()
           │   (atomic update)
           │
           ├─→ subscribers.Snapshot()
           │   (get callback list)
           │
           └─→ Invoke callbacks
               (Finder refresh, etc.)
               │
               ▼
         ┌─────────┐
         │ Finder  │
         │ Refresh │
         └─────────┘
```

## Memory Layout Considerations

### FileManager Memory (Read Mode)

**Additional Memory**:
- `*fsnotify.Watcher`: ~4KB (library overhead)
- Goroutine stack: ~4KB minimum (grows as needed)
- Total: ~8KB additional per read-mode DBFile

**Memory Remains Fixed**:
- No per-row allocations
- No per-event allocations (events processed inline)
- Snapshot creates slice of callback pointers (bounded by subscriber count)
- Complies with constitution requirement for fixed memory

### Concurrency Safety

**Atomic Operations**:
- `currentSize.Swap(newSize)`: Atomic read-modify-write, visible to all goroutines
- `currentSize.Load()`: Atomic read in Size() method

**No Race Conditions**:
- Only watcher goroutine modifies currentSize (read mode)
- Only writer goroutine modifies currentSize (write mode)
- Never both modes active in same FileManager instance
- Mode determined at creation, never changes

**Channel Safety**:
- Events/Errors: Written by fsnotify, read by watcher goroutine
- Proper closing order prevents use-after-close
- Goroutine detects closure via `ok == false` pattern

## Error Condition Mappings

### Watcher Initialization Errors

| Condition | Error Type | Error Message |
|-----------|-----------|---------------|
| fsnotify.NewWatcher() fails | WriteError | "failed to create file watcher" |
| watcher.Add() fails | WriteError | "failed to add watch for database file" |
| System resource limit | WriteError | (underlying error includes inotify limit details) |

### Runtime Errors

| Condition | Behavior | Error Propagation |
|-----------|----------|-------------------|
| ErrEventOverflow | Trigger update cycle anyway | No error returned |
| Callback returns error | Stop processing, exit goroutine | Error lost (logged if desired) |
| os.Stat() fails | Ignore event, continue | No error returned |
| Event on closed channel | Exit goroutine | No error (expected behavior) |

### Close() Errors

| Condition | Behavior | Error Returned |
|-----------|----------|----------------|
| watcher.Close() fails | Continue with file close | Ignored (best-effort) |
| Already closed | No-op (idempotent) | nil |
| Goroutine doesn't exit | Block forever | N/A (no timeout) |

## Testing Implications

### State Validation Points

Tests must verify:
1. Watcher is nil in write mode
2. Watcher is non-nil in read mode after NewDBFile
3. Watcher is closed after Close()
4. Goroutine exits after Close() (via goroutine counting)
5. currentSize updates atomically on Write events
6. Callbacks invoked only when size changes
7. Callbacks NOT invoked when size unchanged
8. Only one update cycle active at a time (stress test)

### Race Condition Tests

Tests must verify:
1. No missed writes during initialization window
2. Concurrent writes all detected (even rapid succession)
3. Close() during active update cycle waits for completion
4. No writes processed after Close() completes

### Error Injection Tests

Tests must verify:
1. watcher.Add() failure fails NewDBFile
2. Callback error stops processing and exits goroutine
3. File deletion during watch handled gracefully (future)

## Alignment with Constitution

This data model maintains alignment with frozenDB Constitution:

- **Immutability First**: No data modification, only observation
- **Data Integrity**: No changes to integrity mechanisms
- **Correctness Over Performance**: Serialization prioritizes correctness
- **Chronological Ordering**: Finder refresh maintains ordering
- **Concurrent Read-Write Safety**: Core feature enables safe concurrent access
- **Single-File Architecture**: Monitors single file, no additional files
- **Spec Test Compliance**: All validation rules testable via spec tests
