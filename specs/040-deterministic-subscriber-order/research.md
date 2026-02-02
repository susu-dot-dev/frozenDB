# Research: Deterministic Subscriber Callback Order

## Problem Analysis

### Root Cause

The `Subscriber.Snapshot()` method in `internal/frozendb/subscriber.go` iterates over a Go map to collect callbacks:

```go
func (s *Subscriber[T]) Snapshot() []T {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    result := make([]T, 0, len(s.callbacks))
    for _, callback := range s.callbacks {  // ← Non-deterministic iteration
        result = append(result, callback)
    }
    return result
}
```

**Issue**: Go maps have **non-deterministic iteration order by design**. This is intentional in Go to prevent developers from relying on map ordering.

**Evidence**: The flaky test `Test_S_039_FR_006_CallbacksInRegistrationOrder` shows callbacks invoked in order `[4, 5, 1, 2, 3]` instead of expected `[1, 2, 3, 4, 5]`.

**Impact**: Violates FR-006 from spec 039 which explicitly requires "callbacks MUST be invoked in registration order."

---

## Solution Decision

### Chosen Approach: Slice of (ID, Callback) Tuples

**Decision**: Replace `map[int64]T` with a slice of structs containing both ID and callback.

**Data Structure**:
```go
type entry[T any] struct {
    id       int64
    callback T
}

type Subscriber[T any] struct {
    mu      sync.Mutex
    entries []entry[T]  // Ordered slice instead of map
    nextID  int64
}
```

**Rationale**:
1. **Natural ordering**: Slices maintain insertion order by design in Go
2. **Simplicity**: Single data structure, straightforward implementation
3. **Performance acceptable**: O(n) unsubscribe is acceptable given usage patterns
4. **Memory efficient**: No wasted space from deleted entries or dual structures
5. **Go idioms**: Slices are the Go way for ordered collections

---

## Alternatives Considered

### Alternative 1: Map + Ordered ID Slice

**Structure**:
```go
type Subscriber[T any] struct {
    callbacks map[int64]T  // Fast lookup
    order     []int64      // Maintains insertion order
    nextID    int64
}
```

**Pros**:
- Could optimize unsubscribe with tombstoning (mark deleted without removing)
- Snapshot can skip unsubscribed IDs naturally

**Cons**:
- More complex - two data structures to maintain
- More memory overhead
- Snapshot needs to check if each ID still exists in map
- Over-engineered for the problem

**Rejected because**: Added complexity not justified by usage patterns

---

### Alternative 2: Slice with Tombstones

**Structure**:
```go
type entry[T any] struct {
    id      int64
    callback T
    deleted bool  // Mark as deleted instead of removing
}
```

**Pros**:
- Unsubscribe is O(n) to find, O(1) to mark deleted
- Maintains order naturally

**Cons**:
- Memory leaks if never compacted
- Snapshot must skip deleted entries
- Added complexity for compaction logic

**Rejected because**: Memory management complexity not worth the marginal performance gain

---

### Alternative 3: Linked List (container/list)

**Pros**:
- Maintains order naturally
- Fast insert/delete operations

**Cons**:
- More allocations (each node is separate)
- More complex code
- Less idiomatic Go

**Rejected because**: Slice-based approach is more idiomatic and sufficient

---

## Usage Pattern Analysis

Based on codebase analysis:

### Production Usage
- **Finders**: Subscribe once during construction, never unsubscribe (lifetime subscription)
- **RowEmitter**: Subscribes once during construction, unsubscribes on Close()
- **FileManager**: Similar pattern - long-lived subscriptions

### Unsubscribe Frequency
- **Rare in production**: Mainly used in cleanup (Close operations)
- **Common in tests**: Used for test isolation and cleanup

### Subscriber Count
- **Typically small**: 1-20 subscribers in production scenarios
- **Not thousands**: Database operations don't spawn massive numbers of subscribers

### Performance Impact
- **O(n) unsubscribe acceptable**: Given rare usage and small n (1-20)
- **O(1) subscribe critical**: Happens at initialization, must be fast
- **O(n) snapshot acceptable**: Already copying all callbacks, linear is expected

---

## Implementation Constraints

### API Compatibility
- **No changes** to `Subscribe(callback T) func() error` signature
- **No changes** to `Snapshot() []T` signature
- **No changes** to unsubscribe closure behavior (idempotent)

### Thread Safety
- **Existing mutex approach**: Continue using `sync.Mutex` for all operations
- **Snapshot pattern**: Lock held only during copy, not during callback execution
- **Race detector**: Must pass with `-race` flag

### Behavioral Compatibility
- **All existing tests**: Must pass without modification
- **Existing subscribers**: FileManager, RowEmitter, all Finders must work unchanged
- **Error handling**: Callback errors already stop chain in order, just needs determinism

---

## Validation Approach

### Test Strategy
1. **Run existing flaky test 100 times**: Must pass all iterations
2. **Run all subscriber unit tests**: Must pass without modification
3. **Run all file_manager_spec_tests**: Must pass without modification
4. **Run with race detector**: Must detect no race conditions

### Performance Validation
- **Benchmark Subscribe()**: Should remain O(1) constant time
- **Benchmark Snapshot()**: Should be O(n) linear time
- **Benchmark Unsubscribe()**: O(n) is acceptable for this rare operation

---

## Key Files to Modify

### Primary Implementation
- `internal/frozendb/subscriber.go` (lines 19-83)
  - Replace `callbacks map[int64]T` with `entries []entry[T]`
  - Add `entry[T]` struct definition
  - Update Subscribe() to append to slice
  - Update Unsubscribe closure to find and remove entry
  - Update Snapshot() to iterate slice in order

### Tests to Validate
- `internal/frozendb/subscriber_test.go` (existing unit tests)
- `internal/frozendb/file_manager_spec_test.go` (Test_S_039_FR_006_CallbacksInRegistrationOrder)

### No Changes Needed
- `internal/frozendb/file_manager.go` (uses Subscriber via interface)
- `internal/frozendb/row_emitter.go` (uses Subscriber via interface)
- `internal/frozendb/*_finder.go` (use Subscriber via interface)

---

## Risk Assessment

### Low Risk
- **API unchanged**: No breaking changes to public interface
- **Usage patterns known**: Well-understood subscriber usage in codebase
- **Comprehensive tests**: Existing test suite validates behavior

### Mitigation
- **Run full test suite**: Before and after to catch regressions
- **Race detector**: Validate thread safety
- **Manual testing**: Run flaky test 100+ times to confirm determinism
