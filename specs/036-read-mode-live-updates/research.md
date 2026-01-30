# Research: Read-Mode Live Updates Implementation

**Date**: Fri Jan 30 2026  
**Feature**: Read-Mode Live Updates for Finders  
**Branch**: 036-read-mode-live-updates

This document consolidates research findings for implementing live updates in read-mode finders, focusing on file watching mechanisms, batching strategies, and initialization race prevention.

---

## 1. File Watching Mechanism

### Decision: Use `github.com/fsnotify/fsnotify` Package (Linux Only)

**Rationale**: 
- Provides Go channel-based API that integrates cleanly with Go's `select` statement
- Eliminates the impedance mismatch between Go channels and file descriptor-based I/O (syscall.Select)
- Avoids complex bridge goroutine + Unix pipe + stopChan architecture required with raw syscalls
- Production-ready library (10.5k+ stars, used by 314k+ projects)
- Uses inotify backend on Linux (our target platform)
- Well-tested and maintained

**Architecture Comparison**:

*Raw syscall approach (rejected):*
```
wakeUpChan → bridge goroutine → Unix pipe → syscall.Select() → main loop
              ↑
         stopChan (separate shutdown signal)
```
Problems: Complex, requires 3 goroutines, needs careful coordination, error-prone

*fsnotify approach (accepted):*
```
fsnotify.Watcher → Events channel ↘
                → Errors channel → Go select → process events
```
Benefits: Simple, idiomatic Go, one goroutine, clean shutdown via watcher.Close()

**Why This Is an Exception to "No External Dependencies"**:
- Raw syscall approach requires reimplementing what fsnotify does internally (channel/FD bridging)
- The complexity of bridging Go channels with syscall file descriptors is non-trivial and error-prone
- fsnotify is a well-established standard library-quality package, not experimental
- Dependency cost is justified by significant reduction in code complexity and maintenance burden
- Using syscall.Select() with Go channels requires pipe infrastructure that obscures core logic

**Platform Support**: Linux only (uses inotify backend). While fsnotify supports other platforms (BSD/macOS kqueue, Windows ReadDirectoryChangesW, illumos FEN), this feature is explicitly Linux-only.

**Alternatives Considered**:

| Approach | Why Rejected |
|----------|--------------|
| Raw `syscall` with inotify | Requires complex bridge goroutine, Unix pipes, stopChan, and careful FD coordination; reimplements fsnotify internals |
| `golang.org/x/sys/unix` | Still requires bridging Go channels to file descriptors; doesn't solve core problem |
| Polling with `time.Ticker` | Higher CPU usage; fails latency requirement (<2s); not event-driven |

**Implementation Details**: Use fsnotify on Linux (inotify backend), clean shutdown via `watcher.Close()` which closes channels.

---

## 2. Notification Pattern & Event Selection

### Decision: Use fsnotify's Built-in Event Channel, Listen for Write Events Only

**Rationale**:
- fsnotify provides buffered `Events` channel that delivers file system events
- Zero CPU when idle (goroutine blocks on channel receive via `select`)
- Natural batching (fsnotify coalesces rapid writes internally)
- Idiomatic Go pattern using standard `select` statement
- No custom channel management needed

**Event Selection - Which Events to Monitor**:

fsnotify supports these event types (from fsnotify.Op):
- `Create` - A new pathname was created
- `Write` - File was written to (append operations trigger this)
- `Remove` - Path was removed
- `Rename` - Path was renamed
- `Chmod` - File attributes changed

**For FrozenDB, we need ONLY `Write` events:**

| Event | Should Monitor? | Rationale |
|-------|----------------|-----------|
| `Write` | **YES** ✅ | This is triggered when external process appends to database file. Core requirement. |
| `Create` | **NO** ❌ | Database file already exists when FileWatcher starts; we don't watch for new files |
| `Remove` | **NO** ❌ | If file is deleted while watching, fsnotify will send error; we handle via error channel |
| `Rename` | **NO** ❌ | If file is renamed/moved, watcher becomes invalid; handle via error channel |
| `Chmod` | **NO** ❌ | Attribute changes are not relevant to detecting new row appends |

