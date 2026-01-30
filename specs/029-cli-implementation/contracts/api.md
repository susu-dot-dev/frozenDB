# CLI API Specification

## Overview

This document specifies the complete command-line interface for the `frozendb` binary. All commands follow Unix conventions: silent success (exit 0) except for `get` which outputs data to stdout, and errors to stderr (exit 1) with structured format.

---

## Command Reference

### 1. frozendb create

**Purpose**: Initialize a new frozenDB database file

**Syntax**:
```bash
frozendb create <path>
```

**Arguments**:
- `<path>`: Filesystem path where the database file will be created (required)
  - Must end with `.fdb` extension (validated by library)
  - Must not already exist (validated by library)
  - Parent directory must exist and be writable (validated by library)

**Options**: None

**Behavior**:
- Creates a new database file with:
  - 64-byte header (signature, version, row_size=4096, skew_ms=5000)
  - Initial checksum row covering the header
- Sets file attributes (append-only, immutable flags)
- Requires sudo elevation for attribute setting

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| PathError | `path_error` | "parent directory does not exist" |
| PathError | `path_error` | "file already exists at path" |
| PathError | `path_error` | "path is not writable" |
| WriteError | `write_error` | "sudo-elevated user context required to create frozenDB files" |
| WriteError | `write_error` | "direct root execution not allowed. Use sudo <username>" |
| InvalidInputError | `invalid_input` | "path must end with .fdb extension" |

**Examples**:
```bash
# Success
$ sudo frozendb create /data/mydb.fdb
$ echo $?
0

# Error: file exists
$ sudo frozendb create /data/mydb.fdb
# Error: file already exists at path
$ echo $?
1

# Error: missing sudo
$ frozendb create /data/newdb.fdb
Error: write_error: sudo-elevated user context required to create frozenDB files
$ echo $?
1
```

**Implementation Notes**:
- Uses `internal.Create()` from `internal/frozendb/create.go`
- Default row_size: 4096 bytes
- Default skew_ms: 5000 milliseconds
- No CLI flags for customizing these defaults (users can use library directly for custom configs)
- **Testing Note**: This command requires sudo elevation and cannot be reliably tested in automated spec tests. FR-001 will be validated manually rather than via spec tests.

---

### 2. frozendb begin

**Purpose**: Start a new transaction on an existing database

**Syntax**:
```bash
frozendb begin --path <file>
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)

**Options**: None

**Behavior**:
- Opens database in write mode
- Checks if transaction already active (error if yes)
- Begins a new transaction
- Persists transaction start to disk (PartialDataRow with ROW_START + 'T')
- Closes database (transaction state persisted in file)

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| PathError | `path_error` | "file does not exist" |
| InvalidActionError | `invalid_action` | "transaction already active" |
| CorruptDatabaseError | `corrupt_database` | "invalid header format" |
| WriteError | `write_error` | "failed to acquire exclusive lock" |

**Examples**:
```bash
# Success
$ frozendb begin --path /data/mydb.fdb
$ echo $?
0

# Error: transaction already active
$ frozendb begin --path /data/mydb.fdb
Error: invalid_action: transaction already active
$ echo $?
1

# Error: file not found
$ frozendb begin --path /data/nonexistent.fdb
Error: path_error: file does not exist
$ echo $?
1
```

**Implementation Notes**:
- Opens database with `frozendb.NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)`
- Checks `db.GetActiveTx() == nil` before calling `db.BeginTx()`
- Transaction persisted to disk; CLI can exit after begin

---

### 3. frozendb commit

**Purpose**: Commit the active transaction to the database

**Syntax**:
```bash
frozendb commit --path <file>
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)

**Options**: None

**Behavior**:
- Opens database in write mode
- Recovers active transaction automatically
- Checks if transaction active (error if no)
- Commits the transaction
  - **Empty transaction** (no add calls): Creates NullRow
  - **Data transaction** (with add calls): Finalizes last row with TC or SC end control
- Closes database

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| PathError | `path_error` | "file does not exist" |
| InvalidActionError | `invalid_action` | "no active transaction" |
| TombstonedError | `tombstoned` | "transaction is tombstoned due to write failure" |
| WriteError | `write_error` | "failed to write commit row" |

**Examples**:
```bash
# Success (empty transaction)
$ frozendb begin --path /data/mydb.fdb
$ frozendb commit --path /data/mydb.fdb
$ echo $?
0

# Success (with data)
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"name":"test"}'
$ frozendb commit --path /data/mydb.fdb
$ echo $?
0

# Error: no active transaction
$ frozendb commit --path /data/mydb.fdb
Error: invalid_action: no active transaction
$ echo $?
1
```

