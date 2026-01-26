package frozendb

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// Transaction represents a single database transaction with maximum 100 DataRow objects.
// The first row must be the transaction start (StartControl = 'T'), and the last row
// is either the end of the transaction or the transaction is still open.
//
// Transaction supports Begin(), AddRow(), and Commit() operations.
// State is inferred from field values:
//   - Inactive: rows empty, empty nil, last nil
//   - Active: last non-nil, empty nil
//   - Committed: empty non-nil, last nil (for empty transactions)
//   - Committed: rows non-empty with transaction-ending control (for data transactions)
//
// After creating a Transaction struct directly, you MUST call Validate() before using it.
type Transaction struct {
	rows            []DataRow       // Single slice of DataRow objects (max 100) - unexported for immutability
	empty           *NullRow        // Empty null row after successful commit
	last            *PartialDataRow // Current partial data row being built
	Header          *Header         // Header reference for row creation
	maxTimestamp    int64           // Maximum timestamp within current transaction (for ordering validation)
	mu              sync.RWMutex    // Mutex for thread safety
	writeChan       chan<- Data     // Write channel for sending Data structs to FileManager
	rowBytesWritten int             // Tracks how many bytes of current PartialDataRow have been written (internal, not initialized by caller)
	tombstone       bool            // Tombstone flag set when write operation fails
	db              DBFile          // File manager interface for reading rows and calculating checksums
	finder          Finder          // Finder interface for notifying of new rows (optional)
}

const (
	CHECKSUM_INTERVAL = 10000 // Checksum rows inserted every 10,000 complete rows
)

// NewTransaction creates a new transaction with automatic checksum row insertion.
// The transaction will automatically insert checksum rows at 10,000-row intervals.
//
// Parameters:
//   - db: DBFile interface for reading rows and calculating checksums
//   - header: Validated header reference containing row_size and configuration
//   - finder: Finder interface for notifying of new rows (required, cannot be nil)
//
// Returns:
//   - *Transaction: New transaction instance
//   - error: Error if setup fails (InvalidInputError, InvalidActionError)
//
// Errors returned:
//   - InvalidInputError: nil parameters
//   - InvalidActionError: Writer already active on FileManager
func NewTransaction(db DBFile, header *Header, finder Finder) (*Transaction, error) {
	if db == nil {
		return nil, NewInvalidInputError("DBFile cannot be nil", nil)
	}
	if finder == nil {
		return nil, NewInvalidInputError("Finder cannot be nil", nil)
	}
	// Create write channel internally
	writeChan := make(chan Data, 100)

	// SetWriter needs a receive-only channel
	if err := db.SetWriter(writeChan); err != nil {
		return nil, err
	}

	tx := &Transaction{
		Header:    header,
		writeChan: writeChan,
		db:        db,
		finder:    finder,
	}

	// Validate the transaction after construction
	if err := tx.Validate(); err != nil {
		return nil, err
	}

	return tx, nil
}

// GetRows returns the rows slice for read-only access.
// Since all fields of DataRow are unexported, modifications to the slice
// elements won't affect the internal transaction state.
// Returns empty slice if transaction is tombstoned.
func (tx *Transaction) GetRows() []DataRow {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	if tx.tombstone {
		return []DataRow{}
	}
	return tx.rows
}

// GetEmptyRow returns the empty NullRow if present, nil otherwise.
// This field is set after a successful empty transaction commit.
// Returns nil if transaction is tombstoned.
func (tx *Transaction) GetEmptyRow() *NullRow {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	if tx.tombstone {
		return nil
	}
	return tx.empty
}

// IsTombstoned returns true if the transaction has been tombstoned due to a write failure.
// Once tombstoned, all subsequent public API calls will return TombstonedError.
func (tx *Transaction) IsTombstoned() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.tombstone
}

// checkTombstone checks if the transaction is tombstoned and returns TombstonedError if so.
// The caller must hold at least a read lock on tx.mu.
func (tx *Transaction) checkTombstone() error {
	if tx.tombstone {
		return NewTombstonedError("transaction is tombstoned due to write failure", nil)
	}
	return nil
}

// extractUUIDv7Timestamp extracts the 48-bit millisecond timestamp from a UUIDv7.
// The timestamp is stored in the first 6 bytes (48 bits) of the UUID.
func extractUUIDv7Timestamp(u uuid.UUID) int64 {
	// UUIDv7 format: first 48 bits are the timestamp in milliseconds
	// Bytes 0-5 contain the timestamp, big-endian
	return int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
		int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
}

