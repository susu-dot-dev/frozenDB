# Feature Specification: Project Structure Refactor & CLI

**Feature Branch**: `028-pkg-internal-cli-refactor`  
**Created**: 2026-01-28  
**Status**: Draft  
**Input**: User description: "refactor the file structure into /pkg, /internal, /examples, and /cmd. As a user, I can build the CLI and it emits Hello world when running frozenDB. (This way we can build more commands in the future). Next, as an internal developer, it is easy for me to develop without circular dependencies importing packages because all of the code is in the internal folder. As a developer, it is easy to know which modules are publicly usable because of a shim re-export layer in /pkg"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - CLI Execution (Priority: P1)

As an end user, I want to build and run the frozenDB CLI so that I can interact with the database through a command-line interface.

**Why this priority**: This is the most critical user-facing deliverable. Without a working CLI, users cannot interact with frozenDB at all. This forms the foundation for all future CLI commands.

**Independent Test**: Can be fully tested by building the CLI binary and executing it without arguments, verifying that "Hello world" is output. Delivers immediate value by providing a working entry point for the application.

**Acceptance Scenarios**:

1. **Given** the project has been built successfully, **When** I run `frozenDB` (or `./frozenDB`), **Then** the CLI outputs "Hello world" to stdout
2. **Given** the project repository is cloned, **When** I run the build command, **Then** a CLI binary named `frozenDB` is created
3. **Given** the CLI binary exists, **When** I execute it from any directory, **Then** it runs without errors and displays the expected output

---

### User Story 2 - Public API Clarity (Priority: P2)

As an external developer integrating frozenDB into my application, I want clear visibility into which modules are publicly usable so that I can confidently import and use the stable API without relying on internal implementation details.

**Why this priority**: This enables external adoption and integration. Clear API boundaries prevent external developers from depending on internal code that may change without notice, reducing breaking changes and support burden.

**Independent Test**: Can be fully tested by examining the `/pkg` directory structure, verifying that all public types and functions are re-exported from a single location, and that import paths are clean. Delivers value by making the public API discoverable and stable.

**Acceptance Scenarios**:

1. **Given** I want to use frozenDB in my Go application, **When** I look at the `/pkg` directory, **Then** I can see exactly which modules are intended for public consumption
2. **Given** I import frozenDB into my code, **When** I use the `/pkg` import path, **Then** I have access to all public APIs without needing to know internal package structure
3. **Given** the public API is defined in `/pkg`, **When** internal implementations change, **Then** my code continues to work without modification as long as the `/pkg` interface remains stable

---

### User Story 3 - Internal Development Without Circular Dependencies (Priority: P2)

As an internal developer working on frozenDB, I want all implementation code in the `/internal` folder so that I can organize code logically without worrying about circular dependency issues between packages.

**Why this priority**: This improves maintainability and development velocity. Circular dependencies are a common pain point in Go projects and can block feature development. Having clear internal boundaries prevents this issue.

**Independent Test**: Can be fully tested by building the project and verifying that all internal code resides in `/internal`, that packages within `/internal` can import each other without circular dependency errors, and that the project compiles successfully. Delivers value by reducing friction in daily development.

**Acceptance Scenarios**:

1. **Given** I am adding a new feature, **When** I need to import another internal package, **Then** the project builds without circular dependency errors
2. **Given** all implementation code is in `/internal`, **When** I run the Go compiler, **Then** it successfully builds without circular import warnings
3. **Given** internal packages need to share code, **When** they import from common internal packages, **Then** dependencies flow cleanly without cycles

---

### User Story 4 - Example Code Discovery (Priority: P3)

As a developer learning frozenDB or evaluating it for adoption, I want example code in a dedicated `/examples` directory so that I can quickly understand how to use the database in real-world scenarios.

**Why this priority**: This accelerates adoption and reduces support questions. While not critical for core functionality, examples significantly improve developer experience and reduce time-to-productivity.

**Independent Test**: Can be fully tested by navigating to `/examples` directory, running example programs, and verifying they demonstrate key frozenDB usage patterns. Delivers value by providing ready-to-run learning materials.

**Acceptance Scenarios**:

1. **Given** I want to learn how to use frozenDB, **When** I navigate to the `/examples` directory, **Then** I find working code samples that demonstrate common use cases
2. **Given** an example exists, **When** I build and run it, **Then** it executes successfully and demonstrates the intended functionality
3. **Given** I'm evaluating frozenDB, **When** I read the example code, **Then** I understand how to integrate the database into my application

---

### Edge Cases

