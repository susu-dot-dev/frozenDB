# Implementation Plan: NullRow Struct Implementation

**Branch**: `010-null-row-struct` | **Date**: 2025-01-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/010-null-row-struct/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implementation of a NullRow struct that provides validation, marshaling, and unmarshaling capabilities for null operations in frozenDB. The struct follows the v1 file format specification with strict adherence to immutability principles and data integrity requirements. The implementation includes error handling patterns consistent with existing codebase (InvalidInputError for validation failures, CorruptDatabaseError for unmarshal failures) and provides comprehensive spec test coverage.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single file append-only format  
**Testing**: Go testing framework with spec tests  
**Target Platform**: Linux/macOS/Windows (cross-platform)  
**Project Type**: Single library project  
**Performance Goals**: <1ms for NullRow operations  
**Constraints**: Fixed memory usage, O(1) operations, append-only immutability  
**Scale/Scope**: Single struct implementation for core database functionality  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: PASSED - All constitutional requirements satisfied. Post-design verification complete.*

- [x] **Immutability First**: NullRow struct is immutable with fixed fields following append-only pattern
- [x] **Data Integrity**: Parity bytes and validation ensure corruption detection for NullRows
- [x] **Correctness Over Performance**: Validation prioritized over speed, <1ms target is achievable without sacrificing correctness
- [x] **Chronological Ordering**: NullRows use uuid.Nil and are excluded from timestamp ordering validation
- [x] **Concurrent Read-Write Safety**: NullRow operations are thread-safe and don't interfere with concurrent reads
- [x] **Single-File Architecture**: NullRows integrate with existing single-file database format
- [x] **Spec Test Compliance**: All 13 functional requirements have corresponding spec tests in null_row_spec_test.go

**Constitutional Compliance**: ✅ FULLY COMPLIANT - No violations detected

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
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
frozendb/
├── null_row.go           # NullRow struct implementation
├── null_row_test.go      # Unit tests for NullRow
├── null_row_spec_test.go  # Spec tests for FR-001 through FR-013
```

**Structure Decision**: Single Go package structure following existing frozenDB conventions. NullRow implementation placed in core frozendb package alongside other database components.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
