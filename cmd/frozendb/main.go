package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/google/uuid"
	internal_frozendb "github.com/susu-dot-dev/frozenDB/internal/frozendb"
	pkg_frozendb "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

// Default configuration values for database creation
const (
	defaultRowSize = 4096 // Default row size in bytes
	defaultSkewMs  = 5000 // Default time skew in milliseconds (5 seconds)
)

// main is the CLI entry point. Routes to subcommand handlers.
// Follows Unix conventions: silent success, errors to stderr, exit codes 0/1.
func main() {
	// Require at least one argument (the subcommand)
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: frozendb <command> [arguments]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  create <path>                  - Initialize new database")
		fmt.Fprintln(os.Stderr, "  begin --path <file>            - Start transaction")
		fmt.Fprintln(os.Stderr, "  commit --path <file>           - Commit transaction")
		fmt.Fprintln(os.Stderr, "  savepoint --path <file>        - Create savepoint")
		fmt.Fprintln(os.Stderr, "  rollback --path <file> [id]    - Rollback transaction")
		fmt.Fprintln(os.Stderr, "  add --path <file> <key> <val>  - Insert key-value pair")
		fmt.Fprintln(os.Stderr, "  get --path <file> <key>        - Retrieve value by key")
		os.Exit(1)
	}

	// Route to subcommand handler
	subcommand := os.Args[1]
	switch subcommand {
	case "create":
		handleCreate()
	case "begin":
		handleBegin()
	case "commit":
		handleCommit()
	case "savepoint":
		handleSavepoint()
	case "rollback":
		handleRollback()
	case "add":
		handleAdd()
	case "get":
		handleGet()
	default:
		printError(pkg_frozendb.NewInvalidInputError(fmt.Sprintf("unknown command: %s", subcommand), nil))
	}
}

