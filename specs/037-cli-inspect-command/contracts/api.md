# API Contract: CLI Inspect Command

**Feature**: 037-cli-inspect-command  
**Date**: 2026-01-30  
**Status**: Complete

This document specifies the complete API contract for the inspect command, including command-line interface, behavior, and integration details.

## Command-Line Interface

### Command Syntax

```bash
frozendb [global-flags] inspect [inspect-flags]
```

### Global Flags (Optional)

- `--path <file>`: Database file path (required for inspect command)
  - Type: string
  - Constraints: Must have .fdb extension, file must exist
  - Error: InvalidInputError if missing or invalid

- `--finder <strategy>`: Finder strategy (not used by inspect, but accepted for consistency)
  - Type: string (case-insensitive)
  - Values: "simple", "inmemory", "binary"
  - Default: "binary"
  - Note: Inspect opens in read-only mode and doesn't use Finder for row iteration

### Inspect-Specific Flags (Optional)

- `--offset <n>`: Starting row index (zero-based)
  - Type: int64
  - Default: 0
  - Constraints: Must be non-negative
  - Error: InvalidInputError if negative
  - Behavior: If offset >= total rows, displays zero rows (not an error)

- `--limit <n>`: Maximum number of rows to display
  - Type: int64
  - Default: -1 (display all remaining rows)
  - Constraints: Any integer value; negative means "no limit"
  - Behavior: If limit is 0, displays zero rows

- `--print-header <bool>`: Display database header information
  - Type: boolean
  - Default: false
  - Valid values (case-insensitive): "true", "false", "t", "f", "1", "0"
  - Error: InvalidInputError if value cannot be parsed as boolean

### Flag Positioning (FR-024)

Flags can appear in any order before or after the `inspect` subcommand:

```bash
# All valid:
frozendb --path db.fdb inspect --offset 10 --limit 5
frozendb inspect --path db.fdb --offset 10 --limit 5
frozendb --path db.fdb --offset 10 inspect --limit 5
```

## Output Format

### TSV (Tab-Separated Values)

All output uses TSV format with tabs (U+0009) as column separators and newlines (U+000A) as row separators.

**Empty Field Representation**: Empty string with no characters between tabs
- Example: `1\tData\t\t\ttrue\tfalse\ttrue\tfalse\tA3` (empty key and value fields)

### Header Table (Optional)

Displayed only when `--print-header` is true.

**Format**:
```
Row Size\tClock Skew\tFile Version
<row_size>\t<skew_ms>\t<version>

```

**Example**:
```
Row Size	Clock Skew	File Version
4096	5000	1

```

**Note**: Blank line separates header table from row data table.

### Row Data Table (Always Displayed)

**Column Headers**:
```
index\ttype\tkey\tvalue\tsavepoint\ttx start\ttx end\trollback\tparity
```

**Column Definitions**:

| Column | Type | Description | Empty When |
|--------|------|-------------|------------|
| index | int64 | Zero-based row index (0 = first checksum) | Never |
| type | string | Row type: "Data", "NullRow", "Checksum", "partial", "error" | Never |
| key | string | UUID key in standard format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) | Checksum, error rows, partial rows without payload |
| value | string | JSON payload or checksum Base64 string | NullRow, error rows, partial rows without payload |
| savepoint | string | "true" or "false" | Checksum, error rows, or when not applicable |
| tx start | string | "true" or "false" | Checksum, error rows, or when not applicable |
| tx end | string | "true" or "false" | Checksum, error rows, partial rows, or when not applicable |
| rollback | string | "true" or "false" | Checksum, error rows, partial rows, or when not applicable |
| parity | string | Two-character uppercase hex string (e.g., "A3") | Partial rows (no parity yet), or error rows when bytes unavailable |

## Row Type Specifications

### Data Row (type="Data")

**When**: RowUnion.DataRow is non-nil after parsing

**Field Mapping**:
- `index`: Row index
- `type`: "Data"
- `key`: UUID from DataRowPayload.Key in standard format
- `value`: JSON string from DataRowPayload.Value (not pretty-printed)
- `savepoint`: "true" if end_control[0] == 'S', else "false"
- `tx start`: "true" if start_control == 'T', else "false"
- `tx end`: "true" if end_control[1] == 'C', else "false"
- `rollback`: "true" if end_control[1] is '0'-'9', else "false"
- `parity`: Hex string from rowBytes[rowSize-3:rowSize-1]

**Example**:
```
1	Data	018d1234-5678-7000-8000-000000000001	{"name":"test"}	false	true	false	false	A3
```

### NullRow (type="NullRow")

**When**: RowUnion.NullRow is non-nil after parsing

**Field Mapping**:
- `index`: Row index
- `type`: "NullRow"
- `key`: UUID from NullRowPayload.Key in standard format
- `value`: "" (empty string)
- `savepoint`: "false"
- `tx start`: "true"
- `tx end`: "true"
- `rollback`: "false"
- `parity`: Hex string from rowBytes[rowSize-3:rowSize-1]

**Example**:
```
2	NullRow	018d1234-5678-7000-0000-000000000000				true	true	false	B7
```