// getChecksumStart returns the offset where the most recent checksum row starts.
// If file size is less than 10,000 data rows, returns HEADER_SIZE (64) for the initial checksum.
// Otherwise, calculates the position of the most recent checksum row based on row count.
func (tx *Transaction) getChecksumStart() int64 {
	fileSize := tx.db.Size()
	rowSize := tx.Header.GetRowSize()

	// If no data yet (file only has header), no checksum
	if fileSize <= int64(HEADER_SIZE) {
		return int64(HEADER_SIZE)
	}

	// Calculate total rows in data section (checksum rows + data rows)
	totalRows := (fileSize - int64(HEADER_SIZE)) / int64(rowSize)

	// If total rows < CHECKSUM_INTERVAL+1, initial checksum is at HEADER_SIZE
	// (checksum row + less than CHECKSUM_INTERVAL data rows)
	if totalRows <= int64(CHECKSUM_INTERVAL+1) {
		return int64(HEADER_SIZE)
	}

	// Number of complete blocks of (CHECKSUM_INTERVAL data rows + checksum row)
	blocks := (totalRows - 1) / (int64(CHECKSUM_INTERVAL) + 1)

	// Offset: HEADER_SIZE + blocks * (CHECKSUM_INTERVAL+1) * rowSize
	return int64(HEADER_SIZE) + blocks*(int64(CHECKSUM_INTERVAL)+1)*int64(rowSize)
}

// shouldInsertChecksum returns true if a checksum row should be inserted.
// Checks if the distance from getChecksumStart() to fileSize() is exactly 10,001 rows.
func (tx *Transaction) shouldInsertChecksum() bool {
	fileSize := tx.db.Size()
	rowSize := tx.Header.GetRowSize()
	checksumStart := tx.getChecksumStart()

	bytesFromChecksum := fileSize - checksumStart
	rowsFromChecksum := bytesFromChecksum / int64(rowSize)

	shouldInsert := rowsFromChecksum == int64(CHECKSUM_INTERVAL+1)
	return shouldInsert
}

// validateRows validates rows by iterating through bytes row_size at a time.
// First row (rowIndex == 0) must be a checksum row, remaining rows must be data/null rows.
// The union_row unmarshalText already ensures exactly one row type is set.
func (tx *Transaction) validateRows(bytes []byte) error {
	rowSize := tx.Header.GetRowSize()
	for i := 0; i < len(bytes); i += rowSize {
		ru := &RowUnion{}

		rowBytes := bytes[i : i+rowSize]
		if err := ru.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError("row validation failed during checksum calculation", err)
		}

		rowIndex := i / rowSize
		if rowIndex == 0 && ru.ChecksumRow == nil {
			return NewCorruptDatabaseError("first row in checksum block must be a checksum row", nil)
		}
		if rowIndex != 0 && ru.ChecksumRow != nil {
			return NewCorruptDatabaseError("checksum row found in middle of data block", nil)
		}
	}

	return nil
}

// insertChecksum calculates the checksum row from bytes and inserts it.
func (tx *Transaction) insertChecksum(bytes []byte) error {
	checksumRow, err := NewChecksumRow(tx.Header.GetRowSize(), bytes)
	if err != nil {
		return NewCorruptDatabaseError("failed to create checksum row", err)
	}

	checksumBytes, err := checksumRow.MarshalText()
	if err != nil {
		return NewCorruptDatabaseError("failed to marshal checksum row", err)
	}

	if err := tx.writeBytes(checksumBytes); err != nil {
		return err
	}

	// Notify finder of new checksum row
	rowUnion := &RowUnion{ChecksumRow: checksumRow}
	if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
		return err
	}

	return nil
}

// checkAndInsertChecksum checks if a checksum row is needed and inserts it if so.
// This method is called after each complete row write (DataRow or NullRow).
// Assumes tx.db is non-nil (validated by tx.Validate()).
// Errors returned:
//   - CorruptDatabaseError: Row validation failure during checksum calculation
//   - WriteError: Failed to write checksum row to file
func (tx *Transaction) checkAndInsertChecksum() error {
	if !tx.shouldInsertChecksum() {
		return nil
	}

	checksumStart := tx.getChecksumStart()
	rowSize := tx.Header.GetRowSize()

	dataStart := checksumStart
	bytesNeeded := int64(CHECKSUM_INTERVAL+1) * int64(rowSize)

	bytes, err := tx.db.Read(dataStart, int32(bytesNeeded))
	if err != nil {
		return err
	}

	if err := tx.validateRows(bytes); err != nil {
		return err
	}

	// Reset rowBytesWritten after checksum insertion so next row starts fresh
	defer func() {
		tx.rowBytesWritten = 0
	}()

	return tx.insertChecksum(bytes)
}

// isActive returns true if the transaction is in active state.
// Active: last non-nil, empty nil
func (tx *Transaction) isActive() bool {
	return tx.last != nil
}

// isCommittedState returns true if the transaction has been committed.
// For empty transactions: empty non-nil
// For data transactions: rows non-empty with transaction-ending control
func (tx *Transaction) isCommittedState() bool {
	// Empty transaction committed
	if tx.empty != nil {
		return true
	}
	// Data transaction committed
	if len(tx.rows) > 0 {
		lastRow := tx.rows[len(tx.rows)-1]
		second := lastRow.EndControl[1]
		if second == 'C' || (second >= '0' && second <= '9') {
			return true
		}
	}
	return false
}

