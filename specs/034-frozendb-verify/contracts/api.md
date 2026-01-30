# API Contract: frozendb verify

**Feature**: 034-frozendb-verify  
**Date**: 2026-01-29  
**Package**: github.com/susu-dot-dev/frozenDB/internal/frozendb

## Public API

### Verify Function

```go
// Verify validates the integrity of a frozenDB file.
//
// Verify performs comprehensive validation using a two-pass approach:
//
// Pass 1 - Checksum Validation:
//   - Validate initial checksum at offset 64 covers header [0..64)
//   - For each expected checksum position (every 10,000 data/null rows):
//     - Read checksum row, parse with ChecksumRow.UnmarshalText()
//     - Calculate byte range covered by this checksum
//     - Read bytes, calculate CRC32, compare to checksum value
//
// Pass 2 - Row Validation:
//   - Validate header with Header.UnmarshalText()
//   - For each row in file:
//     - Call UnmarshalText() to validate structure and parity
//     - Works for all row types (data, null, checksum)
//   - If file doesn't end on row boundary, validate as PartialDataRow
//
// Verify does NOT validate:
//   - Header structure and field values (64-byte header, signature, version, row_size, skew_ms)
//   - All checksum blocks (initial checksum covering header, subsequent checksums every 10,000 rows)
//   - Parity bytes for all rows after the last checksum block
//   - Row format compliance (ROW_START, ROW_END, control bytes, UUID format, JSON validity, padding)
//   - Partial data row validity if present as the last row
//
// Verify does NOT validate:
//   - Transaction nesting or state relationships between rows
//   - UUID timestamp ordering constraints
//   - Savepoint numbering or rollback semantics
//
func Verify(path string) error
```

## Functional Requirements Mapping

### Header Validation (FR-001 to FR-007)

**Implementation - Pass 2**:
- Call `Header.UnmarshalText()` on bytes [0, 64)

**Success**: Returns nil, header populated
**Failure**: Returns `CorruptDatabaseError` with header-specific message

### Checksum Validation (FR-008 to FR-011)

**Implementation - Pass 1**:
1. Calculate all checksum positions from row_size:
   - Position 0: offset 64 (initial checksum)
   - Position i: offset = 64 + rowSize + (i-1)*10000*rowSize + (i-1)*rowSize
2. For each checksum position:
   - Seek to position, read rowSize bytes
   - Parse with `ChecksumRow.UnmarshalText()`
   - Calculate byte range: [previousChecksumOffset, currentOffset)
   - Read bytes in range (can chunk for large ranges)
   - Calculate CRC32 with `crc32.ChecksumIEEE()` (incremental/chunked)
   - Compare to checksum value

**Success**: Checksum matches calculated value
**Failure**: Returns `CorruptDatabaseError` with checksum mismatch details

### Parity Validation (FR-012, FR-013)

**Implementation - Pass 2**:
- Parity validation is **automatic** - happens during row structure validation
- All rows validated in Pass 2 by calling `UnmarshalText()`
- `UnmarshalText()` validates parity internally (step 6 of baseRow.UnmarshalText)
- Per v1_file_format.md: rows after last checksum use parity validation - satisfied by UnmarshalText

**Success**: Parity matches calculated value
**Failure**: Returns `CorruptDatabaseError` with parity mismatch details

**Note**: No separate parity validation logic needed - it's inherent in row parsing

### Row Format Validation (FR-014 to FR-024, FR-037)

**Implementation - Pass 2**: 
- For each row, try to unmarshal as row union:
  - `DataRow.UnmarshalText()` for data rows
  - `NullRow.UnmarshalText()` for null rows  
  - `ChecksumRow.UnmarshalText()` for checksum rows
- If any succeeds, row is valid

**Success**: Row parses and validates successfully
**Failure**: Returns `CorruptDatabaseError` with format-specific message

### Partial Row Validation (FR-025 to FR-032)

