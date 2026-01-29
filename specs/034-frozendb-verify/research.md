# Research: frozendb verify

**Feature**: 034-frozendb-verify  
**Date**: 2026-01-29

## Purpose

This document consolidates research findings about implementing the verify operation, including analysis of existing codebase patterns, validation functions to reuse, and design decisions for the verify API.

## Existing Validation Infrastructure

### Header Validation

**Location**: `internal/frozendb/header.go`

**Key Functions**:
- `Header.UnmarshalText(headerBytes []byte) error` - Parses and validates header structure
- `Header.Validate() error` - Validates header field values

**Validation Coverage**:
- Exact 64-byte size (FR-001) ✓
- Newline at byte 63 (FR-001) ✓
- Signature "fDB" (FR-002) ✓
- Version = 1 (FR-003) ✓
- row_size range 128-65536 (FR-004) ✓
- skew_ms range 0-86400000 (FR-005) ✓
- JSON key ordering (FR-006) - Implicitly validated by strict format string
- Null byte padding (FR-007) ✓

**Decision**: Reuse existing `Header.UnmarshalText()` for all header validation (FR-001 through FR-007). This function already implements all required checks and returns `CorruptDatabaseError` for format violations.

### Checksum Validation

**Location**: `internal/frozendb/checksum.go`

**Key Types**:
- `Checksum` (uint32) - CRC32 value
- `ChecksumRow` - Full checksum row with baseRow[*Checksum]

**Key Functions**:
- `NewChecksumRow(rowSize int, dataBytes []byte) (*ChecksumRow, error)` - Calculates CRC32 using IEEE polynomial
- `ChecksumRow.UnmarshalText(text []byte) error` - Parses checksum row
- `ChecksumRow.Validate() error` - Validates start_control='C', end_control='CS'

**CRC32 Algorithm**: Uses `crc32.ChecksumIEEE()` from Go standard library (IEEE polynomial 0xedb88320) per v1_file_format.md section 6.2.

**Decision**: 
- Use `crc32.ChecksumIEEE()` directly for checksum calculation (FR-009, FR-011)
- Use `ChecksumRow.UnmarshalText()` to parse and validate checksum rows (FR-022)
- Implement verify-specific logic to track checksum coverage and validate blocks

### Parity Validation

**Location**: `internal/frozendb/row.go`

**Key Functions**:
- `baseRow.GetParity() ([2]byte, error)` - Calculates LRC parity using XOR on bytes [0] through [rowSize-4]
- `baseRow.UnmarshalText(text []byte) error` - Validates parity during row parsing (step 6 in UnmarshalText)

**Parity Algorithm**: XOR all bytes from position [0] through [rowSize-4] (inclusive), encode result as 2-character uppercase hex string.

**Decision**: 
- Parity validation (FR-012, FR-013) is **automatic** - no separate logic needed
- All rows must be validated for structure (FR-014 to FR-021), which requires calling `UnmarshalText()`
- `UnmarshalText()` already validates parity internally at step 6 (lines 368-376 in row.go)
- Per v1_file_format.md section 7.2: "For rows not covered by a checksum...implementations SHALL use parity bytes for validation"
- This requirement is satisfied by calling `UnmarshalText()` on every row

### Row Format Validation

**Location**: `internal/frozendb/row.go`, `data_row.go`, `null_row.go`, `partial_data_row.go`

**Key Functions**:
- `baseRow.UnmarshalText(text []byte) error` - Validates ROW_START, ROW_END, start_control, end_control, padding, parity
- `DataRow.UnmarshalText(text []byte) error` - Validates data row specific fields (UUID, JSON payload)
- `NullRow.UnmarshalText(text []byte) error` - Validates null row format
- `PartialDataRow.UnmarshalText(text []byte) error` - Validates partial row states

**Validation Coverage**:
- ROW_START = 0x1F (FR-014) ✓
- ROW_END = 0x0A (FR-015) ✓
- Valid start_control (FR-016) ✓
- Valid end_control (FR-017) ✓
- UUID Base64 encoding (FR-018) ✓
- UUIDv7 structure (FR-019) ✓
- Valid UTF-8 JSON (FR-020) ✓
- Correct padding (FR-021) ✓
- DataRow UUID non-zero check (FR-037) ✓

