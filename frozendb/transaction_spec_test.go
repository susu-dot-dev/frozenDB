package frozendb

import (
	"testing"

	"github.com/google/uuid"
)

// Helper function to create a test header
func createTestHeader() *Header {
	return &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}
}

// Helper function to create a test DataRow
func createTestDataRow(header *Header, startControl StartControl, endControl EndControl, key uuid.UUID, value string) *DataRow {
	return &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: startControl,
			EndControl:   endControl,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}
}

// Test_S_006_FR_001_TransactionStructCreation tests FR-001: Transaction struct MUST store a single slice of DataRow objects with maximum 100 rows
func Test_S_006_FR_001_TransactionStructCreation(t *testing.T) {
	header := createTestHeader()

	// Test: Create transaction with single row
	t.Run("single_row", func(t *testing.T) {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		row := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		if err := row.Validate(); err != nil {
			t.Fatalf("Row validation failed: %v", err)
		}

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if len(tx.GetRows()) != 1 {
			t.Errorf("Expected 1 row, got %d", len(tx.GetRows()))
		}
	})

	// Test: Create transaction with maximum 100 rows
	t.Run("max_100_rows", func(t *testing.T) {
		rows := make([]DataRow, 100)
		for i := 0; i < 100; i++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			var startControl StartControl
			var endControl EndControl
			switch i {
			case 0:
				startControl = START_TRANSACTION
				endControl = ROW_END_CONTROL
			case 99:
				startControl = ROW_CONTINUE
				endControl = TRANSACTION_COMMIT
			default:
				startControl = ROW_CONTINUE
				endControl = ROW_END_CONTROL
			}

			row := createTestDataRow(header, startControl, endControl, key, `{"data":"test"}`)
			if err := row.Validate(); err != nil {
				t.Fatalf("Row validation failed: %v", err)
			}
			rows[i] = *row
		}

		tx := &Transaction{
			rows: rows,
		}

		if len(tx.GetRows()) != 100 {
			t.Errorf("Expected 100 rows, got %d", len(tx.GetRows()))
		}
	})

	// Test: Transaction can be created with empty slice (will fail validation later)
	t.Run("empty_slice", func(t *testing.T) {
		tx := &Transaction{
			rows: []DataRow{},
		}

		if len(tx.GetRows()) != 0 {
			t.Errorf("Expected 0 rows, got %d", len(tx.GetRows()))
		}
	})
}

// Test_S_006_FR_002_DirectIndexingSystem tests FR-002: Transaction struct MUST provide direct indexing where index 0 maps to first element of the slice (which must be transaction start)
func Test_S_006_FR_002_DirectIndexingSystem(t *testing.T) {
	header := createTestHeader()

	key1, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}
	key2, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}
	key3, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
	if err := row1.Validate(); err != nil {
		t.Fatalf("Row validation failed: %v", err)
	}

	row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
	if err := row2.Validate(); err != nil {
		t.Fatalf("Row validation failed: %v", err)
	}

	row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
	if err := row3.Validate(); err != nil {
		t.Fatalf("Row validation failed: %v", err)
	}

	tx := &Transaction{
		rows: []DataRow{*row1, *row2, *row3},
	}

	rows := tx.GetRows()
	// Test: Index 0 maps to first element
	if rows[0].GetKey() != key1 {
		t.Errorf("Index 0 should map to first row key, expected %s, got %s", key1, rows[0].GetKey())
	}

	// Test: Index 1 maps to second element
	if rows[1].GetKey() != key2 {
		t.Errorf("Index 1 should map to second row key, expected %s, got %s", key2, rows[1].GetKey())
	}

	// Test: Index 2 maps to third element
	if rows[2].GetKey() != key3 {
		t.Errorf("Index 2 should map to third row key, expected %s, got %s", key3, rows[2].GetKey())
	}

	// Test: First row must be transaction start
	if rows[0].StartControl != START_TRANSACTION {
		t.Errorf("First row must have StartControl='T', got %c", rows[0].StartControl)
	}
}

