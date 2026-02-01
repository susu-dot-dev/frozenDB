# Research: Subscribe/Unsubscribe Patterns for RowEmitter

**Feature**: 036-row-emitter-layer  
**Date**: 2026-02-01  
**Context**: Synchronous callback subscription system for row completion notifications

## Research Questions

1. Can Go functions be passed to Subscribe() and then passed back to Unsubscribe() for identification?
2. What subscription/unsubscription patterns work reliably in Go?
3. How can subscription management be thread-safe without causing deadlocks?
4. What patterns exist in the current frozenDB codebase for callbacks?

## Existing Codebase Patterns

### Current Callback Pattern: Interface-Based (1:1)

**Pattern**: Transaction holds a single `Finder` interface and calls `OnRowAdded()` directly.

**Relevance**: The existing pattern is 1:1 (one Transaction, one Finder). RowEmitter requires 1:N (one emitter, many subscribers), so dynamic subscription management is needed.

**Files examined**: 
- `internal/frozendb/finder.go` - Finder interface with OnRowAdded method
- `internal/frozendb/transaction.go` - Direct notification to single finder
- `internal/frozendb/simple_finder.go`, `binary_search_finder.go`, `inmemory_finder.go` - Implementations

## Decision 1: Function Pointer Comparison is Not Viable

### Investigation

**Question**: Can we use function pointer comparison for unsubscription?

**Finding**: **No - Go does not allow function comparison except to nil.**

From Go language specification: "Function values are not comparable. Function values may be compared only to the predeclared identifier nil."

**Implication**: Cannot use `Subscribe(callback)` / `Unsubscribe(callback)` pattern where the same function pointer is passed. Any subscription system must use an internal identification mechanism.

## Decision 2: Use Closure-Based Unsubscription

### Alternatives Considered

**Option A: Token-Based**
- Subscribe() returns SubscriptionID
- Unsubscribe(id SubscriptionID) takes the ID
- Pros: Explicit, debuggable IDs
- Cons: Caller must manually track IDs

**Option B: Closure-Based** 
- Subscribe() returns UnsubscribeFunc closure
- Closure internally captures unique ID
- Pros: Ergonomic API, idiomatic Go (matches context.WithCancel)
- Cons: ID hidden in closure (but still debuggable if needed)

**Option C: Interface-Based**
- Define Subscriber interface, use instance comparison
- Pros: Type-safe, matches existing Finder pattern
- Cons: More ceremony, requires interface per layer, less flexible

### Decision: Option B - Closure-Based

**Rationale**:
1. Most ergonomic: `unsubscribe := emitter.Subscribe(callback); defer unsubscribe()`
2. Idiomatic Go pattern (similar to context.WithCancel, signal.NotifyContext)
3. Still uses internal ID system for reliability
4. Allows closures and anonymous functions (more flexible than interface-based)

**Internal mechanism**: Closure captures auto-incremented int64 ID, stored in map[int64]callback

## Decision 3: Thread Safety via Snapshot Pattern

### Investigation

**Challenge**: Subscription management needs thread-safety, but naive locking causes deadlocks:
- If lock held during callback execution, callback cannot Subscribe/Unsubscribe (would try to re-acquire lock)
- Must support callback unsubscribing itself or others during notification

### Alternatives Considered

**Option A: Lock During Execution**
- Hold lock while iterating and calling callbacks
- Result: DEADLOCK if callback calls Subscribe/Unsubscribe

**Option B: No Locking**
- Document that caller must serialize
- Result: Race conditions, not production-ready

**Option C: Snapshot Pattern**
- Lock briefly to copy callback list
- Release lock before executing callbacks
- Result: Thread-safe, deadlock-free

### Decision: Option C - Snapshot Pattern

**Rationale**:
1. **Deadlock prevention**: Lock released before callback execution
2. **Thread-safe**: All map access protected by mutex
3. **Self-unsubscribe works**: Callback in snapshot completes, then removal takes effect
4. **Acceptable trade-off**: Callback that unsubscribes still executes once (desired behavior)

**Pattern**:
1. Lock → copy callbacks to local slice → unlock
2. Execute callbacks from slice (without lock)
3. Subscribe/Unsubscribe lock only during map modification

**Memory ordering**: Go mutex provides happens-before guarantees, no additional synchronization needed

## Decision 4: Error Propagation Strategy

### Requirement Analysis

From spec FR-010: "When a subscriber returns an error during notification, the error MUST propagate back to the write operation and prevent subsequent subscribers from receiving the notification"

### Decision: First Error Stops Chain

**Pattern**: Iterate through snapshot, return immediately on first error

**Implication**: Subscribers earlier in map iteration order are prioritized. This is acceptable as subscribers should not depend on ordering.

**Alternative considered**: Collect all errors. Rejected because spec explicitly states "first error" and "prevent subsequent subscribers."

## Decision 5: Subscription Lifecycle Management

### Edge Cases Researched

**Unsubscribe during notification**:
- Decision: Allow, handle via snapshot (callback completes current execution)
- Rationale: Natural pattern, prevents half-processed state

**Multiple unsubscribe calls**:
- Decision: Idempotent (no-op after first call)
- Rationale: Go map delete() on non-existent key is safe no-op

**Subscribe during notification**:
- Decision: New subscription not in current snapshot, receives future events only
- Rationale: Matches FR-008 (no historical events)

**Subscription ID exhaustion**:
- Decision: Use int64 (9.2 quintillion IDs)
- Rationale: At 1M subscriptions/second, takes 292,471 years to exhaust

## Integration Approach

### DBFile Layer

**API**: Subscribe() returns UnsubscribeFunc
**Callback signature**: `func() error` (no parameters, DBFile-level notification only)
**Thread-safety**: Mutex-protected with snapshot pattern

### RowEmitter Layer

**API**: Subscribe() returns RowEmitterUnsubscribeFunc
**Callback signature**: `func(index int64, row *RowUnion) error` (row-specific data)
**Initialization**: RowEmitter subscribes to DBFile in constructor
**Cleanup**: RowEmitter unsubscribes from DBFile on Close()

### Finder Integration

**Approach**: Finder subscribes to RowEmitter (not directly to Transaction)
**Decoupling**: Transaction → DBFile → RowEmitter → Finder (late binding at each layer)

## Performance Characteristics

**Snapshot overhead**: O(N) time and memory where N = subscriber count
**Expected N**: Small (< 10 subscribers in typical use)
**Lock contention**: Minimal (microseconds for snapshot creation)
**Callback execution**: Bulk of time, not under lock

## Testing Strategy Implications

Critical scenarios identified:
1. Basic subscription/notification flow
2. Multiple concurrent subscribers
3. Self-unsubscribe during callback (no deadlock)
4. Error propagation stops chain
5. Thread-safety (concurrent subscribe while notifying)
6. Idempotent unsubscribe

## Summary

**Key Decisions**:
1. ✅ Closure-based unsubscription (ergonomic API)
2. ✅ Snapshot pattern for thread-safety (deadlock prevention)
3. ✅ First-error-stops-chain for error propagation
4. ✅ Internal int64 ID system (function comparison not viable)

**Rationale**: Balances ergonomics, thread-safety, and alignment with Go idioms while meeting all functional requirements for synchronous callback notification.
