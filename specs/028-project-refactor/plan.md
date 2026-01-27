# Implementation Plan: Project Structure Refactor

**Branch**: `028-project-refactor` | **Date**: 2026-01-27 | **Spec**: `/specs/028-project-refactor/spec.md`
**Input**: Feature specification from `/specs/028-project-refactor/spec.md`

## Summary

Refactor frozenDB project structure to separate public API from internal implementation details. The current architecture exposes all file format structures in the public frozendb package. This refactor moves to a standard Go project layout with `/cmd`, `/pkg/frozendb` (public API only), `/internal` (implementation details), and `/examples` directories.

**User Issue Identified**: "Why is file.go, header.go, row.go, control.go, uuid.go all in the public pkg files? The file structure is an internal detail. Only things derivable from the current top-level frozendb.go struct should be visible"

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single-file append-only database format (.fdb files)  
**Testing**: Go testing with spec tests (Test_S_* prefix) and unit tests  
**Target Platform**: Linux/Unix systems (file locking with flock)  
**Project Type**: Single Go library with CLI tool  
**Constraints**: Append-only immutability, concurrent read-write safety, single-file architecture  
**Scale/Scope**: Currently 49 Go files in frozendb/ package, needs reorganization into public/internal split  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Refactor only changes code organization, not data operations
- [x] **Data Integrity**: No changes to transaction headers or sentinel bytes
- [x] **Correctness Over Performance**: No performance changes - only moving code between packages
- [x] **Chronological Ordering**: No changes to key ordering or time handling
- [x] **Concurrent Read-Write Safety**: No changes to concurrency model
- [x] **Single-File Architecture**: No changes to file format
- [x] **Spec Test Compliance**: All existing spec tests must pass unchanged after refactor

## Target Directory Structure

```text
frozenDB/                           # Repository root
├── cmd/                            # Command-line tools
│   └── frozendb/                   # CLI tool
│       └── main.go                 # Hello world CLI
│
├── pkg/                            # Public libraries for external use
│   └── frozendb/                   # Public API package
│       ├── frozendb.go             # FrozenDB struct, NewFrozenDB, Get, BeginTx, Close
│       ├── transaction.go          # Transaction struct, methods
│       ├── errors.go               # All public error types
│       ├── create.go               # Create function, CreateConfig
│       ├── data_row.go             # DataRow struct
│       ├── header.go               # Header struct
│       ├── constants.go            # StartControl, EndControl types + all public constants
│       └── *_spec_test.go          # Spec tests (co-located)
│
├── internal/                       # Private implementation details
│   ├── fields/                     # Row field structures and serialization
│   │   ├── row.go                  # baseRow[T], sentinel constants
│   │   ├── data_row.go             # DataRow implementation details
│   │   ├── null_row.go             # NullRow struct
│   │   ├── checksum.go             # ChecksumRow struct
│   │   ├── partial_data_row.go     # PartialDataRow struct
│   │   ├── row_union.go            # RowUnion for polymorphic parsing
│   │   └── uuid_helpers.go         # UUIDv7 validation helpers
│   │
│   ├── finder/                     # Query implementations
│   │   ├── finder.go               # Finder interface (internal)
│   │   ├── simple_finder.go        # SimpleFinder
│   │   ├── inmemory_finder.go      # InMemoryFinder
│   │   ├── binary_search_finder.go # BinarySearchFinder
│   │   ├── fuzzy_binary_search.go  # Helper function
│   │   └── *_spec_test.go          # Spec tests
│   │
│   └── fileio/                     # File I/O and management
│       ├── file_manager.go         # DBFile interface, FileManager
│       └── *_spec_test.go          # Spec tests
│
├── examples/                       # Example applications
│   └── hello-world/                # Basic usage example
│       └── main.go
│
├── docs/                           # Documentation (unchanged)
├── specs/                          # Feature specifications (unchanged)
└── Makefile                        # Build targets updated for new structure
```

## File Migration Mapping

### Files Moving to pkg/frozendb/ (Public API - 8 files)

- `frozendb.go` ← `frozendb/frozendb.go`
- `transaction.go` ← `frozendb/transaction.go`
- `errors.go` ← `frozendb/errors.go`
- `create.go` ← `frozendb/create.go` (public parts only)
- `data_row.go` ← `frozendb/data_row.go` (public struct only)
- `header.go` ← `frozendb/header.go` (public struct only)
- `constants.go` ← Extract StartControl/EndControl types and all public constants from `frozendb/row.go` and `frozendb/finder.go`

### Files Moving to internal/fields/ (Row Structures)

- `row.go` ← `frozendb/row.go` (baseRow[T], sentinel constants, internal parts)
- `data_row.go` ← `frozendb/data_row.go` (internal payload implementation)
- `null_row.go` ← `frozendb/null_row.go`
- `checksum.go` ← `frozendb/checksum.go`
- `partial_data_row.go` ← `frozendb/partial_data_row.go`
- `row_union.go` ← `frozendb/row_union.go`
- `uuid_helpers.go` ← `frozendb/uuid_helpers.go`

### Files Moving to internal/finder/ (Query Implementations)

- `finder.go` ← `frozendb/finder.go` (Finder interface only, FinderStrategy constants move to pkg)
- `simple_finder.go` ← `frozendb/simple_finder.go`
- `inmemory_finder.go` ← `frozendb/inmemory_finder.go`
- `binary_search_finder.go` ← `frozendb/binary_search_finder.go`
- `fuzzy_binary_search.go` ← `frozendb/fuzzy_binary_search.go`

### Files Moving to internal/fileio/ (File I/O)

- `file_manager.go` ← `frozendb/file_manager.go`
- `open.go` ← `frozendb/open.go` (validation helpers)

### Test Files

Test files move with their corresponding implementation files, maintaining co-location.

## Import Path Updates

**Before**: `github.com/susu-dot-dev/frozenDB/frozendb`  
**After (external)**: `github.com/susu-dot-dev/frozenDB/pkg/frozendb`  
**After (internal)**: `github.com/susu-dot-dev/frozenDB/internal/{fields,finder,fileio}`

## Build System Updates

Update Makefile to:
- Build `cmd/frozendb` CLI tool
- Build `examples/` applications
- Run tests with new import paths
- Validate all components build successfully

## Migration Phases

1. **Phase 1**: Create new directory structure (`pkg/`, `internal/`, `cmd/`, `examples/`)
2. **Phase 2**: Move public API files to `pkg/frozendb/`
3. **Phase 3**: Move internal files to `internal/{fields,finder,fileio}/`
4. **Phase 4**: Update all import paths throughout codebase
5. **Phase 5**: Update build system (Makefile) and validate

**Validation**: Each phase must maintain working codebase with all tests passing.

## Key Constraints

- `StartControl` and `EndControl` types must be public (fields in `DataRow`)
- All public constants go in `constants.go` along with StartControl/EndControl types
- Sentinel byte constants (`ROW_START`, `ROW_END`) are internal
- Finder interface is internal; only `FinderStrategy` constants are public
- All error types must remain public for `errors.As()` checking
