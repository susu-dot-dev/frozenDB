# Implementation Plan: Read-Mode File Updates

**Branch**: `039-read-mode-updates` | **Date**: 2026-02-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/039-read-mode-updates/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

When FrozenDB is opened in read mode, it must remain up-to-date when another process in write mode updates the file. The implementation will use `github.com/fsnotify/fsnotify` to monitor file changes in read-only mode, ensuring zero timing gaps during initialization and serializing all file size updates and subscriber callbacks to maintain consistency with the existing write-mode serialization model.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: `github.com/google/uuid`, `github.com/fsnotify/fsnotify` (new requirement), Go standard library  
**Storage**: Single append-only file with fixed-width rows (frozenDB v1 file format)  
**Testing**: Go testing (`go test`), table-driven spec tests in `*_spec_test.go` files  
**Target Platform**: Linux (fsnotify Linux compatibility is sufficient per AS-001)  
**Project Type**: Single Go project (database library)  
**Performance Goals**: <1 second latency for Get() operations to reflect newly written keys (SC-001); fixed memory usage independent of database size  
**Constraints**: Zero public API changes (TC-001); only one active update cycle at any time (FR-004); must handle initialization race window (FR-003); file watcher cleanup within 100ms (SC-010)  
**Scale/Scope**: Single database file with unlimited rows; concurrent read and write processes; rapid writes (10,000+ during initialization stress tests per SC-006)  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations
  - *Rationale*: Feature only reads file size and subscribes to updates; no modifications to database content
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included
  - *Rationale*: Feature does not modify transaction structure or integrity mechanisms; existing corruption detection unchanged
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness
  - *Rationale*: Serialized update cycles (FR-004) prioritize correctness; Get() consistency guaranteed (SC-003)
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations
  - *Rationale*: Feature does not affect key ordering or timestamp handling; Finder refreshes to maintain ordering
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption
  - *Rationale*: Core requirement - enables read-mode instances to safely observe write-mode updates (FR-001, FR-002, FR-003)
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
  - *Rationale*: Feature monitors single database file using fsnotify; no additional files introduced
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files
  - *Rationale*: All FR-XXX requirements (FR-001 through FR-008) will have spec tests in `internal/frozendb/file_manager_spec_test.go`

## Project Structure

### Documentation (this feature)

```text
specs/039-read-mode-updates/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # API specifications for FileManager updates
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozenDB/
├── internal/frozendb/
│   ├── file_manager.go            # Core implementation (FileManager struct)
│   ├── file_manager_test.go       # Unit tests
│   └── file_manager_spec_test.go  # Spec tests (Test_S_039_FR_XXX_*)
├── docs/
│   ├── v1_file_format.md          # File format specification (context)
│   ├── error_handling.md          # Error handling guidelines (context)
│   ├── spec_testing.md            # Spec testing guidelines (context)
│   └── testing.md                 # Testing patterns (context)
└── go.mod                         # Dependencies (add fsnotify)
```

**Structure Decision**: Single project structure. All FileManager functionality lives in `internal/frozendb/file_manager.go` with no public API changes. Tests are co-located with implementation following frozenDB conventions.

## Complexity Tracking

No constitutional violations - this section is empty as all constitutional checks passed.
