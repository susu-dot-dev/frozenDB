# API Contract: RowEmitter Layer

**Feature**: 036-row-emitter-layer  
**Date**: 2026-02-01  
**Purpose**: Complete API specification for subscription-based row completion notifications

## Package Structure

```go
package frozendb

import (
    "github.com/google/uuid"
    "sync"
)
```

## Core Types

### Subscriber[T] - Generic Subscription Manager

```go
// Subscriber manages thread-safe subscription/notification with snapshot pattern
type Subscriber[T any] struct {
    mu        sync.Mutex
    callbacks map[int64]T
    nextID    int64
}

func NewSubscriber[T any]() *Subscriber[T]
func (s *Subscriber[T]) Subscribe(callback T) func() error
func (s *Subscriber[T]) Snapshot() []T
```

**Purpose**: Reusable subscription manager that eliminates duplication between FileManager and RowEmitter

**Type Parameter**: `T` is the callback function type

**Thread Safety**: All methods are thread-safe via mutex protection with snapshot pattern

**Usage**: 
- FileManager: `*Subscriber[func() error]`
- RowEmitter: `*Subscriber[func(int64, *RowUnion) error]`

## Overview

This document defines the complete API contract for the RowEmitter layer, including modifications to the DBFile interface and new RowEmitter types. The API uses closure-based subscription with snapshot-based thread-safety to enable decoupled row completion notifications.

## DBFile Interface Changes

### Modified Interface

```go
type DBFile interface {
    // Existing methods (unchanged)
    Read(start int64, size int32) ([]byte, error)
    Size() int64
    Close() error
    SetWriter(dataChan <-chan Data) error
    GetMode() string
    WriterClosed()
    
    // New method for subscription
    Subscribe(callback func() error) (func() error, error)
}
```

### Subscribe Method

**Signature**:
```go
Subscribe(callback func() error) (unsubscribe func() error, err error)
```

**Purpose**: Register a callback to be notified when data is written to the DBFile

**Parameters**:
- `callback func() error`: Function to be called after write operations complete
  - Takes no parameters (DBFile-level notification only)
  - Returns error if processing fails
  - MUST NOT be nil

**Returns**:
- `unsubscribe func() error`: Closure to remove the subscription
  - Idempotent (safe to call multiple times)
  - Returns nil on success
- `err error`: Error if subscription fails
  - `InvalidInputError` if callback is nil

**Implementation**: Delegates to internal `Subscriber[func() error]` instance

**Behavior**:
1. Validates callback is not nil
2. Calls `subscribers.Subscribe(callback)` to get unsubscribe closure
3. After write operations, creates snapshot via `subscribers.Snapshot()`
4. Calls all callbacks synchronously
5. First error from any callback stops chain and propagates to caller

**Thread Safety**: Thread-safe; may be called concurrently with other DBFile methods

**Usage Example**:
```go
dbfile, err := NewDBFile("/path/to/db.fdb", MODE_WRITE)
if err != nil {
    return err
}

// Subscribe to file changes
unsubscribe, err := dbfile.Subscribe(func() error {
    fmt.Println("File was written")
    return nil
})
if err != nil {
    return err
}
defer unsubscribe()

// Write operations will now trigger callback
```

**Error Conditions**:
| Condition | Error Type | When |
|-----------|-----------|------|
| Nil callback | InvalidInputError | Immediate (before subscription) |
| Callback returns error | Caller's error type | During notification |
| Callback panics | Panic propagates | During notification |

**Performance Characteristics**:
- Time: O(1) for subscription with mutex lock
- Space: O(N) where N = number of subscribers
- Notification: O(N) for snapshot creation + callback execution time

## RowEmitter Type

### Type Definition

```go
// RowEmitter monitors DBFile for row completion and notifies subscribers
type RowEmitter struct {
    // Internal fields (implementation detail, not part of public API)
}
```

### Constructor

**Signature**:
```go
func NewRowEmitter(dbfile DBFile) (*RowEmitter, error)
```

**Purpose**: Create a new RowEmitter that monitors the given DBFile for completed rows

**Parameters**:
- `dbfile DBFile`: The DBFile instance to monitor
  - MUST NOT be nil
  - Can be in read or write mode (write mode will emit events, read mode will not)
  - MUST support subscription via Subscribe() method

**Returns**:
- `*RowEmitter`: New RowEmitter instance
- `error`: Error if creation fails
  - `InvalidInputError` if dbfile is nil
  - Propagates errors from dbfile.Subscribe()
  - Errors from querying initial file state

**Behavior**:
1. Queries DBFile for current size and row count
2. Determines if last row is partial
3. Initializes internal state (lastKnownRowCount, lastKnownFileSize)
4. Subscribes to DBFile for future write notifications
5. Initializes empty subscriber map

**Initialization States**:
- **Empty file**: lastKnownRowCount = 0
- **Complete rows only**: lastKnownRowCount = N (where N is complete row count)
- **Partial row present**: lastKnownRowCount = N (partial not counted)

**Usage Example**:
```go
dbfile, _ := NewDBFile("/path/to/db.fdb", MODE_WRITE)
emitter, err := NewRowEmitter(dbfile)
if err != nil {
    return err
}
defer emitter.Close()
```

### Subscribe Method

**Signature**:
```go
func (re *RowEmitter) Subscribe(callback func(index int64, row *RowUnion) error) (func() error, error)
```

**Purpose**: Register a callback to receive notifications when complete rows are written

