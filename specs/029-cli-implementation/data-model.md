# Data Model: CLI Implementation

## Overview

This document defines the new data entities, validation rules, and state transitions introduced by the CLI feature. The CLI is a thin wrapper around the frozenDB library, so most data structures and validation are handled by the underlying library. This document focuses only on CLI-specific data handling.

**Testing Note**: FR-001 (database creation via `frozendb create`) requires sudo elevation and cannot be reliably tested in automated spec tests. FR-001 will be validated manually. FR-002 through FR-007 will have corresponding spec tests in `cmd/frozendb/cli_spec_test.go`.

---

## CLI Command Entity

### Definition

A CLI command represents a single invocation of the `frozendb` binary with a subcommand and associated arguments.

**Attributes**:
- `subcommand`: string - One of: `create`, `begin`, `commit`, `savepoint`, `rollback`, `add`, `get`
- `path`: string - Database file path (from positional arg or `--path` flag)
- `key`: string - UUIDv7 string (for `add` and `get` commands)
- `value`: string - JSON string (for `add` command)
- `savepointId`: int - Savepoint number 0-9 (for `rollback` command, default 0)

**Relationships**:
- Commands operate on frozenDB database files
- Transaction commands (`begin`, `commit`, `savepoint`, `rollback`, `add`) operate on the active transaction within a database
- Each command invocation is independent (no persistent CLI state)

---

## Validation Rules

### CLI-Layer Validation

The CLI performs minimal validation before delegating to the library:

#### 1. Command Structure Validation

**Rule**: Subcommand must be one of the seven recognized commands
- **Valid**: `create`, `begin`, `commit`, `savepoint`, `rollback`, `add`, `get`
- **Error**: InvalidInputError with message "unknown command: {subcommand}"

**Rule**: Required arguments must be present
- `create`: Requires 1 positional argument (path)
- `begin`, `commit`, `savepoint`: Require `--path` flag
- `rollback`: Requires `--path` flag, optional positional argument (savepointId, default 0)
- `add`: Requires `--path` flag and 2 positional arguments (key, value)
- `get`: Requires `--path` flag and 1 positional argument (key)
- **Error**: InvalidInputError with message "missing required argument: {arg_name}"

#### 2. UUIDv7 Format Validation (FR-003)

**Rule**: Key arguments for `add` and `get` commands must be valid UUIDv7 strings

**Validation Steps**:
1. Parse UUID string using `uuid.Parse(keyStr)`
2. Check UUID version: `key.Version() == 7`

**Valid Examples**:
- `01934567-89ab-7def-8123-456789abcdef` (UUIDv7)
- `01934567-89ab-7fff-bfff-ffffffffffff` (UUIDv7)

**Invalid Examples**:
- `not-a-uuid` → Parse error
- `00000000-0000-4000-8000-000000000000` → Wrong version (UUIDv4)
- `` (empty string) → Parse error

**Error**: InvalidInputError with message:
- "invalid UUID format" (parse failure)
- "key must be UUIDv7" (wrong version)

#### 3. JSON Format Validation (FR-004)

**Rule**: Value argument for `add` command must be valid JSON

**Validation Steps**:
1. Check JSON validity using `json.Valid([]byte(valueStr))`

**Valid Examples**:
- `{"name": "test"}` (object)
- `[1, 2, 3]` (array)
- `"string value"` (string)
- `123` (number)
- `true` (boolean)
- `null` (null)

**Invalid Examples**:
- `{invalid}` → Invalid syntax
- `{'single': 'quotes'}` → Invalid syntax (must use double quotes)
- `` (empty string) → Invalid JSON

**Error**: InvalidInputError with message "invalid JSON format"

### Library-Layer Validation (Deferred)

The following validations are performed by the frozenDB library, not the CLI:

#### Path Validation
- Parent directory existence
- File existence (for open) or non-existence (for create)
- .fdb file extension requirement
- Write permissions
- **Error Types**: PathError, InvalidInputError

#### Transaction State Validation
- No active transaction when calling begin
- Active transaction when calling commit/rollback/savepoint/add
- Transaction not tombstoned
- **Error Types**: InvalidActionError, TombstonedError

#### Data Constraints
- UUID timestamp ordering (new_timestamp + skew_ms > max_timestamp)
- Transaction row limit (max 100 data rows)
- Savepoint limit (max 9 savepoints)
- Value size constraints relative to row_size
- **Error Types**: KeyOrderingError, InvalidInputError

#### Database Integrity
- Header format validation
- Checksum validation
- Row structure validation
- **Error Types**: CorruptDatabaseError, ReadError

---

## State Transitions

The CLI is stateless across invocations. Transaction state is persisted in the database file and recovered by the library. The following describes the transaction state flow across CLI commands:

### Transaction Lifecycle State Machine

```
[No Database] 
    |
    | frozendb create <path>
    v
[Database Exists, No Transaction]
    |
    | frozendb begin --path <file>
    v
[Database Exists, Transaction Active]
    |
    | frozendb add --path <file> <key> <value>  (1-100 times)
    v
[Database Exists, Transaction Active with N rows]
    |
    | frozendb savepoint --path <file>  (0-9 times)
    v
[Database Exists, Transaction Active with N rows, M savepoints]
    |
    +---> frozendb commit --path <file>
    |         |
    |         v
    |     [Database Exists, No Transaction] (all rows committed)
    |
    +---> frozendb rollback --path <file> [savepoint_id]
              |
              v
          [Database Exists, No Transaction] (partial or full rollback)
```

### State Transition Rules

1. **create** command:
   - Precondition: Database file does not exist at path
   - Postcondition: Database file exists with header + initial checksum row
   - Transaction state: None (no active transaction)