### Checksum Row (type="Checksum")

**When**: RowUnion.ChecksumRow is non-nil after parsing

**Field Mapping**:
- `index`: Row index
- `type`: "Checksum"
- `key`: "" (empty string)
- `value`: Base64-encoded CRC32 checksum (8 characters with "==" padding)
- `savepoint`: "" (empty string)
- `tx start`: "" (empty string)
- `tx end`: "" (empty string)
- `rollback`: "" (empty string)
- `parity`: Hex string from rowBytes[rowSize-3:rowSize-1]

**Example**:
```
0	Checksum		mPPqCw==					C4
```

### Partial Row (type="partial")

**When**: Row bytes less than row_size and located at end of file

**Field Mapping (depends on PartialRowState)**:

**State 1 (PartialDataRowWithStartControl)**:
- `index`: Row index
- `type`: "partial"
- `key`: "" (empty string)
- `value`: "" (empty string)
- `savepoint`: "" (empty string)
- `tx start`: "true" if start_control == 'T', else "false"
- `tx end`: "" (empty string)
- `rollback`: "" (empty string)
- `parity`: "" (empty string - no parity yet)

**State 2 (PartialDataRowWithPayload)**:
- `index`: Row index
- `type`: "partial"
- `key`: UUID from DataRowPayload.Key in standard format
- `value`: JSON string from DataRowPayload.Value
- `savepoint`: "" (empty string - no end_control yet)
- `tx start`: "true" if start_control == 'T', else "false"
- `tx end`: "" (empty string)
- `rollback`: "" (empty string)
- `parity`: "" (empty string - no parity yet)

**State 3 (PartialDataRowWithSavepoint)**:
- `index`: Row index
- `type`: "partial"
- `key`: UUID from DataRowPayload.Key in standard format
- `value`: JSON string from DataRowPayload.Value
- `savepoint`: "true"
- `tx start`: "true" if start_control == 'T', else "false"
- `tx end`: "" (empty string - no full end_control yet)
- `rollback`: "" (empty string - no full end_control yet)
- `parity`: "" (empty string - no parity yet)

**Example (State 2)**:
```
150	partial	018d1234-5678-7000-8000-000000000099	{"partial":"data"}		true			
```

### Error Row (type="error")

**When**: Row parsing fails (UnmarshalText returns error)

**Field Mapping**:
- `index`: Row index
- `type`: "error"
- `key`: "" (empty string)
- `value`: "" (empty string)
- `savepoint`: "" (empty string)
- `tx start`: "" (empty string)
- `tx end`: "" (empty string)
- `rollback`: "" (empty string)
- `parity`: Hex string if rowBytes available, else "" (empty string)

**Example**:
```
42	error							7F
```

**Behavior**: Continue processing remaining rows, set exit code to 1 after completion.

## Exit Codes

### Success (Exit Code 0)

**Conditions**:
- All rows parsed successfully
- Offset beyond file size (displays zero rows, not an error)
- Limit is 0 (displays zero rows, not an error)

### Error (Exit Code 1)

**Conditions**:
- Any row fails to parse (displayed as type="error")
- Invalid command-line arguments (negative offset, invalid --print-header value)
- File access errors (file not found, permission denied)
- Database corruption errors (invalid header, missing checksum)

**Error Output Format**: Errors printed to stderr in format:
```
Error: <error message>
```

## Function Signatures

### handleInspect

Main handler function for the inspect command.

```go
func handleInspect(path string, finderStrategy FinderStrategy, args []string)
```

**Parameters**:
- `path`: Database file path from --path flag
- `finderStrategy`: Finder strategy from --finder flag (unused for inspect)
- `args`: Remaining arguments for parsing inspect-specific flags

**Returns**: Does not return; exits with code 0 or 1

**Errors**:
- Prints errors to stderr via printError() and exits with code 1
- InvalidInputError: Invalid flags, negative offset, etc.
- PathError: File not found, permission denied
- CorruptDatabaseError: Invalid header, missing checksum

**Side Effects**:
- Opens database file in read-only mode
- Writes TSV output to stdout
- Writes errors to stderr
- Exits process with appropriate code

### parseInspectFlags

Parses inspect-specific flags from args slice.

```go
func parseInspectFlags(args []string) (offset int64, limit int64, printHeader bool, error)
```

**Parameters**:
- `args`: Command-line arguments after subcommand and global flags

**Returns**:
- `offset`: Starting row index (default: 0)
- `limit`: Maximum rows to display (default: -1)
- `printHeader`: Whether to display header table (default: false)
- `error`: InvalidInputError if parsing fails

**Validation**:
- Offset must be non-negative
- PrintHeader must be parseable as boolean
- No duplicate flags allowed

### printHeaderTable

Prints the optional header information table.

```go
func printHeaderTable(header *Header)
```

**Parameters**:
- `header`: Parsed database header

**Output** (to stdout):
```
Row Size\tClock Skew\tFile Version
<rowSize>\t<skewMs>\t<version>

```

**Note**: Includes blank line after table.

