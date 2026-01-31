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
	// Handle version command/flag before anything else
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		handleVersion()
	}

	// Require at least one argument (the subcommand)
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: frozendb <command> [arguments]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  create <path>                                             - Initialize new database")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] begin              - Start transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] commit             - Commit transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] savepoint          - Create savepoint")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] rollback [id]      - Rollback transaction")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] add <key|NOW> <val> - Insert key-value pair")
		fmt.Fprintln(os.Stderr, "  [--path <file>] [--finder <strategy>] get <key>          - Retrieve value by key")
		fmt.Fprintln(os.Stderr, "  [--path <file>] inspect [--offset N] [--limit N] [--print-header BOOL] - Display database contents")
		fmt.Fprintln(os.Stderr, "  [--path <file>] verify                                   - Verify database integrity")
		fmt.Fprintln(os.Stderr, "  version                                                  - Display version information")
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
	case "inspect":
		handleInspect(flags.path, finderStrategy, flags.args)
	default:
		printError(pkg_frozendb.NewInvalidInputError(fmt.Sprintf("unknown command: %s", flags.subcommand), nil))
	}
}

// handleVersion implements the 'version' command and '--version' flag.
// Displays the version from version.go constant.
// Always exits with code 0 (success).
func handleVersion() {
	fmt.Printf("frozendb %s\n", Version)
	os.Exit(0)
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

// handleInspect implements the 'inspect' command.
// Displays database contents in tab-separated format.
func handleInspect(path string, finderStrategy pkg_frozendb.FinderStrategy, args []string) {
	// Parse inspect-specific flags
	offset, limit, printHeader, err := parseInspectFlags(args)
	if err != nil {
		printError(err)
	}

	// Open database file in read mode
	file, err := internal_frozendb.NewDBFile(path, internal_frozendb.MODE_READ)
	if err != nil {
		printError(err)
	}
	defer func() { _ = file.Close() }()

	// Read and parse header
	headerBytes, err := file.Read(0, internal_frozendb.HEADER_SIZE)
	if err != nil {
		printError(err)
	}

	header := &internal_frozendb.Header{}
	if err := header.UnmarshalText(headerBytes); err != nil {
		printError(err)
	}

	// Print optional header table
	if printHeader {
		printHeaderTable(header)
	}

	// Print row data table header
	printRowTableHeader()

	// Calculate total rows: (fileSize - 64) / rowSize
	fileSize := file.Size()
	rowSize := int64(header.GetRowSize())
	totalRows := (fileSize - 64) / rowSize

	// Validate offset
	if offset < 0 {
		printError(pkg_frozendb.NewInvalidInputError("offset cannot be negative", nil))
	}

	// Determine end index based on limit
	var endIndex int64
	if limit < 0 {
		endIndex = totalRows // Display all remaining rows
	} else {
		endIndex = offset + limit
		if endIndex > totalRows {
			endIndex = totalRows
		}
	}

	// Track errors for exit code
	hasErrors := false

	// Iterate through rows
	for index := offset; index < endIndex; index++ {
		row, err := readAndParseRow(file, index, int(rowSize))
		if err != nil {
			// Mark as error but continue processing
			hasErrors = true
			row.Type = "error"
			row.Index = index
		}
		printInspectRow(row)
	}

	// Exit with appropriate code
	if hasErrors {
		os.Exit(1)
	}
	os.Exit(0)
}

// parseInspectFlags parses inspect-specific command flags
func parseInspectFlags(args []string) (offset int64, limit int64, printHeader bool, err error) {
	// Set defaults
	offset = 0
	limit = -1
	printHeader = false

	// Parse flags
	i := 0
	for i < len(args) {
		if i >= len(args) {
			break
		}

		arg := args[i]

		if arg == "--offset" {
			if i+1 >= len(args) {
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--offset requires a value", nil)
			}
			val, parseErr := strconv.ParseInt(args[i+1], 10, 64)
			if parseErr != nil {
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--offset must be a number", parseErr)
			}
			offset = val
			i += 2
			continue
		}

		if arg == "--limit" {
			if i+1 >= len(args) {
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--limit requires a value", nil)
			}
			val, parseErr := strconv.ParseInt(args[i+1], 10, 64)
			if parseErr != nil {
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--limit must be a number", parseErr)
			}
			limit = val
			i += 2
			continue
		}

		if arg == "--print-header" {
			if i+1 >= len(args) {
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--print-header requires a value", nil)
			}
			val := strings.ToLower(args[i+1])
			switch val {
			case "true", "t", "1":
				printHeader = true
			case "false", "f", "0":
				printHeader = false
			default:
				return 0, 0, false, pkg_frozendb.NewInvalidInputError("--print-header must be true or false", nil)
			}
			i += 2
			continue
		}

		// Unknown flag
		return 0, 0, false, pkg_frozendb.NewInvalidInputError(fmt.Sprintf("unknown flag: %s", arg), nil)
	}

	return offset, limit, printHeader, nil
}

// printHeaderTable prints the database header information table
func printHeaderTable(header *internal_frozendb.Header) {
	fmt.Printf("Row Size\tClock Skew\tFile Version\n")
	fmt.Printf("%d\t%d\t%d\n", header.GetRowSize(), header.GetSkewMs(), header.GetVersion())
	fmt.Println() // Blank line separator
}

// printRowTableHeader prints the row data table column headers
func printRowTableHeader() {
	fmt.Printf("index\ttype\tkey\tvalue\tsavepoint\ttx start\ttx end\trollback\tparity\n")
}

// InspectRow represents a single row for display
type InspectRow struct {
	Index     int64
	Type      string
	Key       string
	Value     string
	Savepoint string
	TxStart   string
	TxEnd     string
	Rollback  string
	Parity    string
}

// printInspectRow prints a single row in TSV format
func printInspectRow(row InspectRow) {
	fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		row.Index, row.Type, row.Key, row.Value,
		row.Savepoint, row.TxStart, row.TxEnd, row.Rollback, row.Parity)
}