**Implementation Notes**:
- Opens database with `frozendb.NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)`
- Gets active transaction via `db.GetActiveTx()`
- Calls `tx.Commit()` if transaction exists

---

### 4. frozendb savepoint

**Purpose**: Create a savepoint at the current position in the active transaction

**Syntax**:
```bash
frozendb savepoint --path <file>
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)

**Options**: None

**Behavior**:
- Opens database in write mode
- Recovers active transaction automatically
- Checks if transaction active (error if no)
- Marks current row as a savepoint (max 9 savepoints per transaction)
- Closes database

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| PathError | `path_error` | "file does not exist" |
| InvalidActionError | `invalid_action` | "no active transaction" |
| InvalidActionError | `invalid_action` | "transaction has no rows to mark as savepoint" |
| InvalidInputError | `invalid_input` | "maximum of 9 savepoints per transaction" |
| TombstonedError | `tombstoned` | "transaction is tombstoned due to write failure" |

**Examples**:
```bash
# Success
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"v":1}'
$ frozendb savepoint --path /data/mydb.fdb
$ echo $?
0

# Error: no rows yet
$ frozendb begin --path /data/mydb.fdb
$ frozendb savepoint --path /data/mydb.fdb
Error: invalid_action: transaction has no rows to mark as savepoint
$ echo $?
1

# Error: too many savepoints
$ # After creating 9 savepoints:
$ frozendb savepoint --path /data/mydb.fdb
Error: invalid_input: maximum of 9 savepoints per transaction
$ echo $?
1
```

**Implementation Notes**:
- Opens database with `frozendb.NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)`
- Gets active transaction via `db.GetActiveTx()`
- Calls `tx.Savepoint()` if transaction exists
- Savepoint numbers are 1-indexed (1st savepoint = savepoint 1, etc.)

---

### 5. frozendb rollback

**Purpose**: Rollback the active transaction to a savepoint or to the beginning

**Syntax**:
```bash
frozendb rollback --path <file> [savepoint_id]
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)
- `[savepoint_id]`: Savepoint number (0-9) to rollback to (optional, default: 0)
  - `0`: Full rollback (all rows invalidated)
  - `1-9`: Partial rollback to specified savepoint (rows after savepoint invalidated)

**Options**: None

**Behavior**:
- Opens database in write mode
- Recovers active transaction automatically
- Checks if transaction active (error if no)
- Validates savepoint_id (0-9, must exist if > 0)
- Rolls back to specified savepoint
  - **Full rollback (0)**: Invalidates all rows, closes transaction
  - **Partial rollback (N)**: Invalidates rows after savepoint N, closes transaction
- Closes database

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| PathError | `path_error` | "file does not exist" |
| InvalidActionError | `invalid_action` | "no active transaction" |
| InvalidInputError | `invalid_input` | "savepointId must be between 0 and 9" |
| InvalidInputError | `invalid_input` | "invalid savepoint: savepoint 3 does not exist" |
| TombstonedError | `tombstoned` | "transaction is tombstoned due to write failure" |

**Examples**:
```bash
# Success: full rollback
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"v":1}'
$ frozendb rollback --path /data/mydb.fdb
$ echo $?
0

# Success: partial rollback
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"v":1}'
$ frozendb savepoint --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8124-456789abcdef '{"v":2}'
$ frozendb rollback --path /data/mydb.fdb 1
$ echo $?
0

# Error: savepoint doesn't exist
$ frozendb rollback --path /data/mydb.fdb 5
Error: invalid_input: invalid savepoint: savepoint 5 does not exist
$ echo $?
1

# Error: invalid savepoint number
$ frozendb rollback --path /data/mydb.fdb 10
Error: invalid_input: savepointId must be between 0 and 9
$ echo $?
1
```

**Implementation Notes**:
- Opens database with `frozendb.NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)`
- Gets active transaction via `db.GetActiveTx()`
- Parses savepoint_id from positional argument (default 0 if not provided)
- Calls `tx.Rollback(savepointId)` if transaction exists

---

### 6. frozendb add

**Purpose**: Insert a key-value pair into the active transaction

