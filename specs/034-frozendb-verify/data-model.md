# Data Model: frozendb verify

**Feature**: 034-frozendb-verify  
**Date**: 2026-01-29

## Purpose

This document defines the internal data structures and state management for the verify operation. These are implementation details not exposed in the public API.

## Internal State Structures

### VerifyState (Internal)

Minimal state tracking for two-pass verification.

**Purpose**: Track file metadata and current pass progress.

**Attributes**:
- `rowSize` (int): Row size from header, used to calculate positions
- `fileSize` (int64): Total file size (for detecting partial rows)
- `numDataRows` (int64): Total number of data/null rows (calculated from file size)
- `numChecksumRows` (int): Number of expected checksum rows

**Calculations**:
```go
// After reading header and determining rowSize
numDataRows = (fileSize - 64) / rowSize - numChecksumRows
numChecksumRows = (numDataRows / 10000)
if numDataRows % 10000 == 0 && numDataRows > 0 {
    numChecksumRows++ // Account for initial checksum
}

// Checksum positions
checksumPositions[0] = 64 // Initial checksum
for i := 1; i <= numChecksumRows; i++ {
    checksumPositions[i] = 64 + rowSize + (i-1)*10000*rowSize + rowSize*(i-1)
}
```

**Usage**:
- Calculated once after reading header
- Used in Pass 1 to jump to checksum positions
- Used in Pass 2 to iterate through all rows

### ChecksumValidation (Internal)

Represents checksum validation at a specific position.

**Purpose**: Calculate and compare checksums at expected positions.

**Attributes**:
- `startOffset` (int64): Starting byte offset for checksum coverage (inclusive)
- `endOffset` (int64): Ending byte offset for checksum coverage (exclusive)
- `expectedChecksum` (Checksum): Checksum value from ChecksumRow

**Usage**:
- Calculate expected checksum position from row_size and data row count
- When at checksum position:
  - `startOffset` = last checksum row start (or 0 for initial)
  - `endOffset` = current position (before this checksum row)
  - Read bytes [startOffset, endOffset)
  - Calculate CRC32, compare to expectedChecksum
- Initial checksum: startOffset=0, endOffset=64 (header only)

**Example Offsets** (row_size=256):
```
Offset 0:     Header (64 bytes)
Offset 64:    Initial checksum row (256 bytes) - covers [0, 64)
Offset 320:   Data row 0 (256 bytes)
Offset 576:   Data row 1 (256 bytes)
...
Offset 2560320: Data row 9999 (256 bytes)
Offset 2560576: Second checksum row (256 bytes) - covers [64, 2560576)
Offset 2560832: Data row 10000 (256 bytes)
...
```

## Corruption Error Details

### CorruptionType (Enum)

Categorizes the type of corruption detected for error reporting (FR-033).

**Values**:
- `CorruptionTypeHeader`: Header validation failure (FR-001 to FR-007)
- `CorruptionTypeChecksum`: Checksum mismatch (FR-009, FR-011)
- `CorruptionTypeParity`: Parity byte mismatch (FR-012, FR-013)
- `CorruptionTypeRowFormat`: Invalid row structure (FR-014 to FR-021)
- `CorruptionTypePartialRow`: Invalid partial row (FR-025 to FR-032)

**Note**: This is conceptual for error message formatting, not a new Go type. The `CorruptDatabaseError` message will include these prefixes.

### Error Message Format

**Pattern**: `{CorruptionType}: {Description} at {Location}`

**Examples**:
- `"CorruptionTypeHeader: invalid signature at offset 0"`
- `"CorruptionTypeChecksum: CRC32 mismatch for block 2 (expected A1B2C3D4, got E5F6G7H8) at row 20001"`
- `"CorruptionTypeParity: parity mismatch (expected AB, got CD) at row 15432, offset 2471936"`
- `"CorruptionTypeRowFormat: invalid ROW_START byte at row 500, offset 80192"`
- `"CorruptionTypePartialRow: State 2 partial row has bytes beyond boundary at end of file"`

**Location Information** (FR-034):
- Row number (0-based, for data/null/checksum rows)
- Byte offset in file (for precise corruption location)
- Block number (for checksum errors, counting from 0)

## Row Type Detection

### Row Classification

During sequential reading, verify must determine row type without full parsing.

**Detection Strategy**:

```
1. Check if remaining bytes < rowSize → Partial row
2. Read rowSize bytes
3. Check start_control byte:
   - 'C' → Checksum row
   - 'T' → Data row or Null row (distinguish by end_control)
   - 'R' → Data row (continuation)
4. Parse according to row type
```

**Null Row vs Data Row Distinction**:
- Both can have start_control='T'
- Null rows have end_control='NR'
- Data rows have end_control ∈ {TC, RE, SC, SE, R0-R9, S0-S9}
- Must peek at end_control to distinguish

## Validation Flow State Machine

### Two-Pass Validation

**Pass 1: Checksum Validation**
1. **ReadInitialChecksum**: Read checksum row at offset 64
2. **ValidateHeaderChecksum**: Calculate CRC32 of bytes [0, 64), compare to checksum
3. **IterateChecksums**: For each expected checksum position:
   - Read checksum row
   - Parse with ChecksumRow.UnmarshalText()
   - Calculate byte range: [previousChecksumOffset, currentOffset)
   - Read bytes, calculate CRC32
   - Compare to checksum value

