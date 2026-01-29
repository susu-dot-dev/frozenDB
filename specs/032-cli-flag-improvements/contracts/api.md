# API Contracts: CLI Flag Improvements

**Feature**: 032-cli-flag-improvements  
**Date**: Thu Jan 29 2026  
**Purpose**: Complete API specification for CLI enhancements including method signatures, behavior, and usage examples

---

## Overview

This document specifies the CLI command interface changes for frozenDB. All commands follow Unix conventions: silent success (exit code 0), errors to stderr (exit code 1), and structured output to stdout where applicable.

---

## 1. Global Flag Specifications

### Flag: --path

**Purpose**: Specifies the database file path for all database operations.

**Position**: Can appear before or after the subcommand (FR-001)

**Format**:
```bash
--path <file-path>
```

**Parameters**:
- `<file-path>` (string, required): Absolute or relative path to frozenDB database file

**Usage Examples**:
```bash
# Path before subcommand
frozendb --path mydb.frz begin

# Path after subcommand
frozendb begin --path mydb.frz

# Path with other flags
frozendb --path mydb.frz --finder binary get <key>
frozendb get --finder binary --path mydb.frz <key>
```

**Error Conditions**:
- **Missing**: Exit code 1, stderr: "missing required flag: --path"
- **Duplicate**: Exit code 1, stderr: "duplicate flag: --path"
- **File not found** (context-dependent): Handled by existing database open logic

**Applicable Commands**: begin, commit, savepoint, rollback, add, get (NOT create)

---

### Flag: --finder

**Purpose**: Selects the finder strategy for runtime database index operations.

**Position**: Can appear before or after the subcommand (FR-001)

**Format**:
```bash
--finder <strategy>
```

**Parameters**:
- `<strategy>` (string, optional): Finder strategy name (case-insensitive)
  - Valid values: `simple`, `inmemory`, `binary`
  - Default: `binary` (BinarySearchFinder, per FR-005)

**Usage Examples**:
```bash
# Explicit binary (default)
frozendb --finder binary begin --path mydb.frz

# Finder before subcommand
frozendb --finder simple --path db.frz begin

# Finder after subcommand
frozendb begin --finder simple --path db.frz
```

**Error Conditions**:
- **Invalid value**: Exit code 1, stderr: "invalid finder strategy: <value> (valid: simple, inmemory, binary)"
- **Duplicate**: Exit code 1, stderr: "duplicate flag: --finder"

**Case Sensitivity**: Case-insensitive (FR-005, A-003)
- "simple", "Simple", "SIMPLE" all accepted
- "inmemory", "InMemory", "INMEMORY" all accepted
- "binary", "Binary", "BINARY" all accepted

**Applicable Commands**: begin, commit, savepoint, rollback, add, get (NOT create - finder is runtime-only)

---

## 2. Modified Command: add

### Command Signature

```bash
frozendb [--path <file>] [--finder <strategy>] add <key> <value>
frozendb add [--path <file>] [--finder <strategy>] <key> <value>
```

**Position Flexibility**: Global flags (`--path`, `--finder`) can appear before or after "add" subcommand (FR-001)

### Parameters

- `<key>` (string, required): UUIDv7 key OR special keyword "NOW" (case-insensitive)
  - UUIDv7: Standard UUID format with hyphens (e.g., `018d5c5a-1234-7890-abcd-ef0123456789`)
  - NOW keyword: Any case variation ("now", "NOW", "Now", "nOw") triggers auto-generation (FR-003)
- `<value>` (string, required): Valid JSON value

### NOW Keyword Behavior

**Purpose**: Automatically generates a UUIDv7 key using current system timestamp.

**Case Sensitivity**: Case-insensitive (FR-003)
- "NOW", "now", "Now", "nOw", etc. all trigger generation

**Generation Method**: `uuid.NewV7()` from github.com/google/uuid v1.6.0
- Uses Unix Epoch milliseconds for time component
- Adds 74 bits of randomness for uniqueness
- Maintains chronological ordering (UUIDv7 property)