### printRowTableHeader

Prints the row data table column headers.

```go
func printRowTableHeader()
```

**Output** (to stdout):
```
index\ttype\tkey\tvalue\tsavepoint\ttx start\ttx end\trollback\tparity
```

### printInspectRow

Prints a single row in TSV format.

```go
func printInspectRow(row InspectRow)
```

**Parameters**:
- `row`: InspectRow entity with all fields populated

**Output** (to stdout):
```
<index>\t<type>\t<key>\t<value>\t<savepoint>\t<txStart>\t<txEnd>\t<rollback>\t<parity>
```

**Note**: Empty fields rendered as no characters between tabs.

### readAndParseRow

Reads row bytes from file and parses into InspectRow.

```go
func readAndParseRow(file DBFile, index int64, rowSize int) (InspectRow, error)
```

**Parameters**:
- `file`: Open DBFile in read mode
- `index`: Zero-based row index to read
- `rowSize`: Row size from header

**Returns**:
- `InspectRow`: Parsed row with all fields populated
- `error`: Error if row is corrupted or unreadable

**Behavior**:
- Calculates offset: `64 + index * rowSize`
- Attempts to read row_size bytes
- If EOF detected and last row: parses as PartialDataRow
- If parsing fails: returns InspectRow with type="error"
- Extracts parity bytes from raw rowBytes when available

### extractTransactionFields

Extracts transaction control boolean flags from row control bytes.

```go
func extractTransactionFields(startControl StartControl, endControl EndControl) (savepoint, txStart, txEnd, rollback string)
```

**Parameters**:
- `startControl`: Row start control byte
- `endControl`: Two-byte end control sequence

**Returns**:
- `savepoint`: "true" if endControl[0] == 'S', else "false"
- `txStart`: "true" if startControl == 'T', else "false"
- `txEnd`: "true" if endControl[1] == 'C', else "false"
- `rollback`: "true" if endControl[1] is '0'-'9', else "false"

### extractParity

Extracts parity bytes from raw row bytes.

```go
func extractParity(rowBytes []byte) string
```

**Parameters**:
- `rowBytes`: Raw row bytes (may be full row or partial)

**Returns**:
- Parity hex string (2 characters) if available
- Empty string if rowBytes too short

**Implementation**:
```go
if len(rowBytes) < 4 {
    return ""
}
return string(rowBytes[len(rowBytes)-3 : len(rowBytes)-1])
```

## Integration Notes

### With Existing CLI Infrastructure

**Routing**: Add case in main() switch statement:
```go
case "inspect":
    handleInspect(flags.path, finderStrategy, flags.args)
```

**Error Handling**: Use existing printError() helper for all errors:
```go
if err != nil {
    printError(err)  // Prints to stderr and exits with code 1
}
```

**Flag Parsing**: Extend existing parseGlobalFlags() to handle inspect subcommand, then parse inspect-specific flags in handleInspect()

### With Internal frozenDB Package

**File Access**:
```go
import internal_frozendb "github.com/susu-dot-dev/frozenDB/internal/frozendb"

file, err := internal_frozendb.NewDBFile(path, internal_frozendb.MODE_READ)
```

**Header Parsing**:
```go
headerBytes, _ := file.Read(0, internal_frozendb.HEADER_SIZE)
header := &internal_frozendb.Header{}
header.UnmarshalText(headerBytes)
```

**Row Parsing**:
```go
ru := &internal_frozendb.RowUnion{}
err := ru.UnmarshalText(rowBytes)
if err != nil {
    // Handle as error row
}
```

### Performance Characteristics

**Time Complexity**:
- Header read: O(1)
- Row iteration: O(limit) or O(totalRows - offset) for limit=-1
- Per-row processing: O(1) for fixed-size rows

**Space Complexity**:
- Memory usage: O(1) - constant regardless of database size
- Buffer size: row_size bytes (typically 4096)

**I/O Pattern**:
- Sequential reads with calculated offsets
- One Read() call per row
- No buffering needed (OS handles caching)

## Thread Safety

The inspect command is single-threaded and does not require thread safety considerations:
- Opens file in read-only mode
- No concurrent access within inspect operation
- Safe to run concurrently with other frozenDB operations (file-level read lock)

## Compatibility

**Forward Compatibility**: Inspect command designed to display any valid v1 frozenDB file format
- New row types: Will be displayed as type="error" if not recognized
- New file format versions: Will fail at header validation with CorruptDatabaseError

**Backward Compatibility**: This is a new command; no backward compatibility concerns

## Usage Examples

### Basic Inspection
```bash
frozendb --path test.fdb inspect
```
Output: All rows in TSV format

### With Header Information
```bash
frozendb --path test.fdb inspect --print-header true
```
Output: Header table, then row table

### Pagination
```bash
frozendb --path test.fdb inspect --offset 100 --limit 20
```
Output: Rows 100-119 in TSV format

### Piping to awk
```bash
frozendb --path test.fdb inspect | awk -F'\t' '$2 == "Data" { print $3, $4 }'
```
Output: Key-value pairs for Data rows only