// writeBytes writes bytes for a PartialDataRow or finalized DataRow, automatically
// handling incremental writes by slicing off already-written bytes using rowBytesWritten.
// Takes the full bytes array from MarshalText() and only writes the new portion.
// On successful write, updates tx.rowBytesWritten to the full length.
// Returns an error if the write fails. This is a synchronous operation.
// On write failure, the transaction is tombstoned.
func (tx *Transaction) writeBytes(fullBytes []byte) error {
	if tx.writeChan == nil {
		return NewInvalidActionError("write channel not set", nil)
	}

	// Slice off already-written bytes
	newBytes := fullBytes[tx.rowBytesWritten:]
	if len(newBytes) == 0 {
		// Nothing new to write
		return nil
	}

	// Create response channel with buffer for synchronous wait
	responseChan := make(chan error, 1)

	// Send data to write channel
	data := Data{
		Bytes:    newBytes,
		Response: responseChan,
	}

	select {
	case tx.writeChan <- data:
		// Wait for response
		err := <-responseChan
		if err != nil {
			// FR-006: Tombstone transaction on write failure
			tx.tombstone = true
			return err
		}
		// Update rowBytesWritten to full length after successful write
		tx.rowBytesWritten = len(fullBytes)
		return nil
	default:
		// FR-006: Tombstone transaction on write failure
		tx.tombstone = true
		return NewWriteError("write channel is full or closed", nil)
	}
}

// notifyFinderRowAdded notifies the finder that a complete row was added, if finder is set.
// Should be called after a complete row (DataRow, NullRow, or ChecksumRow) is successfully written.
func (tx *Transaction) notifyFinderRowAdded(row *RowUnion) error {
	// Calculate the row index based on current file size
	fileSize := tx.db.Size()
	rowSize := tx.Header.GetRowSize()
	// Index of the row that was just written
	index := (fileSize - int64(HEADER_SIZE) - int64(rowSize)) / int64(rowSize)

	return tx.finder.OnRowAdded(index, row)
}

// Begin initializes an empty transaction by creating a PartialDataRow in
// PartialDataRowWithStartControl state. This method can only be called when
// the transaction is inactive (all fields empty/nil).
//
// Preconditions:
//   - rows slice must be empty
//   - empty field must be nil
//   - last field must be nil
//   - writeChan must be set
//   - transaction must not be tombstoned
//
// Postconditions:
//   - last field points to new PartialDataRow with start control
//   - rowBytesWritten is automatically updated to 2 (ROW_START + START_TRANSACTION)
//   - PartialDataRow is written to disk via writeChan
//   - All other fields remain unchanged
//
// Returns InvalidActionError if preconditions are not met.
// Returns TombstonedError if transaction is tombstoned.
// Returns WriteError if write operation fails (transaction is tombstoned on failure).
func (tx *Transaction) Begin() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return err
	}

	// Validate preconditions - transaction must be inactive
	if len(tx.rows) > 0 {
		return NewInvalidActionError("Begin() cannot be called when rows exist", nil)
	}
	if tx.empty != nil {
		return NewInvalidActionError("Begin() cannot be called when empty row exists", nil)
	}
	if tx.last != nil {
		return NewInvalidActionError("Begin() cannot be called when partial row exists", nil)
	}

	// Create PartialDataRow with start control
	pdr, err := NewPartialDataRow(tx.Header.GetRowSize(), START_TRANSACTION)
	if err != nil {
		return NewInvalidActionError("failed to create PartialDataRow", err)
	}

	// Write PartialDataRow to disk (FR-001)
	// MarshalText() returns 2 bytes: ROW_START + 'T'
	bytes, err := pdr.MarshalText()
	if err != nil {
		return NewInvalidActionError("failed to marshal PartialDataRow", err)
	}

	// Write bytes synchronously (FR-005)
	// rowBytesWritten is automatically updated by writeBytes
	if err := tx.writeBytes(bytes); err != nil {
		// FR-006: Transaction is tombstoned by writeBytes on error
		return err
	}

	// Update state only after successful write
	tx.last = pdr
	return nil
}

