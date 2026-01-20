package frozendb

import (
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
	rows         []DataRow       // Single slice of DataRow objects (max 100) - unexported for immutability
	empty        *NullRow        // Empty null row after successful commit
	last         *PartialDataRow // Current partial data row being built
	Header       *Header         // Header reference for row creation
	maxTimestamp int64           // Maximum timestamp seen in this transaction for UUID ordering
	mu           sync.RWMutex    // Mutex for thread safety
}

// GetRows returns the rows slice for read-only access.
// Since all fields of DataRow are unexported, modifications to the slice
// elements won't affect the internal transaction state.
func (tx *Transaction) GetRows() []DataRow {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.rows
}

// GetEmptyRow returns the empty NullRow if present, nil otherwise.
// This field is set after a successful empty transaction commit.
func (tx *Transaction) GetEmptyRow() *NullRow {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.empty
}

// GetMaxTimestamp returns the current maximum timestamp seen in this transaction.
// This value is used for UUID timestamp ordering validation.
func (tx *Transaction) GetMaxTimestamp() int64 {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.maxTimestamp
}

// extractUUIDv7Timestamp extracts the 48-bit millisecond timestamp from a UUIDv7.
// The timestamp is stored in the first 6 bytes (48 bits) of the UUID.
func extractUUIDv7Timestamp(u uuid.UUID) int64 {
	// UUIDv7 format: first 48 bits are the timestamp in milliseconds
	// Bytes 0-5 contain the timestamp, big-endian
	return int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
		int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
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

// Begin initializes an empty transaction by creating a PartialDataRow in
// PartialDataRowWithStartControl state. This method can only be called when
// the transaction is inactive (all fields empty/nil).
//
// Preconditions:
//   - rows slice must be empty
//   - empty field must be nil
//   - last field must be nil
//
// Postconditions:
//   - last field points to new PartialDataRow with start control
//   - All other fields remain unchanged
//
// Returns InvalidActionError if preconditions are not met.
func (tx *Transaction) Begin() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

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
	pdr := &PartialDataRow{
		state: PartialDataRowWithStartControl,
		d: DataRow{
			baseRow[*DataRowPayload]{
				Header:       tx.Header,
				StartControl: START_TRANSACTION,
			},
		},
	}

	// Validate the created PartialDataRow
	if err := pdr.Validate(); err != nil {
		return NewInvalidActionError("created PartialDataRow failed validation", err)
	}

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
func (tx *Transaction) AddRow(key uuid.UUID, value string) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

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
	if value == "" {
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

	// Validate: new_timestamp + skew_ms > max_timestamp
	if newTimestamp+skewMs <= tx.maxTimestamp {
		return NewKeyOrderingError("UUID timestamp violates ordering constraint: new_timestamp + skew_ms must be > max_timestamp", nil)
	}

	// Check the current state of the partial row
	if tx.last.GetState() == PartialDataRowWithStartControl {
		// First AddRow after Begin(): add key/value to the existing partial
		// The partial already has START_TRANSACTION from Begin()
		if err := tx.last.AddRow(key, value); err != nil {
			return err
		}
	} else {
		// Subsequent AddRow(): finalize current partial and create new one
		// FR-002, FR-003: Finalize current PartialDataRow with ROW_END_CONTROL (RE)
		dataRow, err := tx.last.EndRow()
		if err != nil {
			return NewInvalidActionError("failed to finalize previous row", err)
		}
		tx.rows = append(tx.rows, *dataRow)

		// FR-004, FR-005: Create new PartialDataRow with ROW_CONTINUE
		// All rows after the first use ROW_CONTINUE
		newPdr := &PartialDataRow{
			state: PartialDataRowWithStartControl,
			d: DataRow{
				baseRow[*DataRowPayload]{
					Header:       tx.Header,
					StartControl: ROW_CONTINUE,
				},
			},
		}

		// Validate the newly created PartialDataRow before adding data
		if err := newPdr.Validate(); err != nil {
			return NewInvalidActionError("created PartialDataRow failed validation", err)
		}

		// Add the key-value data to the new partial
		if err := newPdr.AddRow(key, value); err != nil {
			return err
		}

		tx.last = newPdr
	}

	// FR-015: Update max_timestamp
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
//
// Postconditions:
//   - For empty transactions: empty field points to created NullRow, last is nil
//   - For data transactions: last PartialDataRow is finalized and added to rows[], last is nil
//
// Returns InvalidActionError if preconditions are not met.
func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// Validate preconditions - transaction must be active
	if tx.last == nil {
		return NewInvalidActionError("Commit() requires an active transaction (call Begin() first)", nil)
	}
	if tx.empty != nil {
		return NewInvalidActionError("Commit() cannot be called when transaction is already committed", nil)
	}

	// FR-009: Handle empty transactions (Begin() + Commit() with no AddRow() calls)
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
				Header:       tx.Header,
				StartControl: START_TRANSACTION,
				EndControl:   NULL_ROW_CONTROL,
				RowPayload:   payload,
			},
		}

		// Validate the created NullRow
		if err := nullRow.Validate(); err != nil {
			return NewInvalidActionError("created NullRow failed validation", err)
		}

		// Update transaction state
		tx.empty = nullRow
		tx.last = nil

		return nil
	}

	// FR-008: Handle data transactions - finalize the last PartialDataRow
	if tx.last.GetState() != PartialDataRowWithPayload && tx.last.GetState() != PartialDataRowWithSavepoint {
		return NewInvalidActionError("Commit() requires PartialDataRow with payload", nil)
	}

	// Finalize with commit (Commit() returns DataRow with TC or SC end_control)
	dataRow, err := tx.last.Commit()
	if err != nil {
		return NewInvalidActionError("failed to finalize last row for commit", err)
	}

	tx.rows = append(tx.rows, *dataRow)
	tx.last = nil

	return nil
}

