# Implementation Plan: CLI Flag Improvements

**Branch**: `032-cli-flag-improvements` | **Date**: Thu Jan 29 2026 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/032-cli-flag-improvements/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

This feature enhances the frozenDB CLI with flexible flag positioning (--path and --finder flags can appear before or after subcommands), adds a NOW keyword for automatic UUIDv7 key generation in the add command, modifies add command output to always display the key, and introduces a --finder flag for selecting finder strategies at runtime (simple, inmemory, binary) with BinarySearchFinder as default.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid v1.6.0, Go standard library (flag, encoding/json, os, fmt)  
**Storage**: Single-file append-only database (frozenDB v1 file format)  
**Testing**: Go standard testing package (spec tests in cmd/frozendb/cli_spec_test.go)  
**Target Platform**: Linux/Unix CLI (cross-platform support via Go)  
**Project Type**: Single project (CLI application)  
**Constraints**: CLI must maintain backward compatibility with existing commands; no changes to file format or database behavior  
**Scale/Scope**: CLI enhancements affecting 7 subcommands (create, begin, commit, savepoint, rollback, add, get); NOW keyword and --finder flag are additive  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB  

**Current CLI Implementation**:
- Uses Go's standard `flag` package (flag.NewFlagSet per subcommand)
- Flags must currently appear AFTER subcommand (fixed position)
- Finder strategy is hardcoded to FinderStrategySimple in all CLI commands
- Add command exits silently on success (no key output)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: CLI improvements do not affect data operations - append-only immutability is preserved
- [x] **Data Integrity**: No changes to transaction handling or data writes; NOW keyword generates valid UUIDv7 keys through existing mechanisms
- [x] **Correctness Over Performance**: Flag parsing changes are purely syntactic; finder strategy selection uses existing validated implementations
- [x] **Chronological Ordering**: NOW keyword leverages UUIDv7 time ordering; no changes to key ordering behavior
- [x] **Concurrent Read-Write Safety**: CLI enhancements are per-command invocation; no impact on concurrent database operations
- [x] **Single-File Architecture**: No changes to file format or database architecture
- [x] **Spec Test Compliance**: All functional requirements (FR-001 through FR-006) require corresponding spec tests in cmd/frozendb/cli_spec_test.go

**Gate Status**: ✅ PASSED - No constitutional violations. This feature is purely CLI/UX improvements with no changes to core database behavior, file format, or data integrity guarantees.

### Post-Phase 1 Re-evaluation

**Date**: Thu Jan 29 2026  
**Status**: ✅ CONFIRMED - All constitutional principles remain satisfied after Phase 1 design completion.

**Key Findings**:
1. Data model (data-model.md) confirms no changes to database file format, row structures, or transaction semantics
2. API contracts (contracts/api.md) validate that finder strategies are runtime-only (not persisted)
3. Validation rules (VR-001 through VR-012) ensure flag parsing errors are caught BEFORE database operations
4. NOW keyword implementation uses validated library function (uuid.NewV7())
5. All finder strategies return identical results per existing Finder interface contract (correctness maintained)

**No new constitutional concerns identified in Phase 1 design.**

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
cmd/frozendb/
├── main.go                 # CLI entry point and command routing - MODIFY for flag parsing
├── cli_spec_test.go        # Spec tests for CLI functionality - ADD new tests for FR-001 through FR-006
└── errors.go               # CLI error handling - NO CHANGES expected

pkg/frozendb/
├── frozendb.go             # Public API wrapper - NO CHANGES (finder strategies already exposed)
└── finder.go               # FinderStrategy constants - NO CHANGES (already defined)

internal/frozendb/
├── frozendb.go             # Core FrozenDB implementation - NO CHANGES (finder selection exists)
└── finder.go               # Finder interface and constants - NO CHANGES

tests/ - N/A for this feature (CLI spec tests in cmd/frozendb/cli_spec_test.go)
```

**Structure Decision**: Single project structure. All changes confined to cmd/frozendb/main.go for CLI parsing logic. The existing finder strategies (FinderStrategySimple, FinderStrategyInMemory, FinderStrategyBinarySearch) are already implemented and exposed through pkg/frozendb - we just need to wire the CLI flag to select between them.

## Complexity Tracking

**No violations** - This feature introduces no constitutional violations requiring justification.
