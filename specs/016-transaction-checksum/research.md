# Research: Transaction Checksum Row Insertion

## Decision: File Size-Based Row Count Calculation

**Rationale:**

- No existing row tracking mechanism exists in the codebase
- Row count determined directly from file size and row_size
- Use `db.Size()` to get current file size
- Calculate: `(fileSize - HEADER_SIZE - checksumRows * rowSize) / rowSize`

**Alternatives Considered:**

1. **Sequential file scan on every operation** - Rejected due to performance
   overhead
2. **Persistent row counter in separate file** - Rejected as it violates
   single-file architecture
3. **File size calculation** - Selected approach: O(1) time, no extra storage

**Implementation Approach:**

```go
// Calculate row count from file size
func calculateRowCountFromFileSize(fm DBFile, header *Header) int {
    rowSize := header.GetRowSize()
    fileSize := fm.Size()

    // Skip header (64) and initial checksum (rowSize)
    dataStartOffset := int64(HEADER_SIZE + rowSize)
    dataBytes := fileSize - dataStartOffset

    // Calculate row count (all bytes are complete rows, excluding checksums)
    return int(dataBytes / int64(rowSize))
}
```

---

## Decision: Insert Checksum Rows Between Transactions

**Rationale:**

- Checksum row must appear immediately after 10,000th complete row
- Can occur in the middle of an active transaction
- Does not interrupt transaction continuity
- Inserted immediately after row write completes (AddRow, Commit, Rollback)

**Alternatives Considered:**

1. **Insert before next transaction begins** - Rejected: delays checksum
   insertion unnecessarily
2. **Insert in Begin()** - Rejected: doesn't cover case where 10,000th row is
   last row of transaction
3. **Insert immediately after row write completes** - Selected approach:
   immediate checksum insertion

**Implementation Approach:**

```go
// After each row write, check if checksum needed
func (tx *Transaction) checkAndInsertChecksum() error {
    rowCount := calculateRowCount(tx.db, tx.header)

    if rowCount > 0 && rowCount%10000 == 0 {
        return tx.insertChecksumRow()
    }
    return nil
}

// Call after AddRow(), Commit(), Rollback()
func (tx *Transaction) AddRow(key uuid.UUID, value string) error {
    // ... existing write logic ...

    // Check if checksum needed after row write
    if err := tx.checkAndInsertChecksum(); err != nil {
        return err
    }

    return nil
}
```

---

## Decision: Use NewChecksumRow for Row Creation

**Rationale:**

- `NewChecksumRow(header *Header, dataBytes []byte)` already exists in
  checksum.go
- Function handles CRC32 calculation using IEEE polynomial
- Automatically creates proper ChecksumRow structure with start_control='C',
  end_control='CS'
- Validates row structure before returning

**Alternatives Considered:**

1. **Direct struct creation** - Rejected: bypasses validation logic
2. **Custom checksum calculation** - Rejected: duplicates existing code
3. **Use existing NewChecksumRow** - Selected approach: leverages tested code

**Usage Pattern:**

```go
// Read 10,000 rows for checksum calculation
dataStartOffset := int64(HEADER_SIZE + rowSize)
rowsBytes, err := readRowsForChecksum(tx.db, dataStartOffset, 10000, rowSize)
if err != nil {
    return err
}

// Create checksum row - CRC32 calculated internally
checksumRow, err := NewChecksumRow(tx.header, rowsBytes)
if err != nil {
    return err
}

// Marshal to bytes for writing
checksumBytes, err := checksumRow.MarshalText()
if err != nil {
    return err
}
// Write checksumBytes to file
```

---

## Decision: FileManager Interface for Mocking

**Rationale:**

- User requirement: "Define a FileManager interface on top of the struct for
  easy passing in (& mocking)"
- Enables unit testing with mock FileManager implementations
- Allows NewTransaction to accept interface instead of concrete struct
- Maintains backward compatibility (FileManager implements the interface)

**Alternatives Considered:**

1. **Pass concrete FileManager struct** - Rejected: difficult to mock
2. **DBFile interface** - Selected approach: idiomatic Go pattern
3. **Dependency injection framework** - Rejected: overkill for this use case

**Interface Definition:**

```go
type DBFile interface {
    Read(start int64, size int32) ([]byte, error)
    Size() int64
    Close() error
    SetWriter(dataChan <-chan Data) error
}

// FileManager struct already implements this interface
```