**Parameters**:
- `callback func(index int64, row *RowUnion) error`: Function to call for each completed row
  - `index`: Zero-based position of the completed row in the file
  - `row`: Pointer to RowUnion containing the completed row data
  - Returns error if processing fails
  - MUST NOT be nil

**Returns**:
- `unsubscribe func() error`: Closure to remove the subscription
  - Idempotent (safe to call multiple times)
  - Returns nil on success
- `err error`: Error if subscription fails
  - `InvalidInputError` if callback is nil

**Implementation**: Delegates to internal `Subscriber[func(int64, *RowUnion) error]` instance

**Behavior**:
1. Validates callback is not nil
2. Calls `subscribers.Subscribe(callback)` to get unsubscribe closure
3. When DBFile notifies of writes, queries for new complete rows
4. For each new complete row (in chronological order):
   - Creates snapshot via `subscribers.Snapshot()`
   - Calls each subscriber with row index and data
   - Stops on first error and propagates back

**Thread Safety**: Thread-safe; may be called concurrently

**Usage Example**:
```go
emitter, _ := NewRowEmitter(dbfile)

unsubscribe, err := emitter.Subscribe(func(index int64, row *RowUnion) error {
    fmt.Printf("Row %d completed: %+v\n", index, row)
    return nil
})
if err != nil {
    return err
}
defer unsubscribe()
```

**Important Notes**:
- Only receives notifications for FUTURE rows (no historical replay)
- Multiple rows written between notifications result in separate sequential events
- If initialized with partial row, receives notification when that row completes
- Callbacks execute synchronously on write thread

### Close Method

**Signature**:
```go
func (re *RowEmitter) Close() error
```

**Purpose**: Clean up RowEmitter resources and unsubscribe from DBFile

**Parameters**: None

**Returns**:
- `error`: Error if cleanup fails
  - Errors from DBFile unsubscription
  - Implementation-specific cleanup errors

**Behavior**:
1. Calls DBFile unsubscribe function (from initialization)
2. Clears internal subscriber map (optional, for GC)
3. Marks RowEmitter as closed

**Thread Safety**: Thread-safe; may be called concurrently

**Usage Example**:
```go
emitter, _ := NewRowEmitter(dbfile)
defer emitter.Close() // Always close to prevent resource leaks
```

**Error Conditions**:
| Condition | Error Type | When |
|-----------|-----------|------|
| Already closed | None (idempotent) | Subsequent calls are no-ops |
| DBFile unsubscribe fails | Propagated error | During Close() |

## Callback Signatures

### DBFile Callback

**Type**: `func() error`

**Purpose**: Notified when DBFile is written to (no parameters, file-level only)

**Expected Behavior**:
- Execute quickly (microseconds preferred)
- Return error to stop notification chain
- MUST NOT call Subscribe/Unsubscribe on same DBFile (reentrancy forbidden)
- MUST NOT perform expensive I/O operations

**Error Handling**:
- Returning error stops notification chain
- Error propagates back to write operation
- Subsequent subscribers do NOT receive notification

### RowEmitter Callback

**Type**: `func(index int64, row *RowUnion) error`

**Purpose**: Notified when a complete row is written (includes row data)

**Parameters Definition**: See data-model.md "Row Completion Callback Parameters" section for detailed parameter specifications.

**Parameters**:
- `index int64`: Zero-based position of row in file (>= 0)
- `row *RowUnion`: Complete row data (MUST NOT be nil)

**Expected Behavior**:
- Process row data synchronously
- Return error to stop notification chain
- MUST NOT call Subscribe/Unsubscribe on same RowEmitter (reentrancy forbidden)
- May perform reasonable processing (e.g., index updates)

**Error Handling**:
- Returning error stops notification chain
- Error propagates back through RowEmitter → DBFile → Transaction
- Subsequent subscribers do NOT receive notification
- If processing row N fails, row N+1 is NOT emitted


## Compatibility Notes

### Existing Code Impact
- **DBFile Interface**: New method added (breaking change for mocks/implementations)
- **Transaction**: No changes to public API (internal decoupling only applies to row completion notification; Transaction MAY maintain Finder dependency for other purposes such as querying)
- **Finder Interface**: No changes (OnRowAdded signature unchanged)
- **Finder Implementations**: Must be updated to use subscription pattern

## Testing Implications

### Unit Tests Required
1. DBFile.Subscribe() basic flow
2. DBFile.Subscribe() with multiple subscribers
3. DBFile notification error propagation
4. RowEmitter initialization (empty, complete, partial states)
5. RowEmitter.Subscribe() basic flow
6. RowEmitter notification with multiple rows
7. Thread-safe Subscribe/Unsubscribe/Notify operations
8. Idempotent unsubscribe behavior
9. Self-unsubscribe during callback (no deadlock)

### Spec Tests Required (per FR-XXX)
- FR-001: Transaction writes without direct notification
- FR-002: Transaction has no Finder dependency
- FR-003: DBFile provides subscription mechanism
- FR-004: Finder receives notifications through RowEmitter
- FR-005: RowEmitter monitors and detects complete rows
- FR-006: Multiple independent subscribers
- FR-007: Partial row completion detection
- FR-008: Sequential notifications for multiple rows
- FR-009: No historical event replay
- FR-010: Error propagation stops chain
- FR-011: Synchronous processing
- FR-012: Immediate unsubscribe effect

### Integration Tests Required
1. Full write → notification → finder update flow
2. Multiple finders receiving same notification
3. Error from finder stops transaction commit
4. Partial row completion triggers notification
5. Two rows written triggers two separate notifications
6. Subscription cleanup on Close()
