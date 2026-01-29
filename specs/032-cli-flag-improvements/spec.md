# Feature Specification: CLI Flag Improvements

**Feature Branch**: `032-cli-flag-improvements`  
**Created**: Thu Jan 29 2026  
**Status**: Draft  
**Input**: User description: "CLI improvements. Users can enter the --path command either first, or after the subcommand. E.g. frozendb --path abc begin or frozendb begin --path abc. Next, when calling add, users can use the special key of NOW, which will generate a UUIDv7. Change the add output to return the key in all success cases (whether NOW or a user-passed in key). Next, add a --finder flag (which can be either before or after the sub-command like --path) which lets the user specify the finder strategy. Default to the BinarySearchFinder if not specified"

## Clarifications

### Session 2026-01-29

- Q: When a user types "now" in lowercase (or any mixed case like "Now", "nOw") instead of "NOW" in the add command, what should happen? → A: Accept case-insensitive - treat "now", "NOW", "Now" all as the special keyword that generates UUIDv7
- Q: The edge case mentions finder flag values with mixed case (e.g., "Simple", "BINARY"). Should these be accepted? → A: Accept case-insensitive - "simple", "Simple", "SIMPLE" all map to FinderStrategySimple (consistent with A-003)
- Q: The spec states the add command outputs "just the UUID string" (A-005), but doesn't specify the exact format. What should the add command output look like? → A: Just the UUID string on a single line

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Flexible Flag Positioning (Priority: P1)

Users can specify global flags like `--path` and `--finder` in any position - either before or after the subcommand - to match their mental model and workflow preferences without memorizing specific command syntax rules.

**Why this priority**: This is the foundational UX improvement that makes the CLI more intuitive and user-friendly. It reduces cognitive load and enables users to work more naturally with the tool. All other features depend on proper flag parsing.

**Independent Test**: Can be fully tested by executing commands with flags in different positions (e.g., `frozendb --path db.frz begin` vs `frozendb begin --path db.frz`) and verifying both work identically.

**Acceptance Scenarios**:

1. **Given** a valid database file exists, **When** user runs `frozendb --path mydb.frz begin`, **Then** the command succeeds and begins a transaction
2. **Given** a valid database file exists, **When** user runs `frozendb begin --path mydb.frz`, **Then** the command succeeds and begins a transaction with identical behavior
3. **Given** a valid database file exists, **When** user runs `frozendb --path mydb.frz --finder binary add <key> <value>`, **Then** both flags are recognized correctly
4. **Given** a valid database file exists, **When** user runs `frozendb add --path mydb.frz --finder binary <key> <value>`, **Then** both flags are recognized correctly with identical behavior
5. **Given** user omits the `--path` flag, **When** any command requiring path is executed, **Then** an error message indicates the missing flag
6. **Given** user provides `--path` flag twice in different positions, **When** command is executed, **Then** an error indicates duplicate flag specification

---

### User Story 2 - NOW Keyword for Auto-Generated UUIDv7 Keys (Priority: P2)

Users can use the special keyword "NOW" as the key argument in the `add` command to automatically generate a UUIDv7 timestamp-based key, eliminating the need to manually generate UUIDs for time-series data.

**Why this priority**: This significantly improves developer experience for the common use case of time-series data insertion. However, it's lower priority than flag positioning since it's a convenience feature rather than a core usability fix.

**Independent Test**: Can be fully tested by running `frozendb add --path db.frz NOW '{"test": true}'` and verifying a valid UUIDv7 key is generated and the value is stored correctly.

**Acceptance Scenarios**:

1. **Given** an active transaction exists, **When** user runs `frozendb add --path db.frz NOW '{"data": "value"}'`, **Then** a new UUIDv7 key is generated using the current timestamp and the value is stored
2. **Given** an active transaction exists, **When** user runs `frozendb add --path db.frz NOW '{"data": "value"}'` twice with 1ms between calls, **Then** two distinct UUIDv7 keys are generated in chronological order
3. **Given** the keyword "NOW" (in any case: "now", "Now", "NOW") is provided as the key, **When** the add command completes successfully, **Then** the output includes the generated UUIDv7 key
4. **Given** user provides a literal UUID string instead of "NOW", **When** add command is executed, **Then** the provided UUID is used as-is (existing behavior preserved)
5. **Given** user provides "now" in lowercase or mixed case like "Now", **When** add command is executed, **Then** it is treated identically to "NOW" and generates a UUIDv7 key