// AddRow adds a new key-value pair to the transaction.
//
// The data flow is:
//   - Begin() creates a PartialDataRow with START_TRANSACTION in PartialDataRowWithStartControl state
//   - First AddRow() adds key/value to the existing partial (advances to PartialDataRowWithPayload)
//   - Subsequent AddRow() calls finalize the previous partial (with RE) and create a new one with ROW_CONTINUE
//
// Preconditions:
//   - Transaction must be active (last non-nil, empty nil)
//   - Key must be valid UUIDv7
//   - Value must be non-empty JSON string
//   - Transaction must have < 100 rows total
//   - UUID timestamp must satisfy: new_timestamp + skew_ms > max_timestamp
//   - transaction must not be tombstoned
//
// Postconditions:
//   - If partial had payload: finalized and moved to rows[], new partial created with ROW_CONTINUE
//   - If partial had only start control: key/value added to existing partial
//   - max_timestamp is updated if new_timestamp > previous max_timestamp
//
// Returns:
//   - InvalidActionError: Transaction not active or already committed
//   - InvalidInputError: Invalid UUIDv7, empty value, or >=100 rows
//   - KeyOrderingError: Timestamp ordering violation
//   - TombstonedError: Transaction is tombstoned
func (tx *Transaction) AddRow(key uuid.UUID, value json.RawMessage) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return err
	}

	// FR-001, FR-011: Validate transaction is active
	if !tx.isActive() {
		if tx.isCommittedState() {
			return NewInvalidActionError("AddRow() cannot be called on committed transaction", nil)
		}
		return NewInvalidActionError("AddRow() requires Begin() to be called first", nil)
	}

	// FR-006: Validate UUIDv7
	if err := ValidateUUIDv7(key); err != nil {
		return NewInvalidInputError("invalid UUIDv7 key", err)
	}

	// FR-007: Validate non-empty value
	if len(value) == 0 {
		return NewInvalidInputError("value cannot be empty", nil)
	}

	// FR-010: Validate row count
	// Total rows after this AddRow = len(tx.rows) + 1 (if we finalize) + 1 (new/current partial)
	// Or len(tx.rows) + 1 (if we just add to existing partial)
	// Either way, we're adding one more row to the eventual total
	currentTotal := len(tx.rows)
	if tx.last.GetState() != PartialDataRowWithStartControl {
		currentTotal++ // Current partial will become a row
	}
	if currentTotal >= 100 {
		return NewInvalidInputError("transaction cannot contain more than 100 rows", nil)
	}

	// FR-014, FR-016, FR-017: Validate timestamp ordering
	newTimestamp := extractUUIDv7Timestamp(key)
	skewMs := int64(tx.Header.GetSkewMs())

	// Get maxTimestamp (max of finder's committed rows and transaction's uncommitted rows)
	finderMax := tx.finder.MaxTimestamp()
	maxTimestamp := finderMax
	if tx.maxTimestamp > finderMax {
		maxTimestamp = tx.maxTimestamp
	}

	// Validate: new_timestamp + skew_ms > max_timestamp
	if newTimestamp+skewMs <= maxTimestamp {
		return NewKeyOrderingError("UUID timestamp violates ordering constraint: new_timestamp + skew_ms must be > max_timestamp", nil)
	}

	// Check the current state of the partial row
	if tx.last.GetState() == PartialDataRowWithStartControl {
		// First AddRow after Begin(): add key/value to the existing partial
		// The partial already has START_TRANSACTION from Begin()

		if err := tx.last.AddRow(key, value); err != nil {
			return err
		}

		// Write incremental bytes (FR-002)
		// MarshalText() returns rowSize-5 bytes (complete up to padding)
		allBytes, err := tx.last.MarshalText()
		if err != nil {
			return NewInvalidActionError("failed to marshal PartialDataRow", err)
		}

		// Write only the new bytes (FR-005: synchronous)
		// rowBytesWritten is automatically updated by writeBytes
		if err := tx.writeBytes(allBytes); err != nil {
			// FR-006: Transaction is tombstoned by writeBytes on error
			return err
		}
	} else {
		// Subsequent AddRow(): finalize current partial and create new one (FR-002)
		// Finalize previous PartialDataRow with ROW_END_CONTROL (RE)
		dataRow, err := tx.last.EndRow()
		if err != nil {
			return NewInvalidActionError("failed to finalize previous row", err)
		}

		// Write finalization bytes: RE + parity + ROW_END (5 bytes)
		// Get the complete row bytes - writeBytes will slice off already-written portion
		completeRowBytes, err := dataRow.MarshalText()
		if err != nil {
			return NewInvalidActionError("failed to marshal finalized DataRow", err)
		}

		// Write remaining bytes (FR-005: synchronous)
		// rowBytesWritten is automatically updated by writeBytes
		if err := tx.writeBytes(completeRowBytes); err != nil {
			// FR-006: Transaction is tombstoned by writeBytes on error
			return err
		}

		// Only update state after successful write
		tx.rows = append(tx.rows, *dataRow)
		tx.rowBytesWritten = 0 // Reset for new partial row

		// Notify finder of new row
		rowUnion := &RowUnion{DataRow: dataRow}
		if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
			return err
		}

		// Check and insert checksum row if needed (after complete DataRow write)
		if err := tx.checkAndInsertChecksum(); err != nil {
			return err
		}

		// FR-004, FR-005: Create new PartialDataRow with ROW_CONTINUE
		// All rows after the first use ROW_CONTINUE
		newPdr, err := NewPartialDataRow(tx.Header.GetRowSize(), ROW_CONTINUE)
		if err != nil {
			return NewInvalidActionError("failed to create PartialDataRow", err)
		}

		// Add the key-value data to the new partial
		if err := newPdr.AddRow(key, value); err != nil {
			return err
		}

		// Write new PartialDataRow (rowSize-5 bytes, all new bytes)
		newPartialBytes, err := newPdr.MarshalText()
		if err != nil {
			return NewInvalidActionError("failed to marshal new PartialDataRow", err)
		}

		// Write all bytes (new row, start fresh) (FR-005: synchronous)
		// rowBytesWritten is automatically updated by writeBytes
		if err := tx.writeBytes(newPartialBytes); err != nil {
			// FR-006: Transaction is tombstoned by writeBytes on error
			return err
		}

		tx.last = newPdr
	}

	// Update transaction's maxTimestamp for ordering validation
	// This tracks the max within the current transaction (uncommitted rows)
	if newTimestamp > tx.maxTimestamp {
		tx.maxTimestamp = newTimestamp
	}

	return nil
}

