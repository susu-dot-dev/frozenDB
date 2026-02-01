package frozendb

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// SimpleFinder is a reference implementation of the Finder interface that uses
// direct disk-based scanning without caching or optimization techniques.
// This implementation prioritizes correctness over performance and serves as
// a baseline for validating optimized finder implementations.
//
// Design Philosophy:
//   - Disk-Based Operations: All data comes from direct disk reads via DBFile
//   - Minimal In-Memory State: Only tracks current confirmed database size
//   - One-Row-at-a-Time Processing: Processes individual rows sequentially
//   - Reference Implementation: Intended for correctness validation, not production
//
// Memory Usage: O(row_size) - constant regardless of database size
// Performance: O(n) for GetIndex, O(k) for transaction boundary methods where k <= 101
type SimpleFinder struct {
	dbFile       DBFile     // Database file interface for reading rows
	rowSize      int32      // Size of each row in bytes from header
	size         int64      // Confirmed file size (updated via OnRowAdded)
	maxTimestamp int64      // Maximum timestamp among all complete data and null rows
	mu           sync.Mutex // Protects size, maxTimestamp, and tombstoneErr fields for concurrent access
	tombstoneErr error      // Error that caused tombstone state (nil = healthy)
}

// NewSimpleFinder creates a new SimpleFinder instance.
//
// Parameters:
//   - dbFile: DBFile interface for reading database rows
//   - rowSize: Size of each row in bytes (from database header)
//
// Returns:
//   - *SimpleFinder: Initialized finder instance
//   - error: InvalidInputError if parameters are invalid
//
// The finder initializes with the current database file size from dbFile.Size(),
// representing the extent of data confirmed via OnRowAdded() callbacks.
func NewSimpleFinder(dbFile DBFile, rowSize int32) (*SimpleFinder, error) {
	if dbFile == nil {
		return nil, NewInvalidInputError("dbFile cannot be nil", nil)
	}
	if rowSize < 128 || rowSize > 65536 {
		return nil, NewInvalidInputError(fmt.Sprintf("rowSize must be between 128 and 65536, got %d", rowSize), nil)
	}

	sf := &SimpleFinder{
		dbFile:       dbFile,
		rowSize:      rowSize,
		size:         dbFile.Size(),
		maxTimestamp: 0,
	}

	// Initialize maxTimestamp by scanning existing rows
	if err := sf.initializeMaxTimestamp(); err != nil {
		return nil, err
	}

	return sf, nil
}

// initializeMaxTimestamp scans all existing rows to find the maximum timestamp.
// This is called once during initialization to establish the baseline maxTimestamp.
func (sf *SimpleFinder) initializeMaxTimestamp() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	confirmedSize := sf.size
	totalRows := (confirmedSize - HEADER_SIZE) / int64(sf.rowSize)

	sf.maxTimestamp = 0

	// Scan all rows to find maximum timestamp
	for i := int64(0); i < totalRows; i++ {
		rowBytes, err := sf.readRow(i)
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
					if timestamp > sf.maxTimestamp {
						sf.maxTimestamp = timestamp
					}
				}
			}
		} else if rowUnion.NullRow != nil {
			// Extract timestamp from NullRow key (uuid.Nil) and compare, same as DataRow
			key := rowUnion.NullRow.GetKey()
			timestamp := ExtractUUIDv7Timestamp(key)
			if timestamp > sf.maxTimestamp {
				sf.maxTimestamp = timestamp
			}
		}
		// Skip ChecksumRow and PartialDataRow
	}

	return nil
}

// GetIndex returns the index of the first row containing the specified UUID key.
// Implements linear scanning through all complete rows in the database.
//
// Algorithm:
//  1. Calculate total number of complete rows
//  2. Iterate through each row index
//  3. Read and parse each row as RowUnion
//  4. If row is DataRow and UUID matches, return index
//  5. Skip ChecksumRows, NullRows, and PartialDataRows
//  6. Return KeyNotFoundError if no match found
//
// Time Complexity: O(n) where n is number of rows
// Space Complexity: O(row_size) constant memory
func (sf *SimpleFinder) GetIndex(key uuid.UUID) (int64, error) {
	// Validate input UUID
	if key == uuid.Nil {
		return -1, NewInvalidInputError("key cannot be uuid.Nil", nil)
	}

	// Validate key is UUIDv7
	if err := ValidateUUIDv7(key); err != nil {
		return -1, err
	}

	// Check tombstone state and get confirmed size
	sf.mu.Lock()
	if sf.tombstoneErr != nil {
		sf.mu.Unlock()
		return -1, NewTombstonedError("finder is in permanent error state", sf.tombstoneErr)
	}
	confirmedSize := sf.size
	sf.mu.Unlock()

	// Calculate total complete rows in confirmed size
	totalRows := (confirmedSize - HEADER_SIZE) / int64(sf.rowSize)

	// Linear scan through all rows
	for index := int64(0); index < totalRows; index++ {
		// Read row bytes from disk
		rowBytes, err := sf.readRow(index)
		if err != nil {
			return -1, err
		}

		// Parse row as RowUnion to determine type
		var rowUnion RowUnion
		if err := rowUnion.UnmarshalText(rowBytes); err != nil {
			return -1, NewCorruptDatabaseError(fmt.Sprintf("failed to parse row at index %d", index), err)
		}

		// Only search DataRows
		if rowUnion.DataRow != nil {
			if rowUnion.DataRow.GetKey() == key {
				return index, nil
			}
		}
		// Skip ChecksumRows, NullRows, and any other row types
	}

	// Key not found after scanning all rows
	return -1, NewKeyNotFoundError(fmt.Sprintf("key %s not found in database", key.String()), nil)
}