---

### User Story 3 - Finder Strategy Selection (Priority: P3)

Users can specify the finder strategy using the `--finder` flag to optimize database performance based on their use case (memory-constrained vs. speed-optimized scenarios), with BinarySearchFinder as the intelligent default.

**Why this priority**: This is an advanced optimization feature that affects performance characteristics. Most users will benefit from the smart default, making this lower priority than core UX improvements.

**Independent Test**: Can be fully tested by opening databases with different finder strategies and measuring memory usage and query performance to verify strategy selection works correctly.

**Acceptance Scenarios**:

1. **Given** no `--finder` flag is specified, **When** any command opens a database, **Then** BinarySearchFinder strategy is used by default
2. **Given** user specifies `--finder simple` (or "Simple", "SIMPLE", etc.), **When** database is opened, **Then** FinderStrategySimple is used
3. **Given** user specifies `--finder inmemory` (or "InMemory", "INMEMORY", etc.), **When** database is opened, **Then** FinderStrategyInMemory is used
4. **Given** user specifies `--finder binary` (or "Binary", "BINARY", etc.), **When** database is opened, **Then** FinderStrategyBinarySearch is used
5. **Given** user specifies an invalid finder value like `--finder xyz`, **When** command is executed, **Then** an error message lists valid finder options (simple, inmemory, binary)
6. **Given** the `--finder` flag can be positioned before or after the subcommand, **When** user runs `frozendb --finder simple begin --path db.frz` or `frozendb begin --finder simple --path db.frz`, **Then** both work identically

---

### User Story 4 - Enhanced Add Output with Key Display (Priority: P2)

Users receive the key in add command output to confirm which UUID was used for the insertion, especially useful when working with auto-generated NOW keys or verifying successful additions.

**Why this priority**: This improves visibility and debugging experience, particularly when combined with the NOW feature. It's parallel priority to NOW since they complement each other, allowing users to immediately see the generated UUID.

**Independent Test**: Can be fully tested by executing add commands with both explicit UUIDs and NOW keyword, verifying the output includes the key that was used.

**Acceptance Scenarios**:

1. **Given** an active transaction exists, **When** user runs `frozendb add --path db.frz <uuid> '{"data": "value"}'`, **Then** the UUID is written to stdout as a single line (e.g., `018d5c5a-1234-7890-abcd-ef0123456789`)
2. **Given** an active transaction exists and user provides NOW keyword, **When** add command completes, **Then** the auto-generated UUIDv7 key is written to stdout as a single line
3. **Given** the add command succeeds, **When** output is displayed, **Then** the key is displayed as a standard UUID string format with hyphens on a single line
4. **Given** the add command succeeds, **When** output is displayed, **Then** the output format is consistent for both NOW-generated and user-provided keys (UUID string only, no additional text)

---

### Edge Cases

- What happens when user provides both a global flag (like `--path`) before AND after the subcommand?
  - **Expected**: Error message indicating duplicate flag specification
- What happens when user types "now" in lowercase instead of "NOW"?
  - **Resolved**: Case-insensitive - all variations ("now", "NOW", "Now", etc.) are treated as the special keyword
- What happens when user specifies `--finder` flag with get command but database was created with a different finder?
  - **Expected**: Finder strategy applies to current session only, does not modify database
- What happens when NOW keyword generates a UUID that conflicts with an existing key (UUID collision)?
  - **Expected**: Standard duplicate key error, same as manual UUID collision
- What happens when user provides invalid finder strategy names with mixed case (e.g., "Simple", "BINARY")?
  - **Resolved**: Case-insensitive matching - "simple", "Simple", "SIMPLE" all accepted and map correctly

## Requirements *(mandatory)*

### Functional Requirements

**Flag Parsing & Positioning:**
- **FR-001**: Global flags (`--path` and `--finder`) MUST be accepted in any position relative to the subcommand (before or after) and in any order
- **FR-002**: Required flags MUST be validated for presence and uniqueness, with clear error messages for missing or duplicate flags