// Commit finalizes the transaction.
//
// For empty transactions (Begin() followed immediately by Commit() with no AddRow() calls):
//   - Converts the PartialDataRow to a NullRow
//   - Sets empty field to the NullRow
//
// For data transactions (Begin() followed by one or more AddRow() calls):
//   - Finalizes the last PartialDataRow with proper end_control (TC or SC)
//   - Adds the finalized DataRow to rows[]
//
// Preconditions:
//   - Transaction must be active (last non-nil, empty nil)
//   - empty field must be nil
//   - transaction must not be tombstoned
//
// Postconditions:
//   - For empty transactions: empty field points to created NullRow, last is nil
//   - For data transactions: last PartialDataRow is finalized and added to rows[], last is nil
//
// Returns InvalidActionError if preconditions are not met.
// Returns TombstonedError if transaction is tombstoned.
// Returns WriteError if write operation fails (transaction is tombstoned on failure).
func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return err
	}

	// Validate preconditions - transaction must be active
	if tx.last == nil {
		return NewInvalidActionError("Commit() requires an active transaction (call Begin() first)", nil)
	}
	if tx.empty != nil {
		return NewInvalidActionError("Commit() cannot be called when transaction is already committed", nil)
	}

	// FR-004: Handle empty transactions (Begin() + Commit() with no AddRow() calls)
	if len(tx.rows) == 0 && tx.last.GetState() == PartialDataRowWithStartControl {
		// Create and validate NullRowPayload
		payload := &NullRowPayload{
			Key: uuid.Nil,
		}
		if err := payload.Validate(); err != nil {
			return NewInvalidActionError("created NullRowPayload failed validation", err)
		}

		// Create NullRow with validated payload
		nullRow := &NullRow{
			baseRow[*NullRowPayload]{
				RowSize:      tx.Header.GetRowSize(),
				StartControl: START_TRANSACTION,
				EndControl:   NULL_ROW_CONTROL,
				RowPayload:   payload,
			},
		}

		// Validate the created NullRow
		if err := nullRow.Validate(); err != nil {
			return NewInvalidActionError("created NullRow failed validation", err)
		}

		// Write NullRow to disk (FR-004)
		// MarshalText() returns rowSize bytes (complete row)
		nullRowBytes, err := nullRow.MarshalText()
		if err != nil {
			return NewInvalidActionError("failed to marshal NullRow", err)
		}

		// Write remaining bytes (FR-005: synchronous)
		// rowBytesWritten is automatically updated by writeBytes
		if err := tx.writeBytes(nullRowBytes); err != nil {
			// FR-006: Transaction is tombstoned by writeBytes on error
			return err
		}

		// Update transaction state only after successful write
		tx.empty = nullRow
		tx.last = nil
		tx.rowBytesWritten = 0

		// Notify finder of new row
		rowUnion := &RowUnion{NullRow: nullRow}
		if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
			return err
		}

		// Check and insert checksum row if needed (after NullRow write)
		if err := tx.checkAndInsertChecksum(); err != nil {
			return err
		}

		// Close writer channel now that transaction is finalized
		if tx.writeChan != nil {
			close(tx.writeChan)
			tx.writeChan = nil
		}

		return nil
	}

	// FR-003: Handle data transactions - finalize the last PartialDataRow
	if tx.last.GetState() != PartialDataRowWithPayload && tx.last.GetState() != PartialDataRowWithSavepoint {
		return NewInvalidActionError("Commit() requires PartialDataRow with payload", nil)
	}

	// Finalize with commit (Commit() returns DataRow with TC or SC end_control)
	dataRow, err := tx.last.Commit()
	if err != nil {
		return NewInvalidActionError("failed to finalize last row for commit", err)
	}

	// Write final data row to disk (FR-003)
	// MarshalText() returns rowSize bytes (complete row)
	completeRowBytes, err := dataRow.MarshalText()
	if err != nil {
		return NewInvalidActionError("failed to marshal finalized DataRow", err)
	}

	// Write remaining bytes (FR-005: synchronous)
	// rowBytesWritten is automatically updated by writeBytes
	if err := tx.writeBytes(completeRowBytes); err != nil {
		// FR-006: Transaction is tombstoned by writeBytes on error
		return err
	}

	// Update transaction state only after successful write
	tx.rows = append(tx.rows, *dataRow)
	tx.last = nil
	tx.rowBytesWritten = 0

	// Notify finder of new row
	rowUnion := &RowUnion{DataRow: dataRow}
	if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
		return err
	}

	// Check and insert checksum row if needed (after DataRow commit)
	if err := tx.checkAndInsertChecksum(); err != nil {
		return err
	}

	// Close writer channel now that transaction is finalized
	if tx.writeChan != nil {
		close(tx.writeChan)
		tx.writeChan = nil
	}

	return nil
}

