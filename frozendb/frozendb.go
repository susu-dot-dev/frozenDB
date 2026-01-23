package frozendb

import "sync"

// Access mode constants for opening frozenDB database files
const (
	// MODE_READ opens the database in read-only mode with no lock
	// Multiple readers can access the same file concurrently
	MODE_READ = "read"

	// MODE_WRITE opens the database in read-write mode with exclusive lock
	// Only one writer can access the file at a time
	MODE_WRITE = "write"
)

// FrozenDB represents an open connection to a frozenDB database file
// Instance methods are NOT thread-safe - use one instance per goroutine
// Close() is thread-safe and can be called concurrently from multiple goroutines
type FrozenDB struct {
	// Core file resources
	file DBFile // DBFile interface for file operations

	// Database metadata from header
	header *Header // Parsed header information

	// Transaction state management
	activeTx *Transaction // Current active transaction (nil if none)
	txMu     sync.RWMutex // Mutex for transaction state management
}

// NewFrozenDB opens an existing frozenDB database file with specified access mode
// Parameters:
//   - path: Filesystem path to existing frozenDB database file (.fdb extension required)
//   - mode: Access mode - MODE_READ for read-only, MODE_WRITE for read-write
//
// Returns:
//   - *FrozenDB: Database instance ready for operations
//   - error: InvalidInputError, PathError, CorruptDatabaseError, or WriteError
//
// Thread Safety: Safe for concurrent calls on different files
func NewFrozenDB(path string, mode string) (*FrozenDB, error) {
	// Validate input parameters
	// Open database file using DBFile interface
	dbFile, err := NewDBFile(path, mode)
	if err != nil {
		return nil, err
	}

	// Setup cleanup on error
	var cleanupErr error
	defer func() {
		if cleanupErr != nil {
			_ = dbFile.Close()
		}
	}()

	header, err := validateDatabaseFile(dbFile)
	if err != nil {
		cleanupErr = err
		return nil, err
	}

	// Create FrozenDB instance
	db := &FrozenDB{
		file:   dbFile,
		header: header,
	}

	// Validate the FrozenDB instance (ensures internal consistency)
	if err := db.Validate(); err != nil {
		cleanupErr = err
		return nil, err
	}

	// Recover transaction state if present
	if err := db.recoverTransaction(); err != nil {
		cleanupErr = err
		return nil, err
	}

	return db, nil
}

// Validate validates the FrozenDB struct for internal consistency
// This method is idempotent and can be called multiple times with the same result
func (db *FrozenDB) Validate() error {
	// Validate file handle is not nil
	if db.file == nil {
		return NewInvalidInputError("FrozenDB file handle cannot be nil", nil)
	}

	// Validate header is not nil
	if db.header == nil {
		return NewInvalidInputError("FrozenDB header cannot be nil", nil)
	}

	// Validate header is valid (assumes header was validated during construction)
	// We call Validate() to ensure header remains valid
	if err := db.header.Validate(); err != nil {
		return NewCorruptDatabaseError("FrozenDB header validation failed", err)
	}

	// Validate mode is valid (get from DBFile)
	mode := db.file.GetMode()
	if mode != MODE_READ && mode != MODE_WRITE {
		return NewInvalidInputError("FrozenDB mode must be 'read' or 'write'", nil)
	}

	return nil
}

// Close releases all resources associated with the database connection
// This method is thread-safe and idempotent - multiple concurrent calls are safe
// Returns nil if already closed or cleanup successful
func (db *FrozenDB) Close() error {
	if db.file == nil {
		return nil
	}
	if err := db.file.Close(); err != nil {
		return NewWriteError("failed to close file descriptor", err)
	}
	return nil
}

