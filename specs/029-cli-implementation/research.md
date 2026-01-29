# Research: CLI Implementation

## Overview

This document consolidates research findings for implementing the frozenDB CLI. The CLI is designed as a thin wrapper around the existing frozenDB library, deferring all business logic, validation (beyond basic input format), and database operations to the library layer.

---

## 1. Database Creation

### Decision: Use internal.Create() via internal package

**Rationale**: The database creation functionality already exists in `internal/frozendb/create.go` as the `Create()` function. This function handles all aspects of database initialization including:
- Path validation and filesystem checks
- Sudo context detection and requirements
- Header creation with proper format
- Initial checksum row insertion
- File attribute setting (append-only, immutable flags)
- Comprehensive error handling with structured error types

The CLI will import the internal package directly for the `create` command since this functionality is intentionally not exported from the public API.

**Alternatives Considered**:
1. **Export Create() from pkg/frozendb**: Rejected because database creation is explicitly scoped as a CLI-only operation per the library design. The public API focuses on opening existing databases.
2. **Reimplement create logic in CLI**: Rejected because it duplicates existing, tested logic and violates the principle of deferring to the library.

**Implementation Approach**:
```go
import internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"

// In create command:
config := internal.NewCreateConfig(path, defaultRowSize, defaultSkewMs)
if err := internal.Create(config); err != nil {
    // Format and print error to stderr
}
```

**Default Values**:
- `rowSize`: 4096 (reasonable default for general use, allows ~3.8KB JSON values)
- `skewMs`: 5000 (5 seconds, accommodates reasonable clock skew in distributed systems)

These defaults follow common frozenDB usage patterns and can be hardcoded in the CLI.

---

## 2. Library API Integration

### Decision: Use pkg/frozendb for all read/write operations

**Rationale**: The public API in `pkg/frozendb` exports all necessary functionality for CLI operations except database creation. This API is designed for external consumers and provides the appropriate abstraction level.

**Key Functions Available**:

#### Database Operations:
- `NewFrozenDB(path, mode, strategy)` - Opens database
- `db.Close()` - Closes database connection
- `db.Get(key, value)` - Retrieves value by UUIDv7 key

#### Transaction Operations:
- `db.BeginTx()` - Starts new transaction (automatically calls Begin())
- `tx.AddRow(key, value)` - Inserts key-value pair
- `tx.Commit()` - Commits transaction (handles empty vs data transactions)
- `tx.Rollback(savepointId)` - Rolls back to savepoint (0 = full rollback)
- `tx.Savepoint()` - Creates savepoint
- `tx.Close()` - Closes transaction channel

**Library Behavior Notes**:

1. **Empty Transactions**: When `BeginTx()` is followed immediately by `Commit()` with no `AddRow()` calls, the library automatically creates a NullRow. This is the correct behavior per the file format specification.

2. **Transaction Recovery**: When opening a database in write mode, the library automatically recovers any incomplete transaction state from the file. The CLI should check `db.GetActiveTx()` after opening to determine if a transaction is already active.

3. **Automatic Checksum Insertion**: The library automatically inserts checksum rows every 10,000 data rows. The CLI does not need to handle this.

4. **File Locking**: Opening in `MODE_WRITE` acquires an exclusive lock. The library handles all locking; the CLI only needs to pass the mode.

**Alternatives Considered**:
1. **Use internal package for all operations**: Rejected because the public API exists specifically for this use case and provides the correct abstraction.
2. **Implement transaction logic in CLI**: Rejected because the library handles all state management, validation, and file operations.

---

## 3. Command-Line Argument Parsing

### Decision: Use Go standard library flag package

**Rationale**: The CLI has simple argument requirements that fit well with the standard `flag` package:
- Positional arguments (subcommand, path, key, value)
- One flag type (`--path` used by most commands)
- No complex flag combinations or dependencies
- Standard Go idioms familiar to Go developers

**Parsing Strategy**:
1. Parse subcommand from `os.Args[1]`
2. Use `flag.NewFlagSet()` for each subcommand
3. Parse flags, then access positional args from `flagSet.Args()`

**Example Structure**:
```go
// Subcommand routing
switch subcommand {
case "create":
    // No flags, one positional arg
    if len(os.Args) < 3 {
        printError("missing path argument")
        os.Exit(1)
    }
    path := os.Args[2]
    
case "add":
    // --path flag, two positional args
    fs := flag.NewFlagSet("add", flag.ExitOnError)
    pathFlag := fs.String("path", "", "database file path")
    fs.Parse(os.Args[2:])
    args := fs.Args()
    if len(args) < 2 {
        printError("missing key or value argument")
        os.Exit(1)
    }
    key, value := args[0], args[1]
}
```

**Alternatives Considered**:
1. **cobra/viper**: Rejected as over-engineered for 7 simple commands with minimal flag complexity.
2. **urfave/cli**: Rejected for the same reason; standard library is sufficient and reduces dependencies.
3. **Manual parsing**: Rejected because flag package provides error handling and usage generation.

