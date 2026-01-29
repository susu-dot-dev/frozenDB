package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	internal_frozendb "github.com/susu-dot-dev/frozenDB/internal/frozendb"
	pkg_frozendb "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
)

// Default configuration values for database creation
const (
	defaultRowSize = 4096 // Default row size in bytes
	defaultSkewMs  = 5000 // Default time skew in milliseconds (5 seconds)
)

// globalFlags represents parsed global flags from os.Args
// Per data-model.md: Parsed from os.Args using flexible positioning algorithm
type globalFlags struct {
	path       string   // Database file path (required for all commands except create)
	finder     string   // Finder strategy value (optional, defaults to "binary")
	subcommand string   // The CLI subcommand to execute
	args       []string // Remaining positional arguments for the subcommand
}

// parseGlobalFlags extracts --path, --finder, and subcommand from os.Args with flexible positioning
// Per FR-001: Flags can appear before or after subcommand
// Per VR-001 through VR-006: Validates required flags and detects duplicates
func parseGlobalFlags(osArgs []string) (*globalFlags, error) {
	flags := &globalFlags{
		finder: "", // Empty string means use default (binary)
	}

	seenPath := false
	seenFinder := false

	i := 1 // Skip program name (os.Args[0])
	for i < len(osArgs) {
		arg := osArgs[i]

		// Check for --path flag
		if arg == "--path" {
			if seenPath {
				return nil, pkg_frozendb.NewInvalidInputError("duplicate flag: --path", nil)
			}
			if i+1 >= len(osArgs) {
				return nil, pkg_frozendb.NewInvalidInputError("--path requires a value", nil)
			}
			flags.path = osArgs[i+1]
			seenPath = true
			i += 2
			continue
		}

		// Check for --finder flag
		if arg == "--finder" {
			if seenFinder {
				return nil, pkg_frozendb.NewInvalidInputError("duplicate flag: --finder", nil)
			}
			if i+1 >= len(osArgs) {
				return nil, pkg_frozendb.NewInvalidInputError("--finder requires a value", nil)
			}
			flags.finder = osArgs[i+1]
			seenFinder = true
			i += 2
			continue
		}

		// If not a flag and subcommand is empty, this is the subcommand
		if !strings.HasPrefix(arg, "--") && flags.subcommand == "" {
			flags.subcommand = arg
			i++
			continue
		}

		// Otherwise, this is a positional argument for the subcommand
		flags.args = append(flags.args, arg)
		i++
	}

	// VR-005: At least one subcommand MUST be present
	if flags.subcommand == "" {
		return nil, pkg_frozendb.NewInvalidInputError("missing subcommand", nil)
	}

	return flags, nil
}

// parseFinderStrategy maps case-insensitive finder values to FinderStrategy constants
// Per FR-005: Default to BinarySearchFinder if empty/missing
// Per A-003: Case-insensitive normalization
// Per VR-004: Validate finder value is one of: simple, inmemory, binary
func parseFinderStrategy(value string) (pkg_frozendb.FinderStrategy, error) {
	// Normalize to lowercase for case-insensitive matching
	normalized := strings.ToLower(value)

	switch normalized {
	case "", "binary":
		return pkg_frozendb.FinderStrategyBinarySearch, nil
	case "simple":
		return pkg_frozendb.FinderStrategySimple, nil
	case "inmemory":
		return pkg_frozendb.FinderStrategyInMemory, nil
	default:
		return "", pkg_frozendb.NewInvalidInputError(
			fmt.Sprintf("invalid finder strategy: %s (valid: simple, inmemory, binary)", value),
			nil,
		)
	}
}