// recoverTransaction detects and recovers incomplete transaction state when opening a database file.
// It follows the algorithm: Read the last row -> If closed transaction nothing to do.
// Else, if open, read the last 101 rows (100 data rows + 1 checksum row), then figure out where the transaction starts.
// Also, if the file size doesn't land on a row boundary then you can skip the first read
// since that's guaranteed to be a PartialDataRow.
func (db *FrozenDB) recoverTransaction() error {
	fileSize := db.file.Size()
	rowSize := db.header.GetRowSize()
	dataStart := int64(HEADER_SIZE) + int64(rowSize) // After header + initial checksum row

	// If file only has header + checksum row, no transaction to recover
	if fileSize <= dataStart {
		return nil
	}

	// Check if file size is on a row boundary
	dataSize := fileSize - dataStart
	rowsInData := dataSize / int64(rowSize)
	remainder := dataSize % int64(rowSize)

	// If file size doesn't land on a row boundary, it's a PartialDataRow
	if remainder != 0 {
		// Read the PartialDataRow
		partialStart := dataStart + (rowsInData * int64(rowSize))
		partialBytes, err := db.file.Read(partialStart, int32(remainder))
		if err != nil {
			return NewCorruptDatabaseError("failed to read PartialDataRow", err)
		}

		// Parse PartialDataRow
		partialRow := &PartialDataRow{}
		if err := partialRow.UnmarshalText(partialBytes); err != nil {
			return NewCorruptDatabaseError("invalid PartialDataRow format", err)
		}
		partialRow.d.RowSize = rowSize // Set row size for validation

		// Create transaction with recovered PartialDataRow
		// For recovery, we need to read the transaction rows that came before
		// Read up to 101 rows backwards to find transaction start (100 data rows + 1 checksum row)
		var txRows []DataRow
		if rowsInData > 0 {
			rowsToRead := rowsInData
			if rowsToRead > 101 {
				rowsToRead = 101
			}

			// Read the last rows to reconstruct transaction
			scanStart := dataStart + ((rowsInData - rowsToRead) * int64(rowSize))
			bytesToRead := rowsToRead * int64(rowSize)
			const maxInt32 = int64(^uint32(0) >> 1) // 2^31 - 1
			if bytesToRead > maxInt32 {
				return NewCorruptDatabaseError("too many bytes to read for transaction recovery", nil)
			}
			var scanBytes []byte
			scanBytes, err = db.file.Read(scanStart, int32(bytesToRead))
			if err != nil {
				return NewCorruptDatabaseError("failed to read rows for transaction recovery", err)
			}

			// Parse rows and find transaction start
			txRows, _, err = db.parseTransactionRows(scanBytes, rowSize, int(rowsToRead))
			if err != nil {
				return err
			}
		}
		// If rowsInData == 0, transaction starts with the PartialDataRow itself, so txRows is empty

		// Create transaction with recovered state
		// For read-only mode, create a dummy channel that won't be used
		// For write mode, set up the actual writer
		writeChan := make(chan Data, 100)
		if db.file.GetMode() == MODE_WRITE {
			if err := db.file.SetWriter(writeChan); err != nil {
				return NewWriteError("failed to set writer for recovered transaction", err)
			}
		}
		// For read mode, writeChan exists but is not connected to FileManager
		// Transaction methods that try to write will fail, but GetRows() etc. will work

		tx := &Transaction{
			rows:            txRows,
			last:            partialRow,
			Header:          db.header,
			writeChan:       writeChan,
			db:              db.file,
			rowBytesWritten: len(partialBytes), // Track how much of partial row is written
		}

		// Calculate max timestamp from recovered rows
		for _, row := range txRows {
			timestamp := extractUUIDv7Timestamp(row.RowPayload.Key)
			if timestamp > tx.maxTimestamp {
				tx.maxTimestamp = timestamp
			}
		}
		if partialRow.d.RowPayload != nil {
			timestamp := extractUUIDv7Timestamp(partialRow.d.RowPayload.Key)
			if timestamp > tx.maxTimestamp {
				tx.maxTimestamp = timestamp
			}
		}

		db.txMu.Lock()
		db.activeTx = tx
		db.txMu.Unlock()

		return nil
	}

	// File size is on row boundary - read the last row
	lastRowStart := dataStart + ((rowsInData - 1) * int64(rowSize))
	lastRowBytes, err := db.file.Read(lastRowStart, int32(rowSize))
	if err != nil {
		return NewCorruptDatabaseError("failed to read last row", err)
	}

	// Parse last row to check end control
	ru := &RowUnion{}
	if err := ru.UnmarshalText(lastRowBytes); err != nil {
		// If we can't parse the last row, there's no valid transaction to recover
		// This can happen with corrupted files or edge cases - just return nil
		return nil
	}

	// Check if it's a checksum row - if so, we need to check the row before it
	if ru.ChecksumRow != nil {
		if rowsInData <= 1 {
			// Only checksum row, no transaction
			return nil
		}
		// Read the row before the checksum
		lastRowStart = dataStart + ((rowsInData - 2) * int64(rowSize))
		lastRowBytes, err = db.file.Read(lastRowStart, int32(rowSize))
		if err != nil {
			return NewCorruptDatabaseError("failed to read row before checksum", err)
		}
		// Create new RowUnion for the row before checksum
		ru = &RowUnion{}
		if err := ru.UnmarshalText(lastRowBytes); err != nil {
			// If we can't parse the row before checksum, there's no valid transaction
			// This can happen if the file ends with multiple checksum rows or invalid data
			return nil
		}
		// If the row before checksum is also a checksum row, there's no transaction
		if ru.ChecksumRow != nil {
			return nil
		}
	}

	// Check transaction state from end control
	if ru.DataRow != nil {
		endControl := ru.DataRow.EndControl
		second := endControl[1]

		// Closed transaction endings: C (commit), 0-9 (rollback)
		if second == 'C' || (second >= '0' && second <= '9') {
			// Transaction is closed, nothing to recover
			return nil
		}

		// Open transaction: RE or SE
		if endControl == ROW_END_CONTROL || endControl == SAVEPOINT_CONTINUE {
			// Read last 101 rows to find transaction start (100 data rows + 1 checksum row)
			rowsToRead := rowsInData
			if rowsToRead > 101 {
				rowsToRead = 101
			}

			// Ensure we have at least one row to read
			if rowsToRead == 0 {
				return NewCorruptDatabaseError("no rows to read for transaction recovery", nil)
			}

			scanStart := dataStart + ((rowsInData - rowsToRead) * int64(rowSize))
			bytesToRead := rowsToRead * int64(rowSize)
			const maxInt32 = int64(^uint32(0) >> 1) // 2^31 - 1
			if bytesToRead > maxInt32 {
				return NewCorruptDatabaseError("too many bytes to read for transaction recovery", nil)
			}
			scanBytes, err := db.file.Read(scanStart, int32(bytesToRead))
			if err != nil {
				return NewCorruptDatabaseError("failed to read rows for transaction recovery", err)
			}

			// Parse rows and find transaction start
			txRows, _, err := db.parseTransactionRows(scanBytes, rowSize, int(rowsToRead))
			if err != nil {
				return err
			}

			// Create transaction with recovered state (no partial row for complete last row)
			writeChan := make(chan Data, 100)
			if db.file.GetMode() == MODE_WRITE {
				if err := db.file.SetWriter(writeChan); err != nil {
					return NewWriteError("failed to set writer for recovered transaction", err)
				}
			}
			// For read mode, writeChan exists but is not connected to FileManager

			tx := &Transaction{
				rows:      txRows,
				Header:    db.header,
				writeChan: writeChan,
				db:        db.file,
			}

			// Calculate max timestamp from recovered rows
			for _, row := range txRows {
				timestamp := extractUUIDv7Timestamp(row.RowPayload.Key)
				if timestamp > tx.maxTimestamp {
					tx.maxTimestamp = timestamp
				}
			}

			db.txMu.Lock()
			db.activeTx = tx
			db.txMu.Unlock()

			return nil
		}
	} else if ru.NullRow != nil {
		// NullRow indicates closed transaction
		return nil
	}

	// Unknown row type or state
	return NewCorruptDatabaseError("unable to determine transaction state from last row", nil)
}

