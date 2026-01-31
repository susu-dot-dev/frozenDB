# Data Model: CLI Inspect Command

**Feature**: 037-cli-inspect-command  
**Date**: 2026-01-30  
**Status**: Complete

This document defines the new data entities, validation rules, and state changes introduced by the inspect command feature.

## New Entities

### InspectRow

Represents a single row's display information in the inspect output table.

**Attributes**:
- `Index` (int64): Zero-based row index (0 = first checksum row)
- `Type` (string): Row type identifier - one of: "Data", "NullRow", "Checksum", "partial", "error"
- `Key` (string): UUID key in string format (empty for Checksum, NullRow, error)
- `Value` (string): JSON payload or checksum string (empty for NullRow, partial, error)
- `Savepoint` (string): Boolean string "true"/"false" or empty for non-applicable rows
- `TxStart` (string): Boolean string "true"/"false" or empty for non-applicable rows
- `TxEnd` (string): Boolean string "true"/"false" or empty for non-applicable rows
- `Rollback` (string): Boolean string "true"/"false" or empty for non-applicable rows
- `Parity` (string): Two-character hex string representing parity bytes

**Validation Rules**:
- Index MUST be non-negative
- Type MUST be one of the five defined values
- Boolean fields MUST be "true", "false", or empty string
- Parity MUST be exactly 2 uppercase hex characters or empty string
- Empty string fields are represented with no characters (not "null" or "-")

**State Transitions**: None (immutable output entity)

### InspectConfig

Represents the parsed command-line configuration for the inspect operation.

**Attributes**:
- `Path` (string): Database file path (required)
- `Offset` (int64): Starting row index (default: 0)
- `Limit` (int64): Maximum rows to display (default: -1 for all)
- `PrintHeader` (bool): Whether to display header table (default: false)
- `FinderStrategy` (FinderStrategy): Finder strategy from global flags (default: binary)

**Validation Rules**:
- Path MUST be non-empty string with .fdb extension (validated by NewDBFile)
- Offset MUST be non-negative integer (FR-005)
- Limit MAY be any integer; negative values mean "display all remaining rows"
- PrintHeader MUST be boolean value

**Relationships**:
- InspectConfig → DBFile: One config creates one DBFile connection
- InspectConfig → InspectRow[]: One config generates zero or more InspectRows

### HeaderInfo

Represents database header metadata for the header table display.

**Attributes**:
- `RowSize` (int): Bytes per row from header
- `ClockSkew` (int): Time skew window in milliseconds from header
- `FileVersion` (int): File format version from header

**Validation Rules**:
- Inherited from Header.Validate()
- RowSize MUST be between 128-65536
- ClockSkew MUST be between 0-86400000
- FileVersion MUST be 1 (for v1 format)

**Relationships**:
- HeaderInfo extracted from Header entity (existing)
- HeaderInfo displayed only when InspectConfig.PrintHeader is true

## Modified Entities

None. This feature does not modify existing data structures; it only reads and displays them.

## New Validation Rules

### Offset Validation (FR-005)

**Rule**: Negative offset values are invalid

**Validation Logic**:
```
if offset < 0 {
    return InvalidInputError("offset cannot be negative")
}
```

**Error Mapping**: InvalidInputError with exit code 1

### PrintHeader Validation

**Rule**: PrintHeader flag must be parseable as boolean

**Validation Logic**:
```
validValues := []string{"true", "false", "1", "0", "t", "f"}
normalizedValue := strings.ToLower(value)
if !contains(validValues, normalizedValue) {
    return InvalidInputError("invalid boolean value for --print-header")
}
```

**Error Mapping**: InvalidInputError with exit code 1

### Row Type Detection Logic

**Rule**: Determine row type based on parse results and file position

**Decision Flow**:
1. If EOF occurs before row_size bytes AND this is the last row → type="partial"
2. If UnmarshalText() fails → type="error"
3. If RowUnion.ChecksumRow is non-nil → type="Checksum"
4. If RowUnion.NullRow is non-nil → type="NullRow"
5. If RowUnion.DataRow is non-nil → type="Data"

**Error Mapping**: 
- Parse failures map to type="error" (continue processing, set exit code 1)
- Unexpected conditions map to CorruptDatabaseError (exit immediately)

## State Changes

### File Access Mode

**Change**: Open database file in read-only mode

**Implementation**:
```go
file, err := NewDBFile(path, MODE_READ)
```

**Rationale**: Inspect is a read-only operation; no write lock needed

### Error Accumulation State

**Change**: Track whether any row parsing errors occurred during inspection

**Implementation**:
```go
var hasErrors bool = false

for each row {
    if parseError {
        hasErrors = true
        printErrorRow()
    }
}

if hasErrors {
    os.Exit(1)
} else {
    os.Exit(0)
}
```

**Rationale**: FR-018 requires exit code 1 if any row fails, but processing must continue

## Data Flow

### Inspect Operation Flow

```
1. Parse command-line flags → InspectConfig
2. Validate InspectConfig (offset >= 0, path non-empty)
3. Open database file → DBFile (read-only mode)
4. Read and parse header → HeaderInfo
5. If PrintHeader: Display header table
6. Calculate row range (offset, limit)
7. For each row in range:
   a. Read row bytes from file
   b. Parse row → InspectRow or error
   c. Display row in TSV format
   d. If error: set hasErrors flag
8. Exit with code 0 (no errors) or 1 (has errors)
```