// IsCommitted returns true if the transaction has proper termination (commit or rollback).
// Returns false if the transaction is still open (last row ends with 'E').
// For empty transactions, returns true if empty field is non-nil.
func (tx *Transaction) IsCommitted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

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

// GetCommittedRows returns an iterator function that yields only rows that are committed
// according to v1 file format rollback logic. The iterator function returns:
//   - row: The DataRow if more data is available
//   - more: true if more rows are available, false otherwise
//
// Returns an error if the transaction is invalid or cannot be processed.
func (tx *Transaction) GetCommittedRows() (func() (DataRow, bool), error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

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
func (tx *Transaction) IsRowCommitted(index int) (bool, error) {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

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
func (tx *Transaction) GetSavepointIndices() []int {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
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
//
// Returns CorruptDatabaseError for corruption scenarios or InvalidInputError for logic/instruction errors.
func (tx *Transaction) Validate() error {
	if tx == nil {
		return NewInvalidInputError("Transaction cannot be nil", nil)
	}

	tx.mu.RLock()
	defer tx.mu.RUnlock()

	// Allow empty transactions that have been committed (empty field set)
	if len(tx.rows) == 0 {
		// If empty field is set, transaction is a valid empty transaction
		if tx.empty != nil {
			return nil
		}
		// If last field is set, transaction is in active state (valid during workflow)
		if tx.last != nil {
			return nil
		}
		// Otherwise, this is an uninitialized/inactive transaction - valid for Begin() call
		return nil
	}

	// Check maximum row count
	if len(tx.rows) > 100 {
		return NewInvalidInputError("Transaction cannot contain more than 100 rows", nil)
	}

	// Validate first row has StartControl = 'T'
	firstRow := tx.rows[0]
	if firstRow.StartControl != START_TRANSACTION {
		return NewCorruptDatabaseError("First row must have StartControl='T' (transaction start)", nil)
	}

	// Validate StartControl sequences: T followed by R's
	for i := 1; i < len(tx.rows); i++ {
		if tx.rows[i].StartControl != ROW_CONTINUE {
			return NewCorruptDatabaseError("Subsequent rows must have StartControl='R' (row continuation)", nil)
		}
	}

	// Validate savepoint count (max 9)
	savepointIndices := tx.getSavepointIndicesUnlocked()
	if len(savepointIndices) > 9 {
		return NewInvalidInputError("Transaction cannot contain more than 9 savepoints", nil)
	}

	// Check for transaction termination
	lastRow := tx.rows[len(tx.rows)-1]
	endControl := lastRow.EndControl
	second := endControl[1]

	// Count transaction-ending commands
	terminationCount := 0
	for _, row := range tx.rows {
		ec := row.EndControl
		sec := ec[1]
		if sec == 'C' || (sec >= '0' && sec <= '9') {
			terminationCount++
		}
	}

	// If transaction is still open (last row ends with 'E'), no termination required
	if second == 'E' {
		// Transaction is still open, no termination required
		// But we should check that there's no termination command before the last row
		if terminationCount > 0 {
			return NewCorruptDatabaseError("Transaction ending command found before transaction is complete", nil)
		}
		return nil
	}

	// Transaction has termination - must have exactly one
	if terminationCount != 1 {
		return NewCorruptDatabaseError("Transaction must have exactly one transaction-ending command", nil)
	}

	// Validate rollback target if rollback command
	if second >= '0' && second <= '9' {
		rollbackTarget := int(second - '0')
		if rollbackTarget > 0 {
			// Partial rollback - validate savepoint exists
			if rollbackTarget > len(savepointIndices) {
				return NewInvalidInputError("Rollback target savepoint does not exist", nil)
			}
		}
	}

	return nil
}