- What happens when the CLI is invoked with unknown flags or commands? (Should display helpful usage information)
- What happens if someone tries to import internal packages directly from external code? (Go's `/internal` convention should prevent this at compile time)
- What happens if circular dependencies are accidentally introduced? (Should fail at compile time with clear error)
- What happens when public API in `/pkg` needs to change? (Should follow semantic versioning and maintain backward compatibility)
- What happens if someone tries to build without the correct Go version? (Should provide clear error message)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST organize code into four top-level directories: `/pkg`, `/internal`, `/examples`, and `/cmd`
- **FR-002**: System MUST place the CLI entry point in the `/cmd/frozenDB` directory with a `main.go` that outputs "Hello world" when executed without arguments
- **FR-003**: CLI MUST be buildable into an executable binary named `frozenDB`, added to .gitignore to prevent committing, and built as part of make ci
- **FR-004**: System MUST place all implementation code in the `/internal/frozendb` directory, including all tests with updated import paths
- **FR-005**: System MUST create a minimal public API in `/pkg/frozendb` as a shim layer that re-exports from `/internal/frozendb` ONLY the types and functions needed for core operations: opening databases (`FrozenDB`, `NewFrozenDB`, `MODE_READ`, `MODE_WRITE`), transaction management (`Transaction` with `Begin`, `AddRow`, `Commit`, `Rollback`, `Close`), querying (`FinderStrategy` constants, `GetIndex`, `GetTransactionStart`, `GetTransactionEnd`), and error handling (all error types). Database creation functions (`CreateFrozenDB`, `CreateConfig`, `NewCreateConfig`, `SudoContext`), internal row types (`NullRow`, `DataRow`, `PartialDataRow`), internal transaction methods (`GetEmptyRow`, `GetRows`), and file format details (`Header`) MUST remain internal-only

### Key Entities

- **CLI Binary**: The executable program users run to interact with frozenDB. Located in `/cmd/frozenDB`, outputs "Hello world" as initial implementation. Will handle database creation in future releases.
- **Public API Layer** (`/pkg`): Minimal re-exported interfaces, types, and functions for core operations: opening databases, transactions, querying, and closing. Excludes creation functions and internal row types.
- **Internal Implementation** (`/internal`): Core database logic, file management, transaction handling, database creation, and all implementation details including row types. Not accessible to external projects.
- **Example Programs** (`/examples`): Standalone programs demonstrating frozenDB usage patterns for learning and evaluation. Must use internal package for database creation until CLI supports it.
- **Package Dependencies**: Import relationships between modules, organized to prevent circular dependencies

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in appropriate `*_spec_test.go` files matching the implementation
- Follow naming convention `Test_S_028_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can build the CLI and execute it successfully in under 1 minute from a fresh clone
- **SC-002**: CLI outputs "Hello world" exactly when run without arguments
- **SC-003**: External developers can identify all public APIs by examining only the `/pkg` directory
- **SC-004**: Internal developers can add new internal packages without encountering circular dependency errors
- **SC-005**: All existing functionality works identically after the refactor (zero behavior changes)
- **SC-006**: External code importing frozenDB only needs to import from `/pkg` paths for core operations (open, transaction, query, close)
- **SC-007**: Build time remains within 10% of current build time after refactor
- **SC-008**: Zero errors when attempting to import `/internal` packages from outside the project (enforced by Go compiler)
- **SC-009**: Public API surface in `/pkg/frozendb` includes ONLY types and functions needed for opening, querying, and transacting - excludes all database creation functions and internal row types

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-010**: Zero data loss scenarios in corruption detection tests after refactor
- **SC-011**: All concurrent read/write operations maintain data consistency with new structure
- **SC-012**: Memory usage remains constant regardless of database size after refactor
- **SC-013**: Transaction atomicity preserved in all crash simulation tests with new package structure
- **SC-014**: All existing spec tests pass without modification (except for import path updates)

## Assumptions

- The project uses Go modules (`go.mod` exists)
- The current codebase is in the `/frozendb` directory
- Standard Go conventions will be followed (e.g., `/internal` package privacy)
- The CLI will be expanded with additional commands in future features (this is MVP with "Hello world")
- Semantic versioning will be used for any public API changes
- The build process uses both `make` and direct `go build` commands
- Existing tests are comprehensive and will validate that refactor doesn't break functionality

## Dependencies

- Go 1.25.5 or compatible version
- Existing build toolchain (make, go compiler)
- Current frozenDB codebase and test suite
- No new external dependencies required for this refactor

## Out of Scope

- Adding CLI command parsing or flags (future feature)
- Implementing database commands in the CLI beyond "Hello world" (future feature)
- Creating comprehensive API documentation (separate documentation effort)
- Performance optimization beyond maintaining current performance
- Adding new database features or changing database behavior
- Implementing CLI configuration files or environment variable support
- Adding logging or verbose output modes to the CLI
- Exposing database creation API publicly (will be CLI-only in future; examples may use internal package temporarily)
