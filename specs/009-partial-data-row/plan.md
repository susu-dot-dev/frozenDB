# Implementation Plan: PartialDataRow Struct Implementation

**Branch**: `009-partial-data-row` | **Date**: 2026-01-14 | **Spec**: [/home/anil/code/frozenDB/specs/009-partial-data-row/spec.md](spec.md)
**Input**: Feature specification from `/specs/009-partial-data-row/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implementation of PartialDataRow struct supporting three progressive states (PartialDataRowWithStartControl → PartialDataRowWithPayload → PartialDataRowWithSavepoint) with state-aware validation, serialization, and completion methods. Uses composition pattern (contains DataRow, not embedded) to ensure code reuse while maintaining state immutability and validation integrity.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Go standard library only + github.com/google/uuid  
**Storage**: Single-file append-only database (.fdb extension)  
**Testing**: Go built-in testing framework with spec tests (Test_S_XXX pattern)  
**Target Platform**: Linux server  
**Project Type**: Single Go library package  
**Performance Goals**: Fixed memory usage, O(1) row seeking, correctness over performance  
**Constraints**: Memory must remain fixed regardless of database size, no breaking changes to existing API  
**Scale/Scope**: Single feature addition to existing frozenDB codebase  

**GitHub Repository**: github.com/anilmfz/frozenDB (derived from git remote)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations
  - PartialDataRows follow append-only pattern and become immutable DataRows when completed
  - State transitions only advance forward, no modification of existing data
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included
  - PartialDataRows inherit DataRow validation with ROW_START (0x1F) and ROW_END (0x0A) sentinels
  - Two-tier integrity through state-specific validation and final DataRow validation
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness
  - No premature optimizations; design prioritizes correct state transitions and validation
  - Uses existing DataRow validation patterns without compromising accuracy
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations
  - UUIDv7 keys validated through existing ValidateUUIDv7() function
  - Maintains frozenDB's chronological ordering requirements
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption
  - PartialDataRows can only exist as last row, allowing safe concurrent reads of committed data
  - State immutability prevents corruption during concurrent operations
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery
  - PartialDataRows exist within same single .fdb file structure
  - No additional files or external storage required
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files
  - Design includes spec test plan for all FR-001 through FR-019 functional requirements
  - Tests will follow Test_S_009_FR_XXX naming convention in appropriate files

## Project Structure

### Documentation (this feature)

```text
specs/009-partial-data-row/
 ├── plan.md              # This file (/speckit.plan command output)
 ├── research.md          # Phase 0 output (/speckit.plan command)
 ├── data-model.md        # Phase 1 output (/speckit.plan command)
 ├── quickstart.md        # Phase 1 output (/speckit.plan command)
 ├── contracts/           # Phase 1 output (/speckit.plan command)
 │   └── api.md          # API contract documentation
 └── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── data_row.go              # Existing DataRow implementation
├── partial_data_row.go       # NEW: PartialDataRow implementation
├── partial_data_row_spec_test.go  # NEW: Spec tests for PartialDataRow
├── data_row_spec_test.go    # Existing DataRow spec tests (will need validation updates)
└── ...other existing files
```

**Structure Decision**: Single Go package following existing frozenDB patterns. Implementation will be in the main frozendb package alongside existing DataRow implementation, with spec tests in the same directory following the [filename]_spec_test.go pattern. The partial_data_row.go file will contain the PartialDataRow struct, its methods, and the new InvalidActionError type.

## Complexity Tracking

> **No constitutional violations - all design choices align with frozenDB principles**

| Design Choice | Rationale | Alternatives Considered |
|---------------|------------|------------------------|
| Composition (contains DataRow) | PartialDataRow is NOT a DataRow - methods like Parity() shouldn't be callable, composition prevents this confusion | Embedding rejected - would make PartialDataRow behave like DataRow incorrectly |
| State-specific validation | Different states have different validation requirements (e.g., no UUID validation in PartialDataRowWithStartControl) | Single validation rejected - would validate fields that don't exist in early states |
| New InvalidActionError type | Distinguishes state transition errors from validation errors as required by spec | Reusing existing error types rejected - spec specifically requires InvalidActionError |