**Syntax**:
```bash
frozendb add --path <file> <key> <value>
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)
- `<key>`: UUIDv7 string identifier for the row (required)
- `<value>`: JSON-formatted value as a shell-escaped string (required)

**Options**: None

**Behavior**:
- CLI validates UUIDv7 format and JSON syntax before calling library
- Opens database in write mode
- Recovers active transaction automatically
- Checks if transaction active (error if no)
- Adds row to transaction with key-value pair
- Closes database (row persisted to disk)

**Exit Codes**:
- `0`: Success (silent output)
- `1`: Failure (error to stderr)

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| InvalidInputError | `invalid_input` | "missing required argument: key" |
| InvalidInputError | `invalid_input` | "missing required argument: value" |
| InvalidInputError | `invalid_input` | "invalid UUID format" |
| InvalidInputError | `invalid_input` | "key must be UUIDv7" |
| InvalidInputError | `invalid_input` | "invalid JSON format" |
| PathError | `path_error` | "file does not exist" |
| InvalidActionError | `invalid_action` | "no active transaction" |
| KeyOrderingError | `key_ordering` | "UUID timestamp violates ordering constraint" |
| InvalidInputError | `invalid_input` | "transaction cannot exceed 100 rows" |
| WriteError | `write_error` | "failed to write data row" |
| TombstonedError | `tombstoned` | "transaction is tombstoned due to write failure" |

**Examples**:
```bash
# Success
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"name":"test","count":42}'
$ echo $?
0

# Success: various JSON types
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8124-456789abcdef '"string value"'
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8125-456789abcdef '123'
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8126-456789abcdef 'true'
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8127-456789abcdef 'null'
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8128-456789abcdef '[1,2,3]'
$ echo $?
0

# Error: invalid UUID format
$ frozendb add --path /data/mydb.fdb not-a-uuid '{"v":1}'
Error: invalid_input: invalid UUID format
$ echo $?
1

# Error: wrong UUID version
$ frozendb add --path /data/mydb.fdb 00000000-0000-4000-8000-000000000000 '{"v":1}'
Error: invalid_input: key must be UUIDv7
$ echo $?
1

# Error: invalid JSON
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8129-456789abcdef '{invalid}'
Error: invalid_input: invalid JSON format
$ echo $?
1

# Error: no active transaction
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-812a-456789abcdef '{"v":1}'
Error: invalid_action: no active transaction
$ echo $?
1

# Error: timestamp ordering violation
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8000-456789abcdef '{"v":1}'
Error: key_ordering: UUID timestamp violates ordering constraint
$ echo $?
1
```

**Implementation Notes**:
- CLI validates UUID format with `uuid.Parse()` and version check
- CLI validates JSON syntax with `json.Valid()`
- Opens database with `frozendb.NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)`
- Gets active transaction via `db.GetActiveTx()`
- Calls `tx.AddRow(key, json.RawMessage(value))` if transaction exists
- Library handles timestamp ordering validation, row count limits, and disk writes

**Shell Escaping Notes**:
- Single quotes preserve literal strings: `'{"name":"test"}'`
- Double quotes allow variable expansion: `"{\"name\":\"$NAME\"}"`
- Spaces in JSON require quotes: `'{"key": "value with spaces"}'`

---

### 7. frozendb get

**Purpose**: Retrieve and display a value by UUIDv7 key

**Syntax**:
```bash
frozendb get --path <file> <key>
```

**Arguments**:
- `--path <file>`: Path to existing frozenDB database file (required)
- `<key>`: UUIDv7 string identifier to look up (required)

**Options**: None

**Behavior**:
- CLI validates UUIDv7 format before calling library
- Opens database in read mode (concurrent reads allowed)
- Queries for the specified key
- If found: Prints pretty-formatted JSON value to stdout
- If not found or error: Prints error to stderr
- Closes database

**Exit Codes**:
- `0`: Success (JSON output to stdout)
- `1`: Failure (error to stderr)

**Output Format** (Success):
Pretty-printed JSON with 2-space indentation:
```json
{
  "name": "example",
  "count": 42
}
```

**Error Conditions**:

| Error Type | Code | Example Message |
|------------|------|-----------------|
| InvalidInputError | `invalid_input` | "missing required argument: key" |
| InvalidInputError | `invalid_input` | "invalid UUID format" |
| InvalidInputError | `invalid_input` | "key must be UUIDv7" |
| PathError | `path_error` | "file does not exist" |
| KeyNotFoundError | `key_not_found` | "key not found in database" |
| TransactionActiveError | `transaction_active` | "key exists only in uncommitted transaction" |
| InvalidDataError | `invalid_data` | "failed to unmarshal stored value" |
| ReadError | `read_error` | "failed to read from disk" |
| CorruptDatabaseError | `corrupt_database` | "checksum mismatch detected" |

**Examples**:
```bash
# Success: object
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef
{
  "name": "test",
  "count": 42
}
$ echo $?
0

# Success: array
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8124-456789abcdef
[
  1,
  2,
  3
]
$ echo $?
0

# Success: string
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8125-456789abcdef
"string value"
$ echo $?
0

