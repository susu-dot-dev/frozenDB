# Implementation Plan: Project Structure Refactor & CLI

**Branch**: `028-pkg-internal-cli-refactor` | **Date**: 2026-01-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/028-pkg-internal-cli-refactor/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Refactor frozenDB codebase to establish clean architectural boundaries with public API (`/pkg`), internal implementation (`/internal`), CLI entry point (`/cmd`), and examples (`/examples`). The refactor preserves 100% backward compatibility by re-exporting the existing public API through a shim layer in `/pkg/frozendb`, while organizing internal code to prevent circular dependencies. A minimal CLI that outputs "Hello world" validates the new structure.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: `github.com/google/uuid` (required), Go standard library  
**Storage**: Single-file append-only database format (.fdb files)  
**Testing**: Go testing framework (`go test`), table-driven tests, spec tests in `*_spec_test.go` files  
**Target Platform**: Linux (amd64, arm64)  
**Project Type**: Single library project with CLI component  
**Performance Goals**: Fixed memory usage regardless of database size, maintain current build time within 10%  
**Constraints**: Zero breaking changes to existing public API, all existing tests must pass with only import path updates  
**Scale/Scope**: ~50 source files in frozendb/ package, comprehensive test suite with spec tests and unit tests  

**GitHub Repository**: `github.com/susu-dot-dev/frozenDB`

**Current Public API Surface** (must be preserved):
- Types: `FrozenDB`, `Transaction`, `CreateConfig`, `SudoContext`, `FinderStrategy`, `Header`
- Constants: `MODE_READ`, `MODE_WRITE`, `FinderStrategySimple`, `FinderStrategyInMemory`, `FinderStrategyBinarySearch`, `FILE_EXTENSION`, `FILE_PERMISSIONS`
- Functions: `NewFrozenDB()`, `CreateFrozenDB()`, `NewCreateConfig()`
- Error types: All error types in errors.go (`InvalidInputError`, `PathError`, `WriteError`, `CorruptDatabaseError`, `InvalidActionError`, `KeyOrderingError`, `TombstonedError`, `ReadError`, `KeyNotFoundError`, `TransactionActiveError`, `InvalidDataError`)
- Transaction methods: All public methods except `GetEmptyRow()` and `GetRows()` (to be removed)

**Key Refactoring Decisions**:
1. Internal `NullRow` type should NOT be exposed publicly - Transaction.GetEmptyRow() will be removed
2. Transaction.GetRows() will be removed to prevent exposing internal row structures
3. Database creation functions (`CreateFrozenDB`, `CreateConfig`, `NewCreateConfig`) will NOT be in public API - CLI will handle creation in future
4. All current /frozendb code moves to /internal/frozendb with identical structure
5. /pkg/frozendb provides MINIMAL shim re-exports for core operations only: open, transaction, query, close
6. Examples will use internal package for database creation (temporary until CLI supports creation)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: ✓ Refactor does not modify database behavior - only reorganizes code structure
- [x] **Data Integrity**: ✓ No changes to transaction handling or sentinel bytes - pure code movement
- [x] **Correctness Over Performance**: ✓ No performance optimizations, only structural refactor with <10% build time tolerance
- [x] **Chronological Ordering**: ✓ No changes to key ordering logic - preserves existing implementation
- [x] **Concurrent Read-Write Safety**: ✓ No changes to concurrency patterns - preserves existing thread safety
- [x] **Single-File Architecture**: ✓ No changes to database file format or architecture
- [x] **Spec Test Compliance**: ✓ All functional requirements will have spec tests; existing spec tests preserved with import updates only

**Justification**: This is a pure structural refactor with zero behavior changes. All constitutional principles are maintained because the refactor only reorganizes code locations without modifying any database logic, file format, or operational semantics.

## Project Structure

### Documentation (this feature)