2. **begin** command:
   - Precondition: Database exists, no active transaction
   - Postcondition: Database has active transaction (PartialDataRow persisted)
   - Transaction state: Active

3. **add** command:
   - Precondition: Database exists, active transaction
   - Postcondition: Database has active transaction with additional data row
   - Transaction state: Active (row count incremented)

4. **savepoint** command:
   - Precondition: Database exists, active transaction with at least 1 row
   - Postcondition: Database has active transaction, current row marked as savepoint
   - Transaction state: Active (savepoint count incremented, max 9)

5. **commit** command:
   - Precondition: Database exists, active transaction
   - Postcondition: Database has no active transaction, all rows committed
   - Transaction state: None (transaction closed)
   - **Special case**: Empty transaction (no add calls) creates NullRow

6. **rollback** command:
   - Precondition: Database exists, active transaction
   - Postcondition: Database has no active transaction, rows invalidated per savepoint
   - Transaction state: None (transaction closed)
   - **Behavior by savepointId**:
     - 0: Full rollback (all rows invalidated)
     - 1-9: Partial rollback (rows after savepoint N invalidated)

7. **get** command:
   - Precondition: Database exists
   - Transaction state: Unchanged
   - **Note**: Reads only committed data; active transactions are not visible

### State Validation Errors

**InvalidActionError** scenarios:
- `begin` when transaction already active: "transaction already active"
- `commit` when no transaction active: "no active transaction"
- `rollback` when no transaction active: "no active transaction"
- `savepoint` when no transaction active: "no active transaction"
- `add` when no transaction active: "no active transaction"

**InvalidInputError** scenarios:
- `rollback` with savepointId > current savepoint count: "invalid savepoint: savepoint {id} does not exist"
- `rollback` with savepointId < 0 or > 9: "savepointId must be between 0 and 9"

---

## Error Condition Mapping

The CLI uses FrozenDBError codes for all error reporting. The following maps CLI scenarios to error types:

### CLI-Layer Errors (code: `invalid_input`)

| Scenario | Error Message | Error Type |
|----------|---------------|------------|
| Unknown subcommand | "unknown command: {subcommand}" | InvalidInputError |
| Missing path argument | "missing required argument: path" | InvalidInputError |
| Missing key argument | "missing required argument: key" | InvalidInputError |
| Missing value argument | "missing required argument: value" | InvalidInputError |
| Invalid UUID format | "invalid UUID format" | InvalidInputError |
| Wrong UUID version | "key must be UUIDv7" | InvalidInputError |
| Invalid JSON syntax | "invalid JSON format" | InvalidInputError |

### Library-Layer Errors (deferred)

| Error Type | Error Code | Example Scenarios |
|------------|------------|-------------------|
| PathError | `path_error` | File not found, parent directory missing, permissions denied |
| InvalidActionError | `invalid_action` | Transaction already active, no active transaction |
| CorruptDatabaseError | `corrupt_database` | Invalid header, malformed rows, checksum mismatch |
| KeyOrderingError | `key_ordering` | UUID timestamp violates ordering constraint |
| WriteError | `write_error` | Disk write failure, sudo context issues |
| ReadError | `read_error` | Disk read failure, I/O error |
| KeyNotFoundError | `key_not_found` | Key does not exist in database |
| TransactionActiveError | `transaction_active` | Key exists only in uncommitted transaction |
| InvalidDataError | `invalid_data` | JSON unmarshal failure during get |
| TombstonedError | `tombstoned` | Transaction tombstoned due to write failure |

### Error Output Format (FR-007)

All errors are formatted as: `Error: message` where message is `err.Error()`

Examples:
- `Error: invalid_input: key must be UUIDv7`
- `Error: path_error: parent directory does not exist`
- `Error: invalid_action: transaction already active`
- `Error: key_not_found: key not found in database`

---

## Data Flow

### Command Execution Flow

```
User Input (CLI invocation)
    |
    v
Argument Parsing (flag package)
    |
    v
CLI-Layer Validation (UUIDv7, JSON format)
    |
    +---> Validation Failed: Print error to stderr, exit 1
    |
    v
Library Function Call (create, open, begin, add, etc.)
    |
    v
Library-Layer Validation (path, state, constraints)
    |
    +---> Validation Failed: Return error
    |         |
    |         v
    |     CLI: Format error, print to stderr, exit 1
    |
    v
Library Operation Execution (write to disk, read from disk)
    |
    +---> Operation Failed: Return error
    |         |
    |         v
    |     CLI: Format error, print to stderr, exit 1
    |
    v
Success: 
    - create/begin/commit/savepoint/rollback/add: Silent, exit 0
    - get: Print pretty JSON to stdout, exit 0
```

### Transaction State Persistence

Transaction state is stored in the database file, not in CLI memory:

1. **Begin**: Writes PartialDataRow (ROW_START + 'T') to disk
2. **Add**: Appends key-value data to PartialDataRow or finalizes previous row
3. **Savepoint**: Marks current row with savepoint flag
4. **Commit**: Finalizes last row with TC or SC end control
5. **Rollback**: Finalizes last row with R0-R9 or S0-S9 end control

When reopening the database, the library automatically:
- Detects incomplete transaction (PartialDataRow or open end control)
- Recovers transaction state
- Makes transaction available via `db.GetActiveTx()`

---

## Summary

The CLI data model is minimal because most data structures are handled by the library:

**New Entities**: None (CLI wraps existing library entities)

**New Validation Rules**:
- UUIDv7 format validation (FR-003)
- JSON format validation (FR-004)
- Command argument presence validation

**State Transitions**: Transaction lifecycle managed by library, persisted in database file

**Error Mapping**: CLI errors use InvalidInputError; library errors pass through with original codes

This lightweight data model reflects the CLI's design as a thin wrapper that delegates complex logic to the underlying frozenDB library.