**Decision**: 
- Use existing `UnmarshalText()` methods for full row validation (FR-014 through FR-021, FR-037)
- Each row type's `UnmarshalText()` performs complete structural validation
- Verify operation calls these methods to validate row format compliance

### Partial Data Row Validation

**Location**: `internal/frozendb/partial_data_row.go`

**Key Functions**:
- `PartialDataRow.UnmarshalText(text []byte) error` - Parses and identifies partial row state
- `PartialDataRow.Validate() error` - Validates state-specific requirements

**Partial Row States**:
1. **State 1** (PartialDataRowWithStartControl): ROW_START + start_control only
2. **State 2** (PartialDataRowWithPayload): State 1 + UUID + JSON + padding
3. **State 3** (PartialDataRowWithSavepoint): State 2 + 'S' character

**Decision**: 
- Use `PartialDataRow.UnmarshalText()` to parse and validate partial rows (FR-025 through FR-032)
- Verify must check that partial row (if present) is the last row in file (FR-026)
- Validate no bytes exist after state boundary (FR-031)

## Error Handling Patterns

**Location**: `internal/frozendb/errors.go`, `docs/error_handling.md`

**Relevant Error Types**:
- `InvalidInputError` - For API misuse (wrong parameters)
- `CorruptDatabaseError` - For file format violations, corruption detection
- `ReadError` - For I/O failures during file reading
- `PathError` - For file path issues

**Decision**:
- Use `CorruptDatabaseError` for all validation failures detected by verify (FR-033, FR-034, FR-036)
- Include specific corruption type and location in error message
- Use `ReadError` for I/O failures when reading file
- Use `InvalidInputError` for invalid API parameters (empty path, etc.)

## File Reading Strategy

**Research Question**: How should verify read and validate the file?

**Options Considered**:

1. **Sequential Read with Buffered I/O**
   - Read file sequentially from start to end
   - Use buffered reader for efficient disk access
   - Parse rows one at a time

2. **Memory-Map Entire File**
   - Memory-map the entire file
   - Direct byte access without copying
   - Requires file size fits in virtual address space

3. **Block-Based Reading**
   - Read in fixed-size blocks (e.g., 1MB)
   - Parse rows within each block
   - Handle row boundaries spanning blocks

**Decision**: Sequential Read with Two Passes

**Rationale**:
- **Pass 1**: Validate checksums by jumping to expected positions, reading byte ranges, comparing CRC32
- **Pass 2**: Sequentially read entire file, validate each row with UnmarshalText
- Separates concerns: checksums validate blocks, rows validate structure/parity
- Aligns with frozenDB's "Fixed Memory" constitutional principle
- Memory usage bounded regardless of database size
- Simple implementation with standard `bufio.Reader` or `os.File.ReadAt()`
- Natural fit for fail-fast behavior (FR-038)
- Compatible with all file sizes from empty to unbounded

**Alternatives Rejected**:
- Single-pass interleaved: More complex logic, harder to reason about
- Memory-map: Violates fixed memory principle, fails on large files
- Checksum-only validation: Misses per-row corruption not covered by checksum

## Verify Algorithm Design

**High-Level Flow - Two Pass Approach**:

```
Pass 1: Validate Checksums
1. Read initial checksum row at offset 64
2. Validate initial checksum covers header bytes [0..63]
3. For each expected checksum position (every 10,000 rows):
   a. Calculate position: 64 + rowSize + (n * 10,000 * rowSize)
   b. Read checksum row, call ChecksumRow.UnmarshalText()
   c. Calculate byte range covered: [lastChecksumOffset, currentPosition)
   d. Read those bytes, calculate CRC32 with crc32.ChecksumIEEE()
   e. Compare calculated CRC32 to checksum value from row
4. Stop when no more checksum rows expected (less than 10,000 rows since last)

Pass 2: Validate All Rows
1. Validate header with Header.UnmarshalText()
2. Loop through every row in file:
   a. Read row_size bytes
   b. Call UnmarshalText() on row union (try DataRow/NullRow/ChecksumRow)
   c. If UnmarshalText succeeds, format and parity are valid
3. If file doesn't end on row boundary:
   a. Read remaining bytes
   b. Call PartialDataRow.UnmarshalText()
   c. Validate partial row state
4. Return success or first error
```