---

## 4. Input Validation Strategy

### Decision: CLI validates format; library validates semantics

**Rationale**: Clear separation of concerns maximizes code reuse and correctness:

**CLI Layer Validates**:
- UUIDv7 string format (using `uuid.Parse()` and version check)
- JSON syntax (using `json.Valid()`)
- Argument presence (required flags and positional args)
- Basic command structure

**Library Layer Validates** (CLI defers to library):
- Path validity and filesystem checks
- UUID timestamp ordering constraints
- Transaction state transitions
- Row count limits and savepoint limits
- Database file format and integrity
- Value size constraints relative to row_size

**Implementation Approach**:
```go
// CLI validation example:
func validateUUIDv7(keyStr string) (uuid.UUID, error) {
    key, err := uuid.Parse(keyStr)
    if err != nil {
        return uuid.Nil, NewInvalidInputError("invalid UUID format", err)
    }
    if key.Version() != 7 {
        return uuid.Nil, NewInvalidInputError("key must be UUIDv7", nil)
    }
    return key, nil
}

func validateJSON(valueStr string) (json.RawMessage, error) {
    if !json.Valid([]byte(valueStr)) {
        return nil, NewInvalidInputError("invalid JSON format", nil)
    }
    return json.RawMessage(valueStr), nil
}
```

**Alternatives Considered**:
1. **CLI validates everything**: Rejected because it duplicates library validation logic (path checks, ordering constraints, etc.) and creates maintenance burden.
2. **Library validates everything**: Rejected because library would need to handle string parsing (UUIDs, JSON) which is properly a CLI concern for this use case.
3. **No CLI validation**: Rejected because it produces unhelpful error messages for format errors that can be caught immediately.

---

## 5. Error Formatting and Exit Codes

### Decision: Use simple error format with err.Error()

**Rationale**: The library provides structured errors via `FrozenDBError` base type with error codes. The CLI formats these errors with a simple "Error: " prefix followed by the error's Error() method output.

**Format**: `Error: message` where message is `err.Error()`

**Implementation Strategy**:
```go
func formatError(err error) string {
    return fmt.Sprintf("Error: %s", err.Error())
}

func printError(err error) {
    fmt.Fprintln(os.Stderr, formatError(err))
    os.Exit(1)
}
```

**Exit Codes**:
- `0`: Success (silent output for create/transaction commands, JSON output for get)
- `1`: Any error (CLI validation failure or library error)

**Error Output Examples**:
All errors are formatted with the "Error: " prefix:
- CLI argument errors → `Error: invalid_input: missing required argument: key`
- CLI UUID format errors → `Error: invalid_input: invalid UUID format`
- CLI JSON format errors → `Error: invalid_input: invalid JSON format`
- Library errors → `Error: path_error: database file does not exist (caused by: ...)`

**Alternatives Considered**:
1. **Different exit codes for different error types**: Rejected for simplicity; binary success/failure is sufficient for CLI tools.
2. **Custom CLI error codes separate from library**: Rejected because it creates confusion about which codes mean what.
3. **Structured format with explicit code field**: Rejected in favor of simpler format that includes error details in the message.

---

## 6. Transaction State Management

### Decision: Open database, check for existing transaction, perform operation, close

**Rationale**: Each CLI command operates independently, opening and closing the database within the command execution. This approach:
- Ensures proper resource cleanup
- Handles incomplete transactions from previous runs
- Maintains transaction state in the database file (not in memory)
- Follows Unix tool philosophy (stateless commands)

**Transaction Command Pattern**:
```go
// begin command:
db := frozendb.NewFrozenDB(path, frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
defer db.Close()

// Check if transaction already active
if db.GetActiveTx() != nil {
    return InvalidActionError("transaction already active")
}

tx, err := db.BeginTx()
// Transaction is now active and persisted to disk
// Exit without commit/rollback leaves transaction active for next command

// commit command:
db := frozendb.NewFrozenDB(path, frozendb.MODE_WRITE, frozendb.FinderStrategySimple)
defer db.Close()

tx := db.GetActiveTx()
if tx == nil {
    return InvalidActionError("no active transaction")
}

err := tx.Commit()
// Transaction is now committed and closed
```

**Transaction Lifecycle Across Commands**:
1. `frozendb begin` → Opens db, starts transaction, persists partial row, closes db
2. `frozendb add` → Opens db, recovers transaction, adds row, closes db
3. `frozendb savepoint` → Opens db, recovers transaction, creates savepoint, closes db
4. `frozendb commit` → Opens db, recovers transaction, commits, closes db

The library's automatic transaction recovery on database open makes this pattern work seamlessly.

**Alternatives Considered**:
1. **Long-lived daemon process**: Rejected because it adds complexity (IPC, process management) without clear benefit.
2. **Shared memory for transaction state**: Rejected because the database file itself is the source of truth.
3. **Lock files for state tracking**: Rejected because the library already uses file locks and transaction recovery.

