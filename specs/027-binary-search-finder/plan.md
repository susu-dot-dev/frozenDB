# Implementation Plan: BinarySearchFinder

**Branch**: `027-binary-search-finder` | **Date**: 2025-01-26 | **Spec**: `/home/anil/code/frozenDB/specs/027-binary-search-finder/spec.md`
**Input**: Feature specification from `/specs/027-binary-search-finder/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

The BinarySearchFinder provides O(log n) lookup performance for large frozenDB databases while maintaining fixed memory usage like SimpleFinder. It leverages the generally ascending timestamp ordering of UUIDv7 keys to perform binary search on the database file using FuzzyBinarySearch algorithm, with special handling for checksum rows that occur every 10,000 rows. The implementation copies SimpleFinder exactly except for the GetIndex method which uses FuzzyBinarySearch with logical-to-physical index mapping. Logical indices map to both DataRows and NullRows (both have valid logical indices), and the physical-to-logical mapping is a simple mathematical operation: `physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1`. DataRows must reject UUIDs with all-zero non-timestamp parts (bytes 7, 9-15), and GetIndex() must reject NullRow UUID search keys early.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single file append-only format  
**Testing**: go test with spec tests  
**Target Platform**: Linux server  
**Project Type**: single  
**Performance Goals**: O(log n) lookup time, fixed memory usage  
**Constraints**: Fixed memory regardless of database size, handle checksum rows every 10,000  
**Scale/Scope**: Large/unbounded databases that don't fit in memory  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

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
├── binary_search_finder.go        # BinarySearchFinder implementation
├── binary_search_finder_spec_test.go  # Spec tests for BinarySearchFinder
├── finder.go                      # Finder interface and strategy constants
├── simple_finder.go              # Reference SimpleFinder implementation
├── fuzzy_binary_search.go       # FuzzyBinarySearch algorithm
└── finder_conformance_test.go    # Conformance tests for all finders
```

**Structure Decision**: Single Go project with frozendb package containing all finder implementations

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
