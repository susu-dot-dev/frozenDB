# Research: Read-Mode File Updates

**Feature**: 039-read-mode-updates  
**Date**: 2026-02-01  
**Status**: Phase 0 Complete

## Overview

This document contains research findings that resolve technical unknowns from the specification. The research focuses on understanding fsnotify library usage patterns, initialization race prevention, and serialization mechanisms for file size updates.

## Research Areas

### 1. fsnotify Library Usage Patterns

**Question**: How to properly use github.com/fsnotify/fsnotify to monitor file changes with zero missed events during initialization?

**Decision**: Use fsnotify.NewWatcher() with event loop pattern that starts watch before goroutine initialization

**Rationale**:
- fsnotify provides reliable Linux inotify integration with straightforward API
- Event loop pattern with goroutine enables non-blocking file monitoring
- Starting watch with Add() before event loop goroutine prevents missing events during initialization
- Reading initial file size after watcher is active ensures complete coverage

**Alternatives Considered**:
1. Polling with time.Ticker
   - Rejected: Higher latency, wastes CPU cycles, doesn't meet <1s requirement (SC-001)
2. Manual inotify syscalls
   - Rejected: More complex, error-prone, fsnotify already provides production-ready wrapper
3. Buffered watcher (fsnotify.NewBufferedWatcher)
   - Not needed: Single file with moderate write rate doesn't require buffering

**Key Integration Points**:
- Watcher must be created before starting event processing goroutine
- File watch should be added (via Add()) before goroutine starts to prevent missed events
- Initial file size should be captured to establish baseline for detecting changes
- Watcher lifecycle must be managed carefully to prevent goroutine leaks

**Event Handling Considerations**:
- Listen ONLY for Write events as specified in FR-001
- Event types can be bitmasks, requiring proper flag checking
- ErrEventOverflow should trigger full update cycle
- File path filtering may be needed depending on watch granularity

**Lifecycle Management Considerations**:
- Shutdown must close watcher and wait for goroutine completion
- Channel closure signals goroutine to exit
- Proper cleanup prevents resource leaks

### 2. Initialization Race Prevention

**Question**: How to ensure zero timing gaps between file opening and watcher activation where writes could be missed?

**Decision**: Capture initial file size during DBFile creation, start watcher immediately after, then compare sizes to catch any gap writes

