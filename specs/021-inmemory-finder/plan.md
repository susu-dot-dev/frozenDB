# Implementation Plan: InMemoryFinder

**Branch**: `021-inmemory-finder` | **Date**: 2025-01-24 | **Spec**: /home/anil/code/frozenDB/specs/021-inmemory-finder/spec.md
**Input**: Feature specification from `/specs/021-inmemory-finder/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Design and implement an InMemoryFinder that maintains complete UUID->index mappings in memory for O(1) lookup operations, allowing users to choose between memory-efficient (SimpleFinder) and high-performance (InMemoryFinder) strategies when creating FrozenDB instances.

## Phase 1 Completion Summary

**Completed Artifacts**:
- ✅ `research.md` - All technical unknowns resolved (UUIDv7 integration, memory management, thread safety)
- ✅ `data-model.md` - Complete entity definitions and validation rules for InMemoryFinder
- ✅ `contracts/api.md` - Full API specification with method signatures and usage patterns  
- ✅ Agent context updated (script not available - manual update recommended)

**Key Design Decisions**:
- Use Go built-in maps with pre-allocation for ~40 bytes/row memory usage
- RWMutex for concurrent read access with exclusive write updates
- Implementation determines strategy-to-implementation mapping (no factory over-specification)
- Replace NewFrozenDB signature to require three parameters: filename, mode, strategy (approved breaking change)

**Ready for Phase 2**: Technical approach fully specified with clear implementation path.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single file append-only format (existing frozenDB format)  
**Testing**: Go testing with table-driven tests and benchmarks  
**Target Platform**: Linux/Unix systems  
**Project Type**: Single Go package/library  
**Performance Goals**: O(1) GetIndex, GetTransactionStart, GetTransactionEnd operations for databases up to 100k rows  
**Constraints**: <1ms operation latency, memory usage scales with database size (documented trade-off)  
**Scale/Scope**: Databases small enough to fit index in memory (up to ~100k rows depending on row size)  

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

**Post-Phase 1 Re-evaluation**: ✅ All constitutional requirements maintained. InMemoryFinder design preserves all frozenDB principles while adding performance optimization option.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md         # API specification
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/                      # Core database package
├── finder.go                  # Finder interface definition (existing)
├── simple_finder.go           # SimpleFinder implementation (existing)
├── inmemory_finder.go         # NEW: InMemoryFinder implementation
├── finder_test.go             # Unit tests for finder implementations
├── finder_conformance_test.go # Conformance tests for all finders (existing)
├── frozendb.go                # Main FrozenDB struct and NewFrozenDB function (modify)
├── errors.go                  # Error definitions (may need extensions)
└── [existing files...]        # Other existing frozendb files
```

**Structure Decision**: Single Go package structure with new InMemoryFinder implementation in frozendb/ package alongside existing SimpleFinder. NewFinder function will be added to allow strategy selection during FrozenDB creation.

## Document Content Guidelines

### research.md (Phase 0 Output)
**Purpose**: Research findings that resolve technical unknowns from the specification.

**What to Include**:
- Analysis of existing codebase patterns and protocols
- Research on external libraries or technologies
- Decision rationale for technical choices with alternatives considered
- Existing function usage patterns and integration approaches
- Performance and constraint analysis from current architecture

**What to Exclude**:
- Prescriptive code examples for new functions that don't exist yet
- API specifications or method signatures (go in api.md)
- Implementation details that limit implementation flexibility
- Redundant documentation of existing codebase structure

### data-model.md (Phase 1 Output)
**Purpose**: New data entities, validation rules, and state changes introduced by the feature.

**What to Include**:
- New entity definitions and attributes
- Changes to existing data structures or relationships
- New validation rules specific to the feature
- State transitions and flow logic for new operations
- Error condition mappings for new error types
- Data flow relationships between components

**What to Exclude**:
- API specifications, method signatures, or implementation details
- Error handling patterns or usage examples (go in api.md)
- Existing codebase documentation or redundant information
- General project structure or integration details

### contracts/api.md (Phase 1 Output)
**Purpose**: Complete API specification for the feature.

**What to Include**:
- Method signatures and parameter descriptions
- Return values and error conditions
- Success/failure behavior documentation
- Basic usage examples without complex error handling
- Performance characteristics and thread safety information
- Integration notes and compatibility details

**What to Exclude**:
- Complex error handling patterns or implementation guidance
- Internal data model details or state transitions
- Redundant codebase documentation
- Prescriptive implementation details

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