// GetTransactionStart returns the index of the first row in the transaction
// containing the specified index. Implements backward scanning from input index.
//
// Algorithm:
//  1. Validate input index and ensure it's not a checksum row
//  2. Read row at input index and check if it starts transaction (start_control='T')
//  3. If not, scan backward through preceding rows
//  4. Find first row with start_control='T' in transaction chain
//  5. Return that index or error if no start found
//
// Time Complexity: O(k) where k is distance to start (max ~101)
// Space Complexity: O(row_size) constant memory
func (sf *SimpleFinder) GetTransactionStart(index int64) (int64, error) {
	// Check tombstone state first
	sf.mu.Lock()
	if sf.tombstoneErr != nil {
		sf.mu.Unlock()
		return -1, NewTombstonedError("finder is in permanent error state", sf.tombstoneErr)
	}
	sf.mu.Unlock()

	// Validate index
	if err := sf.validateIndex(index); err != nil {
		return -1, err
	}

	// Read the row at the given index
	currentRow, err := sf.readRowUnion(index)
	if err != nil {
		return -1, err
	}

	// Check if this index points to a checksum row
	if currentRow.ChecksumRow != nil {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}

	// Check if current row starts the transaction
	if sf.rowStartsTransaction(currentRow) {
		return index, nil
	}

	// Scan backward to find transaction start
	for i := index - 1; i >= 0; i-- {
		row, err := sf.readRowUnion(i)
		if err != nil {
			return -1, err
		}

		// Skip checksum rows
		if row.ChecksumRow != nil {
			continue
		}

		// Check if this row starts a transaction
		if sf.rowStartsTransaction(row) {
			return i, nil
		}
	}

	// No transaction start found - database corruption
	return -1, NewCorruptDatabaseError("no transaction start found in backward scan", nil)
}

// GetTransactionEnd returns the index of the last row in the transaction
// containing the specified index. Implements forward scanning from input index.
//
// Algorithm:
//  1. Validate input index and ensure it's not a checksum row
//  2. Read row at input index and check if it ends transaction
//  3. If not, scan forward through subsequent rows
//  4. Find first row with transaction-ending end_control
//  5. Return that index or TransactionActiveError if no end found
//
// Time Complexity: O(k) where k is distance to end (max ~101)
// Space Complexity: O(row_size) constant memory
func (sf *SimpleFinder) GetTransactionEnd(index int64) (int64, error) {
	// Check tombstone state first
	sf.mu.Lock()
	if sf.tombstoneErr != nil {
		sf.mu.Unlock()
		return -1, NewTombstonedError("finder is in permanent error state", sf.tombstoneErr)
	}
	sf.mu.Unlock()

	// Validate index
	if err := sf.validateIndex(index); err != nil {
		return -1, err
	}

	// Read the row at the given index
	currentRow, err := sf.readRowUnion(index)
	if err != nil {
		return -1, err
	}

	// Check if this index points to a checksum row
	if currentRow.ChecksumRow != nil {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}

	// Check if current row ends the transaction
	if sf.rowEndsTransaction(currentRow) {
		return index, nil
	}

	// Get confirmed size for search bounds
	sf.mu.Lock()
	confirmedSize := sf.size
	sf.mu.Unlock()

	totalRows := (confirmedSize - HEADER_SIZE) / int64(sf.rowSize)

	// Scan forward to find transaction end
	for i := index + 1; i < totalRows; i++ {
		row, err := sf.readRowUnion(i)
		if err != nil {
			return -1, err
		}

		// Skip checksum rows
		if row.ChecksumRow != nil {
			continue
		}

		// Check if this row ends a transaction
		if sf.rowEndsTransaction(row) {
			return i, nil
		}
	}

	// No transaction end found - transaction is still active
	return -1, NewTransactionActiveError("transaction has no ending row", nil)
}