**Pass 2: Row Validation**
1. **ValidateHeader**: Header.UnmarshalText() on bytes [0, 64)
2. **IterateRows**: For each row_size chunk:
   - Read row_size bytes
   - Try DataRow.UnmarshalText(), then NullRow.UnmarshalText(), then ChecksumRow.UnmarshalText()
   - If any succeeds, row is valid (structure + parity validated)
3. **HandlePartialRow**: If bytes remain < row_size:
   - Read remaining bytes
   - PartialDataRow.UnmarshalText()
   - Validate state

### Validation per Row Type

**Pass 1 - Checksums Only**:
- Read checksum rows at calculated positions
- Parse with ChecksumRow.UnmarshalText()
- Validate CRC32 matches byte range

**Pass 2 - All Rows**:
- Call UnmarshalText() on every row
- Validates structure (ROW_START, ROW_END, control bytes, padding)
- Validates parity (automatically in UnmarshalText step 6)
- Validates row-type specific fields (UUID, JSON, etc.)

**Result**: 
- FR-008 to FR-011 validated in Pass 1
- FR-012 to FR-024, FR-037 validated in Pass 2
- FR-025 to FR-032 validated after Pass 2 if partial row exists

## Data Flow

```
File (bytes)
    ↓
PASS 1: CHECKSUM VALIDATION
    ↓
Read checksum row at offset 64 → ChecksumRow.UnmarshalText()
    ↓
Calculate CRC32 of bytes [0, 64) → Compare to checksum value
    ↓
For each checksum position i (calculated from rowSize):
    ↓
    position = 64 + rowSize + ((i-1) * 10000 * rowSize) + ((i-1) * rowSize)
    ↓
    Seek to position → Read rowSize bytes → ChecksumRow.UnmarshalText()
    ↓
    Calculate byte range: [lastChecksumPos, currentPos)
    ↓
    Read bytes in range → crc32.ChecksumIEEE() → Compare to checksum
    ↓
    If mismatch: return CorruptDatabaseError (FR-011)
    ↓
Pass 1 Complete ✓

PASS 2: ROW VALIDATION
    ↓
Read header bytes [0, 64) → Header.UnmarshalText() → Validate (FR-001 to FR-007)
    ↓
Seek to offset 64 (first row after header)
    ↓
Loop: Read rowSize bytes at current offset
    ↓
    Try to unmarshal as row union:
        DataRow.UnmarshalText() OR
        NullRow.UnmarshalText() OR
        ChecksumRow.UnmarshalText()
    ↓
    If any succeeds: row is valid (format + parity validated)
    If all fail: return CorruptDatabaseError (FR-014 to FR-024)
    ↓
    Advance offset by rowSize
    ↓
    If EOF: break loop
    ↓
If remaining bytes > 0:
    ↓
    If remaining < rowSize:
        └─ PartialDataRow.UnmarshalText() → Validate state (FR-025 to FR-032)
    Else:
        └─ return CorruptDatabaseError (incomplete row)
    ↓
Pass 2 Complete ✓
    ↓
Success (FR-035)
```

## Memory Usage

**Bounded Memory Principle**:
- No in-memory accumulation of full file
- Pass 1: Seek to checksum positions, read byte ranges on-demand
- Pass 2: Sequential read with single row buffer
- Single row buffer (max 65536 bytes for max row_size)
- VerifyState struct (< 100 bytes)
- Byte range buffer for checksum calculation (max 10,000 * 65536 bytes = ~655 MB worst case, but can be read in chunks and streamed through CRC32)
- Total working memory: Can be kept under 1MB with chunked CRC32 calculation

**Chunked CRC32 for Large Ranges**:
- Instead of reading entire 10k row range into memory, read in 1MB chunks
- Stream chunks through `hash.Hash` (crc32) incrementally
- Final memory: 1MB buffer + single row buffer + state = ~2MB maximum

**Constitutional Compliance**: Satisfies "Performance With Fixed Memory" principle - memory usage does not scale with database size.

## Integration with Existing Types

### Reused from Existing Codebase

- `Header` (header.go): Header structure and validation
- `Checksum` (checksum.go): CRC32 value and encoding
- `ChecksumRow` (checksum.go): Checksum row structure
- `DataRow` (data_row.go): Data row structure and validation
- `NullRow` (null_row.go): Null row structure and validation
- `PartialDataRow` (partial_data_row.go): Partial row states and validation
- `CorruptDatabaseError` (errors.go): Error type for corruption

### New Types Needed

**None**. All validation can be performed using:
- Existing types' `UnmarshalText()` and `Validate()` methods
- Simple offset calculations based on row_size
- The `VerifyState` structure is a simple local variable within the verify function, not an exported type
- Checksum positions calculated statically using arithmetic, no need for a ChecksumBlock type

## Summary

The verify operation uses a **two-pass approach**: Pass 1 validates all checksums by jumping to calculated positions and comparing CRC32 values; Pass 2 sequentially validates every row's structure and parity by calling `UnmarshalText()`. This separation of concerns makes the logic simpler and easier to test. Memory usage remains bounded through chunked CRC32 calculation, maintaining constitutional compliance.
