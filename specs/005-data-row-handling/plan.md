# Implementation Plan: 005-data-row-handling

**Branch**: `005-data-row-handling` | **Date**: 2026-01-11 | **Spec**: specs/005-data-row-handling/spec.md
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement DataRow handling for frozenDB following the established ChecksumRow patterns. The DataRow will provide UUIDv7 key validation using github.com/google/uuid, JSON string payload handling with NULL_BYTE padding, and proper serialization according to v1_file_format.md specification. Implementation will use three-file architecture (data_row.go, data_row_test.go, data_row_spec_test.go) with correct transaction controls (T/R for start, TC/RE/SC/SE/R0-R9/S0-S9 for end). DataRow uses manual struct initialization with Validate() method (no constructor) to support both manual creation and UnmarshalText deserialization. DataRow validates single-row format while multi-row transaction state validation is handled at higher layer.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library only  
**Storage**: Single-file frozenDB database (.fdb extension)  
**Testing**: Go testing framework with table-driven tests and spec tests  
**Target Platform**: Linux server  
**Project Type**: Single project with modular architecture  
**Performance Goals**: Fixed memory usage regardless of database size, O(1) row seeking  
**Constraints**: Append-only immutability, concurrent read-write safety, data integrity non-negotiable  
**Scale/Scope**: Key-value database component for frozenDB  

**GitHub Repository**: github.com/anilmahadev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: DataRow preserves append-only immutability with no delete/modify operations ✅
- [x] **Data Integrity**: DataRow includes sentinel bytes and parity validation for corruption detection ✅
- [x] **Correctness Over Performance**: Fixed memory usage prioritized over optimization speed ✅
- [x] **Chronological Ordering**: DataRow validates UUIDv7 for time-based key ordering ✅
- [x] **Concurrent Read-Write Safety**: DataRow design follows ChecksumRow patterns for thread safety ✅
- [x] **Single-File Architecture**: DataRow integrates with single-file frozenDB design ✅
- [x] **Spec Test Compliance**: All FR requirements will have corresponding spec tests in data_row_spec_test.go ✅

**Phase 1 Design Review**: All constitutional requirements satisfied. No violations detected.

## Project Structure

### Documentation (this feature)

```text
specs/005-data-row-handling/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── data_row.go           # DataRow implementation (main file)
├── data_row_test.go      # Unit tests for DataRow
├── data_row_spec_test.go # Specification tests for FR requirements
├── checksum.go           # Existing ChecksumRow (reference implementation)
├── base_row.go           # Generic baseRow foundation
```

**Structure Decision**: Single project architecture following existing ChecksumRow patterns with three-file organization (implementation, unit tests, spec tests) in the frozendb package.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