```text
specs/028-pkg-internal-cli-refactor/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # Public API specification
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

**BEFORE (Current Structure)**:
```text
frozenDB/
├── frozendb/                    # All implementation code (50+ files)
│   ├── frozendb.go             # Main FrozenDB type
│   ├── transaction.go          # Transaction implementation
│   ├── create.go               # CreateFrozenDB logic
│   ├── errors.go               # Error types
│   ├── *_test.go               # Unit tests
│   ├── *_spec_test.go          # Spec tests
│   └── [all other implementation files]
├── docs/                        # Documentation
├── specs/                       # Feature specifications
├── go.mod
├── Makefile
└── README.md
```

**AFTER (Target Structure)**:
```text
frozenDB/
├── pkg/                         # PUBLIC API - Re-export shim layer
│   └── frozendb/               # Public frozendb package
│       ├── frozendb.go         # Re-exports: FrozenDB, NewFrozenDB, MODE_READ, MODE_WRITE
│       ├── transaction.go      # Re-exports: Transaction (without GetEmptyRow, GetRows)
│       ├── create.go           # Re-exports: CreateFrozenDB, CreateConfig, SudoContext, NewCreateConfig
│       ├── errors.go           # Re-exports: All error types and constructors
│       ├── finder.go           # Re-exports: FinderStrategy constants
│       └── header.go           # Re-exports: Header type
│
├── internal/                    # INTERNAL - Implementation (not importable externally)
│   └── frozendb/               # All internal implementation
│       ├── frozendb.go         # Original implementations
│       ├── transaction.go
│       ├── create.go
│       ├── errors.go
│       ├── finder.go
│       ├── *_test.go           # Unit tests (imports updated)
│       ├── *_spec_test.go      # Spec tests (imports updated)
│       └── [all 50+ original files with identical structure]
│
├── cmd/                         # CLI ENTRY POINTS
│   └── frozendb/               # CLI application
│       └── main.go             # Entry point: outputs "Hello world"
│
├── examples/                    # EXAMPLES - Validate public API
│   └── getting_started/
│       └── main.go             # Example using pkg/frozendb API
│
├── docs/                        # Documentation (unchanged)
├── specs/                       # Feature specifications (unchanged)
├── go.mod                       # Module definition (unchanged)
├── Makefile                     # Build commands (add CLI build target)
├── .gitignore                   # Add /frozendb binary
└── README.md                    # Update with new import paths
```

**Structure Decision**: Using Go's standard project layout with `/pkg` for public API, `/internal` for private implementation, `/cmd` for executables, and `/examples` for usage demonstrations. This follows the widely-adopted Go community conventions as documented in golang-standards/project-layout and used by major projects like Kubernetes, Docker, and HashiCorp tools.

**Key Principles**:
1. **Exact structure preservation in /internal**: The frozendb/ folder moves to internal/frozendb/ with ZERO structural changes - only import paths update
2. **Shim re-export pattern**: /pkg/frozendb files contain only type aliases, const re-declarations, and function forwards
3. **No circular dependencies**: All imports flow from cmd → pkg → internal (never reverse)
4. **Public API cleanup**: Remove GetEmptyRow() and GetRows() from Transaction during shim creation
5. **Validation through examples**: examples/getting_started/ proves the public API is complete and usable

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

**Result**: No violations - constitution check passes completely.

This refactor introduces no complexity violations. All constitutional principles are maintained because the implementation only reorganizes code locations without modifying database logic, file format, or operational semantics.

---

## Post-Design Constitution Re-Check

**Re-evaluation Date**: 2026-01-28 (after Phase 1 completion)

- [x] **Immutability First**: ✓ CONFIRMED - Zero changes to append-only operations or data modification logic
- [x] **Data Integrity**: ✓ CONFIRMED - Zero changes to transaction headers, sentinel bytes, or checksums
- [x] **Correctness Over Performance**: ✓ CONFIRMED - No optimizations introduced; shim layer has zero runtime cost
- [x] **Chronological Ordering**: ✓ CONFIRMED - Zero changes to UUID key ordering or time-based search
- [x] **Concurrent Read-Write Safety**: ✓ CONFIRMED - Zero changes to locking, concurrency, or thread safety
- [x] **Single-File Architecture**: ✓ CONFIRMED - Zero changes to database file format or structure
- [x] **Spec Test Compliance**: ✓ CONFIRMED - All spec tests move to /internal with import updates only; functional requirements FR-001 through FR-007 will have spec tests

**Design Impact Summary**:
- **New Public API Surface**: Shim layer in /pkg/frozendb re-exports existing types
- **API Cleanup**: Removed GetEmptyRow() and GetRows() prevents internal type exposure
- **Import Path Changes**: External users import from /pkg/frozendb instead of /frozendb
- **Test Migration**: All tests move to /internal with zero logic changes
- **CLI Addition**: Minimal main.go with "Hello world" output
- **Example Validation**: getting_started example validates public API completeness

**Constitutional Compliance**: ✅ PASSES - All principles maintained, zero violations

---

## Implementation Artifacts

**Created Documents**:
- ✅ plan.md (this file)
- ✅ research.md (Phase 0 output)
- ✅ data-model.md (Phase 1 output)
- ✅ contracts/api.md (Phase 1 output)

**Next Steps** (Phase 2 - NOT in scope of /speckit.plan command):
- Run `/speckit.tasks` to generate tasks.md with detailed implementation steps
- Implementation proceeds after tasks.md is created and reviewed

**Branch**: `028-pkg-internal-cli-refactor`  
**Status**: Planning Complete - Ready for Phase 2 (Task Generation)
