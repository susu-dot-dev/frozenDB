# Implementation Plan: Linux-Only Platform Restriction

**Branch**: `035-linux-only` | **Date**: 2026-01-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/035-linux-only/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Restrict frozenDB to Linux-only platform support by removing macOS build targets from GitHub Actions release workflow, updating S_033 specifications to remove macOS requirements (FR-011), and updating all documentation references to indicate Linux-only support. This infrastructure change simplifies the codebase and aligns with the use of Linux-specific syscalls.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Go standard library, github.com/google/uuid  
**Storage**: Single-file append-only database format  
**Testing**: Go testing framework (go test), spec tests in *_spec_test.go files  
**Target Platform**: Linux (amd64, arm64) - **CHANGED FROM**: Linux/macOS cross-platform  
**Project Type**: Single project with CLI and library components  
**Performance Goals**: Release workflow completes in <5 minutes (reduced from 10 minutes)  
**Constraints**: Linux-only builds, no cross-platform compatibility testing required  
**Scale/Scope**: Infrastructure change affecting 1 GitHub Actions workflow, 1 spec file (S_033), multiple documentation files, and 1-3 spec test files  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Initial Check (Pre-Phase 0): ✅ PASSED

- [x] **Immutability First**: ✓ N/A - Infrastructure/documentation change, no data operations affected
- [x] **Data Integrity**: ✓ N/A - No transaction or data structure changes
- [x] **Correctness Over Performance**: ✓ N/A - No performance optimizations, only reducing build matrix
- [x] **Chronological Ordering**: ✓ N/A - No key ordering or search changes
- [x] **Concurrent Read-Write Safety**: ✓ N/A - No concurrent operation changes
- [x] **Single-File Architecture**: ✓ N/A - No database file format changes
- [x] **Spec Test Compliance**: ✓ All 5 functional requirements will have corresponding spec tests in cmd/frozendb/cli_spec_test.go following Test_S_035_FR_XXX naming pattern. All tests will use t.Skip() as they validate infrastructure/documentation changes (per spec exceptions).

### Post-Phase 1 Re-check: ✅ PASSED

**Design Review**: Phase 0 research and Phase 1 design (research.md, data-model.md, contracts/api.md) confirm:

- [x] **Immutability First**: ✅ Confirmed - No changes to append-only architecture or data operations
- [x] **Data Integrity**: ✅ Confirmed - No changes to transaction headers, sentinel bytes, or corruption detection
- [x] **Correctness Over Performance**: ✅ Confirmed - Build time reduction (10min → 5min) is from reduced platform matrix, not optimization trade-offs
- [x] **Chronological Ordering**: ✅ Confirmed - No changes to UUIDv7 key ordering or time-based search
- [x] **Concurrent Read-Write Safety**: ✅ Confirmed - No changes to read/write concurrency model
- [x] **Single-File Architecture**: ✅ Confirmed - No changes to single-file database design
- [x] **Spec Test Compliance**: ✅ Confirmed - contracts/api.md defines 5 spec tests (Test_S_035_FR_001 through Test_S_035_FR_005) in cmd/frozendb/cli_spec_test.go, all properly skipped per spec exceptions. Test_S_033_FR_011_WorkflowBuildsMacOSBinaries will be removed per FR-005.

**Conclusion**: This is a pure infrastructure/documentation change with zero impact on frozenDB core principles. All constitution requirements remain satisfied.

## Project Structure

### Documentation (this feature)

```text
specs/035-linux-only/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Single project structure (frozenDB standard layout)

.github/
└── workflows/
    └── release.yml      # MODIFIED: Remove darwin builds, keep only linux/amd64 and linux/arm64

cmd/
└── frozendb/
    └── cli_spec_test.go # MODIFIED: Add Test_S_035_FR_001 through Test_S_035_FR_005 (all skipped)

specs/
├── 033-release-scripts/
│   ├── spec.md          # MODIFIED: Remove/mark obsolete FR-011 (macOS builds)
│   ├── research.md      # MODIFIED: Update platform references to Linux-only
│   ├── plan.md          # MODIFIED: Update target platform to Linux-only
│   ├── data-model.md    # MODIFIED: Update platform matrix to Linux-only
│   └── contracts/
│       └── api.md       # MODIFIED: Remove darwin build targets from examples
└── 034-frozendb-verify/
    ├── spec.md          # MODIFIED: Update cross-platform references to Linux-only (if applicable)
    └── plan.md          # MODIFIED: Update target platform to Linux-only (if applicable)

AGENTS.md                # REVIEWED: No changes needed (already mentions Linux/syscalls)
README.md                # REVIEWED: Check for platform references (if exists)
```

**Structure Decision**: This is a documentation and configuration change affecting existing files. No new source code files are created. Changes are limited to:
1. GitHub Actions workflow configuration (`.github/workflows/release.yml`)
2. Spec documentation files (S_033 and related specs)
3. Spec test file (adding 5 new skipped tests to `cmd/frozendb/cli_spec_test.go`)

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No constitution violations. This is a pure infrastructure and documentation change with no impact on frozenDB core principles.

---

## Phase 0 & Phase 1 Completion Summary

### Artifacts Generated

**Phase 0 - Research** ✅
- `research.md`: Comprehensive inventory of 14 files requiring updates, organized by priority (critical, S_033 supporting, other specs, scripts)
- Identified update patterns for workflow matrix, spec requirements, target platforms, and platform lists
- Documented testing strategy with 5 new spec test functions (all skipped per exceptions)

**Phase 1 - Design & Contracts** ✅
- `data-model.md`: Defined 4 entity types (Platform Matrix Entry, Functional Requirement Entity, Spec Test Function, Platform Reference Entity) with validation rules and state transitions
- `contracts/api.md`: Specified 8 contract sections covering workflow matrix API, requirement obsolescence API, 5 spec test function APIs, documentation update API, success validation API, integration notes, performance characteristics, and testing approach

### Key Decisions

1. **FR-011 Obsolescence Approach**: Mark with strikethrough + "OBSOLETE" note referencing S_035 (maintains traceability vs. complete removal)
2. **Platform Reference Format**: Standardized to "Linux (amd64, arm64)" across all documentation
3. **Spec Test Strategy**: All 5 tests use `t.Skip()` per spec exceptions (infrastructure/documentation validation is manual)
4. **Update Scope**: 14 files total - 1 workflow, 7 S_033 files, 5 other spec files, 1 scripts doc
5. **Test Removal**: `Test_S_033_FR_011_WorkflowBuildsMacOSBinaries` completely removed (not converted to skip)

### Constitution Compliance

✅ **All Checks Passed** - No violations in initial or post-Phase 1 review. This is a pure infrastructure change with zero impact on frozenDB's core database principles.

### Next Steps

Run `/speckit.tasks` to generate implementation task breakdown (Phase 2).