// OnRowAdded updates the finder's internal state when a new row is added to the database.
// This method is called within transaction write lock context and must not attempt
// to acquire additional locks.
//
// If the finder is in tombstone state (after OnError was called), this method returns
// TombstonedError wrapping the original error that caused the tombstone state.
//
// Algorithm:
//  1. Check tombstone state and return TombstonedError if set
//  2. Calculate expected next row index from current size
//  3. Verify input index matches expected index
//  4. Update internal size by adding one row_size
//  5. Update maxTimestamp if the row is a complete DataRow or NullRow
//  6. Return success
//
// Time Complexity: O(1) constant time
// Space Complexity: O(1) memory update
func (sf *SimpleFinder) OnRowAdded(index int64, row *RowUnion) error {
	if row == nil {
		return NewInvalidInputError("row cannot be nil", nil)
	}

	sf.mu.Lock()
	defer sf.mu.Unlock()

	// Check tombstone state first
	if sf.tombstoneErr != nil {
		return NewTombstonedError("finder is in permanent error state", sf.tombstoneErr)
	}

	// Calculate expected next row index
	expectedIndex := (sf.size - HEADER_SIZE) / int64(sf.rowSize)

	if index < expectedIndex {
		return NewInvalidInputError(fmt.Sprintf("row index %d does not match expected position %d (existing data)", index, expectedIndex), nil)
	}

	if index > expectedIndex {
		return NewInvalidInputError(fmt.Sprintf("row index %d skips positions (expected %d)", index, expectedIndex), nil)
	}

	// Update maxTimestamp for complete DataRow or NullRow entries
	if row.DataRow != nil {
		key := row.DataRow.GetKey()
		if key != uuid.Nil {
			if err := ValidateUUIDv7(key); err == nil {
				timestamp := ExtractUUIDv7Timestamp(key)
				if timestamp > sf.maxTimestamp {
					sf.maxTimestamp = timestamp
				}
			}
		}
	} else if row.NullRow != nil {
		// Extract timestamp from NullRow key (uuid.Nil) and compare, same as DataRow
		key := row.NullRow.GetKey()
		timestamp := ExtractUUIDv7Timestamp(key)
		if timestamp > sf.maxTimestamp {
			sf.maxTimestamp = timestamp
		}
	}
	// Skip ChecksumRow and PartialDataRow

	// Update confirmed size
	sf.size += int64(sf.rowSize)

	return nil
}

// MaxTimestamp returns the maximum timestamp among all complete data and null rows.
// Implements O(1) time complexity by returning the cached maxTimestamp value.
func (sf *SimpleFinder) MaxTimestamp() int64 {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	return sf.maxTimestamp
}

// readRow reads a single row from disk at the specified index.
// Helper method for internal use.
func (sf *SimpleFinder) readRow(index int64) ([]byte, error) {
	offset := HEADER_SIZE + index*int64(sf.rowSize)
	return sf.dbFile.Read(offset, sf.rowSize)
}

// readRowUnion reads and parses a row as RowUnion.
// Helper method for internal use.
func (sf *SimpleFinder) readRowUnion(index int64) (*RowUnion, error) {
	rowBytes, err := sf.readRow(index)
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
func (sf *SimpleFinder) validateIndex(index int64) error {
	if index < 0 {
		return NewInvalidInputError("index cannot be negative", nil)
	}

	sf.mu.Lock()
	confirmedSize := sf.size
	sf.mu.Unlock()

	totalRows := (confirmedSize - HEADER_SIZE) / int64(sf.rowSize)
	if index >= totalRows {
		return NewInvalidInputError(fmt.Sprintf("index %d out of bounds (total rows: %d)", index, totalRows), nil)
	}

	return nil
}

// rowStartsTransaction checks if a row starts a transaction (start_control='T').
// Helper method for internal use.
func (sf *SimpleFinder) rowStartsTransaction(row *RowUnion) bool {
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
func (sf *SimpleFinder) rowEndsTransaction(row *RowUnion) bool {
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

// OnError is called when an error occurs during live update processing.
// The Finder enters a permanent tombstone state where all subsequent query
// operations return TombstonedError wrapping the original error.
//
// Note: SimpleFinder does not use FileWatcher (uses on-demand scanning), so this
// method is unlikely to be called in practice. However, it's implemented for
// interface consistency and to support potential future use cases.
func (sf *SimpleFinder) OnError(err error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	// Only set tombstoneErr once (first error wins)
	if sf.tombstoneErr == nil {
		sf.tombstoneErr = err
	}
}

// Close releases resources held by the SimpleFinder.
// SimpleFinder has no resources to clean up (no FileWatcher), so this is a no-op.
//
// Returns:
//   - error: Always returns nil (no cleanup needed)
func (sf *SimpleFinder) Close() error {
	// No-op: SimpleFinder has no resources to release
	return nil
}