**Implementation - After Pass 2**:
1. Check if bytes remain after last complete row
2. If remaining bytes < rowSize, parse with `PartialDataRow.UnmarshalText()`
3. Validate state (State 1, 2, or 3)
4. Ensure it's the last row in file

**Success**: Partial row is valid and positioned correctly
**Failure**: Returns `CorruptDatabaseError` with partial row error

### Error Reporting (FR-033, FR-034)

**Implementation**: All `CorruptDatabaseError` messages include:
- Corruption type (header, checksum, parity, row format, partial row)
- Specific details (expected vs actual values)
- Location (byte offset and/or row number)

### Success/Failure Behavior (FR-035, FR-036, FR-038)

**Implementation**:
- Return nil on complete successful validation (FR-035)
- Return error on any validation failure (FR-036)
- Return immediately on first failure (FR-038)

### Scope Exclusions (FR-039, FR-040)

**Implementation**: Verify does NOT:
- Check transaction nesting (start_control 'T' vs 'R' relationships)
- Validate UUID timestamp ordering
- Verify savepoint numbering or rollback semantics

## Performance Characteristics

**Time Complexity**: O(n) where n = file size in bytes (two passes: checksum validation + row validation)

**Memory Usage**: O(1) - Bounded by:
- Chunk buffer for CRC32 calculation (1MB default)
- Single row buffer (max 65536 bytes)
- State structs (< 100 bytes total)
- Total: ~2MB maximum regardless of file size

**I/O Pattern**: 
- Pass 1: Random access (seek to checksum positions)
- Pass 2: Sequential read from start to end

**Disk Seeks**: Pass 1 requires seeks to checksum positions; Pass 2 is purely sequential

## Compatibility Notes

**File Locking**: Verify does not acquire exclusive locks. It's safe to verify files opened in read mode by other processes. However, verifying a file being actively written may produce false corruption errors if write is in progress.

**Platform Support**: Linux (amd64, arm64)

**File Size Limits**: No practical limit - can verify files from empty (64 bytes + initial checksum) to terabytes

## Integration Examples

### Basic Verification

```go
import "github.com/susu-dot-dev/frozenDB/internal/frozendb"

func checkDatabase(path string) error {
    if err := frozendb.Verify(path); err != nil {
        return fmt.Errorf("database verification failed: %w", err)
    }
    fmt.Println("Database integrity confirmed")
    return nil
}
```

### Handling Different Error Types

```go
import (
    "errors"
    "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

func verifyWithErrorHandling(path string) {
    err := frozendb.Verify(path)
    if err == nil {
        fmt.Println("✓ Database is valid")
        return
    }

    var corruptErr *frozendb.CorruptDatabaseError
    var readErr *frozendb.ReadError
    var inputErr *frozendb.InvalidInputError

    switch {
    case errors.As(err, &corruptErr):
        fmt.Printf("✗ Corruption detected: %v\n", err)
        // Log to monitoring system, alert operators
        
    case errors.As(err, &readErr):
        fmt.Printf("✗ I/O error: %v\n", err)
        // Check disk health, permissions
        
    case errors.As(err, &inputErr):
        fmt.Printf("✗ Invalid input: %v\n", err)
        // Fix API usage
    }
}
```

### Automated Verification

```go
// Run verify as part of startup checks
func healthCheck(dbPath string) error {
    fmt.Println("Verifying database integrity...")
    
    start := time.Now()
    err := frozendb.Verify(dbPath)
    duration := time.Since(start)
    
    if err != nil {
        return fmt.Errorf("health check failed after %v: %w", duration, err)
    }
    
    fmt.Printf("Health check passed in %v\n", duration)
    return nil
}
```

## Spec Test Requirements

All functional requirements (FR-001 through FR-040) will be covered by spec tests in `verify_spec_test.go` following the naming convention:

```go
func Test_S_034_FR_001_HeaderSize(t *testing.T)
func Test_S_034_FR_002_HeaderSignature(t *testing.T)
func Test_S_034_FR_003_HeaderVersion(t *testing.T)
// ... through FR-040
```

Each spec test validates the exact requirement specified in the feature specification.
