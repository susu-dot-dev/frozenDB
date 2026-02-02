package frozendb

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// BinarySearchFinder is a Finder implementation that uses binary search on UUIDv7
// ordered keys to provide O(log n) lookup performance while maintaining constant
// memory usage like SimpleFinder.
//
// Design Philosophy:
//   - Binary Search Operations: Uses FuzzyBinarySearch for O(log n) key lookups
//   - Minimal In-Memory State: Only tracks current database file size and maxTimestamp
//   - Logical Index Mapping: Maps between logical indices (for binary search) and
//     physical indices (accounting for checksum rows)
//   - Constant Memory: O(row_size) - constant regardless of database size
//
// Memory Usage: O(row_size) - constant regardless of database size
// Performance: O(log n) for GetIndex, O(k) for transaction boundary methods where k <= 101
type BinarySearchFinder struct {
	dbFile        DBFile     // Database file interface for reading rows
	rowSize       int32      // Size of each row in bytes from header
	size          int64      // Confirmed file size (updated via OnRowAdded)
	maxTimestamp  int64      // Maximum timestamp among all complete data and null rows
	skewMs        int64      // Time skew window in milliseconds from database header
	tombstonedErr error      // Error that caused this Finder to be tombstoned (nil if not tombstoned)
	mu            sync.Mutex // Protects size, maxTimestamp, skewMs, and tombstonedErr fields for concurrent access
}

// NewBinarySearchFinder creates a new BinarySearchFinder instance.
//
// Parameters:
//   - dbFile: DBFile interface for reading database rows
//   - rowSize: Size of each row in bytes (from database header)
//   - rowEmitter: RowEmitter instance for subscribing to row notifications
//
// Returns:
//   - *BinarySearchFinder: Initialized finder instance
//   - error: InvalidInputError if parameters are invalid
//
// The finder initializes with the current database file size from dbFile.Size(),
// representing the extent of data confirmed via onRowAdded() callbacks.
func NewBinarySearchFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*BinarySearchFinder, error) {
	if dbFile == nil {
		return nil, NewInvalidInputError("dbFile cannot be nil", nil)
	}
	if rowSize < 128 || rowSize > 65536 {
		return nil, NewInvalidInputError(fmt.Sprintf("rowSize must be between 128 and 65536, got %d", rowSize), nil)
	}
	if rowEmitter == nil {
		return nil, NewInvalidInputError("rowEmitter cannot be nil", nil)
	}

	// Read header to get skewMs
	headerBytes, err := dbFile.Read(0, HEADER_SIZE)
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to read header", err)
	}
	if len(headerBytes) != HEADER_SIZE {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("incomplete header read: expected %d bytes, got %d", HEADER_SIZE, len(headerBytes)),
			nil,
		)
	}

	header := &Header{}
	if err := header.UnmarshalText(headerBytes); err != nil {
		return nil, NewCorruptDatabaseError("failed to parse header", err)
	}

	bsf := &BinarySearchFinder{
		dbFile:       dbFile,
		rowSize:      rowSize,
		size:         dbFile.Size(),
		maxTimestamp: 0,
		skewMs:       int64(header.GetSkewMs()),
	}

	// Initialize maxTimestamp by scanning existing rows
	if err := bsf.initializeMaxTimestamp(); err != nil {
		return nil, err
	}

	// Subscribe to RowEmitter for future row notifications
	_, err = rowEmitter.Subscribe(bsf.onRowAdded)
	if err != nil {
		return nil, err
	}

	return bsf, nil
}

// initializeMaxTimestamp scans all existing rows to find the maximum timestamp.
// This is called once during initialization to establish the baseline maxTimestamp.
func (bsf *BinarySearchFinder) initializeMaxTimestamp() error {
	bsf.mu.Lock()
	defer bsf.mu.Unlock()

	confirmedSize := bsf.size
	totalRows := (confirmedSize - HEADER_SIZE) / int64(bsf.rowSize)

	bsf.maxTimestamp = 0

	// Scan all rows to find maximum timestamp
	for i := int64(0); i < totalRows; i++ {
		rowBytes, err := bsf.readRow(i)
		if err != nil {
			// Read error during initialization - fail immediately
			return err
		}

		var rowUnion RowUnion
		if err := rowUnion.UnmarshalText(rowBytes); err != nil {
			// Skip corrupted rows
			continue
		}

		// Only consider complete DataRow and NullRow entries
		if rowUnion.DataRow != nil {
			key := rowUnion.DataRow.GetKey()
			if key != uuid.Nil {
				if err := ValidateUUIDv7(key); err == nil {
					timestamp := ExtractUUIDv7Timestamp(key)
					if timestamp > bsf.maxTimestamp {
						bsf.maxTimestamp = timestamp
					}
				}
			}
		} else if rowUnion.NullRow != nil {
			// Extract timestamp from NullRow key and compare, same as DataRow
			key := rowUnion.NullRow.GetKey()
			timestamp := ExtractUUIDv7Timestamp(key)
			if timestamp > bsf.maxTimestamp {
				bsf.maxTimestamp = timestamp
			}
		}
		// Skip ChecksumRow and PartialDataRow
	}

	return nil
}

