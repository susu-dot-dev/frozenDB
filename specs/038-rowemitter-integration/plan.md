# Implementation Plan: RowEmitter Integration

**Branch**: `038-rowemitter-integration` | **Date**: 2026-02-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/038-rowemitter-integration/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

This refactoring decouples the Transaction component from Finder implementations by introducing a centralized RowEmitter notification hub. The RowEmitter subscribes to DBFile write events and notifies downstream Finder subscribers about completed rows, eliminating direct Transaction→Finder coupling. This architectural change simplifies code maintenance and establishes foundation for future features like read-only instance replication. All existing database operations preserve identical behavior and on-disk format, validated through existing spec test suite.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single-file append-only database (.fdb files)  
**Testing**: Go test framework with spec tests (_spec_test.go) and unit tests (_test.go)  
**Target Platform**: Linux, macOS, Windows (cross-platform Go application)  
**Project Type**: Single project (database library with CLI)  
**Performance Goals**: Fixed memory usage O(row_size), support concurrent read/write operations  
**Constraints**: Memory usage must not scale with database size, maintain O(1) notification delivery  
**Scale/Scope**: Internal refactoring affecting 4 files (frozendb.go, finder.go, transaction.go, and 3 finder implementations)  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations - This is an internal refactoring that does not change write behavior
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included - No changes to transaction format or data integrity mechanisms
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness - Refactoring maintains identical functional behavior, validated by existing spec tests
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations - No changes to key ordering or search mechanisms
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption - RowEmitter uses proper locking and notification subscription is thread-safe
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery - No changes to file architecture
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files - FR-007 requires new spec test, all other requirements validated by existing test suite passing unchanged

## Project Structure

### Documentation (this feature)

```text
specs/038-rowemitter-integration/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # Complete API specification
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/frozendb/
├── finder.go                    # Finder interface (remove OnRowAdded method)
├── simple_finder.go             # SimpleFinder implementation (constructor + subscription changes)
├── inmemory_finder.go           # InMemoryFinder implementation (constructor + subscription changes)
├── binary_search_finder.go      # BinarySearchFinder implementation (constructor + subscription changes)
├── row_emitter.go               # RowEmitter implementation (already exists)
├── frozendb.go                  # NewFrozenDB initialization (RowEmitter integration)
├── transaction.go               # Transaction (remove OnRowAdded calls)
├── file_manager.go              # DBFile interface with subscription support
└── *_spec_test.go               # Spec tests for validation

cmd/frozendb/
└── main.go                      # CLI entry point (no changes expected)

pkg/frozendb/
├── frozendb.go                  # Public API wrapper (no changes to public API)
├── finder.go                    # Public FinderStrategy constants (no changes)
└── transaction.go               # Public Transaction API (no changes)
```

**Structure Decision**: This is a single-project Go library with internal implementation in `internal/frozendb/` and public API in `pkg/frozendb/`. The refactoring affects only internal components - no public API surface changes are required. All changes are confined to the notification flow between DBFile, RowEmitter, and Finder implementations.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