### Error Row Handling Flow

```
1. Attempt to read row_size bytes at calculated offset
2. If read fails:
   a. Extract parity from available bytes (if any)
   b. Create InspectRow with type="error"
   c. Display error row
   d. Set hasErrors = true
   e. Continue to next row
3. Attempt to parse row bytes
4. If parsing fails:
   a. Extract parity from row bytes
   b. Create InspectRow with type="error"
   c. Display error row
   d. Set hasErrors = true
   e. Continue to next row
```

## Transaction Control Extraction Logic

### TxStart Extraction (FR-010)

**Rule**: True if start_control is 'T', false otherwise

**Implementation**:
```go
func extractTxStart(startControl StartControl) string {
    if startControl == START_TRANSACTION {
        return "true"
    }
    return "false"
}
```

### TxEnd Extraction (FR-010)

**Rule**: True if end_control is TC or SC, false otherwise

**Implementation**:
```go
func extractTxEnd(endControl EndControl) string {
    if endControl[1] == 'C' {
        return "true"
    }
    return "false"
}
```

### Savepoint Extraction (FR-010)

**Rule**: True if end_control first character is 'S', false otherwise

**Implementation**:
```go
func extractSavepoint(endControl EndControl) string {
    if endControl[0] == 'S' {
        return "true"
    }
    return "false"
}
```

### Rollback Extraction (FR-010)

**Rule**: True if end_control second character is digit (0-9), false otherwise

**Implementation**:
```go
func extractRollback(endControl EndControl) string {
    if endControl[1] >= '0' && endControl[1] <= '9' {
        return "true"
    }
    return "false"
}
```

## Row Type-Specific Field Mapping

### Data Row Mapping (FR-010)

| InspectRow Field | Source |
|------------------|--------|
| Index | Loop index |
| Type | "Data" |
| Key | DataRowPayload.Key.String() |
| Value | string(DataRowPayload.Value) |
| Savepoint | extractSavepoint(EndControl) |
| TxStart | extractTxStart(StartControl) |
| TxEnd | extractTxEnd(EndControl) |
| Rollback | extractRollback(EndControl) |
| Parity | Extract from rowBytes[rowSize-3:rowSize-1] |

### NullRow Mapping (FR-009)

| InspectRow Field | Source |
|------------------|--------|
| Index | Loop index |
| Type | "NullRow" |
| Key | NullRowPayload.Key.String() |
| Value | "" (empty string) |
| Savepoint | "false" |
| TxStart | "true" |
| TxEnd | "true" |
| Rollback | "false" |
| Parity | Extract from rowBytes[rowSize-3:rowSize-1] |

### Checksum Row Mapping (FR-011)

| InspectRow Field | Source |
|------------------|--------|
| Index | Loop index |
| Type | "Checksum" |
| Key | "" (empty string) |
| Value | Checksum.CRC32Value (Base64 string) |
| Savepoint | "" (empty string) |
| TxStart | "" (empty string) |
| TxEnd | "" (empty string) |
| Rollback | "" (empty string) |
| Parity | Extract from rowBytes[rowSize-3:rowSize-1] |

### Partial Row Mapping (FR-012)

| InspectRow Field | Source |
|------------------|--------|
| Index | Loop index |
| Type | "partial" |
| Key | If state >= PartialDataRowWithPayload: Key.String(), else "" |
| Value | If state >= PartialDataRowWithPayload: string(Value), else "" |
| Savepoint | If state == PartialDataRowWithSavepoint: "true", else "" |
| TxStart | extractTxStart(StartControl) if available, else "" |
| TxEnd | "" (partial rows never have end_control) |
| Rollback | "" (partial rows never have end_control) |
| Parity | "" (partial rows don't have parity bytes yet) |

### Error Row Mapping (FR-017)

| InspectRow Field | Source |
|------------------|--------|
| Index | Loop index |
| Type | "error" |
| Key | "" (empty string) |
| Value | "" (empty string) |
| Savepoint | "" (empty string) |
| TxStart | "" (empty string) |
| TxEnd | "" (empty string) |
| Rollback | "" (empty string) |
| Parity | Extract from rowBytes if available, else "" |

## Edge Case Handling

### Empty Database
- File contains only header (64 bytes) with no checksum row
- Result: Header parsing succeeds, but checksum row read fails
- Behavior: Return CorruptDatabaseError, exit 1

### Database with Only Checksum
- File contains header + checksum row, no data rows
- Result: Display checksum row at index 0, then stop
- Exit code: 0 (no errors)

### Offset Beyond File Size (FR-016)
- offset >= totalRows
- Result: Display header table (if requested), display row table header, display zero data rows
- Exit code: 0 (not an error)

### Limit Zero
- limit = 0
- Result: Display header table (if requested), display row table header, display zero data rows
- Exit code: 0

### Corrupted Checksum Row
- Checksum row fails UnmarshalText() validation
- Result: Display as type="error", continue processing remaining rows
- Exit code: 1

### Partial Row Not at End
- Partial row detected in middle of file (file size suggests more rows follow)
- Result: This violates file format spec section 8.6.4
- Behavior: Display as type="error", continue processing
- Exit code: 1
