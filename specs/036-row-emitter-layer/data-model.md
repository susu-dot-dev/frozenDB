# Data Model: RowEmitter Layer

**Feature**: 036-row-emitter-layer  
**Date**: 2026-02-01  
**Purpose**: Define data structures and state management for the RowEmitter notification system

## Overview

The RowEmitter layer introduces new data structures for managing subscription-based row completion notifications. This document focuses on the entities, their state transitions, and validation rules.

## Data Structures

### Subscriber[T] (New - Generic Subscription Manager)

```go
// Subscriber manages a set of callbacks with thread-safe subscription/notification.
// Uses snapshot pattern to prevent deadlocks during notification.
type Subscriber[T any] struct {
    mu          sync.Mutex
    callbacks   map[int64]T
    nextID      int64
}
```

**Type Parameter**:
- `T`: Callback function type (e.g., `func() error`, `func(int64, *RowUnion) error`)

**Fields**:
- `mu`: Protects callbacks map and nextID during modifications
- `callbacks`: Maps subscription ID to callback function
- `nextID`: Auto-incremented subscription identifier (starts at 1)

**Purpose**: Reusable subscription manager that eliminates duplication between FileManager and RowEmitter. Provides Subscribe() to add callbacks and Snapshot() to get current callback list for notification.

### FileManager (Modified)

```go
type FileManager struct {
    file         atomic.Value       // stores *os.File
    writeChannel atomic.Value       // stores <-chan Data
    writerWg     sync.WaitGroup
    currentSize  atomic.Uint64
    mode         string              // "read" or "write"
    
    // New field for subscription
    subscribers  *Subscriber[func() error]
}
```

**New Field**:
- `subscribers`: Generic subscriber manager for DBFile-level callbacks

### RowEmitter (New)

```go
type RowEmitter struct {
    dbfile              DBFile
    dbfileUnsubscribe   func() error
    
    subscribers         *Subscriber[func(int64, *RowUnion) error]
    
    mu                  sync.Mutex
    lastKnownFileSize   int64
}
```

**Fields**:
- `dbfile`: Reference to monitored DBFile
- `dbfileUnsubscribe`: Closure to unsubscribe from DBFile on Close()
- `subscribers`: Generic subscriber manager for row-level callbacks
- `mu`: Protects lastKnownFileSize
- `lastKnownFileSize`: Bytes in file when last checked (used to calculate completed rows)

## State Management

### RowEmitter Initialization

**Initial State Determination**:
- Receive rowSize as parameter from caller (from header)
- Query DBFile for current size
- Set lastKnownFileSize to current file size
- Store rowSize for calculating row boundaries
- Create Subscriber[T] instance for row-level callbacks
- Subscribe to DBFile for future notifications
- Store DBFile unsubscribe closure for cleanup

**Constructor Parameters**:
- `dbfile DBFile`: The file to monitor
- `rowSize int`: The row size from database header (used for calculating row boundaries)

### State Transitions

#### On DBFile Notification

**Process Flow**:
1. Query new file size from DBFile
2. Compare with lastKnownFileSize to determine if growth occurred
3. Calculate which rows are now complete (mechanical transformation from file size)
4. For each newly completed row in chronological order:
   - Get subscriber snapshot via Subscriber[T].Snapshot()
   - Execute each callback with (index, row)
   - Stop on first error and propagate backward
5. Update lastKnownFileSize to new file size

**Key Pattern**: Lock is held only during file size reads/writes, NOT during callback execution

**Locking**:
```go
// Lock only for state access
mu.Lock()
oldSize := lastKnownFileSize
mu.Unlock()

newSize := dbfile.Size()

// Calculate completed rows from size difference
// (implementation detail, not stored state)

// Snapshot handles its own locking
snapshot := subscribers.Snapshot()

// Execute callbacks (no locks held)
for _, callback := range snapshot {
    callback(index, row)
}

// Lock only for state update
mu.Lock()
lastKnownFileSize = newSize
mu.Unlock()
```

#### Subscription Management

**Subscribe**:
- Delegates to Subscriber[T].Subscribe(callback)
- Returns unsubscribe closure that captures subscription ID
- FileManager: `subscribers.Subscribe(callback)` where callback is `func() error`
- RowEmitter: `subscribers.Subscribe(callback)` where callback is `func(int64, *RowUnion) error`

**Unsubscribe** (via returned closure):
- Idempotent: Calling multiple times is safe (delete on non-existent key is no-op)
- Immediate effect: Future notifications won't include this callback
- During notification: Callback completes current execution (already in snapshot)

**Notification**:
- Get snapshot via Subscriber[T].Snapshot()
- Execute callbacks from snapshot (no lock held)
- Stop on first error

### Locking Strategy

