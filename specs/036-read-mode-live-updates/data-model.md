# Data Model: Read-Mode Live Updates

**Date**: Fri Jan 30 2026  
**Feature**: Read-Mode Live Updates for Finders  
**Branch**: 036-read-mode-live-updates

This document defines the new data entities, state changes, and validation rules introduced by the read-mode live updates feature.

---

## 1. New Entities

### 1.1 FileWatcher

**Purpose**: Internal mechanism that monitors database file for new appends and notifies parent Finder.

**Ownership**: Private internal field of InMemoryFinder and BinarySearchFinder. Created during Finder construction (NewInMemoryFinder/NewBinarySearchFinder) in MODE_READ only. Not accessible outside the Finder implementation.

**Construction**: FileWatcher is constructed internally by InMemoryFinder and BinarySearchFinder constructors. It is NOT constructed by FrozenDB. The Finder passes callback functions to the FileWatcher for row notifications and error handling.

**Attributes**:

| Field | Type | Description | Validation Rules |
|-------|------|-------------|------------------|
| `watcher` | `*fsnotify.Watcher` | fsnotify file system watcher | Must not be nil |
| `lastProcessedSize` | `atomic.Int64` | Last byte position processed | Must be >= HEADER_SIZE, monotonically increasing |
| `dbFile` | `DBFile` | Database file interface for reading | Must not be nil |
| `onRowAdded` | `func(int64, Row)` | Callback to notify parent Finder of new rows | Must not be nil |
| `onError` | `func(error)` | Callback to notify parent Finder of errors | Must not be nil |
| `rowSize` | `int32` | Fixed row size from header | Must match header.rowSize (128-65536) |
| `dbFilePath` | `string` | Path to database file | Must be valid path |

**Key Responsibilities**:
- Detect when database file grows via fsnotify Write events
- Notify parent Finder of new rows via callback function
- Handle errors by calling parent Finder's error callback
- Clean shutdown via watcher.Close() which closes event channels

---

## 2. Modified Entities

### 2.1 Finder Interface

**New Method**:
```go
OnError(err error)
```

**Purpose**: Receives errors from internal FileWatcher, enters tombstone state.

**Tombstone State Behavior**:
- All subsequent `GetIndex()`, `GetTransactionStart()`, `GetTransactionEnd()` calls return `TombstonedError`
- `MaxTimestamp()` continues to work (no error return)
- `OnRowAdded()` returns `TombstonedError` wrapping the original error
- User must close and reopen database to recover

---

### 2.2 InMemoryFinder

**New Fields**:
- `watcher *FileWatcher` - Internal file watcher for read-mode live updates (nil in write-mode)
- `tombstoneErr error` - Stores error that caused tombstone state (nil = healthy)

**Constructor Changes**:
- NewInMemoryFinder now accepts `mode OpenMode` parameter
- In MODE_READ: Constructor creates internal FileWatcher and passes callback functions
- In MODE_WRITE: No FileWatcher created (updates come from Transaction)
- Constructor signature: `NewInMemoryFinder(dbFile DBFile, dbFilePath string, rowSize int32, mode OpenMode) (*InMemoryFinder, error)`
- No longer returns initialSize - FileWatcher lifecycle is fully internal

**Behavior Change**:
- In MODE_READ: Creates internal FileWatcher to receive live updates
- In MODE_WRITE: No FileWatcher (updates come from Transaction)
- After tombstone: Returns `TombstonedError` from all methods including `OnRowAdded()`

---

### 2.3 BinarySearchFinder

**New Fields**:
- `watcher *FileWatcher` - Internal file watcher for read-mode live updates (nil in write-mode)
- `tombstoneErr error` - Stores error that caused tombstone state (nil = healthy)

**Constructor Changes**:
- NewBinarySearchFinder now accepts `mode OpenMode` parameter
- In MODE_READ: Constructor creates internal FileWatcher and passes callback functions
- In MODE_WRITE: No FileWatcher created (updates come from Transaction)
- Constructor signature: `NewBinarySearchFinder(dbFile DBFile, dbFilePath string, rowSize int32, mode OpenMode) (*BinarySearchFinder, error)`
- No longer returns initialSize - FileWatcher lifecycle is fully internal

**Behavior Change**: Same as InMemoryFinder

---

### 2.4 SimpleFinder

**New Field**:
- `tombstoneErr error` - Stores error that caused tombstone state (nil = healthy)

**Behavior Change**:
- No FileWatcher needed (uses on-demand scanning)
- Tombstone behavior same as other Finders

---

## 3. Error Conditions

### 3.1 Initialization Errors

| Error Condition | Error Type | Handling |
|-----------------|------------|----------|
| Finder scan encounters read error | `ReadError` | Return error from NewFinder(); database not opened |
| Finder scan encounters corrupt row | `CorruptDatabaseError` | Return error from NewFinder(); database not opened |
| fsnotify initialization fails during FileWatcher construction | `InternalError` | Return error from NewFinder(); database not opened |