// GetIndex returns the index of the first row containing the specified UUID key
// using binary search. Implements O(log n) lookup performance.
//
// Algorithm:
//  1. Validate input UUIDv7 key
//  2. Reject NullRow UUIDs early by checking if non-timestamp part (bytes 7, 9-15) are all zeros
//  3. Calculate number of logical data rows (DataRows and NullRows, excluding checksum rows)
//  4. Use FuzzyBinarySearch with logical index mapping
//  5. Map found logical index back to physical row index
//  6. Return physical row index or KeyNotFoundError
//
// Time Complexity: O(log n) where n is number of DataRows
// Space Complexity: O(row_size) constant memory
func (bsf *BinarySearchFinder) GetIndex(key uuid.UUID) (int64, error) {
	// FR-011: Check tombstoned state FIRST
	bsf.mu.Lock()
	if bsf.tombstonedErr != nil {
		tombErr := bsf.tombstonedErr
		bsf.mu.Unlock()
		return -1, tombErr
	}
	bsf.mu.Unlock()

	// Validate input UUID
	if key == uuid.Nil {
		return -1, NewInvalidInputError("key cannot be uuid.Nil", nil)
	}

	// Validate key is UUIDv7
	if err := ValidateUUIDv7(key); err != nil {
		return -1, err
	}

	// Reject NullRow UUIDs early (FR-010)
	// Check if non-timestamp part (bytes 7, 9-15) are all zeros
	if IsNullRowUUID(key) {
		return -1, NewInvalidInputError("search key cannot be a NullRow UUID", nil)
	}

	// Get confirmed size for search bounds
	bsf.mu.Lock()
	confirmedSize := bsf.size
	bsf.mu.Unlock()

	// Calculate total complete rows in confirmed size
	totalRows := (confirmedSize - HEADER_SIZE) / int64(bsf.rowSize)

	// Count logical rows (DataRows and NullRows, excluding checksum rows)
	numLogicalRows := bsf.countLogicalRows(totalRows)

	if numLogicalRows == 0 {
		return -1, NewKeyNotFoundError(fmt.Sprintf("key %s not found in database", key.String()), nil)
	}

	// Use FuzzyBinarySearch with logical index mapping
	logicalIndex, err := FuzzyBinarySearch(
		key,
		bsf.skewMs,
		numLogicalRows,
		bsf.getLogicalKey,
	)
	if err != nil {
		// Propagate KeyNotFoundError as-is
		var keyErr *KeyNotFoundError
		if errors.As(err, &keyErr) {
			return -1, err
		}
		// Wrap other errors
		return -1, NewReadError("binary search failed", err)
	}

	// Map logical index back to physical row index
	physicalIndex := bsf.logicalToPhysicalIndex(logicalIndex)

	// Verify the found row actually contains the key
	rowBytes, err := bsf.readRow(physicalIndex)
	if err != nil {
		return -1, err
	}

	var rowUnion RowUnion
	if err := rowUnion.UnmarshalText(rowBytes); err != nil {
		return -1, NewCorruptDatabaseError(fmt.Sprintf("failed to parse row at index %d", physicalIndex), err)
	}

	// Verify it's a DataRow with matching key
	if rowUnion.DataRow == nil {
		return -1, NewKeyNotFoundError(fmt.Sprintf("key %s not found in database", key.String()), nil)
	}

	if rowUnion.DataRow.GetKey() != key {
		return -1, NewKeyNotFoundError(fmt.Sprintf("key %s not found in database", key.String()), nil)
	}

	return physicalIndex, nil
}

