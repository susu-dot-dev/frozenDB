# Implementation Plan: DBFile Read/Write Modes and File Locking

**Branch**: `017-dbfile-abstraction` | **Date**: 2026-01-23 | **Spec**: [link](spec.md)
**Input**: Feature specification from `/specs/017-dbfile-abstraction/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Enhance the DBFile interface to support read/write modes with OS-level file locking, and refactor open.go functions to consolidate file operations through the DBFile interface. This eliminates the current dual-file system approach while maintaining all existing functionality and performance characteristics.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single append-only file format  
**Testing**: go test + spec testing framework (Test_S_ prefix)  
**Target Platform**: Linux server  
**Project Type**: Single project (database library)  
**Performance Goals**: <100ms read mode open, <200ms write mode open, <50ms lock failure  
**Constraints**: Fixed memory usage, concurrent read safety, exclusive write access  
**Scale/Scope**: Database library for embedded use  

**GitHub Repository**: github.com/anomalyco/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations
- [x] **Data Integrity**: File locking enhances data integrity, transaction headers with sentinel bytes preserved
- [x] **Correctness Over Performance**: Mode validation and locking prioritize correctness over performance
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and exclusive writes without data corruption
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files

## Project Structure

### Documentation (this feature)

```text
specs/017-dbfile-abstraction/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output - Breaking change analysis complete
├── data-model.md        # Phase 1 output - Enhanced DBFile interface model
├── contracts/           # Phase 1 output - API contracts and error types
│   └── api.md
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── file_manager.go      # DBFile interface definition + FileManager implementation
├── transaction.go       # DBFile usage via NewTransaction()
├── open.go             # Functions to be refactored for FR-006
├── frozendb.go         # FrozenDB struct with file field to be updated
└── *_spec_test.go      # Spec tests for FR-001 through FR-010
```

**Structure Decision**: Single project structure with focused modifications to existing files in the frozendb package. The changes consolidate the dual file system approach (os.File + DBFile) into a unified DBFile interface with mode-specific locking.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No constitutional violations identified. The design maintains all core principles while enhancing file operation safety through proper locking mechanisms.
