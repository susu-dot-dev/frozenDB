# Implementation Plan: CLI Implementation

**Branch**: `029-cli-implementation` | **Date**: 2026-01-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/029-cli-implementation/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Implement a command-line interface for frozenDB providing seven commands for database management: `create` for database file initialization, `begin/commit/savepoint/rollback` for transaction management, `add` for inserting key-value pairs, and `get` for data retrieval. The CLI will provide a thin wrapper around the existing frozenDB library, performing minimal validation (UUIDv7 format, JSON syntax) before delegating operations to the library layer. All commands follow Unix conventions: silent success (exit code 0) except for `get` which outputs pretty-printed JSON, and errors printed to stderr with structured format "Error [CODE]: message" (exit code 1).

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid (UUIDv7 support), Go standard library  
**Storage**: frozenDB single-file append-only database format (.fdb files)  
**Testing**: Go test with spec tests (Test_S_ prefix for functional requirements)  
**Target Platform**: Linux (amd64, arm64)  
**Project Type**: Single project (CLI tool + library)  
**Performance Goals**: Database creation <1 second, command execution immediate (<100ms for simple operations)  
**Constraints**: Fixed memory usage regardless of database size, atomic transaction semantics, UUIDv7 timestamp ordering  
**Scale/Scope**: CLI with 7 commands, thin wrapper around existing frozenDB library API  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB  
**Go Module Path**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: CLI delegates all write operations to the underlying library which preserves append-only immutability. No delete/modify operations exposed.
- [x] **Data Integrity**: CLI performs no data transformations; library handles all transaction headers, sentinel bytes, and corruption detection.
- [x] **Correctness Over Performance**: CLI prioritizes correctness by validating inputs (UUIDv7, JSON) before delegating to library. No performance shortcuts.
- [x] **Chronological Ordering**: CLI passes UUIDs directly to library which enforces time-based key ordering and handles time variations.
- [x] **Concurrent Read-Write Safety**: CLI makes no assumptions about concurrency; library handles file locking and concurrent operations.
- [x] **Single-File Architecture**: CLI operates on single .fdb files; create command initializes proper format, all operations delegate to library.
- [x] **Spec Test Compliance**: Functional requirements FR-002 through FR-007 will have corresponding spec tests in cmd/frozendb/cli_spec_test.go. FR-001 (database creation) will be manually validated as it requires sudo elevation and cannot be reliably tested in automated spec tests.

## Project Structure

### Documentation (this feature)

```text
specs/029-cli-implementation/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # CLI API specification
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozenDB/
├── cmd/
│   └── frozendb/                # CLI application
│       ├── main.go              # Entry point, command routing
│       ├── cli_spec_test.go     # Spec tests for FR-001 through FR-007
│       ├── commands.go          # Command implementations (NEW)
│       ├── validation.go        # Input validation (NEW)
│       └── errors.go            # Error formatting (NEW)
├── pkg/
│   └── frozendb/                # Public library API (existing)
│       ├── frozendb.go
│       ├── transaction.go
│       └── errors.go
├── internal/
│   └── frozendb/                # Internal implementation (existing)
│       ├── create.go            # Database creation logic
│       ├── transaction.go       # Transaction management
│       └── ...
├── docs/
│   ├── v1_file_format.md       # File format specification
│   ├── spec_testing.md         # Testing guidelines
│   └── error_handling.md       # Error handling guide
└── Makefile                     # Build commands
```

**Structure Decision**: Single project structure with CLI in cmd/frozendb. The CLI is a thin wrapper around the existing pkg/frozendb and internal/frozendb packages. New files will be added to cmd/frozendb/ for command implementations, validation, and error formatting. Existing database creation logic in internal/frozendb/create.go will be exposed through the CLI.

## Complexity Tracking

No constitutional violations to justify. The CLI is a straightforward wrapper around the existing library with no additional complexity.