**Snapshot Pattern in Subscriber[T]**:
- Lock acquired only to copy callbacks map to slice
- Lock released before returning slice
- Caller executes callbacks WITHOUT holding any Subscriber[T] lock
- Prevents deadlock: callbacks can safely call Subscribe/Unsubscribe

**Why This Works**:
- Subscribe/Unsubscribe can be called during callback execution
- No deadlock: Subscriber[T] lock NOT held during callback
- New subscriptions not in current snapshot (receive future events only)
- Unsubscriptions complete current execution (callback already in snapshot)

**Lock Independence**:
- `Subscriber[T].mu`: Protects callbacks map and nextID
- `RowEmitter.mu`: Protects lastKnownFileSize
- These locks are **independent** and **never held simultaneously**
- No lock ordering requirements (they protect different data)

### Row Completion Callback Parameters

**RowEmitter Callback Signature**: `func(index int64, row *RowUnion) error`

**Parameters**:
- `index int64`: Zero-based position of the completed row in the file
- `row *RowUnion`: Complete row data (DataRow, NullRow, or ChecksumRow)

**Alignment**: Matches the `OnRowAdded(index int64, row *RowUnion) error` signature from the Finder interface

**Comparison to DBFile callbacks**:
- DBFile callbacks: `func() error` (no parameters, file-level notification only)
- RowEmitter callbacks: `func(int64, *RowUnion) error` (includes row-specific data)

**Notification Flow** (onDBFileNotification):
```
1. Acquire RowEmitter.mu → read lastKnownFileSize → release
2. Query DBFile.Size() and calculate completed rows (no locks)
3. Call subscribers.Snapshot():
   - Acquires Subscriber[T].mu → copies callbacks → releases
4. Execute callbacks (NO locks held)
5. Acquire RowEmitter.mu → update lastKnownFileSize → release
```

**Key Property**: Locks are held only for brief state access, never during I/O or callbacks

## Partial Row Completion Detection

**Mechanism**: File size change indicates row completion

**How It Works**:
- RowEmitter tracks only `lastKnownFileSize`
- On notification, compares old size vs. new size
- Mechanically calculates which rows are complete from size difference
- File size → row count is a deterministic calculation (not stored state)

**Scenarios**:
1. **Initialized with partial row**: Growth from partial → complete detected by size change
2. **Multiple rows written**: Size change indicates multiple completions
3. **No growth**: No size change = no new complete rows

## Data Flow Relationships

### Write Path
```
Transaction.Add(key, value)
  → DBFile.Write(data)
    → DBFile.notifySubscribers()
      → RowEmitter.onDBFileNotification()
        → RowEmitter.emitRows()
          → Finder.callback(index, row)
```

**Note**: Transaction MAY maintain a direct dependency on Finder for purposes other than row completion notification (e.g., querying for existing values during transaction processing). The decoupling requirement applies specifically to row completion event notification, not to all Transaction-Finder interactions.

### Error Propagation Path
```
Finder.callback returns error
  → RowEmitter stops chain
    → RowEmitter returns error to DBFile
      → DBFile stops chain
        → DBFile returns error to Transaction
          → Transaction can rollback
```

## Validation Rules

### Subscription ID
- MUST be auto-incremented starting from 1
- MUST be unique within instance lifetime
- MUST be monotonically increasing (never reused)
- int64 provides 9.2 quintillion IDs (292,471 years at 1M/sec)

### Callbacks
- MUST NOT be nil when passed to Subscribe()
- MUST execute synchronously
- Error return stops notification chain
- Panic propagates upward (no recovery)

### Thread Safety
- All map operations MUST be mutex-protected
- Snapshot MUST be created before callback execution
- Lock MUST NOT be held during callback execution
- Concurrent Subscribe/Unsubscribe serialized by mutex

### Notification Ordering
- Rows MUST be emitted in chronological order (by index)
- Multiple rows MUST emit separate sequential events (no batching)
- First error MUST stop chain (short-circuit)

## Error Conditions

| Scenario | Error Type | Behavior |
|----------|-----------|----------|
| Callback returns error | Caller-defined | Propagates backward, stops chain |
| Callback panics | Panic | Propagates upward, crashes process |
| Nil callback in Subscribe | InvalidInputError | Immediate return before subscription |
| DBFile query fails | DBFile error | Returned to RowEmitter |
| Unsubscribe after close | None | No-op (idempotent) |

## Testing Implications

### Critical State Transitions
1. Initialization → `lastKnownFileSize = current file size`
2. File grows → size change detected → calculate completed rows from size
3. Subscribe during notification → not in current snapshot
4. Unsubscribe during notification → completes current execution

### Data Integrity
- File size monotonically increases (append-only)
- Size → row calculation is deterministic
- Zero missed rows across size changes
- Chronological ordering maintained