# Success: number
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8126-456789abcdef
123
$ echo $?
0

# Error: key not found
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8999-456789abcdef
Error: key_not_found: key not found in database
$ echo $?
1

# Error: invalid UUID
$ frozendb get --path /data/mydb.fdb not-a-uuid
Error: invalid_input: invalid UUID format
$ echo $?
1

# Error: key in active transaction
$ frozendb begin --path /data/mydb.fdb
$ frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8127-456789abcdef '{"v":1}'
$ frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8127-456789abcdef
Error: transaction_active: key exists only in uncommitted transaction
$ echo $?
1
```

**Implementation Notes**:
- CLI validates UUID format with `uuid.Parse()` and version check
- Opens database with `frozendb.NewFrozenDB(path, MODE_READ, FinderStrategySimple)`
- Calls `db.Get(key, &result)` where result is `interface{}`
- Pretty-prints with `json.MarshalIndent(result, "", "  ")`
- Active transactions are not visible to `get` (returns TransactionActiveError)

**Piping Examples**:
```bash
# Extract specific field with jq
$ frozendb get --path /data/mydb.fdb <key> | jq '.name'
"test"

# Pretty-print is default (no need for jq -C)
$ frozendb get --path /data/mydb.fdb <key>
{
  "nested": {
    "value": 42
  }
}

# Redirect to file
$ frozendb get --path /data/mydb.fdb <key> > output.json
```

---

## Common Patterns

### Transaction Workflow

```bash
# Create database
sudo frozendb create /data/mydb.fdb

# Begin transaction
frozendb begin --path /data/mydb.fdb

# Add multiple rows
frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef '{"user":"alice"}'
frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8124-456789abcdef '{"user":"bob"}'

# Commit
frozendb commit --path /data/mydb.fdb

# Retrieve data
frozendb get --path /data/mydb.fdb 01934567-89ab-7def-8123-456789abcdef
```

### Savepoint and Rollback

```bash
# Begin transaction
frozendb begin --path /data/mydb.fdb

# Add row and create savepoint
frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8125-456789abcdef '{"step":1}'
frozendb savepoint --path /data/mydb.fdb

# Add more rows
frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8126-456789abcdef '{"step":2}'
frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8127-456789abcdef '{"step":3}'

# Rollback to savepoint 1 (keeps step 1, discards steps 2 and 3)
frozendb rollback --path /data/mydb.fdb 1
```

### Error Handling in Scripts

```bash
#!/bin/bash

# Check if create succeeds
if sudo frozendb create /data/mydb.fdb; then
    echo "Database created successfully"
else
    echo "Failed to create database" >&2
    exit 1
fi

# Begin transaction with error handling
if frozendb begin --path /data/mydb.fdb; then
    # Add rows
    frozendb add --path /data/mydb.fdb 01934567-89ab-7def-8128-456789abcdef '{"data":"value"}' || {
        echo "Failed to add row" >&2
        frozendb rollback --path /data/mydb.fdb
        exit 1
    }
    
    # Commit
    frozendb commit --path /data/mydb.fdb || {
        echo "Failed to commit" >&2
        exit 1
    }
fi
```

---

## Integration Notes

### Thread Safety
- Each CLI invocation opens and closes the database
- No persistent CLI process, so no multi-threading concerns
- Library handles file locking (MODE_WRITE = exclusive lock)
- Multiple concurrent reads allowed (MODE_READ = no lock)

### Performance Characteristics
- Database open/close overhead per command (~1-10ms typically)
- Transaction recovery overhead on open if transaction active (~1-50ms depending on transaction size)
- `FinderStrategySimple` used for minimal memory footprint
- Get operations: O(n) scan time (acceptable for CLI use case)

### Compatibility
- Requires Linux (file locking via syscall)
- Go 1.25.5 for UUIDv7 support
- Shell must support argument quoting for JSON values

---

## Summary

The frozenDB CLI provides seven commands covering the complete lifecycle of database operations:

**Lifecycle Commands**:
- `create`: Initialize new database
- `begin`: Start transaction
- `add`: Insert data
- `savepoint`: Mark restore point
- `commit` / `rollback`: Finalize or revert transaction
- `get`: Query data

**Design Principles**:
- Silent success (Unix convention)
- Structured errors with codes (programmatic handling)
- Minimal CLI validation (defer to library)
- Stateless execution (transaction state in file)
- Simple argument parsing (standard library)

**Error Handling**:
- All errors formatted as `Error: message` where message is err.Error()
- Exit code 0 for success, 1 for any failure
- Errors to stderr, data to stdout (pipeable)