---

## 7. Output Formatting

### Decision: Silent success except for get; pretty-print JSON for get

**Rationale**: Follows Unix tool conventions:
- **Silent success**: Commands that modify state (`create`, `begin`, `commit`, `savepoint`, `rollback`, `add`) produce no output on success, only exit code 0
- **Data output**: `get` command outputs to stdout so it can be piped to other tools
- **Error output**: All errors go to stderr with structured format

**JSON Pretty-Printing for get**:
```go
func prettyPrintJSON(value json.RawMessage) error {
    var parsed interface{}
    if err := json.Unmarshal(value, &parsed); err != nil {
        return err
    }
    pretty, err := json.MarshalIndent(parsed, "", "  ")
    if err != nil {
        return err
    }
    fmt.Println(string(pretty))
    return nil
}
```

**Alternatives Considered**:
1. **Verbose mode for all commands**: Rejected because it adds complexity and violates Unix conventions.
2. **Compact JSON for get**: Rejected because requirement FR-006 explicitly mandates pretty-printed output.
3. **Structured output format (TOML, YAML)**: Rejected because requirement specifies JSON and it's simplest for piping.

---

## 8. Finder Strategy Selection

### Decision: Use FinderStrategySimple for all CLI operations

**Rationale**: CLI operations are typically one-off commands, not long-running processes. The `FinderStrategySimple` is appropriate because:
- **Fixed memory**: O(row_size) memory usage regardless of database size
- **CLI use case**: Single query per execution, no benefit from building in-memory index
- **Fast enough**: O(n) search is acceptable for CLI interactive use
- **Simplicity**: No memory management or index building complexity

**Implementation**:
```go
db, err := frozendb.NewFrozenDB(path, mode, frozendb.FinderStrategySimple)
```

**Alternatives Considered**:
1. **FinderStrategyInMemory**: Rejected because it allocates ~40 bytes per row which is wasteful for single-query CLI usage.
2. **FinderStrategyBinarySearch**: Rejected because it requires sorted keys and provides marginal benefit for CLI use case.
3. **User-selectable strategy**: Rejected because it adds unnecessary complexity; users can use the library directly if they need different strategies.

---

## 9. Path Handling

### Decision: Pass path as-is to library, no CLI-layer normalization

**Rationale**: Per clarification in spec.md line 15, paths should be "accepted as-is without any path processing." The library handles all path validation including:
- Parent directory existence checks
- File existence checks (create vs open)
- Permission validation
- Extension validation (.fdb requirement)

**Implementation**:
```go
// CLI layer - no processing:
path := os.Args[2] // or from --path flag
err := internal.Create(internal.NewCreateConfig(path, rowSize, skewMs))
// Library handles all path validation

// For open operations:
db, err := frozendb.NewFrozenDB(path, mode, strategy)
// Library handles all path validation
```

**Alternatives Considered**:
1. **filepath.Clean() normalization**: Rejected per spec requirement to pass path as-is.
2. **Resolve relative paths to absolute**: Rejected per spec requirement.
3. **Validate .fdb extension at CLI layer**: Rejected because library already validates this.

---

## 10. Sudo and Permissions Handling

### Decision: Rely on library error messages for permission issues

**Rationale**: The `create` command requires sudo for setting append-only file attributes. The library's `Create()` function already:
- Detects sudo context via `os.Getuid() == 0`
- Validates calling user context
- Returns structured errors for permission issues
- Prevents direct root execution

The CLI should not add additional sudo detection or handling; just pass through library errors.

**Implementation**:
```go
// CLI just calls create:
err := internal.Create(config)
if err != nil {
    printError(err) // Library error includes sudo context information
    os.Exit(1)
}
```

**Library Error Examples**:
- "sudo-elevated user context required to create frozenDB files"
- "direct root execution not allowed. Use sudo <username> to run as a specific user"
- "parent directory does not exist"

**Alternatives Considered**:
1. **CLI checks for sudo before calling create**: Rejected because it duplicates library logic.
2. **CLI wraps non-sudo execution with helpful message**: Rejected because library already provides clear error messages.
3. **CLI automatically retries with sudo**: Rejected because it's surprising behavior and security risk.

---

## Summary

The research confirms that the frozenDB CLI can be implemented as a thin wrapper with minimal code:

**CLI Responsibilities**:
- Parse command-line arguments using standard library
- Validate input format (UUIDv7 string, JSON syntax)
- Route to appropriate library functions
- Format and output errors to stderr
- Format and output data to stdout (get command only)

**Library Responsibilities** (delegated):
- All database operations (create, open, close)
- All transaction management (begin, commit, rollback, savepoint)
- All data operations (add, get)
- Path validation and filesystem checks
- UUID timestamp ordering validation
- Transaction state persistence and recovery
- File locking and concurrency control
- Data integrity and corruption detection

This design maximizes correctness and maintainability by deferring all complex logic to the well-tested library layer.