// countLogicalRows calculates the number of logical rows (DataRows and NullRows)
// given the total number of physical rows, excluding checksum rows.
//
// Checksum rows occur at physical indices: 0, 10001, 20002, 30003, ...
// Pattern: checksum at index = k * 10001 for k >= 0
// Number of checksum rows up to (and including) index N = floor(N / 10001) + 1 (if N >= 0)
//
// Parameters:
//   - totalRows: Total number of physical rows (including checksum rows)
//
// Returns:
//   - int64: Number of logical rows (DataRows + NullRows, excluding checksum rows)
func (bsf *BinarySearchFinder) countLogicalRows(totalRows int64) int64 {
	if totalRows == 0 {
		return 0
	}
	// Count checksum rows: checksum at indices 0, 10001, 20002, 30003, ...
	// Pattern: checksum at index = k * 10001 for k >= 0
	// Number of checksum rows up to index N = floor(N / 10001) + 1
	numChecksumRows := (totalRows-1)/10001 + 1
	return totalRows - numChecksumRows
}

// logicalToPhysicalIndex converts a logical index (used by FuzzyBinarySearch) to a
// physical row index in the database file.
//
// Formula: physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1
// This accounts for checksum rows at indices: 0, 10001, 20002, 30003, ...
//
// Parameters:
//   - logicalIndex: Index in the logical contiguous array (includes DataRows and NullRows)
//
// Returns:
//   - int64: Physical row index accounting for checksum rows
func (bsf *BinarySearchFinder) logicalToPhysicalIndex(logicalIndex int64) int64 {
	return logicalIndex + (logicalIndex / 10000) + 1
}

// getLogicalKey is an adapter function for FuzzyBinarySearch that returns the UUID key
// at a logical index. This function handles the mapping from logical indices to physical
// indices and extracts UUID keys from DataRows and NullRows.
//
// Parameters:
//   - logicalIndex: Index in the logical contiguous array (includes DataRows and NullRows)
//
// Returns:
//   - uuid.UUID: UUIDv7 key at the logical index (from DataRow or NullRow)
//   - error: ReadError for I/O failures, CorruptDatabaseError for parsing failures,
//     KeyNotFoundError if logical index is out of bounds
func (bsf *BinarySearchFinder) getLogicalKey(logicalIndex int64) (uuid.UUID, error) {
	// Map logical index to physical index
	physicalIndex := bsf.logicalToPhysicalIndex(logicalIndex)

	// Get confirmed size for bounds checking
	bsf.mu.Lock()
	confirmedSize := bsf.size
	bsf.mu.Unlock()

	totalRows := (confirmedSize - HEADER_SIZE) / int64(bsf.rowSize)

	// Check bounds
	if physicalIndex < 0 || physicalIndex >= totalRows {
		return uuid.Nil, NewKeyNotFoundError(fmt.Sprintf("logical index %d maps to out-of-bounds physical index %d", logicalIndex, physicalIndex), nil)
	}

	// Read row at physical index
	rowBytes, err := bsf.readRow(physicalIndex)
	if err != nil {
		return uuid.Nil, NewReadError(fmt.Sprintf("failed to read row at physical index %d", physicalIndex), err)
	}

	// Parse row as RowUnion
	var rowUnion RowUnion
	if err := rowUnion.UnmarshalText(rowBytes); err != nil {
		return uuid.Nil, NewCorruptDatabaseError(fmt.Sprintf("failed to parse row at physical index %d", physicalIndex), err)
	}

	// Extract UUID from DataRow or NullRow
	if rowUnion.DataRow != nil {
		return rowUnion.DataRow.GetKey(), nil
	}
	if rowUnion.NullRow != nil {
		return rowUnion.NullRow.GetKey(), nil
	}

	// If it's a ChecksumRow, this shouldn't happen in normal operation
	// (logical indices should map to DataRows or NullRows)
	// But handle it gracefully by returning an error
	return uuid.Nil, NewCorruptDatabaseError(fmt.Sprintf("logical index %d mapped to checksum row at physical index %d", logicalIndex, physicalIndex), nil)
}