// IsCommitted returns true if the transaction has proper termination (commit or rollback).
// Returns false if the transaction is still open (last row ends with 'E').
// For empty transactions, returns true if empty field is non-nil.
// Returns false if transaction is tombstoned.
func (tx *Transaction) IsCommitted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	if tx.tombstone {
		return false
	}

	// For empty transactions, check if empty field is set
	if tx.empty != nil {
		return true
	}

	// For transactions without rows, they are not committed
	if len(tx.rows) == 0 {
		return false
	}

	lastRow := tx.rows[len(tx.rows)-1]
	endControl := lastRow.EndControl

	// Check if last row has a transaction-ending command
	// Transaction-ending commands: *C (commit) or *0-9 (rollback)
	second := endControl[1]
	if second == 'C' || (second >= '0' && second <= '9') {
		return true
	}

	// If last row ends with 'E', transaction is still open
	return false
}

// Close closes the transaction's writer channel and tombstones the transaction.
// This method should be called when the transaction is no longer needed,
// especially in error scenarios or when explicitly terminating a transaction.
// After calling Close(), the transaction cannot perform any more operations.
//
// Postconditions:
//   - writeChan is closed (allows writerLoop to exit and reset FileManager's writeChannel)
//   - Transaction is tombstoned (all future operations will return errors)
//   - Safe to call multiple times (idempotent)
func (tx *Transaction) Close() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// Already tombstoned - nothing to do
	if tx.tombstone {
		return nil
	}

	// Tombstone the transaction first
	tx.tombstone = true

	// Close the writer channel if it exists
	// This signals writerLoop to exit, which will nil out FileManager's writeChannel
	if tx.writeChan != nil {
		close(tx.writeChan)
		tx.writeChan = nil
	}

	return nil
}

// Savepoint creates a savepoint at the current position in the transaction.
//
// Preconditions:
//   - Transaction must be active (Begin() has been called, not yet committed/rolled back)
//   - Transaction must contain at least one data row
//   - Savepoint count must be less than 9
//   - transaction must not be tombstoned
//
// Postconditions:
//   - Current row is marked as a savepoint (EndControl will use 'S' prefix when finalized)
//   - Transaction state transitions to PartialDataRowWithSavepoint
//
// Returns InvalidActionError if preconditions are not met.
// Returns TombstonedError if transaction is tombstoned.
func (tx *Transaction) Savepoint() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return err
	}

	// Validate transaction is active
	if !tx.isActive() {
		if tx.isCommittedState() {
			return NewInvalidActionError("Savepoint() cannot be called on committed transaction", nil)
		}
		return NewInvalidActionError("Savepoint() requires Begin() to be called first", nil)
	}

	// Validate at least one data row exists
	// A data row exists if:
	//   - We have finalized rows (len(tx.rows) > 0), OR
	//   - The current partial row has payload (state != PartialDataRowWithStartControl)
	hasDataRow := len(tx.rows) > 0 || tx.last.GetState() != PartialDataRowWithStartControl
	if !hasDataRow {
		return NewInvalidActionError("cannot savepoint empty transaction", nil)
	}

	// Validate savepoint limit (max 9)
	savepointIndices := tx.getSavepointIndicesUnlocked()
	if len(savepointIndices) >= 9 {
		return NewInvalidActionError("transaction cannot have more than 9 savepoints", nil)
	}

	// Call PartialDataRow.Savepoint()
	if err := tx.last.Savepoint(); err != nil {
		return NewInvalidActionError("failed to create savepoint", err)
	}

	return nil
}