**NOW Keyword:**
- **FR-003**: The `add` command MUST recognize "NOW" (case-insensitive: "now", "NOW", "Now", etc.) as a special keyword that generates a valid UUIDv7 using the current system timestamp

**Add Command Output:**
- **FR-004**: The `add` command MUST output the key (whether user-provided or NOW-generated) to stdout as a single line containing only the UUID string with hyphens (e.g., `018d5c5a-1234-7890-abcd-ef0123456789`) upon successful completion; errors MUST exit with non-zero status and write error messages to stderr without outputting a key

**Finder Strategy:**
- **FR-005**: The `--finder` flag MUST accept three values ("simple", "inmemory", "binary") in a case-insensitive manner, default to "binary" when not specified, and display an error with valid options for invalid values
- **FR-006**: All commands that open databases (begin, commit, savepoint, rollback, add, get) MUST apply the specified finder strategy; the `create` command MUST ignore the `--finder` flag without error (finder strategy is a runtime concern for reading existing databases, not database creation)

### Key Entities *(include if feature involves data)*

- **CLI Global Flags**: Flags that apply across all subcommands (`--path`, `--finder`) and can be positioned flexibly
- **NOW Keyword**: Special reserved keyword recognized by the `add` command to trigger UUIDv7 generation
- **Finder Strategy**: Runtime configuration choice affecting how the database indexes and retrieves records (simple, inmemory, binary)
- **Command Output**: Structured response including keys, values, and success indicators for user feedback

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `cmd/frozendb/cli_spec_test.go` for CLI-specific functionality
- Follow naming convention `Test_S_032_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can successfully execute commands with flags in any position (before or after subcommand) 100% of the time
- **SC-002**: Users can generate UUIDv7 keys using NOW keyword without external UUID generation tools
- **SC-003**: Users receive key output from add commands, enabling them to immediately reference the inserted record
- **SC-004**: Users can select appropriate finder strategy for their performance requirements without code changes
- **SC-005**: All spec tests pass 100% of the time after implementation
- **SC-006**: No regression in existing CLI functionality - all current commands continue to work as before

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-007**: UUIDv7 keys generated via NOW keyword maintain proper time ordering and uniqueness
- **SC-008**: Finder strategy selection does not affect data correctness - all strategies return identical query results
- **SC-009**: Flag parsing errors are caught before any database operations occur, preventing partial writes
- **SC-010**: All CLI operations maintain existing transaction semantics and data integrity guarantees

## Assumptions

- **A-001**: The existing CLI uses Go's `flag` package for argument parsing, which will be replaced or augmented to support flexible flag positioning
- **A-002**: "NOW" keyword is case-insensitive (accepts "now", "NOW", "Now", etc.) to provide a forgiving user experience and reduce typing errors
- **A-003**: The `--finder` flag values ("simple", "inmemory", "binary") are case-insensitive for better UX
- **A-004**: Default finder strategy (BinarySearchFinder) provides best balance of performance and memory usage for typical workloads
- **A-005**: The `add` command will output just the UUID string on a single line to stdout (e.g., `018d5c5a-1234-7890-abcd-ef0123456789`), maintaining simplicity and ease of parsing in shell scripts
- **A-006**: UUIDv7 generation using NOW relies on system clock accuracy; clock skew handling (if any) is determined by the underlying frozenDB implementation, not CLI configuration
- **A-007**: Multiple NOW keywords in rapid succession (within the same millisecond) will generate unique UUIDs due to UUIDv7's monotonic counter bits that ensure uniqueness when timestamps collide

## Dependencies

- **D-001**: Requires existing UUIDv7 generation capability from `github.com/google/uuid` package
- **D-002**: Depends on existing finder strategy implementations (Simple, InMemory, BinarySearch)
- **D-003**: May require alternative CLI parsing library or custom implementation to support flexible flag positioning (Go's standard `flag` package has fixed position requirements)

## Out of Scope

- **OS-001**: Changing the database file format or finder strategy storage (finder is runtime-only)
- **OS-002**: Adding new finder strategies beyond the existing three
- **OS-003**: Providing configuration files for default flag values
- **OS-004**: Adding short flag alternatives (e.g., `-p` for `--path`)
- **OS-005**: Supporting positional flag syntax mixing with traditional argument formats
- **OS-006**: Adding NOW-style keywords for other commands beyond `add`
