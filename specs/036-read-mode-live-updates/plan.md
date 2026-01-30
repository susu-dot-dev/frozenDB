# Implementation Plan: Read-Mode Live Updates for Finders

**Branch**: `036-read-mode-live-updates` | **Date**: Fri Jan 30 2026 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/036-read-mode-live-updates/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

FrozenDB finders opened in read-mode must detect and incorporate new keys added by concurrent write processes, without requiring database reopening. The implementation uses Go's standard library inotify (via fsnotify syscalls) to monitor file changes, with a batching mechanism that processes multiple row additions per notification while maintaining the Finder protocol requirement to notify after every row write. The design ensures race-free initialization by coordinating file scanning and watcher startup to prevent missing writes during the initialization window.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, github.com/fsnotify/fsnotify, Go standard library  
**Storage**: Single append-only file database with fixed-width rows  
**Testing**: Go test framework (go test), spec tests (Test_S_036_FR_XXX pattern), unit tests  
**Target Platform**: Linux (inotify-based file watching via fsnotify)  
**Project Type**: Single library project (database engine)  
**Performance Goals**: <2 second latency for live update detection, support for 1000 writes/sec with batching  
**Constraints**: Fixed memory usage (no scaling with DB size for SimpleFinder), no dropped updates during initialization, finder protocol compatibility (OnRowAdded after each write)  
**Scale/Scope**: Database files of arbitrary size, concurrent multi-process read/write access  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design (Phase 0)

- [x] **Immutability First**: Design preserves append-only immutability - file watching only monitors appends, no modifications
- [x] **Data Integrity**: No changes to transaction integrity or sentinel bytes; watchers only detect complete rows with ROW_END
- [x] **Correctness Over Performance**: Batching optimizations maintain correctness by tracking last notified position and never skipping rows
- [N/A] **Chronological Ordering**: Not applicable - file watching doesn't affect key ordering or time-based operations
- [x] **Concurrent Read-Write Safety**: Design specifically enables safe concurrent reads during writes by detecting appends without blocking
- [x] **Single-File Architecture**: No changes to single-file architecture; watchers monitor the existing database file
- [x] **Spec Test Compliance**: All 8 functional requirements (FR-001 through FR-008) will have corresponding spec tests in appropriate _spec_test.go files

### Post-Design (Phase 1) - Re-evaluation

- [x] **Immutability First**: ✅ CONFIRMED - FileWatcher only reads appended data via processBatch(); never modifies existing rows; lastProcessedSize is monotonically increasing
- [x] **Data Integrity**: ✅ CONFIRMED - Three-phase initialization ensures all rows are indexed exactly once; OnRowAdded maintains Finder consistency; partial row handling via row boundary calculations
- [x] **Correctness Over Performance**: ✅ CONFIRMED - Wake-up flag with careful ordering (clear flag → read size → process batch) prevents missed rows; batching improves performance without sacrificing correctness; Phase 3 catch-up loop ensures convergence
- [N/A] **Chronological Ordering**: ✅ NOT APPLICABLE - File watching operates on row indices, not timestamps; UUIDv7 ordering unaffected
- [x] **Concurrent Read-Write Safety**: ✅ CONFIRMED - FileWatcher uses atomic operations (wakeFlag, lastProcessedSize) for lock-free coordination; Finder methods use RWMutex for thread safety; DBFile.Read uses atomic size tracking
- [x] **Single-File Architecture**: ✅ CONFIRMED - FileWatcher monitors single .fdb file; no additional index files or metadata; inotify descriptors reference same file
- [x] **Spec Test Compliance**: ✅ CONFIRMED - All 8 functional requirements mapped to spec test names in contracts/api.md section 7.3; spec tests will be placed in file_watcher_spec_test.go, finder_spec_test.go, and finder implementation _spec_test.go files

**Constitution Compliance**: ✅ ALL CHECKS PASS

## Project Structure

### Documentation (this feature)