// Rollback rolls back the transaction to a specified savepoint or fully closes it.
//
// Parameters:
//   - savepointId: Target savepoint number (0-9)
//   - 0: Full rollback (invalidate all rows)
//   - 1-9: Partial rollback to specified savepoint
//
// Preconditions:
//   - Transaction must be active
//   - savepointId must be valid (0 to current savepoint count)
//   - transaction must not be tombstoned
//
// Postconditions:
//   - For savepointId = 0: All rows invalidated, NullRow created if transaction empty
//   - For savepointId > 0: Rows up to savepoint committed, subsequent rows invalidated
//   - Transaction is closed and no longer active
//   - Appropriate end control encoding applied (R0-R9, S0-S9)
//
// Returns:
//   - nil on success
//   - InvalidActionError if transaction is inactive
//   - InvalidInputError if savepointId is out of valid range
//   - TombstonedError if transaction is tombstoned
func (tx *Transaction) Rollback(savepointId int) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return err
	}

	// Validate transaction is active
	if !tx.isActive() {
		if tx.isCommittedState() {
			return NewInvalidActionError("Rollback() cannot be called on committed transaction", nil)
		}
		return NewInvalidActionError("Rollback() requires Begin() to be called first", nil)
	}

	// Validate savepointId range
	if savepointId < 0 || savepointId > 9 {
		return NewInvalidInputError("savepointId must be between 0 and 9", nil)
	}

	// Validate savepoint target exists (for partial rollback)
	savepointIndices := tx.getSavepointIndicesUnlocked()
	if savepointId > 0 && savepointId > len(savepointIndices) {
		return NewInvalidInputError("rollback target savepoint does not exist", nil)
	}

	// Handle empty transaction (Begin() + Rollback() with no AddRow)
	if len(tx.rows) == 0 && tx.last.GetState() == PartialDataRowWithStartControl {
		// Create NullRowPayload
		payload := &NullRowPayload{
			Key: uuid.Nil,
		}
		if err := payload.Validate(); err != nil {
			return NewInvalidActionError("created NullRowPayload failed validation", err)
		}

		// Create NullRow with validated payload
		nullRow := &NullRow{
			baseRow[*NullRowPayload]{
				RowSize:      tx.Header.GetRowSize(),
				StartControl: START_TRANSACTION,
				EndControl:   NULL_ROW_CONTROL,
				RowPayload:   payload,
			},
		}

		// Validate the created NullRow
		if err := nullRow.Validate(); err != nil {
			return NewInvalidActionError("created NullRow failed validation", err)
		}

		// Write NullRow to disk (similar to Commit())
		nullRowBytes, err := nullRow.MarshalText()
		if err != nil {
			return NewInvalidActionError("failed to marshal NullRow", err)
		}

		if err := tx.writeBytes(nullRowBytes); err != nil {
			// Transaction is tombstoned by writeBytes on error
			return err
		}

		// Update transaction state only after successful write
		tx.empty = nullRow
		tx.last = nil
		tx.rowBytesWritten = 0

		// Notify finder of new row
		rowUnion := &RowUnion{NullRow: nullRow}
		if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
			return err
		}

		// Check and insert checksum row if needed (after NullRow write)
		if err := tx.checkAndInsertChecksum(); err != nil {
			return err
		}

		// Close writer channel now that transaction is finalized
		if tx.writeChan != nil {
			close(tx.writeChan)
			tx.writeChan = nil
		}

		return nil
	}

	// Handle data transaction - finalize current partial row with rollback end control
	dataRow, err := tx.last.Rollback(savepointId)
	if err != nil {
		return NewInvalidActionError("failed to finalize last row for rollback", err)
	}

	// Write final data row to disk (similar to Commit())
	completeRowBytes, err := dataRow.MarshalText()
	if err != nil {
		return NewInvalidActionError("failed to marshal finalized DataRow", err)
	}

	// Write remaining bytes (synchronous)
	if err := tx.writeBytes(completeRowBytes); err != nil {
		// Transaction is tombstoned by writeBytes on error
		return err
	}

	// Update transaction state only after successful write
	tx.rows = append(tx.rows, *dataRow)
	tx.last = nil
	tx.rowBytesWritten = 0

	// Notify finder of new row
	rowUnion := &RowUnion{DataRow: dataRow}
	if err := tx.notifyFinderRowAdded(rowUnion); err != nil {
		return err
	}

	// Check and insert checksum row if needed (after rollback DataRow write)
	if err := tx.checkAndInsertChecksum(); err != nil {
		return err
	}

	// Close writer channel now that transaction is finalized
	if tx.writeChan != nil {
		close(tx.writeChan)
		tx.writeChan = nil
	}

	return nil
}

// GetCommittedRows returns an iterator function that yields only rows that are committed
// according to v1 file format rollback logic. The iterator function returns:
//   - row: The DataRow if more data is available
//   - more: true if more rows are available, false otherwise
//
// Returns an error if the transaction is invalid or cannot be processed.
// Returns TombstonedError if transaction is tombstoned.
func (tx *Transaction) GetCommittedRows() (func() (DataRow, bool), error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return nil, err
	}

	// Determine which rows are committed based on transaction ending
	committedIndices := tx.calculateCommittedIndicesUnlocked()

	// Create iterator
	index := 0
	return func() (DataRow, bool) {
		if index >= len(committedIndices) {
			return DataRow{}, false
		}
		rowIndex := committedIndices[index]
		index++
		return tx.rows[rowIndex], true
	}, nil
}