// parseTransactionRows parses rows from bytes and finds where the current transaction starts.
// Returns the transaction rows and the index where the transaction starts in the scanned rows.
func (db *FrozenDB) parseTransactionRows(bytes []byte, rowSize int, numRows int) ([]DataRow, int, error) {
	var txRows []DataRow
	txStartIndex := -1

	// Scan backwards to find transaction start (row with start_control 'T')
	for i := numRows - 1; i >= 0; i-- {
		rowStart := i * rowSize
		rowBytes := bytes[rowStart : rowStart+rowSize]

		ru := &RowUnion{}
		if err := ru.UnmarshalText(rowBytes); err != nil {
			return nil, -1, NewCorruptDatabaseError("failed to parse row during transaction recovery", err)
		}

		// Skip checksum rows
		if ru.ChecksumRow != nil {
			continue
		}

		// Check for transaction start
		if ru.DataRow != nil && ru.DataRow.StartControl == START_TRANSACTION {
			txStartIndex = i
			break
		}
	}

	if txStartIndex == -1 {
		return nil, -1, NewCorruptDatabaseError("transaction start not found in scanned rows", nil)
	}

	// Collect all rows from transaction start to end
	for i := txStartIndex; i < numRows; i++ {
		rowStart := i * rowSize
		rowBytes := bytes[rowStart : rowStart+rowSize]

		ru := &RowUnion{}
		if err := ru.UnmarshalText(rowBytes); err != nil {
			return nil, -1, NewCorruptDatabaseError("failed to parse transaction row", err)
		}

		// Skip checksum rows
		if ru.ChecksumRow != nil {
			continue
		}

		if ru.DataRow != nil {
			txRows = append(txRows, *ru.DataRow)
		}
	}

	return txRows, txStartIndex, nil
}

// GetActiveTx returns the current active transaction or nil if no transaction is active.
// Thread-safe using read lock on FrozenDB.txMu.
// Returns reference to actual Transaction object (not copy).
func (db *FrozenDB) GetActiveTx() *Transaction {
	db.txMu.RLock()
	defer db.txMu.RUnlock()

	// Return nil if no active transaction
	if db.activeTx == nil {
		return nil
	}

	// Check if transaction is still active (not committed/rolled back)
	// For recovered transactions, we check the last row's end control
	if db.activeTx.IsCommitted() {
		// Transaction is committed, no longer active
		return nil
	}

	return db.activeTx
}

// BeginTx creates a new transaction if no active transaction exists.
// Returns error if transaction creation fails or conflicts with existing active transaction.
// Thread-safe using write lock on FrozenDB.txMu.
func (db *FrozenDB) BeginTx() (*Transaction, error) {
	db.txMu.Lock()
	defer db.txMu.Unlock()

	// Check if active transaction already exists
	if db.activeTx != nil {
		// Verify it's still active (not committed)
		if !db.activeTx.IsCommitted() {
			return nil, NewInvalidActionError("active transaction already exists", nil)
		}
		// Transaction was committed, clear it
		db.activeTx = nil
	}

	// Create new transaction
	tx, err := NewTransaction(db.file, db.header)
	if err != nil {
		return nil, err
	}

	// Initialize transaction with Begin()
	if err := tx.Begin(); err != nil {
		return nil, err
	}

	// Store as active transaction
	db.activeTx = tx

	return tx, nil
}
