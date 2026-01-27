# Implementation Plan: FBS UUIDv7 Refactor

**Branch**: `026-fbs-uuidv7-refactor` | **Date**: 2026-01-27 | **Spec**: specs/026-fbs-uuidv7-refactor/spec.md
**Input**: Feature specification from `/specs/026-fbs-uuidv7-refactor/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Refactor the existing FuzzyBinarySearch algorithm to work with UUIDv7 keys instead of int64 timestamps while preserving the O(log n) + k performance characteristics. The algorithm will extract timestamp portions from UUIDv7 keys for binary search comparison but use full UUID equality during the linear scan phase to handle multiple keys with identical timestamps.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single append-only file format per frozenDB v1 specification  
**Testing**: Go testing framework with testify for assertions  
**Target Platform**: Linux server (cross-platform compatible)  
**Project Type**: Single Go package/library  
**Performance Goals**: O(log n) + k complexity where k = UUIDv7 entries in skew window  
**Constraints**: Fixed memory usage (O(1) space complexity), concurrent read-write safety  
**Scale/Scope**: Database search algorithm for immutable key-value store  

**GitHub Repository**: git@github.com:frozenDB/frozenDB.git

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Phase 0 Check (✓ PASSED)
- [x] **Immutability First**: Refactored search algorithm operates on immutable data structure, no modifications required
- [x] **Data Integrity**: Algorithm preserves existing data integrity checks, operates on validated UUIDv7 keys
- [x] **Correctness Over Performance**: Algorithm maintains correctness through UUID equality matching in linear scan
- [x] **Chronological Ordering**: Uses UUIDv7 timestamp extraction for time-based ordering with skew window handling
- [x] **Concurrent Read-Write Safety**: Read-only operation safe for concurrent access during writes
- [x] **Single-File Architecture**: Operates within existing single-file database architecture
- [x] **Spec Test Compliance**: All 5 functional requirements will have corresponding spec tests in fuzzy_binary_search_spec_test.go

### Post-Phase 1 Design Check (✓ CONFIRMED)
- [x] **Immutability First**: Data model confirms read-only algorithm operation with no data modification
- [x] **Data Integrity**: Design incorporates existing UUID validation and error handling patterns
- [x] **Correctness Over Performance**: Linear scan ensures exact UUID matching, preserving correctness
- [x] **Chronological Ordering**: API specification confirms timestamp extraction for ordering with skew handling
- [x] **Concurrent Read-Write Safety**: Read-only nature confirmed in data flow relationships
- [x] **Single-File Architecture**: Integration within existing frozenDB package structure confirmed
- [x] **Spec Test Compliance**: Detailed spec test requirements defined for all 5 functional requirements

## Project Structure

### Documentation (this feature)

```text
specs/026-fbs-uuidv7-refactor/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/
│   └── api.md           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── fuzzy_binary_search.go              # Main implementation to be refactored
├── fuzzy_binary_search_test.go         # Unit tests to be adapted
├── fuzzy_binary_search_spec_test.go    # Spec tests to be created
├── uuid_helpers.go                     # Existing UUID validation functions
├── errors.go                          # Error handling functions
└── [other existing files...]
```

**Structure Decision**: Single Go package structure maintaining existing frozenDB architecture. The refactored algorithm will be implemented in the existing `fuzzy_binary_search.go` file with corresponding updates to test files.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
