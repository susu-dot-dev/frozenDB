# Implementation Plan: RowEmitter Layer for Decoupled Row Coordination

**Branch**: `036-row-emitter-layer` | **Date**: 2026-02-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/036-row-emitter-layer/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

This feature introduces a RowEmitter layer to decouple row completion notification logic from Transaction, DBFile, and Finder components. The RowEmitter monitors DBFile for size changes, determines when complete rows are written, and notifies subscribers using a closure-based subscription pattern with snapshot-based thread-safety. This enables multiple independent components to receive row completion events without tight coupling, supporting synchronous error propagation and proper handling of partial rows that complete after initialization.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single append-only file format (frozenDB v1 file format)  
**Testing**: Go testing with `make test` (spec tests via `make test-spec`, unit tests via `make test-unit`)  
**Target Platform**: Linux/Unix systems (file locking via flock)  
**Project Type**: Single project (core database package)  
**Performance Goals**: Fixed memory usage regardless of database size, microsecond-level callback overhead  
**Constraints**: Thread-safe subscription management without deadlocks, synchronous callback execution, no historical event replay  
**Scale/Scope**: Small subscriber count (< 10 typical), support for 1000+ row writes with zero missed events  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations - RowEmitter only monitors file growth, never modifies data
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included - RowEmitter does not modify transaction structure
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness - Snapshot pattern prioritizes correctness over minimal overhead
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations - RowEmitter emits rows in chronological order as written
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption - Subscription snapshot pattern is thread-safe
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery - RowEmitter monitors single DBFile
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files - Will be implemented per docs/spec_testing.md

## Project Structure

### Documentation (this feature)

```text
specs/036-row-emitter-layer/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command) - COMPLETE
├── data-model.md        # Phase 1 output (/speckit.plan command) - COMPLETE
├── contracts/           # Phase 1 output (/speckit.plan command) - COMPLETE
│   └── api.md
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozenDB/
├── internal/frozendb/   # Core database package (internal)
│   ├── file_manager.go  # FileManager (DBFile implementation) - will gain Subscribe() method
│   ├── subscriber.go    # New: Generic Subscriber[T] type for thread-safe callbacks
│   ├── subscriber_test.go # New: Unit tests for Subscriber[T]
│   ├── row_emitter.go   # New: RowEmitter implementation
│   ├── row_emitter_test.go # New: Unit tests for RowEmitter
│   ├── row_emitter_spec_test.go # New: Spec tests for FR-001 through FR-012
│   ├── transaction.go   # Modified: decoupled from row completion notification (may still depend on Finder for queries)
│   ├── finder.go        # Finder interface (unchanged)
│   ├── simple_finder.go # Will subscribe to RowEmitter
│   ├── binary_search_finder.go # Will subscribe to RowEmitter
│   ├── inmemory_finder.go # Will subscribe to RowEmitter
│   ├── file_manager_spec_test.go # Existing + new specs for Subscribe() method
│   └── transaction_spec_test.go # Existing + validation that direct notification removed
├── docs/                # Documentation
└── specs/               # Feature specifications
```

**Structure Decision**: Single project structure with core database package in `internal/frozendb/`. The generic `Subscriber[T]` type is extracted to its own file (`subscriber.go`) since it's used by both FileManager and RowEmitter, promoting code reuse. All new RowEmitter functionality is co-located with existing database components. Spec tests follow the pattern `[file]_spec_test.go` in the same package directory as implementation files per docs/spec_testing.md.

## Complexity Tracking

> **No constitutional violations identified. This section is intentionally empty.**

The RowEmitter design adheres to all constitutional principles without requiring complexity trade-offs.
