# Data Model: Deterministic Subscriber Callback Order

## Overview

This document describes the data structure changes to the `Subscriber` type to support deterministic callback ordering.

---

## Data Structure Changes

### Current Implementation (Non-Deterministic)

```go
type Subscriber[T any] struct {
    mu        sync.Mutex
    callbacks map[int64]T  // ← Non-deterministic iteration order
    nextID    int64
}
```

**Issue**: Map iteration order is randomized in Go, causing callbacks to execute in unpredictable order.

---

### New Implementation (Deterministic)

```go
type entry[T any] struct {
    id       int64  // Unique subscription ID
    callback T      // Callback function
}

type Subscriber[T any] struct {
    mu      sync.Mutex
    entries []entry[T]  // ← Ordered slice maintains insertion order
    nextID  int64
}
```

**Key Changes**:
1. **Replace map with slice**: `map[int64]T` → `[]entry[T]`
2. **New entry struct**: Pairs subscription ID with callback function
3. **No other fields changed**: Mutex and nextID remain the same

---

## Operation Behavior

### Subscribe Operation

**Before**:
```go
s.callbacks[id] = callback  // Insert into map
```

**After**:
```go
s.entries = append(s.entries, entry[T]{id: id, callback: callback})  // Append to slice
```

**Complexity**: O(1) - append to slice (amortized)

---

### Unsubscribe Operation

**Before**:
```go
delete(s.callbacks, id)  // O(1) map deletion
```

**After**:
```go
// Find entry by ID and remove from slice
for i, e := range s.entries {
    if e.id == id {
        s.entries = append(s.entries[:i], s.entries[i+1:]...)
        break
    }
}
```

**Complexity**: O(n) - linear search and removal
**Acceptable**: Unsubscribe is rare in production (mainly cleanup/Close operations)

---

### Snapshot Operation

**Before**:
```go
for _, callback := range s.callbacks {  // ← Random order
    result = append(result, callback)
}
```

**After**:
```go
for _, entry := range s.entries {  // ← Insertion order
    result = append(result, entry.callback)
}
```

**Complexity**: O(n) - unchanged, but now deterministic order
**Guarantee**: Returns callbacks in exact registration order

---

## Validation Rules

### Invariants

1. **Unique IDs**: Each entry in `entries` slice has a unique `id` field
2. **Sequential IDs**: IDs are assigned sequentially from `nextID` counter
3. **Order preservation**: Slice maintains insertion order (Go guarantee)
4. **No gaps**: Unsubscribe removes entry, no tombstones or gaps

### Thread Safety

- **Mutex protection**: All operations on `entries` slice protected by `mu`
- **No lock during callback**: Snapshot() releases lock before returning slice
- **Idempotent unsubscribe**: Deleting non-existent ID is no-op (safe)

---

## Memory Characteristics

### Space Complexity

- **Current (map)**: O(n) where n = active subscriptions
- **New (slice)**: O(n) where n = active subscriptions
- **No change**: Same memory usage, no deleted entry tracking

### Growth Behavior

- **Append operations**: Slice grows by doubling capacity (Go standard)
- **Remove operations**: Slice shrinks immediately, no deferred cleanup needed
- **No memory leaks**: Removed entries immediately freed (no tombstones)

---

## State Transitions

### Subscription Lifecycle

```
[Not Subscribed]
      ↓ Subscribe()
[Active] ← entry added to slice at position n
      ↓ Unsubscribe()
[Removed] ← entry removed from slice, other entries maintain relative order
```

### Example: Order Preservation After Unsubscribe

**Initial state**: `[{id:1, cb:A}, {id:2, cb:B}, {id:3, cb:C}]`

**After unsubscribe(2)**: `[{id:1, cb:A}, {id:3, cb:C}]`

**Snapshot order**: `[A, C]` (maintains original relative order: A before C)

---

## Error Conditions

### No New Error Types

This refactoring introduces **no new error conditions**. All existing error handling remains unchanged:

- **nil callback**: Already validated in Subscribe(), returns InvalidInputError
- **Double unsubscribe**: Already idempotent (no-op), no error returned
- **Concurrent access**: Already protected by mutex, no race conditions

---

## Integration Impact

### Components Using Subscriber

**No changes needed** to any consumers:

- ✅ `FileManager` - uses Subscriber via existing API
- ✅ `RowEmitter` - uses Subscriber via existing API  
- ✅ `SimpleFinder` - subscribes via RowEmitter
- ✅ `InMemoryFinder` - subscribes via RowEmitter
- ✅ `BinarySearchFinder` - subscribes via RowEmitter

**Reason**: Public API unchanged, only internal representation differs.

---

## Testing Validation

### Data Integrity Checks

1. **Order verification**: Register callbacks [1,2,3,4,5], verify Snapshot returns same order
2. **Unsubscribe order**: Remove middle callback, verify remaining maintain relative order
3. **Empty state**: Unsubscribe all, verify Snapshot returns empty slice
4. **Thread safety**: Concurrent Subscribe/Unsubscribe/Snapshot under race detector

### Performance Validation

1. **Subscribe**: Benchmark should show O(1) constant time
2. **Snapshot**: Benchmark should show O(n) linear time  
3. **Unsubscribe**: O(n) acceptable (rare operation)