// readAndParseRow reads and parses a single row from the database
func readAndParseRow(file internal_frozendb.DBFile, index int64, rowSize int) (InspectRow, error) {
	// Calculate offset: 64 (header) + index * rowSize
	offset := int64(64) + index*int64(rowSize)

	// Read row bytes
	rowBytes, err := file.Read(offset, int32(rowSize))
	if err != nil {
		// Check if this is a partial row at end of file
		if strings.Contains(err.Error(), "exceeds file size") {
			// Try to read remaining bytes
			remaining := file.Size() - offset
			if remaining > 0 && remaining < int64(rowSize) {
				partialBytes, readErr := file.Read(offset, int32(remaining))
				if readErr == nil {
					return parsePartialRow(index, partialBytes, rowSize)
				}
			}
		}
		// Return error row
		return InspectRow{
			Index: index,
			Type:  "error",
		}, err
	}

	// Extract parity before parsing
	parity := extractParity(rowBytes)

	// Parse row with RowUnion
	ru := &internal_frozendb.RowUnion{}
	if err := ru.UnmarshalText(rowBytes); err != nil {
		// Return error row with parity
		return InspectRow{
			Index:  index,
			Type:   "error",
			Parity: parity,
		}, err
	}

	// Determine row type and extract fields
	if ru.ChecksumRow != nil {
		checksumValue, _ := ru.ChecksumRow.RowPayload.MarshalText()
		return InspectRow{
			Index:  index,
			Type:   "Checksum",
			Key:    "",
			Value:  string(checksumValue),
			Parity: parity,
		}, nil
	}

	if ru.NullRow != nil {
		return InspectRow{
			Index:     index,
			Type:      "NullRow",
			Key:       ru.NullRow.RowPayload.Key.String(),
			Value:     "",
			Savepoint: "false",
			TxStart:   "true",
			TxEnd:     "true",
			Rollback:  "false",
			Parity:    parity,
		}, nil
	}

	if ru.DataRow != nil {
		payload := ru.DataRow.RowPayload
		startControl := ru.DataRow.StartControl
		endControl := ru.DataRow.EndControl

		savepoint, txStart, txEnd, rollback := extractTransactionFields(startControl, endControl)

		return InspectRow{
			Index:     index,
			Type:      "Data",
			Key:       payload.Key.String(),
			Value:     string(payload.Value),
			Savepoint: savepoint,
			TxStart:   txStart,
			TxEnd:     txEnd,
			Rollback:  rollback,
			Parity:    parity,
		}, nil
	}

	if ru.NullRow != nil {
		return InspectRow{
			Index:     index,
			Type:      "NullRow",
			Key:       ru.NullRow.RowPayload.Key.String(),
			Value:     "",
			Savepoint: "false",
			TxStart:   "true",
			TxEnd:     "true",
			Rollback:  "false",
			Parity:    parity,
		}, nil
	}

	if ru.DataRow != nil {
		payload := ru.DataRow.RowPayload
		startControl := ru.DataRow.StartControl
		endControl := ru.DataRow.EndControl

		savepoint, txStart, txEnd, rollback := extractTransactionFields(startControl, endControl)

		return InspectRow{
			Index:     index,
			Type:      "Data",
			Key:       payload.Key.String(),
			Value:     string(payload.Value),
			Savepoint: savepoint,
			TxStart:   txStart,
			TxEnd:     txEnd,
			Rollback:  rollback,
			Parity:    parity,
		}, nil
	}

	// Unknown row type
	return InspectRow{
		Index:  index,
		Type:   "error",
		Parity: parity,
	}, fmt.Errorf("unknown row type")
}

