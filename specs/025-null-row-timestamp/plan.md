# Implementation Plan: NullRow Timestamp Modification

**Branch**: `025-null-row-timestamp` | **Date**: 2026-01-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/025-null-row-timestamp/spec.md`

## Summary

Modify NullRow implementation to use UUIDv7 values with timestamps matching current maxTimestamp instead of uuid.Nil. Centralize scattered UUID functions into uuid_helpers.go and create NewNullRow constructor for proper validation and error handling.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single file append-only database (frozenDB v1 format)  
**Testing**: Go testing framework with spec tests (Test_S_025_FR_XXX_*)  
**Target Platform**: Linux/Unix systems (frozenDB cross-platform)  
**Project Type**: Single project - Go library with database functionality  
**Performance Goals**: O(1) maxTimestamp lookup, constant memory usage, UUID bit manipulation  
**Constraints**: Memory usage must remain constant regardless of database size, O(1) UUID operations  
**Scale/Scope**: Database row operations, UUID centralization across 6+ source files  

**GitHub Repository**: Obtain full repository path using `git remote get-url origin` for import statements in documentation and examples. Example: `github.com/user/repo` from `git@github.com:user/repo.git`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files

## Project Structure

### Documentation (this feature)

```text
specs/025-null-row-timestamp/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# frozenDB single project structure (chosen)
frozendb/
├── uuid_helpers.go       # NEW: Centralized UUID utility functions
├── null_row.go          # MODIFIED: Update to use new constructor
├── data_row.go          # MODIFIED: Remove duplicate UUID functions
├── transaction.go        # MODIFIED: Remove duplicate UUID functions
├── simple_finder.go      # MODIFIED: Import uuid_helpers
├── inmemory_finder.go    # MODIFIED: Import uuid_helpers
├── errors.go            # UNCHANGED: Existing error types used
└── [other files]        # MODIFIED: Update imports as needed
```

**Structure Decision**: Single project structure with centralized UUID utilities in frozendb package

## Complexity Tracking

No constitutional violations - design maintains all frozenDB principles.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |
