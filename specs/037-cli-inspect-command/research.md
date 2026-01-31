# Research: CLI Inspect Command

**Feature**: 037-cli-inspect-command  
**Date**: 2026-01-30  
**Status**: Complete

This document contains research findings for implementing the `inspect` command in the frozenDB CLI.

## Decision: Row Reading Strategy

**Chosen**: Streaming row-by-row read using FileManager.Read() with sequential seeks

**Rationale**:
- FileManager already provides Read(start int64, size int32) method for reading arbitrary file ranges
- Each row is fixed-width (row_size bytes), enabling O(1) offset calculation: `offset = 64 + index * row_size`
- Streaming approach ensures constant memory usage regardless of database size (NFR-001)
- DBFile interface already supports read-only mode via NewDBFile(path, MODE_READ)

**Alternatives Considered**:
1. **Load entire file into memory**: Rejected due to NFR-003 requirement to support arbitrarily large database files
2. **Buffered reading (read N rows at once)**: Rejected for simplicity; single-row reading is sufficient and simpler
3. **Use existing Finder implementations**: Rejected because Finder only locates DataRows by UUID; inspect needs to display all row types including ChecksumRows and PartialDataRows

**Implementation Pattern**:
```go
// Open file in read mode
file, err := NewDBFile(path, MODE_READ)

// Read header to get row_size
headerBytes, _ := file.Read(0, HEADER_SIZE)
header := &Header{}
header.UnmarshalText(headerBytes)

// Read initial checksum row (index 0)
checksumBytes, _ := file.Read(64, int32(header.GetRowSize()))

// Stream remaining rows
for index := 1; index < totalRows; index++ {
    offset := 64 + int64(index) * int64(header.GetRowSize())
    rowBytes, err := file.Read(offset, int32(header.GetRowSize()))
    // Handle partial row if at EOF
    // Parse and print row
}
```

## Decision: Row Type Detection and Parsing

**Chosen**: Use RowUnion.UnmarshalText() with custom PartialDataRow detection

**Rationale**:
- RowUnion already handles DataRow, NullRow, and ChecksumRow parsing based on control bytes
- PartialDataRow detection requires checking file size: if EOF occurs before row_size bytes, it's partial
- File format spec section 8.6.4 states: "If the row is exactly row_size bytes: MUST be parsed as a data row or checksum row depending on the start_control. If the row is less than row_size bytes: MUST be parsed as a PartialDataRow"

**Alternatives Considered**:
1. **Extend RowUnion to include PartialDataRow**: Rejected because PartialDataRow is not a complete row type and RowUnion expects exactly row_size bytes
2. **Try parsing as complete row first, fallback to partial**: Rejected as inefficient; file size check is O(1)

**Implementation Pattern**:
```go
// Attempt to read full row
rowBytes, err := file.Read(offset, int32(rowSize))

if err != nil && errors.Is(err, InvalidInputError) && strings.Contains(err.Error(), "exceeds file size") {
    // Partial row detected - read remaining bytes
    remaining := file.Size() - offset
    partialBytes, _ := file.Read(offset, int32(remaining))
    // Parse as PartialDataRow using existing PartialDataRow.UnmarshalText()
} else if err != nil {
    // Other error - mark as "error" type
    return InspectRow{Type: "error", Index: index}
} else {
    // Complete row - parse with RowUnion
    ru := &RowUnion{}
    if err := ru.UnmarshalText(rowBytes); err != nil {
        return InspectRow{Type: "error", Index: index}
    }
}
```

## Decision: Output Format (TSV)

**Chosen**: Tab-separated values (TSV) with column headers

**Rationale**:
- Spec clarification (Session 2026-01-30) explicitly requests "Tab-separated values (TSV) with column headers"
- TSV is standard format for Unix tools (awk, grep, sed) per FR-006
- Spec clarification states blank values should be "Empty string (no characters between tabs)"
- Standard Go library provides fmt.Printf() for TSV formatting

**Alternatives Considered**:
1. **Fixed-width columns**: Rejected because values (especially JSON) can be arbitrarily long
2. **CSV format**: Rejected because TSV is simpler for command-line parsing (no need to escape tabs in JSON)

**Implementation Pattern**:
```go
// Header table (if --print-header)
fmt.Printf("Row Size\tClock Skew\tFile Version\n")
fmt.Printf("%d\t%d\t%d\n", header.GetRowSize(), header.GetSkewMs(), header.GetVersion())
fmt.Println()  // Blank line separator

// Row data table
fmt.Printf("index\ttype\tkey\tvalue\tsavepoint\ttx start\ttx end\trollback\tparity\n")
for each row {
    fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", 
        index, rowType, key, value, savepoint, txStart, txEnd, rollback, parity)
}
```

## Decision: Transaction Control Field Extraction

**Chosen**: Implement helper functions to extract boolean flags from control bytes

**Rationale**:
- FR-010 defines complex logic for tx_start, tx_end, savepoint, and rollback fields based on start_control and end_control
- DataRow already exposes StartControl and EndControl fields
- Helper functions improve readability and testability

**Implementation Pattern**:
```go
func extractTxStart(startControl StartControl) bool {
    return startControl == START_TRANSACTION // 'T'
}

func extractTxEnd(endControl EndControl) bool {
    // TC or SC
    return endControl[1] == 'C'
}

func extractSavepoint(endControl EndControl) bool {
    // S* patterns
    return endControl[0] == 'S'
}

func extractRollback(endControl EndControl) bool {
    // R[0-9] or S[0-9] patterns
    if endControl[1] >= '0' && endControl[1] <= '9' {
        return true
    }
    return false
}
```

## Decision: Error Handling and Exit Codes

**Chosen**: Continue on row parsing errors, collect error count, exit with code 1 if any errors

**Rationale**:
- FR-017: "If a row fails to parse, system MUST display that row with type='error', all other columns empty string (no characters) except index and parity (if available), and continue processing remaining rows"
- FR-018: "If any row fails to parse during the entire inspection operation, system MUST set exit code to 1 after completing all output"
- FR-019: "If all rows parse successfully, system MUST exit with code 0"

**Alternatives Considered**:
1. **Exit immediately on first error**: Rejected by FR-017 which requires continuing after errors
2. **Return error details in error row value field**: Rejected by FR-017 which requires "all other columns empty string"

**Implementation Pattern**:
```go
var hasErrors bool

for each row {
    row, err := parseRow(...)
    if err != nil {
        hasErrors = true
        printErrorRow(index)
        continue
    }
    printRow(row)
}

if hasErrors {
    os.Exit(1)
}
os.Exit(0)
```

## Decision: Offset and Limit Handling

**Chosen**: Calculate starting offset, iterate with limit counter

**Rationale**:
- FR-014: "The index column MUST use zero-based indexing where index 0 = first checksum row (at offset 64)"
- FR-015: "The offset parameter MUST follow the same indexing as the index column"
- FR-016: "If offset is greater than the number of rows in the database, system MUST succeed (exit code 0) and display zero rows"
- Limit=-1 means display all remaining rows

**Implementation Pattern**:
```go
// Calculate total rows
fileSize := file.Size()
totalRows := (fileSize - 64) / int64(rowSize)

// Validate offset
if offset < 0 {
    return InvalidInputError("offset cannot be negative")
}

// Offset beyond file size is not an error
if offset >= totalRows {
    // Print headers but no rows
    os.Exit(0)
}

// Determine ending index
var endIndex int64
if limit < 0 {
    endIndex = totalRows  // Display all remaining
} else {
    endIndex = min(offset + int64(limit), totalRows)
}

// Iterate from offset to endIndex
for index := offset; index < endIndex; index++ {
    // Read and print row
}
```

## Decision: Parity Extraction from Row Bytes

**Chosen**: Extract parity bytes directly from raw row bytes before parsing

**Rationale**:
- Parity bytes are at fixed positions [N-3..N-2] per file format spec section 5.2
- For error rows that fail parsing, we still need to display parity if available
- Raw bytes are always available, even when UnmarshalText() fails

**Implementation Pattern**:
```go
func extractParity(rowBytes []byte) string {
    rowSize := len(rowBytes)
    if rowSize < 4 {
        return ""  // Not enough bytes for parity
    }
    // Parity is at positions [N-3..N-2]
    parityBytes := rowBytes[rowSize-3 : rowSize-1]
    return string(parityBytes)
}
```

## Decision: CLI Flag Parsing

**Chosen**: Extend existing parseGlobalFlags() pattern with command-specific flag parsing

**Rationale**:
- Existing CLI commands use parseGlobalFlags() for --path and --finder
- FR-024: "System MUST follow the CLI convention of using flexible flag positioning"
- Inspect command has additional flags: --offset, --limit, --print-header
- Simplest approach: parse inspect-specific flags in handleInspect() function

**Alternatives Considered**:
1. **Use a CLI library (cobra, urfave/cli)**: Rejected to maintain consistency with existing manual parsing
2. **Extend parseGlobalFlags() to handle all flags**: Rejected because not all commands need these flags

**Implementation Pattern**:
```go
func handleInspect(path string, finderStrategy FinderStrategy, args []string) {
    // Default values per spec
    offset := int64(0)
    limit := int64(-1)
    printHeader := false

    // Parse command-specific flags from args
    i := 0
    for i < len(args) {
        arg := args[i]
        if arg == "--offset" {
            // Parse offset value
        } else if arg == "--limit" {
            // Parse limit value
        } else if arg == "--print-header" {
            // Parse boolean value
        }
        i++
    }

    // Execute inspect logic
}
```

## Decision: Integration with Existing Error Types

**Chosen**: Reuse pkg/frozendb error types via printError() helper

**Rationale**:
- Existing CLI commands use printError() function that formats errors and exits with code 1
- Error types already defined in pkg/frozendb/errors.go (InvalidInputError, PathError, CorruptDatabaseError, etc.)
- Inspect command follows same error handling pattern as other CLI commands

**Implementation Pattern**:
```go
// From cmd/frozendb/main.go
func printError(err error) {
    fmt.Fprintln(os.Stderr, formatError(err))
    os.Exit(1)
}

// In handleInspect()
if offset < 0 {
    printError(pkg_frozendb.NewInvalidInputError("offset cannot be negative", nil))
}
```

## Integration Points

### With FileManager (internal/frozendb/file_manager.go)
- Use `NewDBFile(path, MODE_READ)` to open database in read-only mode
- Use `Read(start int64, size int32)` for streaming row reads
- Use `Size() int64` to calculate total rows and detect EOF

### With Header (internal/frozendb/header.go)
- Use `Header.UnmarshalText()` to parse header
- Use `Header.GetRowSize()` to determine row size for offset calculations
- Use `Header.GetSkewMs()` and `Header.GetVersion()` for header table display

### With RowUnion (internal/frozendb/row_union.go)
- Use `RowUnion.UnmarshalText()` to parse complete rows (DataRow, NullRow, ChecksumRow)
- Check which field is non-nil to determine row type

### With PartialDataRow (internal/frozendb/partial_data_row.go)
- Use `PartialDataRow.UnmarshalText()` for partial row parsing when EOF detected
- Use `PartialDataRow.GetState()` to determine partial row state

### With Error Types (pkg/frozendb/errors.go)
- Use `NewInvalidInputError()` for validation errors (negative offset, invalid --print-header value)
- Use `NewPathError()` for file access errors (file not found, permission denied)
- Use `NewCorruptDatabaseError()` wrapped from UnmarshalText() failures

## Performance Considerations

### Memory Usage
- Fixed memory per iteration: one row buffer (row_size bytes, typically 4096)
- Header struct: ~32 bytes
- Row parsing structures: ~200 bytes maximum
- Total memory usage: O(1) regardless of database size ✅ Meets NFR-001

### I/O Performance
- Sequential file reads: one syscall per row
- No buffering needed: OS filesystem cache handles read-ahead
- Seek operations: O(1) using calculated offsets

### Edge Cases
- Empty database (only header + checksum): Display checksum row only
- Database with only header (no checksum): Error during checksum validation
- Huge offset (beyond EOF): Display headers but no rows, exit 0
- Limit=0: Display headers but no rows
- Corrupted row in middle of file: Display as "error" type, continue with next row

## Spec Test Coverage

All 22 functional requirements (FR-001 through FR-024, excluding removed FR-023) will have corresponding spec tests following the pattern:
- Test function naming: `Test_S_037_FR_XXX_Description()`
- Location: `cmd/frozendb/cli_spec_test.go`
- Co-located with implementation in cmd/frozendb/main.go

Example spec test structure:
```go
func Test_S_037_FR_001_AcceptsPathFlag(t *testing.T) {
    // Setup: create test database
    // Execute: run inspect command with --path flag
    // Assert: command succeeds and displays output
}

func Test_S_037_FR_017_DisplaysErrorTypeForCorruptedRow(t *testing.T) {
    // Setup: create database with intentionally corrupted row
    // Execute: run inspect command
    // Assert: output contains "error" type for corrupted row, other rows display correctly
}
```
