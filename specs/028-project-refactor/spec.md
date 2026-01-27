# Feature Specification: Project Structure Refactor

**Feature Branch**: `028-project-refactor`  
**Created**: 2026-01-27  
**Status**: Draft  
**Input**: User description: "028 folder-structure. Refactor the project file structure to have /cmd, /pkg/frozendb, /internal, and /examples. The user story is as an external developer, I know which FrozenDB structs and functions I can rely upon when building out applications that utilize FrozenDB. As an internal developer, I know which code needs to be stable and maintained, and which are internal implementation details that can be changed. As a developer, I have a simple hello-world CLI that builds and runs, which will be populated with more commands in future specs. Acceptance criteria includes: make CI enforces that the cli tool builds, the examples build, and the main pkg builds. There is a separate namespace for public and internal packages. All of the files in pkg/frozendb are intended to be consumed by external developers. All of the tests pass without logic changes"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Clear API Boundaries for External Developers (Priority: P1)

As an external developer building applications with FrozenDB, I need to clearly understand which structs and functions are part of the public API that I can rely on, and which are internal implementation details that may change.

**Why this priority**: Essential for developer experience and API stability - external developers need confidence that their code won't break with internal changes

**Independent Test**: External developer can import `github.com/susu-dot-dev/frozenDB/pkg/frozendb` and use only the documented public API without accessing internal packages

**Acceptance Scenarios**:

1. **Given** I am an external developer, **When** I import the frozendb package, **Then** I can only access public API functions and structs
2. **Given** I am reviewing the codebase, **When** I look in pkg/frozendb, **Then** all files contain only exported types and functions intended for external use
3. **Given** I am using internal code, **When** I try to import internal packages, **Then** Go's compiler prevents external access

---

### User Story 2 - Maintained Internal Code Organization (Priority: P1)

As an internal developer maintaining FrozenDB, I need to know which code constitutes the stable public API versus internal implementation details that can be freely refactored.

**Why this priority**: Critical for maintainability and team velocity - clear boundaries enable confident refactoring of internal code

**Independent Test**: Internal developer can modify any code in internal/ directory without breaking the public API contract

**Acceptance Scenarios**:

1. **Given** I am an internal developer, **When** I look at the project structure, **Then** internal/ directory contains all implementation details
2. **Given** I need to refactor code, **When** I modify files in internal/, **Then** public API in pkg/ remains unchanged
3. **Given** I am reviewing code organization, **When** I examine package boundaries, **Then** internal code cannot leak into public API

---

### User Story 3 - Functional CLI Interface (Priority: P2)

As a developer, I need a simple CLI tool that builds and runs successfully, providing a foundation for future command implementations.

**Why this priority**: Essential for distribution and user interaction - CLI is the primary interface for end users

**Independent Test**: Developer can build and run the CLI tool with a simple "hello world" functionality

**Acceptance Scenarios**:

1. **Given** I have cloned the repository, **When** I run `make build`, **Then** the CLI tool compiles successfully
2. **Given** the CLI is built, **When** I run the executable, **Then** it outputs "hello world" message
3. **Given** I want to test the CLI, **When** I run `go run ./cmd/frozendb`, **Then** it executes without errors

---

### User Story 4 - Working Example Applications (Priority: P2)

As a developer learning FrozenDB, I need example applications that demonstrate how to use the public API correctly.

**Why this priority**: Important for adoption and developer onboarding - examples reduce learning curve

**Independent Test**: Developer can build and run example applications using the public API

**Acceptance Scenarios**:

1. **Given** I am exploring the project, **When** I look in examples/, **Then** I find working example applications
2. **Given** I want to test examples, **When** I run `make build` for examples, **Then** they compile successfully
3. **Given** I run an example, **When** I execute it, **Then** it demonstrates basic FrozenDB usage

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Project structure MUST separate public API (pkg/frozendb) from internal implementation (internal/)
- **FR-002**: All files in pkg/frozendb MUST contain only exported types and functions intended for external consumption
- **FR-003**: cmd/frozendb MUST contain a functional CLI tool with hello-world functionality
- **FR-004**: examples/ directory MUST contain working example applications that build successfully
- **FR-005**: All existing tests MUST pass without logic changes after the refactor
- **FR-006**: Go module imports MUST be updated to reflect new package structure
- **FR-007**: `make ci` MUST enforce that CLI tool builds, examples build, and main package builds
- **FR-008**: internal/ directory MUST contain all non-public implementation details
- **FR-009**: No internal packages MUST be accessible from external imports
- **FR-010**: All package boundaries MUST be enforced by Go's visibility rules

### Key Entities *(include if feature involves data)*

- **Public API Package**: All exported types, functions, and interfaces that external developers can depend on
- **Internal Packages**: Implementation details, utilities, and code that can be freely refactored
- **CLI Tool**: Command-line interface that uses the public API
- **Example Applications**: Demonstrative code showing proper usage of the public API

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: External developers can identify public API by examining only pkg/frozendb directory
- **SC-002**: Internal developers can confidently refactor code in internal/ without breaking external contracts
- **SC-003**: CLI tool builds and runs successfully on all supported platforms
- **SC-004**: All example applications build and demonstrate working FrozenDB usage
- **SC-005**: Make CI pipeline validates that all components build successfully
- **SC-006**: 100% of existing tests pass without logic changes

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-007**: Zero regressions in existing functionality - all tests pass unchanged
- **SC-008**: Public API contracts remain identical before and after refactor
- **SC-009**: No internal implementation details leak into public API surface
- **SC-010**: Package boundaries are enforced by Go compiler with no visibility violations

## Assumptions

- Current frozendb/ directory contains the public API that should move to pkg/frozendb
- Existing tests will continue to work with updated import paths
- Makefile will need updates to reflect new build targets
- Go module will maintain the same import path for external users
- No functional changes to the database implementation are required