// Test_S_006_FR_006_GetCommittedRowsIterator tests FR-006: GetCommittedRows() method MUST return an iterator function that yields only rows that are committed according to v1 file format rollback logic
func Test_S_006_FR_006_GetCommittedRowsIterator(t *testing.T) {
	header := createTestHeader()

	// Test: Clean commit - all rows should be returned
	t.Run("clean_commit", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows failed: %v", err)
		}

		count := 0
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
			count++
		}

		if count != 3 {
			t.Errorf("Expected 3 committed rows, got %d", count)
		}

		if len(committedKeys) != 3 || committedKeys[0] != key1 || committedKeys[1] != key2 || committedKeys[2] != key3 {
			t.Errorf("Committed rows mismatch: expected [%s, %s, %s], got %v", key1, key2, key3, committedKeys)
		}
	})

	// Test: Full rollback - no rows should be returned
	t.Run("full_rollback", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, FULL_ROLLBACK, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows failed: %v", err)
		}

		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		if count != 0 {
			t.Errorf("Expected 0 committed rows for full rollback, got %d", count)
		}
	})

	// Test: Partial rollback to savepoint - only rows up to savepoint should be returned
	t.Run("partial_rollback_to_savepoint", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		// Rollback to savepoint 1 (created on row1)
		rollbackEndControl := EndControl{'R', '1'}
		row3 := createTestDataRow(header, ROW_CONTINUE, rollbackEndControl, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows failed: %v", err)
		}

		count := 0
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
			count++
		}

		if count != 1 {
			t.Errorf("Expected 1 committed row (up to savepoint), got %d", count)
		}

		if len(committedKeys) != 1 || committedKeys[0] != key1 {
			t.Errorf("Committed row mismatch: expected [%s], got %v", key1, committedKeys)
		}
	})
}

// Test_S_006_FR_007_CommitRollbackLogic tests FR-007: GetCommittedRows() iterator MUST handle partial rollbacks to savepoints, full rollbacks, and clean commits correctly
func Test_S_006_FR_007_CommitRollbackLogic(t *testing.T) {
	header := createTestHeader()

	// Test: Multiple savepoints with rollback to middle savepoint
	t.Run("multiple_savepoints_rollback", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		// Row 1: Transaction start + savepoint 1
		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		// Row 2: Continue
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		// Row 3: Continue + savepoint 2
		row3 := createTestDataRow(header, ROW_CONTINUE, SAVEPOINT_CONTINUE, key3, `{"data":"third"}`)
		row3.Validate()
		// Row 4: Rollback to savepoint 1
		rollbackEndControl := EndControl{'R', '1'}
		row4 := createTestDataRow(header, ROW_CONTINUE, rollbackEndControl, key4, `{"data":"fourth"}`)
		row4.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3, *row4},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows failed: %v", err)
		}

		count := 0
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
			count++
		}

		// Should only return row1 (up to savepoint 1)
		if count != 1 {
			t.Errorf("Expected 1 committed row (up to savepoint 1), got %d", count)
		}

		if len(committedKeys) != 1 || committedKeys[0] != key1 {
			t.Errorf("Committed row mismatch: expected [%s], got %v", key1, committedKeys)
		}
	})

	// Test: Savepoint with commit
	t.Run("savepoint_with_commit", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		// Row 1: Transaction start + savepoint 1 + continue
		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		// Row 2: Continue + commit
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows failed: %v", err)
		}

		count := 0
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
			count++
		}

		// Should return both rows (commit makes all rows valid)
		if count != 2 {
			t.Errorf("Expected 2 committed rows, got %d", count)
		}

		if len(committedKeys) != 2 || committedKeys[0] != key1 || committedKeys[1] != key2 {
			t.Errorf("Committed rows mismatch: expected [%s, %s], got %v", key1, key2, committedKeys)
		}
	})
}

// Test_S_006_FR_003_TransactionStartValidation tests FR-003: The first row of the slice MUST be the start of the transaction (StartControl = 'T'), verified by Validate()
func Test_S_006_FR_003_TransactionStartValidation(t *testing.T) {
	header := createTestHeader()

	// Test: Valid transaction start
	t.Run("valid_transaction_start", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Valid transaction start should pass validation: %v", err)
		}
	})

	// Test: Invalid transaction start (starts with R)
	t.Run("invalid_transaction_start", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Transaction starting with R should fail validation")
		}

		// Should return CorruptDatabaseError
		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("Expected CorruptDatabaseError, got %T", err)
		}
	})
}

// Test_S_006_FR_004_IsCommittedMethod tests FR-004: IsCommitted() method MUST return true only when transaction has proper termination (commit or rollback)
func Test_S_006_FR_004_IsCommittedMethod(t *testing.T) {
	header := createTestHeader()

	// Test: Committed transaction (ends with TC)
	t.Run("committed_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if !tx.IsCommitted() {
			t.Error("IsCommitted() should return true for committed transaction")
		}
	})

	// Test: Rolled back transaction (ends with R0)
	t.Run("rolled_back_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, FULL_ROLLBACK, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if !tx.IsCommitted() {
			t.Error("IsCommitted() should return true for rolled back transaction (has termination)")
		}
	})

	// Test: Open transaction (ends with RE)
	t.Run("open_transaction", func(t *testing.T) {
		key, _ := uuid.NewV7()

		row := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})
}