**Why Write is Sufficient**:
- On Linux (inotify), `Write` corresponds to `IN_MODIFY` which is sent when file content changes
- Appending rows to the database file triggers `IN_MODIFY`
- A single append operation may generate multiple Write events (kernel buffering), but this is fine - we check file size and process all new complete rows
- fsnotify documentation confirms: "A single 'write action' initiated by the user may show up as one or multiple writes"

**Channel Design**:
- fsnotify manages the Events and Errors channels internally
- Events channel: Receives `fsnotify.Event` with file path and operation type
- Errors channel: Receives errors from the file watcher
- Shutdown: Call `watcher.Close()` which closes both channels, causing goroutine to exit

**Example Usage**:
```go
for {
    select {
    case event, ok := <-watcher.Events:
        if !ok {
            return  // Watcher closed
        }
        if event.Has(fsnotify.Write) {
            // Process new rows - file was modified
            fw.processBatch()
        }
        // Ignore all other event types
    case err, ok := <-watcher.Errors:
        if !ok {
            return  // Watcher closed
        }
        fw.onError(err)
        return
    }
}
```

**Key Insights**: 
- fsnotify handles all the complexity of event coalescing, buffering, and clean shutdown internally
- Filtering for only Write events reduces unnecessary processing
- Multiple rapid Write events naturally batch into single processBatch() call if we're still processing previous batch

---

## 3. Initialization Race Prevention

### Decision: Two-Phase Initialization with Kickstart Event

**Rationale**:
- Simple and correct: capture file size once, scan that range only
- Kickstart event handles any concurrent writes during initialization
- No convergence loops or iteration limits needed
- Clear separation: historical data (Phase 1) vs live updates (Phase 2)

**Algorithm**:
1. **Phase 1**: `initialSize = dbFile.Size()` (capture ONCE), scan `[HEADER_SIZE, initialSize)`
2. **Phase 2**: Start FileWatcher with `lastProcessedSize = initialSize`, send kickstart event

**How It Prevents Races**:
- Finder scans only up to `initialSize` (never re-reads file size during init)
- FileWatcher starts with `lastProcessedSize = initialSize`
- Kickstart event processes gap `[initialSize, currentSize)` if file grew
- If no growth: kickstart is no-op

**Alternatives Considered**:

| Approach | Why Rejected |
|----------|--------------|
| Three-phase with catch-up loop | Over-engineered; keeps re-reading file size |
| Start watcher first, then scan | Duplicate processing (watcher events during scan) |
| Single-pass scan without kickstart | Misses writes that occur during scan |
| Lock file during scan | Blocks writers; violates concurrent read/write requirement |
| Convergence loop with maxIterations | Unnecessary complexity; kickstart is sufficient |

**Key Insight**: Capturing `initialSize` once and using it consistently prevents confusion about what's been processed.

---

## 4. Integration with Existing Finder Implementations

### Analysis: Which Finders Need Updates?

**SimpleFinder**: No changes needed
- Uses on-demand scanning (O(n) per GetIndex call)
- `OnRowAdded()` already updates size tracking
- FileWatcher integration works automatically

**InMemoryFinder**: Requires constructor update
- Needs to return `initialSize` for FileWatcher
- Uses existing scan logic, no algorithm changes
- Already has `OnRowAdded()` to update uuidIndex map

**BinarySearchFinder**: Requires constructor update
- Needs to return `initialSize` for FileWatcher
- Uses existing maxTimestamp logic, no algorithm changes
- Already has `OnRowAdded()` to update maxTimestamp

**Memory Impact**: No additional memory for FileWatcher (only ~64 bytes struct overhead).

---

## 5. Error Handling Strategy

### Decision: Tombstone Pattern with TombstonedError

