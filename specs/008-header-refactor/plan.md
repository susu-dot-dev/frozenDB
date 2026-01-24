# Implementation Plan: Header Refactor

**Branch**: `008-header-refactor` | **Date**: 2025-01-14 | **Spec**: /specs/008-header-refactor/spec.md
**Input**: Feature specification from `/specs/008-header-refactor/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Refactor frozenDB Header struct to eliminate dual header creation pattern by implementing MarshalText() method, moving functionality to dedicated header.go file, and aligning with DataRow/ChecksumRow patterns while maintaining 100% backward compatibility.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Go standard library only (encoding/json, fmt, strings, bytes)  
**Storage**: Single-file frozenDB database (.fdb extension)  
**Testing**: go test with spec testing framework (Test_S_XXX_FR_XXX naming)  
**Target Platform**: Linux server  
**Project Type**: Single project library  
**Performance Goals**: No performance regressions, maintain fixed memory usage  
**Constraints**: Must preserve 64-byte header format exactly, maintain all existing APIs  
**Scale/Scope**: Internal refactoring affecting Header struct and create.go usage patterns  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Refactor does not modify data operations, preserves append-only immutability with no delete/modify operations
- [x] **Data Integrity**: Refactor maintains existing transaction headers with sentinel bytes for corruption detection
- [x] **Correctness Over Performance**: Refactor is code organization improvement, no performance optimizations that could compromise correctness
- [x] **Chronological Ordering**: Refactor does not affect key ordering or time handling, maintains existing chronological key ordering
- [x] **Concurrent Read-Write Safety**: Refactor does not change read-write patterns, maintains concurrent safety
- [x] **Single-File Architecture**: Refactor maintains single-file frozenDB database with append-only architecture
- [x] **Spec Test Compliance**: All functional requirements will have corresponding spec tests in header_spec_test.go files

**Phase 1 Design Re-evaluation**: ✅ PASSED - Design maintains all constitutional principles with no violations. The refactor is purely organizational and API improvement without affecting core frozenDB architecture or data integrity guarantees.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── frozendb.go          # Main DB interface
├── open.go              # File opening and reading
├── create.go            # Database creation (modified - removes Header/generateHeader)
├── header.go            # NEW: Header struct and all methods (moved from create.go)
├── data_row.go          # DataRow struct and methods (existing)
├── checksum.go          # ChecksumRow struct and methods (existing)
├── transaction.go       # Transaction handling (existing)
├── errors.go            # Error definitions (existing)
├── row.go               # Base row functionality (existing)
└── *_spec_test.go       # Spec tests for each component
```

**Structure Decision**: Single project with dedicated file for Header functionality, following existing pattern where major components have dedicated files (data_row.go, checksum.go, transaction.go). New header.go will contain Header struct, all methods, and related constants moved from create.go.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