// GetTransactionStart returns the index of the first row in the transaction
// containing the specified index. Implements backward scanning from input index.
//
// FR-011: This method MUST check tombstoned state FIRST and return TombstonedError if set.
//
// Implementation: Identical to SimpleFinder implementation
//
// Time Complexity: O(k) where k is distance to start (max ~101)
// Space Complexity: O(row_size) constant memory
func (bsf *BinarySearchFinder) GetTransactionStart(index int64) (int64, error) {
	// FR-011: Check tombstoned state FIRST
	bsf.mu.Lock()
	if bsf.tombstonedErr != nil {
		tombErr := bsf.tombstonedErr
		bsf.mu.Unlock()
		return -1, tombErr
	}
	bsf.mu.Unlock()

	// Validate index
	if err := bsf.validateIndex(index); err != nil {
		return -1, err
	}

	// Read the row at the given index
	currentRow, err := bsf.readRowUnion(index)
	if err != nil {
		return -1, err
	}

	// Check if this index points to a checksum row
	if currentRow.ChecksumRow != nil {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}

	// Check if current row starts the transaction
	if bsf.rowStartsTransaction(currentRow) {
		return index, nil
	}

	// Scan backward to find transaction start
	for i := index - 1; i >= 0; i-- {
		row, err := bsf.readRowUnion(i)
		if err != nil {
			return -1, err
		}

		// Skip checksum rows
		if row.ChecksumRow != nil {
			continue
		}

		// Check if this row starts a transaction
		if bsf.rowStartsTransaction(row) {
			return i, nil
		}
	}

	// No transaction start found - database corruption
	return -1, NewCorruptDatabaseError("no transaction start found in backward scan", nil)
}

// GetTransactionEnd returns the index of the last row in the transaction
// containing the specified index. Implements forward scanning from input index.
//
// FR-011: This method MUST check tombstoned state FIRST and return TombstonedError if set.
//
// Implementation: Identical to SimpleFinder implementation
//
// Time Complexity: O(k) where k is distance to end (max ~101)
// Space Complexity: O(row_size) constant memory
func (bsf *BinarySearchFinder) GetTransactionEnd(index int64) (int64, error) {
	// FR-011: Check tombstoned state FIRST
	bsf.mu.Lock()
	if bsf.tombstonedErr != nil {
		tombErr := bsf.tombstonedErr
		bsf.mu.Unlock()
		return -1, tombErr
	}
	bsf.mu.Unlock()

	// Validate index
	if err := bsf.validateIndex(index); err != nil {
		return -1, err
	}

	// Read the row at the given index
	currentRow, err := bsf.readRowUnion(index)
	if err != nil {
		return -1, err
	}

	// Check if this index points to a checksum row
	if currentRow.ChecksumRow != nil {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}

	// Check if current row ends the transaction
	if bsf.rowEndsTransaction(currentRow) {
		return index, nil
	}

	// Get confirmed size for search bounds
	bsf.mu.Lock()
	confirmedSize := bsf.size
	bsf.mu.Unlock()

	totalRows := (confirmedSize - HEADER_SIZE) / int64(bsf.rowSize)

	// Scan forward to find transaction end
	for i := index + 1; i < totalRows; i++ {
		row, err := bsf.readRowUnion(i)
		if err != nil {
			return -1, err
		}

		// Skip checksum rows
		if row.ChecksumRow != nil {
			continue
		}

		// Check if this row ends a transaction
		if bsf.rowEndsTransaction(row) {
			return i, nil
		}
	}

	// No transaction end found - transaction is still active
	return -1, NewTransactionActiveError("transaction has no ending row", nil)
}