```text
specs/036-read-mode-live-updates/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # API specifications for file watcher and finder updates
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozenDB/
├── pkg/frozendb/              # Public API
│   ├── finder.go              # Finder interface (no changes needed)
│   └── frozendb.go            # FrozenDB struct (integration point)
├── internal/frozendb/         # Internal implementation
│   ├── file_watcher.go        # NEW: File watching implementation (Linux inotify)
│   ├── file_watcher_spec_test.go  # NEW: Spec tests for file watcher
│   ├── finder.go              # Finder interface (no changes needed)
│   ├── finder_spec_test.go    # NEW/UPDATED: Spec tests for live update behavior
│   ├── simple_finder.go       # SimpleFinder implementation (no changes needed)
│   ├── inmemory_finder.go     # InMemoryFinder implementation (needs three-phase init)
│   ├── inmemory_finder_spec_test.go  # UPDATED: Spec tests for InMemoryFinder live updates
│   ├── binary_search_finder.go     # BinarySearchFinder implementation (needs three-phase init)
│   ├── binary_search_finder_spec_test.go  # UPDATED: Spec tests for BinarySearchFinder live updates
│   └── frozendb.go            # FrozenDB implementation (integration point)
└── docs/                      # Documentation
    └── v1_file_format.md      # Existing file format spec (reference only)
```

**Structure Decision**: Single library project structure. File watching implementation will be added to internal/frozendb/ package using github.com/fsnotify/fsnotify package. Linux-only implementation using fsnotify's inotify backend. All three finder strategies (SimpleFinder, InMemoryFinder, BinarySearchFinder) evaluated; only InMemoryFinder and BinarySearchFinder require updates for three-phase initialization.

## Complexity Tracking

**No Violations Identified** - No constitutional violations require justification. The design adheres to all frozenDB principles:

- Uses append-only architecture (immutability)
- Maintains data integrity with existing sentinel bytes and checksums
- Prioritizes correctness with three-phase initialization and atomic operations
- Supports concurrent read-write safety via lock-free coordination
- Works within single-file architecture
- Includes comprehensive spec test coverage

---

## Planning Complete

### Deliverables Generated

**Phase 0 (Research)**:
- ✅ `research.md` - Comprehensive research on Go inotify usage, batching strategy, and initialization race prevention

**Phase 1 (Design)**:
- ✅ `data-model.md` - Entity definitions, state transitions, validation rules
- ✅ `contracts/api.md` - Complete API specifications with method signatures, behavior, error handling

### Key Design Decisions

1. **File Watching**: Use `github.com/fsnotify/fsnotify` package for inotify events via Go channels; Linux-only implementation
2. **Batching**: fsnotify coalesces events internally; natural batching via event channel (zero CPU when idle)
3. **Initialization**: Three-phase algorithm (anchor → scan → catch-up) ensures no missed rows
4. **Integration**: FileWatcher is internal to Finder implementations; each Finder constructor (InMemoryFinder, BinarySearchFinder) receives a mode parameter and creates its own internal FileWatcher when mode is MODE_READ. SimpleFinder does not create a watcher. FrozenDB is responsible only for passing the mode parameter to Finder constructors during database opening.

### Next Steps (Phase 2 - Implementation)

The implementation phase should proceed with:

1. **File Watcher Core** (`file_watcher.go` using fsnotify package - internal to frozendb)
2. **Finder Updates** (add mode parameter to constructors; create internal FileWatcher in read-mode; handle initialization race condition internally)
3. **Integration** (update `frozendb.go` to pass mode to Finder constructors; Finder manages watcher lifecycle)
4. **Spec Tests** (all 8 functional requirements: FR-001 through FR-008)
5. **Integration Tests** (multi-process scenarios, concurrency stress tests on Linux)

**Branch**: `036-read-mode-live-updates`  
**Planning Date**: Fri Jan 30 2026  
**Status**: ✅ Planning Complete - Ready for Implementation
