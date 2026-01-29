# Implementation Plan: Release Scripts & Version Management

**Branch**: `033-release-scripts` | **Date**: 2026-01-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/033-release-scripts/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

This feature adds version management and automated release infrastructure to frozenDB. It provides: (1) a script to bump version numbers in go.mod and a generated version.go file, create release branches, and push to remote; (2) a CLI version command that displays the embedded version; (3) GitHub Actions workflows to automatically build and publish multi-platform binaries when releases are created.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Standard library, github.com/google/uuid (existing)  
**Storage**: Version information stored in go.mod and generated version.go file  
**Testing**: go test with spec tests following docs/spec_testing.md conventions  
**Target Platform**: CLI scripts (Bash for maintainers on Unix-like systems), GitHub Actions (Linux runners), CLI binaries (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64)  
**Project Type**: Infrastructure tooling - scripts, GitHub Actions workflows, and CLI commands  
**Performance Goals**: Version bump script completes <2 minutes, version command <100ms, GitHub Actions builds <10 minutes  
**Constraints**: Script must be idempotent, no external dependencies for scripts, GitHub Actions standard runner limits  
**Scale/Scope**: Single repository, 1-2 scripts, 1 GitHub Actions workflow, 1 CLI subcommand  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Note**: This is an infrastructure/tooling feature (release scripts, version management, CI/CD). Most frozenDB constitution principles relate to database operations and do not apply to this feature. Constitutional compliance is evaluated for applicable principles only.

- [N/A] **Immutability First**: Not applicable - feature does not involve database data operations
- [N/A] **Data Integrity**: Not applicable - feature does not write to database files
- [✓] **Correctness Over Performance**: Version bump script and CLI version command prioritize correctness; performance goals are well within acceptable ranges
- [N/A] **Chronological Ordering**: Not applicable - feature does not involve key ordering or search
- [N/A] **Concurrent Read-Write Safety**: Not applicable - feature does not involve concurrent database access
- [N/A] **Single-File Architecture**: Not applicable - feature works with repository files (go.mod, version.go) not database files
- [✓] **Spec Test Compliance**: All functional requirements FR-001 through FR-016 will have corresponding spec tests. FR-010 through FR-014 (GitHub Actions workflows) will use t.Skip() with comment "GitHub Actions workflows are manually tested" as specified in the spec

**Initial Evaluation** (before Phase 0): ✅ PASSED

**Re-evaluation** (after Phase 1 design): ✅ PASSED - Design confirms constitutional compliance:
- Correctness prioritized in all components (validated version format, atomic git operations, clear error messages)
- Spec tests planned for all testable requirements (scripts, CLI command)
- GitHub Actions workflow requirements documented with t.Skip() as specified
- No database operations involved, so database-related principles remain N/A

## Project Structure

### Documentation (this feature)

```text
specs/033-release-scripts/
├── spec.md              # Feature specification
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md           # CLI and script API specifications
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Infrastructure and tooling additions
scripts/
└── bump-version.sh      # Version bump script (Bash)

cmd/frozendb/
├── main.go              # Add version command handler
├── version.go           # Generated version constant (created by bump-version script)
└── main_spec_test.go    # Spec tests for CLI version command (FR-006, FR-007, FR-008)

.github/workflows/
├── ci.yml               # Existing CI workflow
└── release.yml          # New: Build and publish release binaries (FR-010 through FR-014)

scripts_spec_test/       # Spec tests for version bump script functionality
└── bump_version_spec_test.go  # Spec tests for FR-001 through FR-005, FR-009, FR-015, FR-016

# Modified files
go.mod                   # Updated by bump-version script with new version
Makefile                 # Optional: Add release-related targets
```

**Structure Decision**: This feature adds infrastructure tooling alongside existing code. Scripts are placed in a new `/scripts` directory for maintainer tools. The version.go file is generated in cmd/frozendb/ where the CLI lives. GitHub Actions workflow is added to `.github/workflows/`. Spec tests for scripts are placed in a dedicated `scripts_spec_test/` directory at repo root since they test script behavior rather than Go package code.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

N/A - No constitutional violations. All applicable principles are satisfied.