**Rationale**:
- Capturing size during DBFile creation establishes baseline before watcher starts
- Starting watcher immediately after minimizes (but doesn't eliminate) race window
- File system guarantees that writes from other processes are atomic at syscall level
- Any writes in the gap will be caught by first watcher event or size comparison

**Race Window Analysis**:
```
Time ----->
T0: DBFile opens file, reads initial size (S0)
T1: Start watcher with watcher.Add()
T2: Watcher goroutine begins processing events
T3: Another process appends to file (size now S1)
```

**Scenarios**:
1. Write at T3 > T1: Watcher catches via Write event ✓
2. Write between T0-T1: Gap window - handled by comparing sizes in first update cycle
3. Concurrent writes during initialization: Multiple Write events queued, all processed serially

**Implementation Strategy**:
- Store initial size in atomic.Uint64 before starting watcher
- First watcher event compares current file size to stored size
- If different, triggers update cycle even if Write event wasn't for the specific change
- This handles both gap writes and concurrent writes during startup

### 3. Serialized Update Cycle Design

**Question**: How to ensure only one update cycle (size update + callbacks) executes at a time, matching write-mode serialization?

**Decision**: Use single goroutine with select statement processing events serially, calling callbacks synchronously within event handler

**Rationale**:
- Single goroutine naturally serializes event processing (only one event handled at a time)
- Synchronous callback invocation within event handler ensures size update completes before callbacks
- Matches existing write-mode pattern where writerLoop goroutine serializes writes
- Simple implementation with no additional locking required

**Serialization Approach**:
- Single goroutine with select statement naturally serializes event processing
- Synchronous callback invocation within event handler ensures size update completes before callbacks
- Matches existing write-mode pattern where writerLoop goroutine serializes writes
- Simple implementation with no additional locking required

**Key Properties**:
- Only one event handler iteration executes at a time
- Size update completes before callbacks via sequential execution
- Callbacks invoked in registration order
- No concurrent update cycles possible

**Alternatives Considered**:
1. Mutex-based serialization
   - Rejected: More complex, unnecessary since single goroutine provides serialization
2. Channel-based work queue
   - Rejected: Over-engineered for this use case, adds latency
3. Async callback invocation
   - Rejected: Violates FR-004 requirement for serialized callbacks

### 4. Existing Subscription Mechanism Integration

**Question**: How does the existing Subscribe() mechanism work and how should file watcher integrate with it? How should callback errors be handled?

**Decision**: Reuse existing FileManager.Subscribe() and Subscriber[func() error] pattern, treating file watcher events as triggers for subscriber callbacks. Callbacks MUST handle their own errors by tombstoning themselves before returning.

**Analysis of Existing Code** (file_manager.go:182-198, 302-309):
The existing Subscribe() mechanism uses a Subscriber[func() error] pattern. In write mode, processWrite invokes callbacks synchronously after successful writes. Callbacks return errors that stop the callback chain on first error.

**Integration Pattern**:
- File watcher uses same callback invocation pattern as processWrite
- Subscribers don't need to know whether update came from write-mode or read-mode watcher
- Finder subscribes once, gets notified on both write-mode writes and read-mode file changes
- Callback error handling: stop on first error, BUT errors have nowhere to propagate to

**Critical Error Handling Insight**:
Update cycles run in background goroutines (file watcher in read mode, writerLoop in write mode) with **no user action to return errors to**. When a callback returns an error:

1. FileManager stops invoking remaining callbacks (FR-006, FR-009)
2. The callback that failed MUST have handled its own error state before returning
3. For Finders: Set tombstoned flag BEFORE returning error (FR-010)
4. FileManager cannot fix the callback's state - this is the callback's responsibility

**Finder Tombstoning Pattern**:
All Finder implementations must add a tombstoned state flag. When Finder refresh encounters ANY error, it must set tombstoned=true BEFORE returning the error. After tombstoning, all public Finder methods must check the tombstoned state first and return TombstonedError without accessing potentially stale data.

**Why This Pattern is Essential**:
- Refresh errors occur asynchronously (no user action triggered them)
- If Finder continues operating with stale state → data inconsistency
- Tombstoning ensures: async error → error on next user action (Get call)
- Permanent tombstone state prevents any chance of serving stale data

**Mode Independence**: This pattern is IDENTICAL in both read mode and write mode. The Finder doesn't know or care which goroutine called its Refresh() - it just ensures correctness.

### 5. File Watcher Failure Handling

**Question**: What should happen when file watcher fails to start during DBFile creation?

**Decision**: Fail DBFile creation entirely with descriptive error (fail-fast approach per FR-007)

**Rationale**:
- Consistent with frozenDB philosophy: correctness over availability
- Read-mode instances without working watcher would return stale data, violating user expectations
- Failing fast makes the problem visible immediately rather than causing silent data staleness
- User can retry or fall back to write-mode instance if needed

**Error Mapping**:
- fsnotify.NewWatcher() fails → WriteError("failed to create file watcher", err)
- watcher.Add() fails → WriteError("failed to add watch for file", err)
- System resource limits → WriteError with clear message about inotify limits

**No Fallback Mode**:
- DO NOT fall back to static-size read mode
- DO NOT continue with partial functionality
- Clean up watcher if any step fails
- Return error immediately to caller

### 6. Close() Blocking Behavior

**Question**: Should Close() block until active update cycle completes, or interrupt immediately?

**Decision**: Block Close() until active update cycle completes (FR-008)

**Rationale**:
- Prevents race conditions where callbacks access file after Close()
- Ensures clean shutdown with no orphaned goroutines (SC-008)
- Matches existing WriterClosed() pattern which blocks until writer goroutine completes
- Implementation uses done channel signal and WaitGroup blocking

**Timeout Consideration**:
- Spec requires cleanup within 100ms under normal conditions (SC-010)
- No timeout in implementation - blocking is by design
- If callback hangs, Close() hangs (caller's responsibility to ensure callbacks are fast per AS-004)

### 7. File Size Update Atomicity

**Question**: How to ensure file size updates are atomic and visible to concurrent readers?

**Decision**: Use atomic.Uint64 for currentSize, already present in FileManager

**Analysis of Existing Code** (file_manager.go:43, 69, 126, 174-176, 300):
The FileManager struct already has a currentSize field of type atomic.Uint64. In write mode, processWrite updates it using Add(). The Size() method returns the atomic value using Load(), which is thread-safe.

**Integration**:
- File watcher uses same atomic.Uint64 as write mode
- Swap operation enables atomic read-modify-write pattern
- No additional synchronization needed
- Size() method already returns atomic value, works for both modes

### 8. Metadata-Only Update Handling

**Question**: What happens when fsnotify fires Write event but file size hasn't changed?

**Decision**: Run update cycle, detect no size change, skip callback invocation (FR-005 clarification)

**Rationale**:
- Linux inotify fires IN_MODIFY for both data and metadata changes
- Checking file size is cheap compared to callback overhead
- Listening only to fsnotify.Write (not Chmod) reduces metadata-only events
- Early return when size unchanged prevents unnecessary callback processing

**Implementation Approach**:
Check if new size equals old size after atomic swap; if equal, skip callback invocation and continue to next event.

## Performance Considerations

### Latency
- fsnotify on Linux uses inotify, which provides near-instant notifications (<1ms typical)
- Event loop processing is synchronous but fast (file stat + callback invocation)
- Expected total latency: <100ms for Get() to see new keys after write (well under SC-001's 1s requirement)

### Memory
- Single fsnotify.Watcher instance: ~4KB overhead
- Event loop goroutine: ~4KB stack (grows as needed)
- No per-row or per-key memory usage
- Memory usage remains fixed regardless of database size (satisfies constitution)

### CPU
- Watcher goroutine is idle when no events (blocked on channel receive)
- File stat syscall on each Write event: negligible overhead
- Callback processing depends on subscribers (typically Finder refresh)

## Edge Cases Documented

### File Deletion/Rename During Watch
- Not explicitly required by spec (missing from edge cases section)
- fsnotify will send Remove/Rename events
- Recommendation: Treat as fatal error, tombstone FileManager
- Implementation deferred until explicit requirement added

### Rapid Successive Modifications
- Handled by serialization: events queue in channel, processed one at a time
- Linux may coalesce multiple writes into fewer events (AS-005)
- Each event triggers update cycle, which checks current size
- System remains eventually consistent

### Events After Close() Initiated
- Close() blocks until goroutine exits (FR-008)
- After close(done), goroutine exits on next select iteration
- Watcher.Close() closes Events channel, causing goroutine to return
- No events processed after Close() completes

### File Size Reduction (Truncation)
- Resolved as "not possible by OS semantics" per clarifications
- Append-only operations cannot reduce file size
- No special handling required

## Implementation Checklist

Based on research, implementation must:

- [ ] Add github.com/fsnotify/fsnotify to go.mod
- [ ] Add watcher field to FileManager (single *fsnotify.Watcher field)
- [ ] Create watcher only when mode == MODE_READ
- [ ] Initialize watcher immediately after file open in NewDBFile
- [ ] Start watcher goroutine after Add() but before returning from NewDBFile
- [ ] Listen only for fsnotify.Write events
- [ ] Detect closed channels properly in event loop
- [ ] Compare old vs new size, skip callbacks if unchanged
- [ ] Invoke callbacks synchronously in registration order
- [ ] Stop on first callback error (existing behavior)
- [ ] Handle watcher.Add() failure by failing NewDBFile entirely
- [ ] Modify Close() to close watcher and wait for goroutine exit
- [ ] Ensure zero goroutine leaks (verified by tests)
- [ ] Add tombstoned state to all Finder implementations
- [ ] Check tombstoned state in all public Finder methods

## References

- fsnotify documentation: https://pkg.go.dev/github.com/fsnotify/fsnotify
- Linux inotify: https://man7.org/linux/man-pages/man7/inotify.7.html
- Existing FileManager implementation: internal/frozendb/file_manager.go
- Spec document: specs/039-read-mode-updates/spec.md
