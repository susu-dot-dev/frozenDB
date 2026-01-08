# Implementation Plan: Database File Creation

**Branch**: `001-create-db` | **Date**: 2025-01-08 | **Spec**: [Database File Creation](./spec.md)
**Input**: Feature specification from `/specs/001-create-db/spec.md`

## Summary

Implement the `Create(config CreateConfig) error` function in `frozendb/create.go` that creates new frozenDB database files with atomic operations, sudo context validation, append-only protection, and proper ownership handling. The implementation must use Linux-specific system calls for file attributes and maintain constitutional requirements of immutability, data integrity, and correctness.

## Technical Context

**Language/Version**: Go 1.21+  
**Primary Dependencies**: Go standard library (os, syscall, uuid), no external dependencies  
**Storage**: Single file database with frozenDB v1 format (immutable, append-only)  
**Testing**: Go testing package with table-driven tests, mocking for syscalls  
**Target Platform**: Linux filesystems that support chattr +a (append-only) attribute  
**Project Type**: Single project with public API package  
**Performance Goals**: Fixed memory usage regardless of parameters, minimized disk operations  
**Constraints**: Must use O_CREAT|O_EXCL flags, single atomic file creation, fdatasync() before attribute setting  
**Scale/Scope**: Single file creation operation with configurable row size (128-65536) and time skew (0-86400000ms)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with chattr +a protection, no delete/modify operations
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection, atomic file creation with O_CREAT|O_EXCL
- [x] **Correctness Over Performance**: Prioritizes atomic operations and proper error handling over speed optimizations
- [x] **Chronological Ordering**: Design supports UUIDv7 time-based key ordering with configurable time skew
- [x] **Concurrent Read-Write Safety**: O_CREAT|O_EXCL prevents race conditions, thread-safe function design
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery

## Project Structure

### Documentation (this feature)

```text
.specify/features/001-create-db/
├── plan.md              # This file (/speckit.plan command output) ✅ COMPLETE
├── research.md          # Phase 0 output (/speckit.plan command) ✅ COMPLETE
├── data-model.md        # Phase 1 output (/speckit.plan command) ✅ COMPLETE
├── quickstart.md        # Phase 1 output (/speckit.plan command) ✅ COMPLETE
├── contracts/           # Phase 1 output (/speckit.plan command) ✅ COMPLETE
│   └── api-contract.md  # Complete API and internal contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command - NEXT STEP)
```

### Source Code (repository root)

```text
frozendb/
├── errors.go              # FrozenDBError base struct and specific error types (reused across project)
├── create.go              # Public Create function + CreateConfig struct + all private helper functions
├── create_test.go          # Unit tests for Create function
└── spec_tests/            # Spec tests (constitutional requirement)
    ├── 0001_create_db_test.go     # Spec tests for FR-001 through FR-035
```

**Structure Decision**: Single project structure with public API package at top-level `frozendb/` following Go conventions. Internal implementations are private within the package using lowercase functions, and spec tests are in `frozendb/spec_tests/` per constitutional requirements.

## Spec Testing Strategy

Per frozenDB Constitution, all functional requirements (FR-001 through FR-032) MUST have corresponding spec tests in `frozendb/spec_tests/0001_create_db_test.go` following pattern `TestFR_XXX_Description()`. Spec tests are distinct from unit tests and validate functional requirements from user/system perspective.

Functional requirements are NOT considered implemented until:
1. Implementation code exists and compiles
2. All corresponding spec tests pass
3. No existing spec tests are broken
4. Success criteria are met

**Strict Compliance** (per docs/spec_testing.md):
- Existing spec tests MUST NOT be modified after implementation
- Previous spec test files MUST NOT be edited to accommodate new implementations
- Spec test requirements MUST NOT be changed without user permission
- Any breaking changes require explicit user approval and specification updates

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Linux-specific syscalls | Append-only attribute requires chattr +a which is Linux-only | Cross-platform approaches would require subprocess calls and lose atomicity guarantees |
| Sudo context validation | Required for chattr +a privileges and proper ownership handling | Running as root would create files owned by root, inaccessible to original user |

## Plan Completion Status

✅ **Testing Strategy Deferred**: Removed pre-specified testutils - mocking approach determined during task planning
✅ **Test Structure Fixed**: Spec tests moved to frozendb/spec_tests/ per constitutional requirements
✅ **All Paths Corrected**: Updated plan to use top-level frozendb/ module (not pkg/frozendb/)
✅ **Documentation Paths Fixed**: Updated from specs/ to .specify/features/ for correct file locations
✅ **Removed Redundant Sections**: Simplified spec testing strategy section
✅ **Removed Unnecessary Test Directories**: Eliminated test/unit/, test/integration/ since spec tests are primary focus
✅ **Removed internal/ Directory**: Private functions go directly in create.go with lowercase names
✅ **Removed common_test_helpers.go**: Only constitutionally required test_helpers.go remains
✅ **Updated Quickstart Import Paths**: Fixed import to use actual GitHub repository (susu-dot-dev/frozenDB/frozendb)
✅ **Updated Template Guidance**: Added git remote instructions to .specify/templates/plan-template.md for future reference
✅ **Improved API Design**: Changed from positional parameters to CreateConfig struct for clarity and extensibility
✅ **Simplified API Contract**: Removed over-specified validation functions and sudo context complexity
✅ **Final API Structure**: Clean, constitutionally compliant, and ready for implementation
✅ **Simplified API Contract**: Removed over-specification and focused on core functionality
✅ **Updated All Documentation**: Consistent CreateConfig API across plan, data-model, contracts, and quickstart
| Direct syscall usage | FS_IOC_SETFLAGS ioctl requires direct syscall for atomic attribute setting | Subprocess calls to chattr would introduce race conditions and cleanup complexity |
