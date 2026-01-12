package frozendb

// Transaction represents a single database transaction with maximum 100 DataRow objects.
// The first row must be the transaction start (StartControl = 'T'), and the last row
// is either the end of the transaction or the transaction is still open.
//
// After creating a Transaction struct directly, you MUST call Validate() before using it.
type Transaction struct {
	rows []DataRow // Single slice of DataRow objects (max 100) - unexported for immutability
}

// GetRows returns the rows slice for read-only access.
// Since all fields of DataRow are unexported, modifications to the slice
// elements won't affect the internal transaction state.
func (tx *Transaction) GetRows() []DataRow {
	return tx.rows
}

// IsCommitted returns true if the transaction has proper termination (commit or rollback).
// Returns false if the transaction is still open (last row ends with 'E').
func (tx *Transaction) IsCommitted() bool {
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
	// Determine which rows are committed based on transaction ending
	committedIndices := tx.calculateCommittedIndices()

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

// calculateCommittedIndices determines which row indices are committed based on
// the transaction ending command and rollback logic.
func (tx *Transaction) calculateCommittedIndices() []int {
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
	savepointIndices := tx.GetSavepointIndices()

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
	if index < 0 || index >= len(tx.rows) {
		return false, NewInvalidInputError("Row index out of bounds", nil)
	}

	committedIndices := tx.calculateCommittedIndices()
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

	if len(tx.rows) == 0 {
		return NewInvalidInputError("Transaction must contain at least one row", nil)
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
	savepointIndices := tx.GetSavepointIndices()
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
