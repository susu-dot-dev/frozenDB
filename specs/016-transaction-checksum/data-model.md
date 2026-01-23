# Data Model: Transaction Checksum Row Insertion

## Overview

This feature adds automatic checksum row insertion at 10,000-row intervals within frozenDB transactions. The implementation modifies `Transaction` type to insert checksum rows transparently after writing the 10,000th complete row.

## Modified Types

### Transaction

**New Field**:
- `db` DBFile - Interface for reading rows and calculating checksums

**New Behavior**:
- After each row write (AddRow, Commit, Rollback), calculate row count from file size
- Insert checksum row IMMEDIATELY after write completes if row count is exactly 10,000
- Use `db.Size()` to get current file size

---

### DBFile (New Interface)

**Purpose**: Interface for file operations, enabling mock implementations for testing

**Methods**:
- `Read(start int64, size int32) ([]byte, error)` - Read bytes from file at specified offset
- `Size() int64` - Get current file size
- `Close() error` - Close the file
- `SetWriter(dataChan <-chan Data) error` - Set write channel for appending data

**Implementing Types**:
- `FileManager` struct (existing implementation in file_manager.go)

---

### RowUnion (New Type)

**Purpose**: Union type holding pointers to all row types for generic unmarshaling

**Fields**:
- `DataRow` *DataRow - Pointer to DataRow or nil
- `NullRow` *NullRow - Pointer to NullRow or nil
- `ChecksumRow` *ChecksumRow - Pointer to ChecksumRow or nil

**Methods**:
- `UnmarshalText(text []byte) error` - Unmarshals row bytes by examining control bytes and validating parity

**Invariant**: Exactly one pointer will be non-nil after successful `UnmarshalText()` call

---

## Row Counting for Checksums

### Counted Toward Checksum Interval

| Row Type | Counted? | Reason |
|-----------|-----------|---------|
| DataRow | ✅ YES | Complete row with data |
| NullRow | ✅ YES | Complete row (single-row transaction) |

### Excluded from Checksum Interval

| Row Type | Counted? | Reason |
|-----------|-----------|---------|
| ChecksumRow | ❌ NO | Integrity row itself, not data |
| PartialDataRow | ❌ NO | Incomplete, not committed |

---

## Checksum Insertion Logic

### Row Count Calculation from File Size

**Algorithm**:
```go
// 1. Get current file size
fileSize := db.Size()

// 2. Calculate data bytes (exclude header and checksum rows)
dataStartOffset := int64(HEADER_SIZE + rowSize) // After header + initial checksum
dataBytes := fileSize - dataStartOffset

// 3. Calculate row count
rowCount := dataBytes / int64(rowSize)
```

### Checksum Trigger

**Condition**:
```go
needsChecksum := rowCount == 10000 || rowCount == 20000 || rowCount == 30000 ...
// Or: rowCount > 0 && rowCount % 10000 == 0
```

### Checksum Insertion Timing

**Insert immediately after row write completes**:
- After `AddRow()` writes a DataRow
- After `Commit()` writes a DataRow or NullRow
- After `Rollback()` writes a DataRow or NullRow
- Checksum row is inserted BEFORE any further writes occur

**Location**:
- Checksum row appears immediately after the 10,000th complete row
- Can appear between rows within an active transaction
- Does not interrupt transaction continuity - transaction continues normally

### Checksum Calculation Process

1. **Read 10,000 rows** from file starting from last checksum position
2. **Determine row type** from control bytes (start_control at [1], end_control at [rowSize-5:rowSize-4])
3. **Unmarshal directly** into correct row type using `RowUnion.UnmarshalText()`
4. **Concatenate** all row bytes
5. **Calculate CRC32** using IEEE polynomial (0xedb88320)
6. **Create ChecksumRow** via `NewChecksumRow(header, dataBytes)`
7. **Write ChecksumRow** to file

**RowUnion Type**:
- `DataRow` *DataRow - Data row with key-value
- `NullRow` *NullRow - Single-row transaction
- `ChecksumRow` *ChecksumRow - Checksum integrity row

**Validation:**
- `RowUnion.UnmarshalText()` uses control bytes to determine type
- Calls individual row type's `UnmarshalText()` which automatically validates parity
- Returns `CorruptDatabaseError` if parity mismatch

**Checksum Row Format**:
- `start_control = 'C'`
- `end_control = 'CS'`
- Contains CRC32 value (Base64 encoded, 8 bytes)
- Fixed width: `rowSize` bytes

---

## Key Constraints

### Checksum Placement
- Checksum rows must appear immediately after 10,000th complete row (10,000, 20,000, 30,000...)
- Checksum rows are inserted immediately after row write completes
- Can appear between rows within an active transaction
- Does not affect transaction boundaries, savepoints, or committed row counts

### Data Integrity
- All row parity must be validated before calculating checksum
- Use `RowUnion.UnmarshalText()` to determine row type from control bytes
- `RowUnion.UnmarshalText()` creates RowUnion with pointers to DataRow, NullRow, ChecksumRow
- Calls individual row type's `UnmarshalText()` which automatically validates parity and returns CorruptDatabaseError on mismatch
- CRC32 checksum covers all bytes since previous checksum (or header for first checksum)
- RowUnion ensures exactly one row type is non-nil after successful unmarshal

### Transparency
- Checksum rows must NOT appear in query results
- Checksum rows must NOT be counted as committed data
- Transaction logic must behave identically whether checksum rows are present or not

### File Positioning
- All checksum positions derived from file size and row count
- Row count calculated as: `(fileSize - HEADER_SIZE - checksumRows * rowSize) / rowSize`
- No explicit offset tracking needed