// Test_S_006_FR_005_OpenTransactionHandling tests FR-005: IsCommitted() method MUST handle edge case where transaction is still open (last row ends with E)
func Test_S_006_FR_005_OpenTransactionHandling(t *testing.T) {
	header := createTestHeader()

	// Test: Single row transaction still open
	t.Run("single_row_open", func(t *testing.T) {
		key, _ := uuid.NewV7()

		row := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})

	// Test: Multiple row transaction still open
	t.Run("multiple_row_open", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})

	// Test: Open transaction with savepoint
	t.Run("open_transaction_with_savepoint", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})
}

// Test_S_006_FR_008_SavepointDetection tests FR-008: GetSavepointIndices() method MUST identify all savepoint locations using EndControl patterns with S as first character
func Test_S_006_FR_008_SavepointDetection(t *testing.T) {
	header := createTestHeader()

	// Test: No savepoints
	t.Run("no_savepoints", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		savepointIndices := tx.GetSavepointIndices()
		if len(savepointIndices) != 0 {
			t.Errorf("Expected 0 savepoints, got %d", len(savepointIndices))
		}
	})

	// Test: Single savepoint
	t.Run("single_savepoint", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		savepointIndices := tx.GetSavepointIndices()
		if len(savepointIndices) != 1 {
			t.Errorf("Expected 1 savepoint, got %d", len(savepointIndices))
		}
		if len(savepointIndices) > 0 && savepointIndices[0] != 0 {
			t.Errorf("Expected savepoint at index 0, got %d", savepointIndices[0])
		}
	})

	// Test: Multiple savepoints
	t.Run("multiple_savepoints", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, SAVEPOINT_CONTINUE, key2, `{"data":"second"}`)
		row2.Validate()
		row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		savepointIndices := tx.GetSavepointIndices()
		if len(savepointIndices) != 2 {
			t.Errorf("Expected 2 savepoints, got %d", len(savepointIndices))
		}
		if len(savepointIndices) >= 2 {
			if savepointIndices[0] != 0 {
				t.Errorf("Expected first savepoint at index 0, got %d", savepointIndices[0])
			}
			if savepointIndices[1] != 1 {
				t.Errorf("Expected second savepoint at index 1, got %d", savepointIndices[1])
			}
		}
	})
}

// Test_S_006_FR_009_SavepointIndices tests FR-009: GetSavepointIndices() method MUST return indices for easy reference within the slice
func Test_S_006_FR_009_SavepointIndices(t *testing.T) {
	header := createTestHeader()

	// Test: Savepoints at various positions
	t.Run("savepoints_at_various_positions", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		// Row 0: Transaction start + continue (no savepoint)
		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		// Row 1: Continue + savepoint
		row2 := createTestDataRow(header, ROW_CONTINUE, SAVEPOINT_CONTINUE, key2, `{"data":"second"}`)
		row2.Validate()
		// Row 2: Continue (no savepoint)
		row3 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key3, `{"data":"third"}`)
		row3.Validate()
		// Row 3: Continue + savepoint
		row4 := createTestDataRow(header, ROW_CONTINUE, SAVEPOINT_COMMIT, key4, `{"data":"fourth"}`)
		row4.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3, *row4},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		savepointIndices := tx.GetSavepointIndices()
		expectedIndices := []int{1, 3}
		if len(savepointIndices) != len(expectedIndices) {
			t.Errorf("Expected %d savepoints, got %d", len(expectedIndices), len(savepointIndices))
		}
		for i, expected := range expectedIndices {
			if i < len(savepointIndices) && savepointIndices[i] != expected {
				t.Errorf("Savepoint %d: expected index %d, got %d", i, expected, savepointIndices[i])
			}
		}
	})
}