// onRowAdded updates the finder's internal state when a new row is added to the database.
// This method is called within transaction write lock context and must not attempt
// to acquire additional locks.
//
// Implementation: Identical to SimpleFinder implementation
//
// Time Complexity: O(1) constant time
// Space Complexity: O(1) memory update
func (bsf *BinarySearchFinder) onRowAdded(index int64, row *RowUnion) error {
	if row == nil {
		return NewInvalidInputError("row cannot be nil", nil)
	}

	bsf.mu.Lock()
	defer bsf.mu.Unlock()

	// Calculate expected next row index
	expectedIndex := (bsf.size - HEADER_SIZE) / int64(bsf.rowSize)

	if index < expectedIndex {
		err := NewInvalidInputError(fmt.Sprintf("row index %d does not match expected position %d (existing data)", index, expectedIndex), nil)
		// FR-010: Set tombstoned error BEFORE returning
		bsf.tombstonedErr = NewTombstonedError("finder tombstoned due to onRowAdded error", err)
		return err
	}

	if index > expectedIndex {
		err := NewInvalidInputError(fmt.Sprintf("row index %d skips positions (expected %d)", index, expectedIndex), nil)
		// FR-010: Set tombstoned error BEFORE returning
		bsf.tombstonedErr = NewTombstonedError("finder tombstoned due to onRowAdded error", err)
		return err
	}

	// Update maxTimestamp for complete DataRow or NullRow entries
	if row.DataRow != nil {
		key := row.DataRow.GetKey()
		if key != uuid.Nil {
			if err := ValidateUUIDv7(key); err == nil {
				timestamp := ExtractUUIDv7Timestamp(key)
				if timestamp > bsf.maxTimestamp {
					bsf.maxTimestamp = timestamp
				}
			}
		}
	} else if row.NullRow != nil {
		// Extract timestamp from NullRow key and compare, same as DataRow
		key := row.NullRow.GetKey()
		timestamp := ExtractUUIDv7Timestamp(key)
		if timestamp > bsf.maxTimestamp {
			bsf.maxTimestamp = timestamp
		}
	}
	// Skip ChecksumRow and PartialDataRow

	// Update confirmed size
	bsf.size += int64(bsf.rowSize)

	return nil
}

// MaxTimestamp returns the maximum timestamp among all complete data and null rows.
// Implements O(1) time complexity by returning the cached maxTimestamp value.
// Note: This method returns the maxTimestamp value even if the Finder is tombstoned,
// since maxTimestamp represents historical data that remains valid.
func (bsf *BinarySearchFinder) MaxTimestamp() int64 {
	bsf.mu.Lock()
	defer bsf.mu.Unlock()
	return bsf.maxTimestamp
}

// readRow reads a single row from disk at the specified index.
// Helper method for internal use.
func (bsf *BinarySearchFinder) readRow(index int64) ([]byte, error) {
	offset := HEADER_SIZE + index*int64(bsf.rowSize)
	return bsf.dbFile.Read(offset, bsf.rowSize)
}

// readRowUnion reads and parses a row as RowUnion.
// Helper method for internal use.
func (bsf *BinarySearchFinder) readRowUnion(index int64) (*RowUnion, error) {
	rowBytes, err := bsf.readRow(index)
	if err != nil {
		return nil, err
	}

	var rowUnion RowUnion
	if err := rowUnion.UnmarshalText(rowBytes); err != nil {
		return nil, NewCorruptDatabaseError("failed to parse row", err)
	}

	return &rowUnion, nil
}

// validateIndex validates that an index is within bounds and non-negative.
// Helper method for internal use.
func (bsf *BinarySearchFinder) validateIndex(index int64) error {
	if index < 0 {
		return NewInvalidInputError("index cannot be negative", nil)
	}

	bsf.mu.Lock()
	confirmedSize := bsf.size
	bsf.mu.Unlock()

	totalRows := (confirmedSize - HEADER_SIZE) / int64(bsf.rowSize)
	if index >= totalRows {
		return NewInvalidInputError(fmt.Sprintf("index %d out of bounds (total rows: %d)", index, totalRows), nil)
	}

	return nil
}

// rowStartsTransaction checks if a row starts a transaction (start_control='T').
// Helper method for internal use.
func (bsf *BinarySearchFinder) rowStartsTransaction(row *RowUnion) bool {
	if row.DataRow != nil {
		return row.DataRow.StartControl == START_TRANSACTION
	}
	if row.NullRow != nil {
		return row.NullRow.StartControl == START_TRANSACTION
	}
	return false
}

// rowEndsTransaction checks if a row ends a transaction.
// Transaction-ending end_control values: TC, SC, R0-R9, S0-S9, NR
// Helper method for internal use.
func (bsf *BinarySearchFinder) rowEndsTransaction(row *RowUnion) bool {
	// NullRows always end transactions
	if row.NullRow != nil {
		return true
	}

	// DataRows with transaction-ending end_control
	if row.DataRow != nil {
		ec := row.DataRow.EndControl
		// Check for commit: TC, SC
		if ec == TRANSACTION_COMMIT || ec == SAVEPOINT_COMMIT {
			return true
		}
		// Check for rollback: R0-R9, S0-S9
		first := ec[0]
		second := ec[1]
		if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
			return true
		}
	}

	return false
}
