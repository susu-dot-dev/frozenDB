# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

The 004 struct validation and immutability feature standardizes validation patterns across all frozenDB structs that can have invalid states. This implements consistent Validate() methods, constructor integration, and field immutability through unexported fields with getter functions. The feature ensures data integrity by preventing invalid struct state and post-construction modification.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Go standard library only (os, encoding/json, sync, etc.)  
**Storage**: Single-file append-only database (.fdb extension)  
**Testing**: go test with spec testing (Test_S_004_* functions)  
**Target Platform**: Linux server  
**Project Type**: Single-project Go package  
**Performance Goals**: Fixed memory usage, validation overhead < 1ms per struct  
**Constraints**: No external dependencies, must maintain data integrity guarantees  
**Scale/Scope**: 9 core structs requiring validation patterns  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Field immutability through unexported fields prevents post-construction modification
- [x] **Data Integrity**: Struct validation ensures only valid state enters database operations
- [x] **Correctness Over Performance**: Validation prioritizes correctness, minimal performance impact
- [x] **Chronological Ordering**: Header validation enforces skewMs for proper time ordering
- [x] **Concurrent Read-Write Safety**: Immutable structs enable safe concurrent read access
- [x] **Single-File Architecture**: Validation supports single-file database format
- [x] **Spec Test Compliance**: All FR-001 through FR-014 requirements have spec tests planned in appropriate *_spec_test.go files

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
├── frozendb.go           # FrozenDB struct (needs Validate())
├── create.go              # CreateConfig struct (standardize validation)
├── open.go                # Header parsing (needs NewHeader constructor)
├── row.go                 # Row structs (StartControl, EndControl, baseRow)
├── checksum.go            # ChecksumRow and Checksum structs (standardize)
└── errors.go              # Error types (already compliant)

tests/
├── frozendb_spec_test.go  # FR-001 through FR-014 spec tests
├── create_spec_test.go     # CreateConfig validation tests
├── row_spec_test.go        # Row control validation tests
└── checksum_spec_test.go    # ChecksumRow validation tests
```

**Structure Decision**: Single Go package with struct-specific files. Each struct's Validate() method implementation goes in its source file, with corresponding spec tests in *_spec_test.go files.

## Complexity Tracking

No constitutional violations requiring justification. All design decisions comply with frozenDB constitutional principles.

| Decision | Constitutional Alignment |
|----------|-------------------------|
| Field immutability (unexported + getters) | Immutability First - prevents post-construction modification |
| Struct validation with Validate() methods | Data Integrity - ensures only valid state enters operations |
| Constructor pattern integration | Correctness Over Performance - validation prioritized over micro-optimizations |
| Parent-child validation responsibilities | Single-File Architecture - maintains clean validation boundaries |
| Spec test coverage for all FR requirements | Spec Test Compliance - all functional requirements validated |