**NewTransaction Signature:**

```go
func NewTransaction(db DBFile, header *Header, writeChan chan<- Data) (*Transaction, error) {
    // Set writer on FileManager
    if err := db.SetWriter(writeChan); err != nil {
        return nil, err
    }

    tx := &Transaction{
        // ... initialize fields ...
        db: db,    // Store interface
        writeChan:    writeChan,
        // ... other fields ...
    }

    return tx, nil
}
```

---

## Decision: Parity Validation Before Checksum Calculation

**Rationale:**

- Per v1_file_format.md section 7.4: "When calculating a new checksum for a
  block of rows, implementations MUST validate the parity of all rows in that
  block before calculating the checksum."
- Ensures data integrity at checksum creation time
- Parity bytes are at positions [rowSize-3:rowSize-2]
- LRC parity is calculated via XOR of bytes [0:rowSize-4]

**Alternatives Considered:**

1. **Skip parity validation** - Rejected: violates spec requirement
2. **Validate on read instead of write** - Rejected: doesn't protect checksum
   integrity
3. **Validate during checksum calculation** - Selected approach: spec-compliant

**Implementation:**

```go
// RowUnion holds pointers to all possible row types
// Exactly one pointer will be non-nil after unmarshaling
type RowUnion struct {
    DataRow     *DataRow
    NullRow     *NullRow
    ChecksumRow *ChecksumRow
}

// UnmarshalText unmarshals a row by examining control bytes first
// Reads start_control at position [1] and end_control at [rowSize-5:rowSize-4]
// Unmarshals directly into the correct type - no trial and error
func (ru *RowUnion) UnmarshalText(rowBytes []byte) error {
    startControl := rowBytes[1]
    endControl := string(rowBytes[rowSize-5 : rowSize-3])

    switch {
    case startControl == byte(CHECKSUM_ROW) && endControl == "CS":
        // ChecksumRow
        cr := &ChecksumRow{}
        if err := cr.UnmarshalText(rowBytes); err != nil {
            return nil, err
        }
        return &RowUnion{ChecksumRow: cr}, nil

    case startControl == byte(START_TRANSACTION) && endControl == "NR":
        // NullRow
        nr := &NullRow{}
        if err := nr.UnmarshalText(rowBytes); err != nil {
            return nil, err
        }
        return &RowUnion{NullRow: nr}, nil

    case startControl == byte(START_TRANSACTION), startControl == byte(ROW_CONTINUE):
        // DataRow (could also be NullRow based on end_control, handled above)
        dr := &DataRow{}
        if err := dr.UnmarshalText(rowBytes); err != nil {
            return nil, err
        }
        return &RowUnion{DataRow: dr}, nil

    default:
        return nil, NewCorruptDatabaseError(
            fmt.Sprintf("unknown row type: start_control=%c, end_control=%s", startControl, endControl),
            nil,
        )
    }
}

// Note: Row validation for checksum calculation is handled by Transaction.collectValidatedRowsForChecksum()
// which reads rows, validates them using RowUnion.UnmarshalText() (automatically validates parity),
// and collects complete DataRow and NullRow bytes for checksum calculation.
        if rowUnion.DataRow == nil && rowUnion.NullRow == nil && rowUnion.ChecksumRow == nil {
            return NewCorruptDatabaseError("unmarshaled row union has no valid type", nil)
        }
    }
    return nil
}
```

---

# Summary of Implementation Decisions

| Decision                     | Approach                             | Key Considerations                                   |
| ---------------------------- | ------------------------------------ | ---------------------------------------------------- |
| **Row Count Calculation**    | File size calculation                | O(1) time, no extra storage                          |
| **Checksum Insertion Point** | After row write completes            | Immediate insertion, can be in middle of transaction |
| **Checksum Row Creation**    | Use NewChecksumRow                   | Leverages existing tested code                       |
| **FileManager Interface**    | Define DBFile                        | Enables mocking                                      |
| **Parity Validation**        | Validate before checksum calculation | Spec requirement (section 7.4)                       |

These decisions provide a complete implementation strategy that:

- Satisfies all functional requirements from spec.md
- Maintains backward compatibility
- Follows existing code patterns and Go idioms
- Provides O(1) time complexity for row count calculation
- Ensures data integrity through parity validation
- Enables comprehensive testing via interface-based design