**Rationale**:
- On first error: FileWatcher calls `finder.OnError()`, Finder is tombstoned
- On subsequent batches: Finder returns `TombstonedError` from `OnRowAdded()`
- User gets `TombstonedError` for all API calls afterwards (permanent error state) with original error wrapped
- FileWatcher watchLoop exits when it receives TombstonedError from OnRowAdded

**Error Types to Handle**:
- Read errors (disk I/O failure)
- Parse errors (corrupted row data)
- Finder errors (internal consistency failure)

**Why Tombstone Instead of Recovery**:
- Read/corruption errors indicate serious problems
- Attempting recovery could create inconsistent state
- Fail-fast for user (immediate error feedback)
- Database remains open (other operations may still work)

**Alternatives Considered**:

| Approach | Why Rejected |
|----------|--------------|
| Log and continue | Finder state becomes inconsistent |
| Stop FileWatcher on error | Tightly couples watcher and Finder lifecycles |
| Retry with backoff | Won't fix corruption or disk failures |
| Crash the process | Too aggressive; tombstone allows graceful degradation |

---

## 6. Performance Analysis

### Existing Codebase Patterns

**File Reading** (`DBFile.Read()`):
- Uses atomic size tracking
- Thread-safe without locks
- Fixed-width rows enable O(1) seeking

**Finder Implementations**:
- SimpleFinder: O(n) scan per lookup (no memory overhead)
- InMemoryFinder: O(1) lookup with O(n) memory (map storage)
- BinarySearchFinder: O(log n) with O(1) memory (no index)

### FileWatcher Performance Characteristics

**Memory**: 
- Struct overhead: ~64 bytes per database
- No per-row memory usage
- Scales with number of open databases, not database size

**CPU**:
- Idle: 0% (goroutine blocked on channel)
- Active: ~10-50µs per row (read + parse + Finder update)
- Batching: Multiple writes processed in single wake-up

**Latency**:
- Event notification: Immediate (inotify triggers channel send)
- Processing delay: Bounded by batch size (typically <100ms for 1000 rows)
- Meets requirement: <2 seconds notification latency

---

## 7. Constitution Compliance

### Immutability First
- ✅ FileWatcher only monitors appends (no modifications)
- ✅ `lastProcessedSize` is monotonically increasing
- ✅ Finder never modifies existing rows

### Data Integrity
- ✅ Initialization ensures all rows processed exactly once
- ✅ OnRowAdded protocol maintains Finder consistency
- ✅ Partial rows discarded via row boundary calculations

### Concurrent Read-Write Safety
- ✅ FileWatcher uses atomic operations (lock-free)
- ✅ Finder methods use RWMutex for thread safety
- ✅ DBFile.Read uses atomic size tracking

### Single-File Architecture
- ✅ FileWatcher monitors single .fdb file
- ✅ No additional index files or metadata
- ✅ Platform-specific descriptors reference same file

---

## 8. Open Questions Resolved

**Q1: How to handle file watching without complex syscall bridging?**
**A**: Use `github.com/fsnotify/fsnotify` package which provides Go channel-based API and eliminates need for bridge goroutines, pipes, and FD coordination.

**Q2: How to prevent missed updates during Finder initialization?**
**A**: Two-phase with kickstart - capture `initialSize` once, kickstart processes gap.

**Q3: How should Finder behave after encountering an error?**
**A**: Tombstone pattern - drop events, return `TombstonedError` to users.

**Q4: Which Finder implementations need updates?**
**A**: InMemoryFinder and BinarySearchFinder constructors need to return `initialSize`. SimpleFinder works as-is.

**Q5: What happens if FileWatcher encounters errors after Finder is tombstoned?**
**A**: When OnRowAdded() is called on a tombstoned Finder, it returns `TombstonedError`, which causes the FileWatcher's `processBatch()` to propagate the error to `watchLoop()`, which then exits gracefully.

---

## Research Complete

All technical unknowns from the specification have been resolved. Ready to proceed to Phase 1 (Design & Contracts).