**Policy**: Fail fast during initialization.

---

### 3.2 Runtime Errors

| Error Condition | Error Type | Handling |
|-----------------|------------|----------|
| FileWatcher encounters read error | `ReadError` | Call `finder.OnError(err)`, Finder tombstoned |
| FileWatcher encounters corrupted row | `CorruptDatabaseError` | Call `finder.OnError(err)`, Finder tombstoned |
| `OnRowAdded()` returns error | Various | Call `finder.OnError(err)`, Finder tombstoned |

**Policy**: On any error during live updates, Finder enters permanent tombstone state.

---

### 3.3 Concurrency Edge Cases

| Scenario | Resolution |
|----------|------------|
| Writes during Finder initialization | Kickstart mechanism processes gap after initialization (see section 7) |
| Multiple rapid writes | Batched processing reduces overhead |
| Partial row write | Incomplete rows discarded until ROW_END detected |
| FileWatcher error after tombstone | OnRowAdded returns TombstonedError, causing watchLoop to exit |

---

## 4. Data Relationships

```
FrozenDB
└── finder (Finder interface)
    ├── SimpleFinder
    ├── InMemoryFinder
    │   └── watcher (*FileWatcher) ← internal, MODE_READ only
    └── BinarySearchFinder
        └── watcher (*FileWatcher) ← internal, MODE_READ only
```

**Data Flow** (MODE_READ):
1. External writer appends to database file
2. OS generates file change notification (inotify on Linux)
3. fsnotify library detects change and sends event to Events channel
4. Finder's internal FileWatcher receives event from channel
5. FileWatcher calls `onRowAdded(idx, row)` callback
6. Finder updates internal state
7. User queries see updated data

**Data Flow** (MODE_WRITE):
1. User calls `db.Add(key, value)`
2. Transaction writes to file
3. Transaction calls `finder.OnRowAdded(idx, row)` directly
4. No FileWatcher involved

---

## 5. Validation Rules

### 5.1 Finder Initialization

- Finder must capture initial file size exactly once
- Must scan from HEADER_SIZE to initial file size
- In MODE_READ: Must handle concurrent writes during initialization

### 5.2 Tombstone State

- Once `tombstoneErr != nil`, Finder is permanently tombstoned
- All query methods must check `tombstoneErr` and return `TombstonedError` if set
- `OnRowAdded()` must check `tombstoneErr` and return `TombstonedError` if set

### 5.3 FileWatcher Operation

- FileWatcher is a private field of parent Finder, created during Finder construction
- FileWatcher lifecycle is managed entirely by Finder (not by FrozenDB)
- FrozenDB is unaware of FileWatcher existence
- Must process rows in sequential order (no skips, no duplicates)
- Must call error callback on first error and stop processing

---

## 6. Constraints

### Immutability Constraints
- FileWatcher only monitors APPEND operations
- Existing rows never modified
- File position tracking is monotonically increasing

### Correctness Constraints
- `OnRowAdded()` called exactly once per row
- Rows processed in sequential order
- Initialization gap handled by kickstart mechanism

### Performance Constraints
- FileWatcher adds minimal memory overhead (~128 bytes struct + fsnotify internal state)
- Idle CPU usage ~0% (event-driven via channels, not polling)
- Active processing: O(n) where n = number of new rows
- Linux-only (inotify backend)

---

## 7. Kickstart Mechanism (Initialization Race Prevention)

**Purpose**: Ensures zero data loss during the window between Finder initialization scan and FileWatcher activation.

**Problem**: If writes occur during Finder initialization, they might be missed:
1. Finder captures `initialSize` at time T0
2. Finder scans rows from HEADER_SIZE to initialSize (takes time)
3. External writer adds rows at time T1 (during scan)
4. Finder creates FileWatcher at time T2
5. FileWatcher starts monitoring at time T3
6. **Risk**: Rows written between T0 and T3 might be missed

**Solution - Two-Phase Initialization with Kickstart**:

### Phase 1: Anchor
```
initialSize = dbFile.Size()  // Captured once, before scan
```

### Phase 2: Initial Scan
```
Scan rows from HEADER_SIZE to initialSize
Build internal state (indexes, etc.)
```

### Phase 3: Kickstart (Catch-up)
```
FileWatcher.watchLoop() starts:
1. currentSize = dbFile.Size()  // Read current size
2. if currentSize > initialSize:
     gap = currentSize - initialSize
     processBatch(initialSize, currentSize)  // Process missed rows
3. Enter main event loop (monitor for future writes)
```

**Guarantees**:
- Every row written between T0 and T3 is processed by kickstart in Phase 3
- Every row written after T3 is processed by normal event loop
- No duplicates (scan stops at initialSize, kickstart starts at initialSize)
- No gaps (kickstart processes [initialSize, currentSize) before entering event loop)

**Implementation Note**: The kickstart happens inside FileWatcher.watchLoop(), immediately after watcher initialization and before entering the main select loop for events.

---

This data model defines all entity changes and validation rules for the read-mode live updates feature.