**Why Two Passes**:
- **Pass 1 (Checksums)**: Validates cryptographic integrity of large blocks - catches corruption efficiently
- **Pass 2 (Rows)**: Validates every row's structure and parity - catches per-row corruption
- Simpler logic: checksum pass doesn't care about row types, row pass doesn't care about checksums
- Easier to reason about: each pass has one responsibility

**Checksum Pass Details**:
- Positions calculated statically: no need to parse all rows
- Jump directly to expected checksum positions
- Read byte ranges between checksums for CRC32 calculation
- Validates FR-008, FR-009, FR-010, FR-011

**Row Pass Details**:
- Sequential read through entire file
- UnmarshalText() validates structure (FR-014 to FR-021) and parity (FR-012, FR-013)
- Works for all row types (data, null, checksum)
- Validates FR-022 (checksum row format), FR-023 (null row format), FR-024 (null row UUID)

**Partial Row Handling**:
- After row pass completes, check if bytes remain
- If remaining bytes < row_size: parse as PartialDataRow
- Validates FR-025 through FR-032

**Fail-Fast Behavior** (FR-038):
- Return immediately on first validation failure in either pass
- Checksum failures caught in pass 1
- Row format/parity failures caught in pass 2

## Scope Boundaries

**What Verify DOES NOT Check** (per specification):

- **Transaction Nesting** (FR-039): Does not validate that transactions are properly nested or that start_control 'T' vs 'R' follows transaction state
- **UUID Timestamp Ordering** (FR-040): Does not validate that UUID timestamps are in ascending order respecting skew_ms
- **Transaction Semantics**: Does not validate savepoint numbering, rollback targets, or commit/rollback logic
- **Data Relationships**: Does not validate foreign keys, referential integrity, or cross-row relationships

**Rationale**: Verify focuses on cryptographic and structural integrity. Transaction semantics validation is orthogonal and would require different tooling (e.g., transaction consistency checker).

## API Design

**Function Signature**:

```go
// Verify validates a frozenDB file for corruption
// Returns nil if file is valid, error with corruption details if invalid
func Verify(path string) error
```

**Alternative Considered**:

```go
// VerifyWithDetails returns detailed validation results
func VerifyWithDetails(path string) (*VerifyResult, error)

type VerifyResult struct {
    Valid             bool
    RowsVerified      int
    ChecksumBlocks    int
    BytesVerified     int64
    CorruptionDetails *CorruptionError // nil if valid
}
```

**Decision**: Start with simple `Verify(path string) error`

**Rationale**:
- Meets specification requirements (FR-035, FR-036)
- User Story P1 only requires success/failure with error details
- User Story P3 (performance reporting) marked as P3 priority
- Can add `VerifyWithDetails()` later if needed without breaking changes
- Simpler API is easier to use and test

**Alternatives Rejected**:
- Detailed result struct: Over-engineering for P1/P2 requirements
- Context parameter: Not needed for read-only, fail-fast operation
- Options struct: No configuration options specified in requirements

## Summary of Decisions

| Decision | Rationale |
|----------|-----------|
| Reuse `Header.UnmarshalText()` | Covers all header validation (FR-001 to FR-007) |
| Use `crc32.ChecksumIEEE()` directly | Standard library IEEE CRC32 per spec |
| Reuse row `UnmarshalText()` methods | Full row format validation (FR-014 to FR-021) AND parity validation (FR-012, FR-013) |
| Use `CorruptDatabaseError` | Correct error type for corruption detection |
| Two-pass validation | Pass 1: checksums (block integrity), Pass 2: all rows (structure/parity) |
| Fail-fast on first error | Simplifies reporting, meets user expectations |
| Simple `Verify(path string) error` API | Sufficient for P1/P2 requirements |
| Exclude transaction validation | Per FR-039, FR-040 scope boundaries |
| Calculate checksum positions statically | Positions determined by row_size arithmetic, no dynamic tracking needed |
| Parity validation via UnmarshalText | Row structure validation (FR-014-FR-021) requires UnmarshalText, which validates parity automatically |

## Next Steps

Proceed to Phase 1:
- Define data model for verify operation (VerifyState internal struct)
- Create API contract specification (Verify function documentation)
