# Feature Specification: CLI Implementation

**Feature Branch**: `029-cli-implementation`  
**Created**: 2026-01-28  
**Status**: Draft  
**Input**: User description: "Implement a CLI for frozenDB with the following commands: `frozendb create <path>` to initialize a new database file, `frozendb begin --path <file>` to start a transaction, `frozendb commit --path <file>` to commit a transaction, `frozendb savepoint --path <file>` to create a savepoint, `frozendb rollback --path <file>` to rollback a transaction (discarding all savepoints), `frozendb add --path <file> <key> <value>` to insert data where key is a UUIDv7 and value is JSON, and `frozendb get --path <file> <key>` to retrieve data. Transaction commands (begin, commit, savepoint, rollback, add) should be silent on success (exit code 0) and print errors to stderr (exit code 1). The get command should print pretty-formatted JSON to stdout on success and errors to stderr on failure."

## Clarifications

### Session 2026-01-28

- Q: Should the CLI support creating/managing savepoints explicitly? → A: Yes, via `frozendb savepoint --path <file>` command
- Q: When rollback is called, does it discard all savepoints or roll back to the most recent? → A: Rollback discards all savepoints and performs full transaction rollback
- Q: Should error messages include specific validation details or just general categories? → A: Error messages include descriptive message from err.Error()
- Q: When the user provides the `--path` flag, should the path be resolved/normalized by the CLI? → A: Accepted as-is without any path processing
- Q: When the CLI encounters an error from the underlying frozenDB library, how should it format the error message to stderr? → A: Format as "Error: " followed by err.Error()
- Q: When a user runs `frozendb create <path>` and the database is successfully created, should the CLI output anything? → A: Exit silently with code 0 (consistent with transaction commands)
- Q: When CLI-layer validation fails (e.g., missing required arguments, invalid UUIDv7 format), what error code should be used in the "Error [CODE]: message" format? → A: Use "Error: " followed by err.Error()

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Database Initialization (Priority: P1)

A system administrator or developer needs to initialize a new frozenDB database file before any operations can be performed. This is the foundational capability that enables all other database operations.

**Why this priority**: Without the ability to create a database, no other operations are possible. This is the absolute prerequisite for using frozenDB.

**Independent Test**: Can be fully tested by invoking the create command with a valid path, verifying the database file is created with proper structure and permissions, and delivers a usable database file.

**Acceptance Scenarios**:

1. **Given** user has appropriate filesystem permissions, **When** user runs `sudo frozendb create /path/to/new.fdb`, **Then** the database file is created with elevated permissions and the command exits silently with code 0
2. **Given** an existing file at the specified path, **When** user runs `frozendb create /path/to/existing.fdb` or `bad/path.fdb`, **Then** the command fails with an error indicating the the appropriate error

---

### User Story 2 - Database Modification via CLI (Priority: P2)

As a developer, given a database in any existing valid state, I want to execute simple commands to modify the database without writing library code, enabling quick iteration and debugging.

**Why this priority**: Enables fast development cycles by providing direct database manipulation capabilities through simple, memorable commands. Essential for testing transaction behavior and data operations.

**Independent Test**: Can be tested by executing individual commands against a database and verifying the database state changes as expected through inspection.

**Acceptance Scenarios**:

1. **Given** a database in any valid state, **When** user executes `frozendb begin --path /path/to/db.fdb`, **Then** a new transaction is opened and the command exits with code 0
2. **Given** an active transaction, **When** user executes `frozendb commit --path /path/to/db.fdb`, **Then** the transaction is committed and the command exits with code 0
3. **Given** an active transaction, **When** user executes `frozendb add --path /path/to/db.fdb 01934567-89ab-cdef-0123-456789abcdef '{"data": "test"}'`, **Then** the data is added to the transaction and the command exits with code 0
4. **Given** no active transaction, **When** user executes `frozendb add --path /path/to/db.fdb 01934567-89ab-cdef-0123-456789abcdef '{"data": "test"}'`, **Then** the command fails, prints error to stderr, and exits with code 1
5. **Given** an active transaction, **When** user executes `frozendb savepoint --path /path/to/db.fdb`, **Then** a savepoint is created and the command exits with code 0
6. **Given** an active transaction, **When** user executes `frozendb rollback --path /path/to/db.fdb`, **Then** all uncommitted changes are discarded, all savepoints are cleared, and the command exits with code 0

---

### User Story 3 - Quick Data Retrieval (Priority: P2)

As a developer or operator, I want a quick way to query rows using the CLI for debugging or scripting purposes, without writing library code.

**Why this priority**: Essential for verifying data integrity, debugging database contents, and enabling shell scripts that need to read database values.

**Independent Test**: Can be tested by pre-populating a database with known data, then executing get commands and verifying the correct values are returned to stdout.

**Acceptance Scenarios**:

1. **Given** a database with existing data for key "01934567-89ab-cdef-0123-456789abcdef", **When** user executes `frozendb get --path /path/to/db.fdb 01934567-89ab-cdef-0123-456789abcdef`, **Then** the value is printed to stdout as pretty-printed JSON and the command exits with code 0
2. **Given** a database without the requested key, **When** user executes `frozendb get --path /path/to/db.fdb nonexistent-key`, **Then** an error message is printed to stderr and the command exits with code 1

---

### Edge Cases