**Usage Examples**:
```bash
# Generate key with NOW keyword
frozendb add --path db.frz NOW '{"event": "login"}'

# Case variations
frozendb add --path db.frz now '{"event": "logout"}'
frozendb add --path db.frz Now '{"event": "register"}'

# Capture generated key
key=$(frozendb add --path db.frz NOW '{"data": "value"}')
echo "Inserted key: $key"
```

### Success Output

**Changed Behavior** (FR-004): Add command now outputs the key to stdout.

**Format**: Single line containing UUID string with hyphens
```
018d5c5a-1234-7890-abcd-ef0123456789
```

**Output Rules**:
- Destination: stdout (NOT stderr)
- Format: Standard UUID string via `key.String()`
- Termination: Single newline character
- Consistency: Identical format for user-provided and NOW-generated keys

**Before/After Comparison**:
```bash
# BEFORE (spec 031 and earlier)
$ frozendb add --path db.frz <uuid> '{"x": 1}'
$ # Silent success - no output

# AFTER (spec 032)
$ frozendb add --path db.frz <uuid> '{"x": 1}'
018d5c5a-1234-7890-abcd-ef0123456789

$ frozendb add --path db.frz NOW '{"x": 1}'
018d5c5b-4567-7890-abcd-ef0123456789
```

### Error Conditions

| Condition | Exit Code | Error Message (stderr) |
|-----------|-----------|------------------------|
| Missing --path | 1 | "missing required flag: --path" |
| Missing key argument | 1 | "missing required argument: key" |
| Missing value argument | 1 | "missing required argument: value" |
| Invalid UUID format | 1 | "invalid UUID format" (existing) |
| Non-UUIDv7 key | 1 | "key must be UUIDv7" (existing) |
| Invalid JSON value | 1 | "invalid JSON format" (existing) |
| No active transaction | 1 | "no active transaction" (existing) |
| Duplicate key | 1 | Database error (existing) |
| UUID generation failure | 1 | "failed to generate UUIDv7: <reason>" |

### Success Conditions

- Exit code: 0
- Stdout: UUID key string (single line)
- Stderr: Empty
- Side effect: Row added to active transaction

### Thread Safety

Not applicable - CLI commands are single-process invocations. Concurrent safety is handled by database-level file locking (existing behavior).

---

## 3. Modified Commands: begin, commit, savepoint, rollback, get

### Finder Strategy Support

**Change**: These commands now accept optional `--finder` flag to select finder strategy.

**Signatures**:
```bash
frozendb [--path <file>] [--finder <strategy>] begin
frozendb [--path <file>] [--finder <strategy>] commit
frozendb [--path <file>] [--finder <strategy>] savepoint
frozendb [--path <file>] [--finder <strategy>] rollback [<savepoint-id>]
frozendb [--path <file>] [--finder <strategy>] get <key>
```

**Position Flexibility**: Both `--path` and `--finder` can appear before or after subcommand.

**Behavioral Changes**:
- Previously: Finder strategy hardcoded to `FinderStrategySimple`
- Now: Finder strategy determined by `--finder` flag (default: `FinderStrategyBinarySearch`)

**Example Usage**:
```bash
# Begin transaction with binary search finder (default)
frozendb begin --path db.frz

# Begin with explicit simple finder (memory-constrained)
frozendb begin --finder simple --path db.frz

# Get value with inmemory finder (performance)
frozendb --finder inmemory get --path db.frz <key>

# Commit with default finder
frozendb commit --path db.frz --finder binary
```

**Compatibility**: Existing commands continue to work - `--finder` is optional with sensible default.

---

## 4. Unchanged Command: create

### No Changes

The `create` command is **unchanged** by this feature:
- No `--path` flag (uses positional argument)
- No `--finder` flag (finder is runtime concern, not stored in database)

**Signature** (unchanged):
```bash
frozendb create <path>
```

**Rationale**: Finder strategy is a runtime decision for opening databases, not a database configuration stored in the file (per OS-001).

---

## 5. Flag Parsing Rules

### Ordering Flexibility

**Rule**: Global flags (`--path`, `--finder`) can appear in any position relative to subcommand.

**Valid Combinations**:
```bash
# All flags before subcommand
frozendb --path db.frz --finder binary begin

# All flags after subcommand
frozendb begin --path db.frz --finder binary

# Mixed positioning
frozendb --path db.frz begin --finder binary
frozendb --finder binary begin --path db.frz
```

