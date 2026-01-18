# frozenDB - Agent Guidelines

This document contains build commands and coding standards for agentic development of frozenDB, an immutable key-value store built in Go.

## Build & Development Commands

This is a Go 1.25.5 project using `github.com/google/uuid` and standard library dependencies:

### Using Make (Recommended)
```bash
make ci              # Run complete CI pipeline (deps, tidy, fmt, lint, test, build)
make test            # Run all tests with verbose output
make test-spec       # Run spec tests only (Test_S_ prefix)
make test-unit       # Run unit tests only (exclude Test_S_ prefix)
make build           # Build the project
```

## Project Structure

```
frozenDB/
├── frozendb/      # Core database package (public API)
├── docs/          # Documentation including file format specs
├── specs/         # Feature specifications and requirements
├── .github/       # GitHub workflows
└── .specify/      # Development templates and scripts
```

## Essential Context Files

**CRITICAL:** When implementing any database file or in-memory structure features, ALWAYS load:
- `docs/v1_file_format.md` - Complete file format specification

**Additional context for implementation:**
- `docs/spec_testing.md` - Spec testing guidelines and requirements
- `docs/error_handling.md` - How error handling should work in the codebase
- Relevant spec files in `specs/` directory for feature requirements
- `AGENTS.md` - This file for coding standards and build commands

## Code Style Guidelines

### General Principles
- Follow standard Go formatting and conventions
- Use `gofmt` for consistent formatting, run via `make fmt`
- Keep functions small and focused
- Prefer composition over inheritance
- Design for concurrency where applicable

### Naming Conventions
- **Package names**: short, lowercase, single words when possible
- **Functions**: camelCase, exported functions start with capital letter
- **Variables**: camelCase, prefer descriptive names over abbreviations
- **Constants**: UPPER_SNAKE_CASE for exported constants, camelCase for unexported
- **Interfaces**: typically end in "er" suffix (Reader, Writer) or describe behavior
- **Error types**: should end with "Error" suffix

### Error Handling
- All errors should be structured, deriving from the base FrozenDBError struct
- MUST read docs/error_handling.md to understand the rules before designing or implementing errors

### Types and Interfaces
- Use concrete types when implementation is fixed
- Define interfaces for behavior that needs to be mocked or swapped
- Keep interfaces small and focused (interface segregation)
- Use struct embedding carefully

### Testing
- Write table-driven tests for multiple scenarios
- Use subtests for related test cases
- Test both success and error paths
- Include benchmarks for performance-critical code
- Run comprehensive checks with `make ci`, individual tests with direct `go test`

## frozenDB Specific Guidelines

### File Format Implementation
**CRITICAL:** When implementing any database file or in-memory structure features, ALWAYS load:
- `docs/v1_file_format.md` - Complete file format specification

#### Append-Only Architecture
- Data is never modified in place—only appended
- Enables safe concurrent reads during writes
- Fixed-width rows enable O(1) seeking and binary search
- Use ROW_START (0x1F) and ROW_END (0x0A) sentinels for integrity

#### Transaction Semantics
- All writes occur within transactions
- Use start_control `T` for begin, `R` for continuation
- End_control encodes savepoints and termination (TC, RE, SC, SE, R0-R9, S0-S9)
- Implement checksum rows every 10,000 data rows (CRC32)

### UUIDv7 Key Handling
- All keys must be UUIDv7 for proper time ordering
- Validate UUID version on insertion
- Leverage time component for binary search optimization
- Handle configurable time skew for distributed systems

### Performance & Reliability
- Profile memory usage - should be fixed, not scale with database size
- Use OS-level file locks (flocks) for write mode exclusivity
- Implement sentinel bytes for transaction integrity
- Minimize allocations in hot paths

## Active Technologies
- Go 1.25.5 + github.com/google/uuid, Go standard library (010-null-row-struct)
- Single file append-only format (010-null-row-struct)

## Recent Changes
- 010-null-row-struct: Added Go 1.25.5 + github.com/google/uuid, Go standard library