// Test_S_006_FR_010_IsRowCommittedMethod tests FR-010: IsRowCommitted(index) method MUST determine if specific row at index is committed
func Test_S_006_FR_010_IsRowCommittedMethod(t *testing.T) {
	header := createTestHeader()

	// Test: Check committed row
	t.Run("committed_row", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		committed, err := tx.IsRowCommitted(0)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if !committed {
			t.Error("Row 0 should be committed")
		}

		committed, err = tx.IsRowCommitted(1)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if !committed {
			t.Error("Row 1 should be committed")
		}
	})

	// Test: Check rolled back row
	t.Run("rolled_back_row", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, FULL_ROLLBACK, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		committed, err := tx.IsRowCommitted(0)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if committed {
			t.Error("Row 0 should not be committed (full rollback)")
		}

		committed, err = tx.IsRowCommitted(1)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if committed {
			t.Error("Row 1 should not be committed (full rollback)")
		}
	})

	// Test: Out of bounds index
	t.Run("out_of_bounds", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		_, err := tx.IsRowCommitted(999)
		if err == nil {
			t.Error("IsRowCommitted should return error for out of bounds index")
		}
	})
}

// Test_S_006_FR_011_TransactionWideRollbackLogic tests FR-011: IsRowCommitted(index) method MUST apply transaction-wide rollback logic to individual row queries
func Test_S_006_FR_011_TransactionWideRollbackLogic(t *testing.T) {
	header := createTestHeader()

	// Test: Partial rollback - rows after savepoint should not be committed
	t.Run("partial_rollback_logic", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		// Row 0: Transaction start + savepoint 1
		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		// Row 1: Continue (after savepoint)
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		// Row 2: Rollback to savepoint 1
		rollbackEndControl := EndControl{'R', '1'}
		row3 := createTestDataRow(header, ROW_CONTINUE, rollbackEndControl, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Row 0 should be committed (up to savepoint 1)
		committed, err := tx.IsRowCommitted(0)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if !committed {
			t.Error("Row 0 should be committed (up to savepoint)")
		}

		// Row 1 should not be committed (after savepoint)
		committed, err = tx.IsRowCommitted(1)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if committed {
			t.Error("Row 1 should not be committed (after savepoint)")
		}

		// Row 2 should not be committed (rollback command)
		committed, err = tx.IsRowCommitted(2)
		if err != nil {
			t.Fatalf("IsRowCommitted failed: %v", err)
		}
		if committed {
			t.Error("Row 2 should not be committed (rollback command)")
		}
	})
}

// Test_S_006_FR_012_ValidateScanAllRows tests FR-012: Validate() method MUST scan all rows in the slice to check for inconsistencies
func Test_S_006_FR_012_ValidateScanAllRows(t *testing.T) {
	header := createTestHeader()

	// Test: Valid transaction passes validation
	t.Run("valid_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Valid transaction should pass validation: %v", err)
		}
	})

	// Test: Invalid StartControl sequence detected
	t.Run("invalid_start_control_sequence", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		// Row 2 has invalid StartControl (should be R, but we'll use T)
		row2 := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Transaction with invalid StartControl sequence should fail validation")
		}
	})
}

// Test_S_006_FR_013_FirstRowValidation tests FR-013: Validate() method MUST verify the first row has StartControl = 'T' (transaction start)
func Test_S_006_FR_013_FirstRowValidation(t *testing.T) {
	header := createTestHeader()

	// Test: First row with T passes
	t.Run("valid_first_row", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("First row with T should pass validation: %v", err)
		}
	})

	// Test: First row with R fails
	t.Run("invalid_first_row", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("First row with R should fail validation")
		}

		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("Expected CorruptDatabaseError, got %T", err)
		}
	})
}

// Test_S_006_FR_014_StartControlSequences tests FR-014: Validate() method MUST ensure proper StartControl sequences (T followed by R's for subsequent rows)
func Test_S_006_FR_014_StartControlSequences(t *testing.T) {
	header := createTestHeader()

	// Test: Valid sequence T, R, R
	t.Run("valid_sequence", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Valid sequence should pass validation: %v", err)
		}
	})

	// Test: Invalid sequence T, T (second row should be R)
	t.Run("invalid_sequence", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Invalid sequence should fail validation")
		}
	})
}