// handleCreate implements the 'create' command.
// Creates a new database file with default row_size and skew_ms.
// Requires sudo elevation for setting file attributes.
func handleCreate() {
	// Expect exactly one positional argument: the path
	if len(os.Args) < 3 {
		printError(pkg_frozendb.NewInvalidInputError("missing required argument: path", nil))
	}
	if len(os.Args) > 3 {
		printError(pkg_frozendb.NewInvalidInputError("too many arguments for create command", nil))
	}

	path := os.Args[2]

	// Create config with default values
	config := internal_frozendb.NewCreateConfig(path, defaultRowSize, defaultSkewMs)

	// Call internal Create function
	if err := internal_frozendb.Create(config); err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleBegin implements the 'begin' command.
// Starts a new transaction on the specified database.
func handleBegin() {
	// Parse --path flag
	fs := flag.NewFlagSet("begin", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_WRITE, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Check if transaction already active
	if db.GetActiveTx() != nil {
		printError(pkg_frozendb.NewInvalidActionError("transaction already active", nil))
	}

	// Begin transaction
	_, err = db.BeginTx()
	if err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleCommit implements the 'commit' command.
// Commits the active transaction.
func handleCommit() {
	// Parse --path flag
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_WRITE, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Get active transaction
	tx := db.GetActiveTx()
	if tx == nil {
		printError(pkg_frozendb.NewInvalidActionError("no active transaction", nil))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleSavepoint implements the 'savepoint' command.
// Creates a savepoint at the current position in the active transaction.
func handleSavepoint() {
	// Parse --path flag
	fs := flag.NewFlagSet("savepoint", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_WRITE, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Get active transaction
	tx := db.GetActiveTx()
	if tx == nil {
		printError(pkg_frozendb.NewInvalidActionError("no active transaction", nil))
	}

	// Create savepoint
	if err := tx.Savepoint(); err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleRollback implements the 'rollback' command.
// Rolls back the active transaction to a savepoint or to the beginning.
func handleRollback() {
	// Parse --path flag
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Parse optional savepoint_id positional argument (default: 0 = full rollback)
	args := fs.Args()
	savepointId := 0
	if len(args) > 0 {
		var err error
		savepointId, err = strconv.Atoi(args[0])
		if err != nil {
			printError(pkg_frozendb.NewInvalidInputError("savepointId must be a number", err))
		}
		if savepointId < 0 || savepointId > 9 {
			printError(pkg_frozendb.NewInvalidInputError("savepointId must be between 0 and 9", nil))
		}
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_WRITE, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Get active transaction
	tx := db.GetActiveTx()
	if tx == nil {
		printError(pkg_frozendb.NewInvalidActionError("no active transaction", nil))
	}

	// Rollback transaction
	if err := tx.Rollback(savepointId); err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleAdd implements the 'add' command.
// Inserts a key-value pair into the active transaction.
func handleAdd() {
	// Parse --path flag
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Parse positional arguments: key and value
	args := fs.Args()
	if len(args) < 2 {
		if len(args) < 1 {
			printError(pkg_frozendb.NewInvalidInputError("missing required argument: key", nil))
		}
		printError(pkg_frozendb.NewInvalidInputError("missing required argument: value", nil))
	}

	keyStr := args[0]
	valueStr := args[1]

	// Validate UUIDv7 format (FR-003)
	key, err := validateUUIDv7(keyStr)
	if err != nil {
		printError(err)
	}

	// Validate JSON format (FR-004)
	value, err := validateJSON(valueStr)
	if err != nil {
		printError(err)
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_WRITE, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Get active transaction
	tx := db.GetActiveTx()
	if tx == nil {
		printError(pkg_frozendb.NewInvalidActionError("no active transaction", nil))
	}

	// Add row to transaction
	if err := tx.AddRow(key, value); err != nil {
		printError(err)
	}

	// Success: exit silently with code 0 (per FR-005)
	os.Exit(0)
}

// handleGet implements the 'get' command.
// Retrieves a value by UUIDv7 key and prints it as pretty-formatted JSON.
func handleGet() {
	// Parse --path flag
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	pathFlag := fs.String("path", "", "database file path")
	_ = fs.Parse(os.Args[2:]) // ExitOnError flag set, no need to check error

	if *pathFlag == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Parse positional argument: key
	args := fs.Args()
	if len(args) < 1 {
		printError(pkg_frozendb.NewInvalidInputError("missing required argument: key", nil))
	}

	keyStr := args[0]

	// Validate UUIDv7 format (FR-003)
	key, err := validateUUIDv7(keyStr)
	if err != nil {
		printError(err)
	}

	// Open database in read mode
	db, err := pkg_frozendb.NewFrozenDB(*pathFlag, pkg_frozendb.MODE_READ, pkg_frozendb.FinderStrategySimple)
	if err != nil {
		printError(err)
	}
	defer func() { _ = db.Close() }() // Error ignored - exit on errors

	// Get value by key
	var result interface{}
	if err := db.Get(key, &result); err != nil {
		printError(err)
	}

	// Pretty-print JSON to stdout (FR-006)
	if err := prettyPrintJSON(result); err != nil {
		printError(pkg_frozendb.NewInvalidDataError("failed to format JSON output", err))
	}

	// Success: exit with code 0
	os.Exit(0)
}

// validateUUIDv7 validates that a string is a valid UUIDv7.
// Returns the parsed UUID or an InvalidInputError.
// Per FR-003: "Keys must be valid UUIDv7 strings".
func validateUUIDv7(keyStr string) (uuid.UUID, error) {
	key, err := uuid.Parse(keyStr)
	if err != nil {
		return uuid.Nil, pkg_frozendb.NewInvalidInputError("invalid UUID format", err)
	}
	if key.Version() != 7 {
		return uuid.Nil, pkg_frozendb.NewInvalidInputError("key must be UUIDv7", nil)
	}
	return key, nil
}

// validateJSON validates that a string contains valid JSON.
// Returns the JSON as RawMessage or an InvalidInputError.
// Per FR-004: "Values must be valid JSON".
func validateJSON(valueStr string) (json.RawMessage, error) {
	if !json.Valid([]byte(valueStr)) {
		return nil, pkg_frozendb.NewInvalidInputError("invalid JSON format", nil)
	}
	return json.RawMessage(valueStr), nil
}

// prettyPrintJSON prints JSON with 2-space indentation to stdout.
// Per FR-006: "Output must be pretty-printed JSON with consistent formatting".
func prettyPrintJSON(value interface{}) error {
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(pretty))
	return nil
}