// calculateCommittedIndicesUnlocked is the unlocked version of calculateCommittedIndices.
// The caller must hold at least a read lock on tx.mu.
func (tx *Transaction) calculateCommittedIndicesUnlocked() []int {
	if len(tx.rows) == 0 {
		return []int{}
	}

	lastRow := tx.rows[len(tx.rows)-1]
	endControl := lastRow.EndControl
	second := endControl[1]

	// If transaction is still open (ends with 'E'), no rows are committed
	if second == 'E' {
		return []int{}
	}

	// If commit (ends with 'C'), all rows are committed
	if second == 'C' {
		indices := make([]int, len(tx.rows))
		for i := range tx.rows {
			indices[i] = i
		}
		return indices
	}

	// Rollback case: second is '0'-'9'
	rollbackTarget := int(second - '0')

	// Full rollback (R0 or S0): no rows committed
	if rollbackTarget == 0 {
		return []int{}
	}

	// Partial rollback: find savepoint indices and commit up to target savepoint
	savepointIndices := tx.getSavepointIndicesUnlocked()

	// Target savepoint is at savepointIndices[rollbackTarget-1]
	// rollbackTarget is guaranteed to be <= len(savepointIndices) by Validate()
	targetSavepointIndex := savepointIndices[rollbackTarget-1]

	// Commit all rows from start (index 0) through target savepoint (inclusive)
	indices := make([]int, targetSavepointIndex+1)
	for i := 0; i <= targetSavepointIndex; i++ {
		indices[i] = i
	}
	return indices
}

// IsRowCommitted determines if the specific row at the given index is committed.
// Applies transaction-wide rollback logic to individual row queries.
// Returns an error if the index is out of bounds.
// Returns TombstonedError if transaction is tombstoned.
func (tx *Transaction) IsRowCommitted(index int) (bool, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	// FR-006: Check if tombstoned
	if err := tx.checkTombstone(); err != nil {
		return false, err
	}

	if index < 0 || index >= len(tx.rows) {
		return false, NewInvalidInputError("Row index out of bounds", nil)
	}

	committedIndices := tx.calculateCommittedIndicesUnlocked()
	for _, committedIndex := range committedIndices {
		if committedIndex == index {
			return true, nil
		}
	}
	return false, nil
}

// GetSavepointIndices identifies all savepoint locations within the transaction
// using EndControl patterns with 'S' as first character.
// Returns indices for easy reference within the slice.
// Returns empty slice if transaction is tombstoned.
func (tx *Transaction) GetSavepointIndices() []int {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	if tx.tombstone {
		return []int{}
	}
	return tx.getSavepointIndicesUnlocked()
}

// getSavepointIndicesUnlocked is the unlocked version of GetSavepointIndices.
// The caller must hold at least a read lock on tx.mu.
func (tx *Transaction) getSavepointIndicesUnlocked() []int {
	var savepointIndices []int
	for i, row := range tx.rows {
		// Savepoint is created when first character of EndControl is 'S'
		if row.EndControl[0] == 'S' {
			savepointIndices = append(savepointIndices, i)
		}
	}
	return savepointIndices
}

// Validate scans all rows in the slice to ensure transaction integrity.
// Verifies:
//   - First row has StartControl = 'T' (transaction start)
//   - Proper StartControl sequences (T followed by R's for subsequent rows)
//   - Savepoint consistency and rollback target validity
//   - Only one transaction termination within range (or transaction is still open)
//   - DBFile (db field) is set if writeChan is set (transactions created via NewTransaction)
//
// After Validate() passes, tx.db can be assumed to be non-nil if writeChan is set.
//
// Returns CorruptDatabaseError for corruption scenarios or InvalidInputError for logic/instruction errors.
// Returns TombstonedError if transaction is tombstoned.
func (tx *Transaction) Validate() error {
	if tx == nil {
		return NewInvalidInputError("Transaction cannot be nil", nil)
	}

	tx.mu.RLock()
	defer tx.mu.RUnlock()

	if tx.Header == nil {
		return NewInvalidInputError("Header cannot be nil", nil)
	}

	if tx.db == nil {
		return NewInvalidInputError("DBFile cannot be nil", nil)
	}

	if tx.writeChan == nil {
		return NewInvalidInputError("Write channel cannot be nil", nil)
	}

	if tx.finder == nil {
		return NewInvalidInputError("Finder cannot be nil", nil)
	}

	if len(tx.rows) != 0 || tx.empty != nil || tx.last != nil {
		return NewInvalidInputError("Transaction must be inactive", nil)
	}
	return nil
}