// main is the CLI entry point. Routes to subcommand handlers.
// Follows Unix conventions: silent success, errors to stderr, exit codes 0/1.
func main() {
	// Require at least one argument (the subcommand)
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: frozendb <command> [arguments]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  create <path>                                    - Initialize new database")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] begin     - Start transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] commit    - Commit transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] savepoint - Create savepoint")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] rollback [id] - Rollback transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] add <key|NOW> <val> - Insert key-value pair")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] get <key>         - Retrieve value by key")
		os.Exit(1)
	}

	// Special case: 'create' command uses positional argument, not --path flag
	if os.Args[1] == "create" {
		handleCreate()
		return
	}

	// Parse global flags with flexible positioning
	flags, err := parseGlobalFlags(os.Args)
	if err != nil {
		printError(err)
	}

	// VR-001: Validate --path is present for commands requiring it
	if flags.path == "" {
		printError(pkg_frozendb.NewInvalidInputError("missing required flag: --path", nil))
	}

	// Parse finder strategy (validates and provides default)
	finderStrategy, err := parseFinderStrategy(flags.finder)
	if err != nil {
		printError(err)
	}

	// Route to subcommand handler with parsed flags
	switch flags.subcommand {
	case "begin":
		handleBegin(flags.path, finderStrategy)
	case "commit":
		handleCommit(flags.path, finderStrategy)
	case "savepoint":
		handleSavepoint(flags.path, finderStrategy)
	case "rollback":
		handleRollback(flags.path, finderStrategy, flags.args)
	case "add":
		handleAdd(flags.path, finderStrategy, flags.args)
	case "get":
		handleGet(flags.path, finderStrategy, flags.args)
	default:
		printError(pkg_frozendb.NewInvalidInputError(fmt.Sprintf("unknown command: %s", flags.subcommand), nil))
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
func handleBegin(path string, finderStrategy pkg_frozendb.FinderStrategy) {
	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_WRITE, finderStrategy)
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
func handleCommit(path string, finderStrategy pkg_frozendb.FinderStrategy) {
	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_WRITE, finderStrategy)
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
func handleSavepoint(path string, finderStrategy pkg_frozendb.FinderStrategy) {
	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_WRITE, finderStrategy)
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
func handleRollback(path string, finderStrategy pkg_frozendb.FinderStrategy, args []string) {
	// Parse optional savepoint_id positional argument (default: 0 = full rollback)
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
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_WRITE, finderStrategy)
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
func handleAdd(path string, finderStrategy pkg_frozendb.FinderStrategy, args []string) {
	// Parse positional arguments: key and value
	if len(args) < 2 {
		if len(args) < 1 {
			printError(pkg_frozendb.NewInvalidInputError("missing required argument: key", nil))
		}
		printError(pkg_frozendb.NewInvalidInputError("missing required argument: value", nil))
	}

	keyStr := args[0]
	valueStr := args[1]

	// Check for NOW keyword (case-insensitive per FR-003, A-002)
	var key uuid.UUID
	var err error
	if strings.ToLower(keyStr) == "now" {
		// Generate UUIDv7 using current timestamp
		key, err = uuid.NewV7()
		if err != nil {
			printError(pkg_frozendb.NewInvalidInputError("failed to generate UUIDv7", err))
		}
	} else {
		// Validate user-provided UUIDv7 format
		key, err = validateUUIDv7(keyStr)
		if err != nil {
			printError(err)
		}
	}

	// Validate JSON format (FR-004)
	value, err := validateJSON(valueStr)
	if err != nil {
		printError(err)
	}

	// Open database in write mode
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_WRITE, finderStrategy)
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

	// Success: output the key to stdout (FR-004, VR-009 through VR-012)
	fmt.Println(key.String())
	os.Exit(0)
}

// handleGet implements the 'get' command.
// Retrieves a value by UUIDv7 key and prints it as pretty-formatted JSON.
func handleGet(path string, finderStrategy pkg_frozendb.FinderStrategy, args []string) {
	// Parse positional argument: key
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
	db, err := pkg_frozendb.NewFrozenDB(path, pkg_frozendb.MODE_READ, finderStrategy)
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
