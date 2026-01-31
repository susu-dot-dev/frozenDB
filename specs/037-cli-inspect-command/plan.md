# Implementation Plan: CLI Inspect Command

**Branch**: `037-cli-inspect-command` | **Date**: 2026-01-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/037-cli-inspect-command/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Add an `inspect` command to the frozenDB CLI that reads and displays database contents in a human-readable, tab-separated table format. The command accepts `--path`, optional `--offset`, `--limit`, and `--print-header` flags. It displays all row types (Data, NullRow, Checksum, partial, error) with their transaction control information. Uses streaming row-by-row reading for constant memory usage regardless of database size.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single-file append-only frozenDB format (v1_file_format.md)  
**Testing**: Go testing framework, spec tests in cmd/frozendb/cli_spec_test.go  
**Target Platform**: Linux (cross-platform CLI)  
**Project Type**: Single project (CLI application)  
**Performance Goals**: Streaming read-print-read approach for constant memory usage  
**Constraints**: Memory usage must not scale with database size; supports arbitrarily large database files  
**Scale/Scope**: Must handle databases with millions of rows efficiently

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: ✅ Inspect is read-only operation, no delete/modify operations
- [x] **Data Integrity**: ✅ Inspect command validates and displays parity information, reports corruption as error rows
- [x] **Correctness Over Performance**: ✅ Streaming approach prioritizes correctness; each row validated before display
- [x] **Chronological Ordering**: ✅ Read-only operation, preserves existing time-based key ordering
- [x] **Concurrent Read-Write Safety**: ✅ Opens database in read-only mode, safe for concurrent access
- [x] **Single-File Architecture**: ✅ Operates on single database file via --path flag
- [x] **Spec Test Compliance**: ✅ All 22 functional requirements will have spec tests in cmd/frozendb/cli_spec_test.go

## Project Structure

### Documentation (this feature)

```text
specs/037-cli-inspect-command/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # API specification for inspect command
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/frozendb/                    # CLI application entry point
├── main.go                      # CLI routing and command handlers
├── errors.go                    # CLI error formatting
├── version.go                   # Version constant
└── cli_spec_test.go            # Spec tests for all CLI commands (including inspect)

pkg/frozendb/                    # Public API package
├── frozendb.go                  # Main database interface
├── finder.go                    # Finder interface and strategy types
├── transaction.go               # Transaction API
├── errors.go                    # Public error types
└── public_api_spec_test.go     # Public API spec tests

internal/frozendb/               # Internal implementation package
├── frozendb.go                  # Internal database implementation
├── create.go                    # Database creation
├── open.go                      # Database opening
├── file_manager.go              # File I/O operations
├── header.go                    # Header parsing/serialization
├── row.go                       # Row parsing base functionality
├── data_row.go                  # DataRow implementation
├── null_row.go                  # NullRow implementation
├── partial_data_row.go          # PartialDataRow implementation
├── checksum.go                  # Checksum row implementation
├── transaction.go               # Transaction implementation
├── finder.go                    # Finder interface and base
├── simple_finder.go             # SimpleFinder implementation
├── binary_search_finder.go      # BinarySearchFinder implementation
├── fuzzy_binary_search.go       # Fuzzy binary search algorithm
├── inmemory_finder.go           # InMemoryFinder implementation
├── verify.go                    # Database verification functions
├── row_union.go                 # Row type union for parsing
├── uuid_helpers.go              # UUID utility functions
└── errors.go                    # Internal error types

docs/                            # Documentation
├── v1_file_format.md           # Complete file format specification
├── error_handling.md           # Error handling guidelines
└── spec_testing.md             # Spec testing requirements
```

**Structure Decision**: This is a single-project structure with clear separation between CLI (cmd/frozendb), public API (pkg/frozendb), and internal implementation (internal/frozendb). The inspect command will be implemented in cmd/frozendb/main.go with spec tests in cmd/frozendb/cli_spec_test.go, following the existing pattern for CLI commands.

## Complexity Tracking

No constitutional violations. This feature is a read-only inspection command that aligns with all frozenDB principles.