- What happens when a user provides invalid JSON as the value argument to add? - Prints "Error: message" to stderr, exits with code 1
- How does the system handle add command with an invalid UUIDv7 key? - Prints "Error: validation error description" to stderr, exits with code 1
- What happens when file paths contain spaces, special characters, or are relative vs absolute? - CLI passes path as-is to underlying library; library handles path resolution and validation
- How does the system handle very large JSON values in add commands? - Accepted as long as valid JSON and within system memory limits
- How does the system behave when the database file is locked by another process? - Prints "Error: library error message" to stderr, exits with code 1
- How does the system handle Unicode or special characters in keys and values? - Validated per UUIDv7 spec for keys; accepted in values as valid JSON
- What happens when the database file becomes corrupted during CLI operations? - Prints "Error: library error message" to stderr, exits with code 1
- How does the system handle concurrent CLI operations on the same database? - Handled by underlying library file locking
- What happens when get is called with a non-existent key? - Prints "Error: message" to stderr, exits with code 1
- What happens when get is called with an invalid UUIDv7 key? - Prints "Error: validation error" to stderr, exits with code 1
- What happens when --path argument is missing from any command? - Prints "Error: usage error message" to stderr, exits with code 1
- What happens when add is called with missing key or value arguments? - Prints "Error: usage error message" to stderr, exits with code 1

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide seven commands: `create <path>`, `begin --path <file>`, `commit --path <file>`, `savepoint --path <file>`, `rollback --path <file>`, `add --path <file> <key> <value>`, and `get --path <file> <key>`
- **FR-002**: All `--path` arguments MUST be passed as-is to the underlying library without CLI-layer normalization or resolution
- **FR-003**: The `add` and `get` commands MUST validate that key arguments are valid UUIDv7 strings before calling the underlying library
- **FR-004**: The `add` command MUST validate that the value argument is valid JSON before calling the underlying library
- **FR-005**: All commands except `get` MUST produce no stdout output on success and exit with code 0
- **FR-006**: The `get` command MUST print the retrieved value as pretty-printed JSON to stdout on success and exit with code 0
- **FR-007**: All commands MUST print errors to stderr in the format "Error: message" where message is err.Error() and exit with code 1 on failure

### Key Entities *(include if feature involves data)*

- **Subcommand**: One of create, begin, commit, savepoint, rollback, add, or get
- **Key**: UUIDv7 string used to identify rows for add and get operations
- **Value**: JSON-formatted data provided as a string argument to the add command
- **Database File**: The target frozenDB file specified via the --path flag

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that validate the requirement exactly as specified. These tests focus on functional validation and are distinct from unit tests.

See project documentation for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create a new database with a single command invocation in under 1 second; create command produces no stdout output on success
- **SC-002**: Users can execute individual transaction commands with 100% success rate when conditions are valid
- **SC-003**: Users receive descriptive error messages to stderr for 100% of failed command attempts
- **SC-004**: Exit codes correctly reflect success (0) or failure (1) for 100% of executions
- **SC-005**: Get command outputs pretty-printed JSON to stdout for 100% of successful retrievals
- **SC-006**: Transaction commands and create command produce no stdout output, only stderr on errors

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-007**: All committed data via add command is retrievable with 100% accuracy through subsequent get commands
- **SC-008**: Rollback operations correctly discard uncommitted changes with zero data persistence
- **SC-009**: Transaction boundaries are maintained correctly with no data leakage between transactions
- **SC-010**: UUIDv7 key validation prevents 100% of invalid key insertions
- **SC-011**: JSON value serialization maintains data fidelity for all JSON types (strings, numbers, objects, arrays, booleans, null)
- **SC-012**: Get command returns values exactly as stored, with proper JSON formatting

## Assumptions

- Users have basic familiarity with command-line interfaces and JSON syntax
- The underlying frozenDB library API is already implemented and provides the necessary functions for database operations
- Database file locking and concurrency control are handled by the underlying library, not the CLI layer
- Error messages from the underlying library are sufficiently descriptive to be passed through to users via stderr
- The CLI binary will be accessible from the system command line
- The value argument for the add command is provided as a single shell-escaped JSON string
- File permissions for database creation follow standard OS conventions unless elevated permissions are used
- The create command will initialize the database with the proper file format structure
- Transaction commands open the database in read-write mode
- The get command may open the database in read-only or read-write mode (implementation detail)
- Path arguments are passed directly to the underlying library without CLI-layer normalization or resolution
- Error messages use the format "Error: " followed by err.Error() from the FrozenDBError

## Dependencies

- Underlying frozenDB library for database operations
- FrozenDBError class for structured error codes and messages
- Operating system file locking mechanisms for write exclusivity
- Command-line argument parsing capability
- JSON parsing and validation capability
- Standard output and error streams

## Scope

### In Scope

- Database file creation via `create` command with path argument; silent on success
- Transaction management via `begin`, `commit`, `savepoint`, `rollback` subcommands with --path flag
- Data insertion via `add` subcommand with --path flag, key argument, and value argument
- Data retrieval via `get` subcommand with --path flag and key argument
- JSON value parsing and validation for add command
- Silent execution for create and transaction commands (no stdout, errors to stderr)
- Pretty-printed JSON output to stdout for get
- Error messages to stderr with format "Error: message" where message is err.Error()
- Exit code management based on success/failure (0 or 1)
- UUIDv7 key validation

### Out of Scope

- Interactive REPL (Read-Eval-Print Loop) mode
- Command history or auto-completion
- Configuration files or persistent CLI settings
- Database schema management or migrations
- Multi-database operations in a single command
- Authentication or access control at the CLI level
- Query language beyond simple key lookups
- Batch command execution (users can script multiple individual commands if needed)
- Progress indicators for long-running operations
- Verbose or debug output modes
- Command aliasing or shortcuts
- Shell integration or auto-complete scripts
- Stdin-based command input (all commands use arguments and flags)
- JSON output for transaction commands (transaction commands are silent on success)
- Path normalization or resolution at the CLI layer (passed to library as-is)