**Invalid Combinations**:
```bash
# Duplicate flags
frozendb --path db1.frz --path db2.frz begin  # ERROR: duplicate --path

# Missing required flags
frozendb begin  # ERROR: missing --path (for commands requiring it)

# Invalid flag values
frozendb --finder invalid begin --path db.frz  # ERROR: invalid finder strategy
```

### Duplicate Detection

**Behavior**: Specifying the same flag twice results in error.

**Error Messages**:
- `--path` duplicated: "duplicate flag: --path"
- `--finder` duplicated: "duplicate flag: --finder"

**Exit Code**: 1 (error)

---

## 6. Backward Compatibility

### Compatibility Summary

| Aspect | Compatibility | Notes |
|--------|---------------|-------|
| Existing commands | ✅ Full | All existing commands work unchanged |
| Flag positioning | ✅ Backward compatible | Flags after subcommand still supported (existing syntax) |
| Add command output | ⚠️ Changed | Now outputs key to stdout (was silent) |
| Finder strategy | ✅ Improved default | Default changed from Simple to BinarySearch |
| Error messages | ✅ Compatible | New errors for new features only |

### Migration Notes

**Scripts using `add` command**:
- **Before**: `frozendb add --path db.frz <key> <value>` (silent success)
- **After**: `frozendb add --path db.frz <key> <value>` (outputs key to stdout)
- **Impact**: Scripts may need to suppress/capture stdout if they don't expect output
- **Mitigation**: Redirect stdout if needed: `frozendb add --path db.frz <key> <value> >/dev/null`

**Default finder strategy**:
- Default finder changed from Simple to BinarySearch
- Existing commands work unchanged (--finder is optional)

---

## 7. Integration Notes

### Shell Scripting

**Capturing NOW-generated keys**:
```bash
#!/bin/bash
key=$(frozendb add --path events.frz NOW '{"timestamp": "2026-01-29", "event": "login"}')
echo "Event logged with key: $key"

# Use key in subsequent operations
frozendb get --path events.frz "$key"
```

**Batch operations**:
```bash
#!/bin/bash
db="metrics.frz"
frozendb begin --path "$db"

for metric in latency throughput errors; do
    key=$(frozendb add --path "$db" NOW "{\"metric\": \"$metric\", \"value\": $RANDOM}")
    echo "Logged $metric with key: $key"
done

frozendb commit --path "$db"
```

### Error Handling in Scripts

```bash
#!/bin/bash
set -e  # Exit on error

key=$(frozendb add --path db.frz NOW '{"data": "value"}' 2>&1)
if [ $? -eq 0 ]; then
    echo "Success: $key"
else
    echo "Error: $key" >&2
    exit 1
fi
```

---

## 8. Summary of API Changes

### New Features

1. **Flexible flag positioning** (FR-001): `--path` and `--finder` work before or after subcommand
2. **NOW keyword** (FR-003): Auto-generates UUIDv7 keys in add command (case-insensitive)
3. **Add command output** (FR-004): Outputs inserted key to stdout
4. **Finder strategy selection** (FR-005, FR-006): `--finder` flag for runtime strategy choice

### Modified Signatures

**Before**:
```bash
frozendb add --path <file> <key> <value>  # Silent success
frozendb begin --path <file>              # Hardcoded Simple finder
```

**After**:
```bash
frozendb [--path <file>] [--finder <strategy>] add <key|NOW> <value>  # Outputs key
frozendb [--finder <strategy>] begin [--path <file>]                   # Selectable finder, flexible flags
```

### Compatibility Matrix

| Feature | Backward Compatible | Forward Compatible | Notes |
|---------|---------------------|-------------------|-------|
| Flag positioning | ✅ Yes | N/A | Old syntax still works |
| --finder flag | ✅ Yes (optional) | N/A | Defaults to binary |
| NOW keyword | ✅ Yes (additive) | N/A | Existing UUIDs still work |
| Add output | ⚠️ Changed | N/A | Now outputs key (was silent) |

**Breaking Change**: Add command now outputs to stdout. Scripts relying on silent success may need adjustment (redirect stdout if unwanted).
