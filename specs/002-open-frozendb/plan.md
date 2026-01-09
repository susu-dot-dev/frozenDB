# Implementation Plan: Open frozenDB Files

**Branch**: `002-open-frozendb` | **Date**: 2026-01-09 | **Spec**: `/specs/002-open-frozendb/spec.md`
**Input**: Feature specification from `/specs/002-open-frozendb/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

The primary requirement is to implement a NewFrozenDB function that opens existing frozenDB database files in both read and write modes with proper concurrent access control. The technical approach involves:

1. **File Access Control**: Implement OS-level file locking with no locks for readers (append-only safe) and exclusive locks for writers
2. **Header Validation**: Validate frozenDB v1 format headers per v1_file_format.md specification
3. **Resource Management**: Proper cleanup of file descriptors and locks with idempotent Close() methods
4. **Error Handling**: Structured error types including the new CorruptDatabaseError for validation failures

The implementation will create a primary frozendb.go file containing the FrozenDB struct and related functionality, with proper support for concurrent reads while maintaining write exclusivity.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Standard library only (os, syscall, encoding/json, sync)  
**Storage**: Single file-based database with append-only immutability  
**Testing**: Go testing framework with table-driven tests and spec tests  
**Target Platform**: Linux (file locking requires syscall-specific implementations)  
**Project Type**: Single project with embedded database library  
**Performance Goals**: <100ms database opening, <10ms resource cleanup, fixed memory usage  
**Constraints**: OS-level file locking, immutable append-only storage, UUIDv7 key ordering  
**Scale/Scope**: Unlimited concurrent readers, single writer per database file  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

### Key Design Decisions

**Thread Safety Policy**:
- `NewFrozenDB()`: Thread-safe for concurrent calls on different files
- `Close()`: **Thread-safe and idempotent** - can be called concurrently from multiple goroutines
- Instance methods: Not thread-safe (one instance per goroutine pattern)

**Rationale for Close() Thread Safety**:
- Cleanup method often called from defer statements in different goroutines
- May be invoked from signal handlers or panic recovery
- Prevents double-close scenarios using mutex-protected state
- Enables safe concurrent access patterns during shutdown

**Locking Strategy Correction**:
- **Readers**: No locks needed (append-only files are safe for concurrent reads)
- **Writers**: Exclusive locks only (prevent concurrent appends)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations (open functionality only reads existing files)
- [x] **Data Integrity**: Header validation per v1_file_format.md detects corruption; transactions handled by existing create functionality
- [x] **Correctness Over Performance**: File locking and validation prioritize data integrity over speed optimizations
- [x] **Chronological Ordering**: Design supports time-based key ordering inherited from create functionality
- [x] **Concurrent Read-Write Safety**: Design supports multiple concurrent readers with exclusive writer access
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in frozendb_open_spec_test.go files

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── errors.go           # Existing error types (will add CorruptDatabaseError)
├── create.go           # Existing database creation functionality  
├── create_test.go      # Existing unit tests for create functionality
├── create_spec_test.go # Existing spec tests for create functionality
├── open.go             # NEW: Primary file for database opening functionality
├── open_test.go        # NEW: Unit tests for open functionality  
├── open_spec_test.go   # NEW: Spec tests for open functionality
└── frozendb.go         # NEW: Main FrozenDB struct and public API
```

**Structure Decision**: Single project with Go package structure. The new frozendb.go file will contain the primary FrozenDB struct as requested by the user, keeping the API simple and focused. Open functionality will be implemented in open.go with proper separation of concerns while maintaining a clean public interface.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
