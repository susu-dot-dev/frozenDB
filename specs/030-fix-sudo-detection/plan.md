# Implementation Plan: Bug Fix - Sudo Detection Logic

**Branch**: `030-fix-sudo-detection` | **Date**: Thu Jan 29 2026 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/030-fix-sudo-detection/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Fix the sudo detection logic in `internal/frozendb/create.go` to check for SUDO_USER environment variable BEFORE checking if UID is root. Currently, the code rejects all root UID processes, including valid `sudo frozendb create` commands. The fix will reorder the checks to allow sudo execution while still blocking direct root execution.

## Technical Context

**Language/Version**: Go 1.25.6  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single append-only file format (.fdb)  
**Testing**: Go testing (go test), table-driven tests, spec tests in *_spec_test.go files  
**Target Platform**: Linux (requires root/sudo for append-only attribute setting)  
**Project Type**: Single project (CLI + library)  
**Performance Goals**: Fixed memory usage (independent of database size), O(1) seeking for binary search  
**Constraints**: Must not modify existing spec tests without permission, must maintain append-only immutability  
**Scale/Scope**: Bug fix affecting Create operation in internal/frozendb/create.go  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

**Affected Files**:
- `internal/frozendb/create.go` (lines 270-284) - Bug location: UID check before SUDO_USER check
- `internal/frozendb/create_spec_test.go` - Existing spec tests that must not be modified (Spec 001)
- New spec tests will be added for Spec 030 functional requirements

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Not applicable - bug fix does not affect data operations or append-only behavior
- [x] **Data Integrity**: Not applicable - bug fix does not affect write operations or data integrity
- [x] **Correctness Over Performance**: Bug fix prioritizes correctness (proper sudo detection) over any performance considerations
- [x] **Chronological Ordering**: Not applicable - bug fix does not affect key ordering or time-based operations
- [x] **Concurrent Read-Write Safety**: Not applicable - bug fix does not affect concurrent operations
- [x] **Single-File Architecture**: Not applicable - bug fix does not affect database file architecture
- [x] **Spec Test Compliance**: All functional requirements (FR-001, FR-002) will have corresponding spec tests in create_spec_test.go following Test_S_030_FR_XXX naming convention

## Project Structure

### Documentation (this feature)

```text
specs/030-fix-sudo-detection/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/frozendb/
├── create.go                 # Implementation - Bug location (lines 270-284)
├── create_test.go            # Unit tests
├── create_spec_test.go       # Spec tests (existing Spec 001 + new Spec 030)
├── errors.go                 # Error types (WriteError, PathError, InvalidInputError)
└── [other implementation files]

docs/
├── spec_testing.md           # Spec testing guidelines
├── v1_file_format.md         # File format specification
└── error_handling.md         # Error handling guidelines
```

**Structure Decision**: This is a single-project Go codebase with internal packages. The bug fix affects only the `internal/frozendb/create.go` file. Spec tests will be added to the existing `create_spec_test.go` file following the Test_S_030_FR_XXX naming convention. No unit tests from `create_test.go` will be modified.

## Complexity Tracking

> **No violations - this section is empty**

This bug fix does not introduce any complexity that violates frozenDB Constitution principles. The fix is a simple reordering of existing checks to correct the logic flow.