// parsePartialRow parses a partial row at end of file
func parsePartialRow(index int64, rowBytes []byte, fullRowSize int) (InspectRow, error) {
	// Parse with PartialDataRow
	partial := &internal_frozendb.PartialDataRow{}
	if err := partial.UnmarshalText(rowBytes); err != nil {
		return InspectRow{
			Index: index,
			Type:  "error",
		}, err
	}

	row := InspectRow{
		Index: index,
		Type:  "partial",
	}

	state := partial.GetState()

	// For PartialDataRow, we need to access the internal DataRow fields
	// Since these are not exported, we can only determine state-based information

	// We can't directly access the internal fields of PartialDataRow
	// So we'll just show the state-based information we have

	// State 1: Only start_control available
	// State 2: start_control + payload available
	// State 3: start_control + payload + savepoint marker available

	if state == 3 { // PartialDataRowWithSavepoint
		row.Savepoint = "true"
	}

	return row, nil
}

// extractParity extracts parity bytes from row bytes
func extractParity(rowBytes []byte) string {
	rowSize := len(rowBytes)
	if rowSize < 4 {
		return ""
	}
	// Parity is at positions [N-3:N-1]
	return string(rowBytes[rowSize-3 : rowSize-1])
}

// extractTransactionFields extracts transaction control fields from control bytes
func extractTransactionFields(startControl internal_frozendb.StartControl, endControl internal_frozendb.EndControl) (savepoint, txStart, txEnd, rollback string) {
	// TxStart: true if start_control is 'T'
	if startControl == internal_frozendb.START_TRANSACTION {
		txStart = "true"
	} else {
		txStart = "false"
	}

	// TxEnd: true if end_control[1] is 'C'
	if endControl[1] == 'C' {
		txEnd = "true"
	} else {
		txEnd = "false"
	}

	// Savepoint: true if end_control[0] is 'S'
	if endControl[0] == 'S' {
		savepoint = "true"
	} else {
		savepoint = "false"
	}

	// Rollback: true if end_control[1] is '0'-'9'
	if endControl[1] >= '0' && endControl[1] <= '9' {
		rollback = "true"
	} else {
		rollback = "false"
	}

	return
}