// Test_S_006_FR_015_SavepointConsistency tests FR-015: Validate() method MUST validate savepoint consistency and rollback target validity
func Test_S_006_FR_015_SavepointConsistency(t *testing.T) {
	header := createTestHeader()

	// Test: Valid rollback to existing savepoint
	t.Run("valid_rollback_target", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, SAVEPOINT_CONTINUE, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()
		rollbackEndControl := EndControl{'R', '1'}
		row3 := createTestDataRow(header, ROW_CONTINUE, rollbackEndControl, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Valid rollback target should pass validation: %v", err)
		}
	})

	// Test: Invalid rollback to non-existent savepoint
	t.Run("invalid_rollback_target", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		// Rollback to savepoint 1, but no savepoint exists
		rollbackEndControl := EndControl{'R', '1'}
		row2 := createTestDataRow(header, ROW_CONTINUE, rollbackEndControl, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Rollback to non-existent savepoint should fail validation")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	// Test: Too many savepoints (max 9)
	t.Run("too_many_savepoints", func(t *testing.T) {
		rows := make([]DataRow, 11) // 10 savepoints + 1 commit
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			var endControl EndControl
			if i < 9 {
				endControl = SAVEPOINT_CONTINUE
			} else {
				endControl = TRANSACTION_COMMIT
			}

			var startControl StartControl
			if i == 0 {
				startControl = START_TRANSACTION
			} else {
				startControl = ROW_CONTINUE
			}

			row := createTestDataRow(header, startControl, endControl, key, `{"data":"test"}`)
			row.Validate()
			rows[i] = *row
		}

		// Add commit row
		key, _ := uuid.NewV7()
		commitRow := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key, `{"data":"commit"}`)
		commitRow.Validate()
		rows[10] = *commitRow

		tx := &Transaction{
			rows: rows,
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Transaction with more than 9 savepoints should fail validation")
		}
	})
}

// Test_S_006_FR_016_TransactionTermination tests FR-016: Validate() method MUST ensure only one transaction termination within range (or transaction is still open)
func Test_S_006_FR_016_TransactionTermination(t *testing.T) {
	header := createTestHeader()

	// Test: Single termination (commit)
	t.Run("single_termination_commit", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Single termination should pass validation: %v", err)
		}
	})

	// Test: Multiple terminations (invalid)
	t.Run("multiple_terminations", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, TRANSACTION_COMMIT, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
		row2.Validate()
		row3 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key3, `{"data":"third"}`)
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Multiple terminations should fail validation")
		}
	})

	// Test: Open transaction (no termination)
	t.Run("open_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
		row1.Validate()
		row2 := createTestDataRow(header, ROW_CONTINUE, ROW_END_CONTROL, key2, `{"data":"second"}`)
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if err := tx.Validate(); err != nil {
			t.Errorf("Open transaction should pass validation: %v", err)
		}
	})
}

// Test_S_006_FR_017_ErrorTypes tests FR-017: Validate() method MUST return CorruptDatabaseError for corruption scenarios or InvalidInputError for logic/instruction errors
func Test_S_006_FR_017_ErrorTypes(t *testing.T) {
	header := createTestHeader()

	// Test: CorruptDatabaseError for invalid first row
	t.Run("corrupt_database_error", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key, `{"data":"test"}`)
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Should return error for invalid first row")
		}

		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("Expected CorruptDatabaseError, got %T", err)
		}
	})

	// Test: InvalidInputError for too many savepoints
	t.Run("invalid_input_error", func(t *testing.T) {
		// Create transaction with 10 savepoints (max is 9)
		rows := make([]DataRow, 10)
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			var startControl StartControl
			if i == 0 {
				startControl = START_TRANSACTION
			} else {
				startControl = ROW_CONTINUE
			}

			row := createTestDataRow(header, startControl, SAVEPOINT_CONTINUE, key, `{"data":"test"}`)
			row.Validate()
			rows[i] = *row
		}

		tx := &Transaction{
			rows: rows,
		}

		err := tx.Validate()
		if err == nil {
			t.Error("Should return error for too many savepoints")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})
}

// Test_S_006_FR_018_ThreadSafety tests FR-018: Transaction struct MUST be thread-safe for concurrent read access (immutable underlying data)
func Test_S_006_FR_018_ThreadSafety(t *testing.T) {
	header := createTestHeader()

	key1, _ := uuid.NewV7()
	key2, _ := uuid.NewV7()

	row1 := createTestDataRow(header, START_TRANSACTION, ROW_END_CONTROL, key1, `{"data":"first"}`)
	row1.Validate()
	row2 := createTestDataRow(header, ROW_CONTINUE, TRANSACTION_COMMIT, key2, `{"data":"second"}`)
	row2.Validate()

	tx := &Transaction{
		rows: []DataRow{*row1, *row2},
	}

	if err := tx.Validate(); err != nil {
		t.Fatalf("Transaction validation failed: %v", err)
	}

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Concurrent reads should not cause issues
			_ = tx.IsCommitted()
			_, _ = tx.IsRowCommitted(0)
			_ = tx.GetSavepointIndices()
			iter, err := tx.GetCommittedRows()
			if err == nil {
				for _, more := iter(); more; _, more = iter() {
					// Read rows
				}
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, thread safety is maintained
}
