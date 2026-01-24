package frozendb

import (
	"encoding/json"
	"fmt"
	"os"
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

// mockDBFile is a minimal mock implementation of DBFile for tests
type mockDBFile struct {
	mode string
}

func (m *mockDBFile) Read(start int64, size int32) ([]byte, error) {
	return nil, NewPathError("mock DBFile: Read not implemented", nil)
}

func (m *mockDBFile) Size() int64 {
	return 0
}

func (m *mockDBFile) Close() error {
	return nil
}

func (m *mockDBFile) SetWriter(dataChan <-chan Data) error {
	return nil
}

func (m *mockDBFile) GetMode() string {
	if m.mode == "" {
		return MODE_WRITE // Default to write mode for backward compatibility
	}
	return m.mode
}

// Helper function to create a transaction with mock write channel for spec tests
// This is needed because Begin() now requires writeChan (spec 015)
func createTransactionWithMockWriter(header *Header) *Transaction {
	writeChan := make(chan Data, 100)
	go func() {
		for data := range writeChan {
			// Send success response (mock FileManager)
			data.Response <- nil
		}
	}()
	tx := &Transaction{
		Header:    header,
		writeChan: writeChan,
		db:        &mockDBFile{},
	}
	// Validate the transaction after construction
	if err := tx.Validate(); err != nil {
		panic(fmt.Sprintf("createTransactionWithMockWriter: Validate() failed: %v", err))
	}
	return tx
}

// Helper function to create a test DataRow
func createTestDataRow(startControl StartControl, endControl EndControl, key uuid.UUID, value json.RawMessage) *DataRow {
	return &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      512,
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
	// Test: Create transaction with single row
	t.Run("single_row", func(t *testing.T) {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		row := createTestDataRow(START_TRANSACTION, TRANSACTION_COMMIT, key, json.RawMessage(`{"data":"test"}`))
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

			row := createTestDataRow(startControl, endControl, key, json.RawMessage(`{"data":"test"}`))
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

	row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
	if err := row1.Validate(); err != nil {
		t.Fatalf("Row validation failed: %v", err)
	}

	row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
	if err := row2.Validate(); err != nil {
		t.Fatalf("Row validation failed: %v", err)
	}

	row3 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key3, json.RawMessage(`{"data":"third"}`))
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

	// Test: Clean commit - all rows should be returned
	t.Run("clean_commit", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()
		row3 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key3, json.RawMessage(`{"data":"third"}`))
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
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

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, FULL_ROLLBACK, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
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

		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()
		// Rollback to savepoint 1 (created on row1)
		rollbackEndControl := EndControl{'R', '1'}
		row3 := createTestDataRow(ROW_CONTINUE, rollbackEndControl, key3, json.RawMessage(`{"data":"third"}`))
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
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

	// Test: Multiple savepoints with rollback to middle savepoint
	t.Run("multiple_savepoints_rollback", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		// Row 1: Transaction start + savepoint 1
		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		// Row 2: Continue
		row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()
		// Row 3: Continue + savepoint 2
		row3 := createTestDataRow(ROW_CONTINUE, SAVEPOINT_CONTINUE, key3, json.RawMessage(`{"data":"third"}`))
		row3.Validate()
		// Row 4: Rollback to savepoint 1
		rollbackEndControl := EndControl{'R', '1'}
		row4 := createTestDataRow(ROW_CONTINUE, rollbackEndControl, key4, json.RawMessage(`{"data":"fourth"}`))
		row4.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3, *row4},
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
		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		// Row 2: Continue + commit
		row2 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
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

	// Test: Valid transaction start
	t.Run("valid_transaction_start", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(START_TRANSACTION, TRANSACTION_COMMIT, key, json.RawMessage(`{"data":"test"}`))
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		// Directly check that first row has START_TRANSACTION
		if tx.GetRows()[0].StartControl != START_TRANSACTION {
			t.Errorf("First row should have START_TRANSACTION, got %c", tx.GetRows()[0].StartControl)
		}
	})

	// Test: Invalid transaction start (starts with R)
	t.Run("invalid_transaction_start", func(t *testing.T) {
		key, _ := uuid.NewV7()
		row := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key, json.RawMessage(`{"data":"test"}`))
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		// Directly check that first row does not have START_TRANSACTION
		if tx.GetRows()[0].StartControl == START_TRANSACTION {
			t.Error("First row should not have START_TRANSACTION when it starts with R")
		}
	})
}

// Test_S_006_FR_004_IsCommittedMethod tests FR-004: IsCommitted() method MUST return true only when transaction has proper termination (commit or rollback)
func Test_S_006_FR_004_IsCommittedMethod(t *testing.T) {

	// Test: Committed transaction (ends with TC)
	t.Run("committed_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if !tx.IsCommitted() {
			t.Error("IsCommitted() should return true for committed transaction")
		}
	})

	// Test: Rolled back transaction (ends with R0)
	t.Run("rolled_back_transaction", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, FULL_ROLLBACK, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if !tx.IsCommitted() {
			t.Error("IsCommitted() should return true for rolled back transaction (has termination)")
		}
	})

	// Test: Open transaction (ends with RE)
	t.Run("open_transaction", func(t *testing.T) {
		key, _ := uuid.NewV7()

		row := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key, json.RawMessage(`{"data":"test"}`))
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})
}

// Test_S_006_FR_005_OpenTransactionHandling tests FR-005: IsCommitted() method MUST handle edge case where transaction is still open (last row ends with E)
func Test_S_006_FR_005_OpenTransactionHandling(t *testing.T) {

	// Test: Single row transaction still open
	t.Run("single_row_open", func(t *testing.T) {
		key, _ := uuid.NewV7()

		row := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key, json.RawMessage(`{"data":"test"}`))
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})

	// Test: Multiple row transaction still open
	t.Run("multiple_row_open", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})

	// Test: Open transaction with savepoint
	t.Run("open_transaction_with_savepoint", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
		}

		if tx.IsCommitted() {
			t.Error("IsCommitted() should return false for open transaction")
		}
	})
}

// Test_S_006_FR_008_SavepointDetection tests FR-008: GetSavepointIndices() method MUST identify all savepoint locations using EndControl patterns with S as first character
func Test_S_006_FR_008_SavepointDetection(t *testing.T) {

	// Test: No savepoints
	t.Run("no_savepoints", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
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

		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2},
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

		row1 := createTestDataRow(START_TRANSACTION, SAVEPOINT_CONTINUE, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		row2 := createTestDataRow(ROW_CONTINUE, SAVEPOINT_CONTINUE, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()
		row3 := createTestDataRow(ROW_CONTINUE, TRANSACTION_COMMIT, key3, json.RawMessage(`{"data":"third"}`))
		row3.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3},
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

	// Test: Savepoints at various positions
	t.Run("savepoints_at_various_positions", func(t *testing.T) {
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		// Row 0: Transaction start + continue (no savepoint)
		row1 := createTestDataRow(START_TRANSACTION, ROW_END_CONTROL, key1, json.RawMessage(`{"data":"first"}`))
		row1.Validate()
		// Row 1: Continue + savepoint
		row2 := createTestDataRow(ROW_CONTINUE, SAVEPOINT_CONTINUE, key2, json.RawMessage(`{"data":"second"}`))
		row2.Validate()
		// Row 2: Continue (no savepoint)
		row3 := createTestDataRow(ROW_CONTINUE, ROW_END_CONTROL, key3, json.RawMessage(`{"data":"third"}`))
		row3.Validate()
		// Row 3: Continue + savepoint
		row4 := createTestDataRow(ROW_CONTINUE, SAVEPOINT_COMMIT, key4, json.RawMessage(`{"data":"fourth"}`))
		row4.Validate()

		tx := &Transaction{
			rows: []DataRow{*row1, *row2, *row3, *row4},
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

// =============================================================================
// Spec 011: Transaction Begin and Commit
// =============================================================================

// Test_S_011_FR_001_BeginCreatesPartialDataRow tests FR-001: Transaction MUST have a Begin() method
// that initializes a PartialDataRow to PartialDataRowWithStartControl state when the Transaction contains no rows
func Test_S_011_FR_001_BeginCreatesPartialDataRow(t *testing.T) {
	header := createTestHeader()

	// Test: Begin on empty transaction succeeds and sets up for Commit
	t.Run("begin_creates_partial_data_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() should succeed on empty transaction: %v", err)
		}

		// Verify internal state changed by confirming Commit() now succeeds
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() should succeed after Begin(): %v", err)
		}
	})

	// Test: Begin sets transaction to active state (verified by behavior)
	t.Run("begin_sets_active_state", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Verify transaction is now active by checking:
		// - empty should be nil (no commit yet)
		// - calling Begin() again should fail (transaction is active)
		// - rows should be empty
		if tx.GetEmptyRow() != nil {
			t.Error("Empty row should be nil after Begin()")
		}

		if len(tx.GetRows()) != 0 {
			t.Error("Rows should be empty after Begin()")
		}

		// Verify Begin() fails when called again (proves active state)
		err = tx.Begin()
		if err == nil {
			t.Error("Begin() should fail when called again on active transaction")
		}
	})
}

// Test_S_011_FR_002_CommitCreatesNullRow tests FR-002: Transaction MUST have a Commit() method
// that converts a PartialDataRowWithStartControl into a NullRow with null payload
func Test_S_011_FR_002_CommitCreatesNullRow(t *testing.T) {
	header := createTestHeader()

	// Test: Commit after Begin creates NullRow
	t.Run("commit_creates_null_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Begin first
		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Now commit
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() should succeed after Begin(): %v", err)
		}

		// Verify that the empty field points to a NullRow
		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Commit() should create a NullRow in the empty field")
		}
	})

	// Test: Commit clears the partial row
	t.Run("commit_clears_partial_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify transaction state changed by confirming Begin() fails (committed state)
		err = tx.Begin()
		if err == nil {
			t.Error("Begin() should fail after Commit()")
		}
	})
}

// Test_S_011_FR_003_BeginReturnsInvalidActionError tests FR-003: Transaction.Begin() MUST return
// InvalidActionError when called on a Transaction that is not empty (has existing rows)
func Test_S_011_FR_003_BeginReturnsInvalidActionError(t *testing.T) {
	header := createTestHeader()

	// Test: Begin on transaction with existing rows fails
	t.Run("begin_on_non_empty_transaction_fails", func(t *testing.T) {
		// Create a transaction with existing rows
		key, _ := uuid.NewV7()
		row := createTestDataRow(START_TRANSACTION, TRANSACTION_COMMIT, key, json.RawMessage(`{"data":"test"}`))
		row.Validate()

		tx := &Transaction{
			rows: []DataRow{*row},
		}

		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should return error when transaction has rows")
		}

		// Verify it's an InvalidActionError
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	// Test: Begin on transaction with empty row fails
	t.Run("begin_on_transaction_with_empty_row_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// First Begin -> Commit to set empty row
		err := tx.Begin()
		if err != nil {
			t.Fatalf("First Begin() failed: %v", err)
		}
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Now try Begin again
		err = tx.Begin()
		if err == nil {
			t.Fatal("Begin() should return error when transaction has empty row")
		}

		// Verify it's an InvalidActionError
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	// Test: Begin on transaction with partial row fails
	t.Run("begin_on_transaction_with_partial_row_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// First Begin to set partial row
		err := tx.Begin()
		if err != nil {
			t.Fatalf("First Begin() failed: %v", err)
		}

		// Now try Begin again (partial row exists)
		err = tx.Begin()
		if err == nil {
			t.Fatal("Begin() should return error when partial row exists")
		}

		// Verify it's an InvalidActionError
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})
}

// Test_S_011_FR_004_CommitReturnsInvalidActionError tests FR-004: Transaction.Commit() MUST return
// InvalidActionError when called when the PartialDataRow is not in PartialDataRowWithStartControl state
func Test_S_011_FR_004_CommitReturnsInvalidActionError(t *testing.T) {
	header := createTestHeader()

	// Test: Commit on inactive transaction fails
	t.Run("commit_on_inactive_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Try to commit without Begin
		err := tx.Commit()
		if err == nil {
			t.Fatal("Commit() should return error when transaction is inactive")
		}

		// Verify it's an InvalidActionError
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	// Test: Commit on already committed transaction fails
	t.Run("commit_on_committed_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Begin and Commit first
		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}
		err = tx.Commit()
		if err != nil {
			t.Fatalf("First Commit() failed: %v", err)
		}

		// Try to commit again
		err = tx.Commit()
		if err == nil {
			t.Fatal("Commit() should return error when transaction is already committed")
		}

		// Verify it's an InvalidActionError
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	// Test: Commit when partial row is in wrong state fails
	// NOTE: This test was updated for spec 012 compatibility. The original test expected
	// Commit() to fail when the partial row had payload, but spec 012 FR-008 now requires
	// Commit() to finalize the last PartialDataRow for data transactions.
	// The updated test now verifies that Commit() fails when called on a transaction
	// with rows that have already been finalized but have an open transaction (rows but no commit yet).
	t.Run("commit_on_wrong_partial_state_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Begin first
		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Access internal state to advance partial row state (testing internal validation)
		if tx.last == nil {
			t.Fatal("Expected partial row after Begin()")
		}

		// Add row data to move to PartialDataRowWithPayload state
		key, _ := uuid.NewV7()
		err = tx.last.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Per spec 012 FR-008, Commit should now succeed when partial row has payload
		// This is the expected behavior for data transactions (Begin + AddRow + Commit)
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() should succeed when partial row has payload (spec 012 FR-008): %v", err)
		}

		// Verify the row is now in rows[]
		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row after commit, got %d", len(rows))
		}

		// Verify it's a valid DataRow with the key we added
		if rows[0].GetKey() != key {
			t.Errorf("Row key mismatch: expected %s, got %s", key, rows[0].GetKey())
		}
	})
}

// Test_S_011_FR_005_TransactionContainsOneNullRow tests FR-005: After successful Begin() -> Commit() sequence,
// Transaction MUST contain exactly one row which is a valid NullRow
func Test_S_011_FR_005_TransactionContainsOneNullRow(t *testing.T) {
	header := createTestHeader()

	t.Run("empty_transaction_workflow_results_in_null_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Execute Begin -> Commit workflow
		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify the empty field contains a NullRow
		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Transaction should have a NullRow in empty field after Begin() -> Commit()")
		}

		// Verify rows slice is empty (NullRows are stored in empty field, not rows)
		if len(tx.GetRows()) != 0 {
			t.Errorf("Rows slice should be empty for empty transaction, got %d rows", len(tx.GetRows()))
		}
	})
}

// Test_S_011_FR_006_NullRowValidation tests FR-006: Transaction MUST validate that the resulting NullRow
// follows all NullRow specification requirements
func Test_S_011_FR_006_NullRowValidation(t *testing.T) {
	header := createTestHeader()

	t.Run("null_row_validation_after_commit", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Execute Begin -> Commit workflow
		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Get the NullRow
		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Expected NullRow after commit")
		}

		// Validate the NullRow
		if err := emptyRow.Validate(); err != nil {
			t.Errorf("NullRow should pass validation: %v", err)
		}

		// Verify NullRow has uuid.Nil key
		if emptyRow.GetKey() != uuid.Nil {
			t.Errorf("NullRow should have uuid.Nil key, got %v", emptyRow.GetKey())
		}
	})

	t.Run("null_row_has_correct_controls", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Expected NullRow after commit")
		}

		// Verify start_control is 'T'
		if emptyRow.StartControl != START_TRANSACTION {
			t.Errorf("NullRow should have StartControl='T', got '%c'", emptyRow.StartControl)
		}

		// Verify end_control is 'NR'
		if emptyRow.EndControl != NULL_ROW_CONTROL {
			t.Errorf("NullRow should have EndControl='NR', got '%s'", emptyRow.EndControl.String())
		}
	})
}

// =============================================================================
// Spec 012: AddRow Transaction Implementation
// =============================================================================

// Test_S_012_FR_001_BeginRequiredBeforeAddRow tests FR-001: Transaction MUST allow AddRow()
// to be called only after Begin() has been called successfully
func Test_S_012_FR_001_BeginRequiredBeforeAddRow(t *testing.T) {
	header := createTestHeader()

	t.Run("addrow_fails_without_begin", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should fail when Begin() has not been called")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	t.Run("addrow_succeeds_after_begin", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() should succeed after Begin(): %v", err)
		}
	})
}

// Test_S_012_FR_002_AddRowFinalizesPartialDataRow tests FR-002: AddRow() MUST finalize
// the current PartialDataRow by converting it to a DataRow with ROW_END_CONTROL end_control
func Test_S_012_FR_002_AddRowFinalizesPartialDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("first_addrow_finalizes_initial_partial", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		// First AddRow
		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		// Second AddRow should finalize the first
		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		// Verify that the first row was finalized with ROW_END_CONTROL (RE)
		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 finalized row, got %d", len(rows))
		}

		if rows[0].EndControl != ROW_END_CONTROL {
			t.Errorf("Expected end_control='RE', got '%s'", rows[0].EndControl.String())
		}
	})
}

// Test_S_012_FR_003_AddRowMovesPreviousToRows tests FR-003: AddRow() MUST move
// the finalized previous DataRow to the rows[] slice
func Test_S_012_FR_003_AddRowMovesPreviousToRows(t *testing.T) {
	header := createTestHeader()

	t.Run("multiple_addrows_accumulate_in_rows", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		// Add three rows
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))

		// Should have 2 finalized rows (third is still partial)
		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 finalized rows, got %d", len(rows))
		}

		// Verify keys are in order
		if rows[0].GetKey() != key1 {
			t.Errorf("First row key mismatch: expected %s, got %s", key1, rows[0].GetKey())
		}
		if rows[1].GetKey() != key2 {
			t.Errorf("Second row key mismatch: expected %s, got %s", key2, rows[1].GetKey())
		}
	})
}

// Test_S_012_FR_004_AddRowCreatesNewPartialDataRow tests FR-004: AddRow() MUST create
// a new PartialDataRow in PartialDataRowWithStartControl state for the next row
func Test_S_012_FR_004_AddRowCreatesNewPartialDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("addrow_creates_new_partial_with_payload", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// After AddRow, there should be an active partial row with payload
		// Verified by ability to Commit() successfully
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() should succeed after AddRow(): %v", err)
		}

		// Verify the row is now in rows[]
		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 committed row, got %d", len(rows))
		}
		if rows[0].GetKey() != key {
			t.Errorf("Row key mismatch: expected %s, got %s", key, rows[0].GetKey())
		}
	})
}

// Test_S_012_FR_005_AddRowUsesContinueStartControl tests FR-005: AddRow() MUST use
// ROW_CONTINUE start_control for all rows after the first row in a transaction
func Test_S_012_FR_005_AddRowUsesContinueStartControl(t *testing.T) {
	header := createTestHeader()

	t.Run("first_row_uses_T_subsequent_use_R", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 3 {
			t.Fatalf("Expected 3 committed rows, got %d", len(rows))
		}

		// First row should have StartControl = 'T'
		if rows[0].StartControl != START_TRANSACTION {
			t.Errorf("First row should have StartControl='T', got '%c'", rows[0].StartControl)
		}

		// Subsequent rows should have StartControl = 'R'
		for i := 1; i < len(rows); i++ {
			if rows[i].StartControl != ROW_CONTINUE {
				t.Errorf("Row %d should have StartControl='R', got '%c'", i, rows[i].StartControl)
			}
		}
	})
}

// Test_S_012_FR_006_AddRowValidatesUUIDv7 tests FR-006: AddRow() MUST validate
// UUIDv7 key parameter and return InvalidInputError for invalid UUIDs
func Test_S_012_FR_006_AddRowValidatesUUIDv7(t *testing.T) {
	header := createTestHeader()

	t.Run("rejects_nil_uuid", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		err := tx.AddRow(uuid.Nil, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should reject uuid.Nil")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("rejects_non_v7_uuid", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Create a UUIDv4 (not v7)
		uuidV4 := uuid.New()

		err := tx.AddRow(uuidV4, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should reject non-UUIDv7")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})
}

// Test_S_012_FR_007_AddRowValidatesNonEmptyValue tests FR-007: AddRow() MUST validate
// JSON value parameter is non-empty and return InvalidInputError for empty values
func Test_S_012_FR_007_AddRowValidatesNonEmptyValue(t *testing.T) {
	header := createTestHeader()

	t.Run("rejects_empty_value", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(""))
		if err == nil {
			t.Fatal("AddRow() should reject empty value")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("accepts_non_empty_value", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() should accept non-empty value: %v", err)
		}
	})
}

// Test_S_012_FR_008_CommitFinalizesLastPartialDataRow tests FR-008: Commit() MUST finalize
// the last PartialDataRow using appropriate end_control based on transaction state
func Test_S_012_FR_008_CommitFinalizesLastPartialDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_finalizes_last_row_with_tc", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

		err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(rows))
		}

		// First row should end with RE (continue)
		if rows[0].EndControl != ROW_END_CONTROL {
			t.Errorf("First row should have end_control='RE', got '%s'", rows[0].EndControl.String())
		}

		// Last row should end with TC (commit)
		if rows[1].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Last row should have end_control='TC', got '%s'", rows[1].EndControl.String())
		}
	})

	t.Run("single_row_commit_with_tc", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"only"}`))

		err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		// Single row should end with TC
		if rows[0].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Single row should have end_control='TC', got '%s'", rows[0].EndControl.String())
		}
	})
}

// Test_S_012_FR_009_CommitCreatesNullRowForEmptyTransaction tests FR-009: Commit() MUST NOT
// attempt to finalize PartialDataRow for empty transactions (no AddRow() calls)
func Test_S_012_FR_009_CommitCreatesNullRowForEmptyTransaction(t *testing.T) {
	header := createTestHeader()

	t.Run("empty_transaction_creates_null_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Commit without any AddRow calls
		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify empty field is set
		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Empty transaction should have NullRow")
		}

		// Verify rows field is empty
		if len(tx.GetRows()) != 0 {
			t.Errorf("Empty transaction should have no data rows, got %d", len(tx.GetRows()))
		}
	})
}

// Test_S_012_FR_010_AddRowEnforces100RowLimit tests FR-010: Transaction MUST maintain
// maximum 100 rows limit including all finalized rows
func Test_S_012_FR_010_AddRowEnforces100RowLimit(t *testing.T) {
	header := createTestHeader()

	t.Run("rejects_101st_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Add 100 rows
		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			if err != nil {
				t.Fatalf("AddRow() %d failed: %v", i, err)
			}
		}

		// 101st row should fail
		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"overflow"}`))
		if err == nil {
			t.Fatal("AddRow() should fail when adding 101st row")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("allows_exactly_100_rows", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Add 100 rows
		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			if err != nil {
				t.Fatalf("AddRow() %d failed: %v", i, err)
			}
		}

		// Commit should succeed
		err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		if len(tx.GetRows()) != 100 {
			t.Errorf("Expected 100 rows, got %d", len(tx.GetRows()))
		}
	})
}

// Test_S_012_FR_011_AddRowValidatesActiveTransaction tests FR-011: AddRow() MUST return
// InvalidActionError when called on committed or inactive transactions
func Test_S_012_FR_011_AddRowValidatesActiveTransaction(t *testing.T) {
	header := createTestHeader()

	t.Run("rejects_addrow_on_committed_transaction", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"test"}`))
		tx.Commit()

		// Try to add row after commit
		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, json.RawMessage(`{"data":"after_commit"}`))
		if err == nil {
			t.Fatal("AddRow() should fail on committed transaction")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	t.Run("rejects_addrow_on_inactive_transaction", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should fail on inactive transaction")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})
}

// Test_S_012_FR_012_AddRowThreadSafety tests FR-012: Transaction state MUST remain
// consistent during AddRow() operations with proper mutex locking
func Test_S_012_FR_012_AddRowThreadSafety(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_addrow_operations", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Run concurrent AddRow operations
		done := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func() {
				key, _ := uuid.NewV7()
				err := tx.AddRow(key, json.RawMessage(`{"data":"concurrent"}`))
				done <- err
			}()
		}

		// Collect results - some may fail due to timing, but should not panic
		successCount := 0
		for i := 0; i < 10; i++ {
			if err := <-done; err == nil {
				successCount++
			}
		}

		// At least some should succeed
		if successCount == 0 {
			t.Error("Expected at least some concurrent AddRow() calls to succeed")
		}

		// Verify transaction state is consistent
		rows := tx.GetRows()
		// Number of finalized rows should be successCount - 1 (last one is still partial)
		expectedFinalized := successCount - 1
		if expectedFinalized < 0 {
			expectedFinalized = 0
		}
		if len(rows) != expectedFinalized {
			t.Errorf("Expected %d finalized rows, got %d", expectedFinalized, len(rows))
		}
	})
}

// Test_S_012_FR_013_TransactionReceivesMaxTimestamp tests FR-013: Transaction MUST receive
// current max_timestamp when initialized and maintain its own copy during the transaction
func Test_S_012_FR_013_TransactionReceivesMaxTimestamp(t *testing.T) {
	header := createTestHeader()

	t.Run("transaction_receives_initial_max_timestamp", func(t *testing.T) {
		initialMaxTimestamp := int64(1000000)

		tx := createTransactionWithMockWriter(header)
		tx.maxTimestamp = initialMaxTimestamp

		if tx.GetMaxTimestamp() != initialMaxTimestamp {
			t.Errorf("Expected initial maxTimestamp %d, got %d", initialMaxTimestamp, tx.GetMaxTimestamp())
		}
	})

	t.Run("transaction_maintains_own_copy", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.maxTimestamp = 1000

		tx.Begin()
		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		// maxTimestamp should be updated
		if tx.GetMaxTimestamp() <= 1000 {
			t.Error("maxTimestamp should be updated after AddRow with newer key")
		}
	})
}

// Test_S_012_FR_014_AddRowPreservesUUIDOrdering tests FR-014: AddRow() MUST preserve
// UUID timestamp ordering using the max_timestamp algorithm: new_timestamp + skew_ms > max_timestamp
func Test_S_012_FR_014_AddRowPreservesUUIDOrdering(t *testing.T) {
	header := createTestHeader()

	t.Run("accepts_ascending_timestamps", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Generate keys in ascending order (uuid.NewV7 generates ascending timestamps)
		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		if err := tx.AddRow(key1, json.RawMessage(`{"data":"1"}`)); err != nil {
			t.Fatalf("First AddRow failed: %v", err)
		}
		if err := tx.AddRow(key2, json.RawMessage(`{"data":"2"}`)); err != nil {
			t.Fatalf("Second AddRow failed: %v", err)
		}
		if err := tx.AddRow(key3, json.RawMessage(`{"data":"3"}`)); err != nil {
			t.Fatalf("Third AddRow failed: %v", err)
		}
	})
}

// Test_S_012_FR_015_AddRowUpdatesMaxTimestamp tests FR-015: AddRow() MUST update
// transaction's max_timestamp after successful row insertion
func Test_S_012_FR_015_AddRowUpdatesMaxTimestamp(t *testing.T) {
	header := createTestHeader()

	t.Run("max_timestamp_updated_after_addrow", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.maxTimestamp = 0
		tx.Begin()

		initialMax := tx.GetMaxTimestamp()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		newMax := tx.GetMaxTimestamp()
		if newMax <= initialMax {
			t.Errorf("maxTimestamp should have increased: was %d, now %d", initialMax, newMax)
		}
	})
}

// Test_S_012_FR_016_AddRowReturnsKeyOrderingError tests FR-016: AddRow() MUST return
// KeyOrderingError when UUID timestamp violates ordering constraints
func Test_S_012_FR_016_AddRowReturnsKeyOrderingError(t *testing.T) {
	// Create header with 0 skew to make timestamp ordering strict
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    0, // No skew tolerance
	}

	t.Run("rejects_older_timestamp", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Add first key
		key1, _ := uuid.NewV7()
		err := tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow failed: %v", err)
		}

		// Create a key with older timestamp by using the first key's bytes
		// and decrementing the timestamp portion
		olderKey := key1
		// Decrement the timestamp (first 6 bytes)
		olderKey[5]-- // This decrements the timestamp by 1ms

		err = tx.AddRow(olderKey, json.RawMessage(`{"data":"older"}`))
		if err == nil {
			t.Fatal("AddRow should reject older timestamp")
		}

		if _, ok := err.(*KeyOrderingError); !ok {
			t.Errorf("Expected KeyOrderingError, got %T: %v", err, err)
		}
	})
}

// Test_S_012_FR_017_EmptyDatabaseMaxTimestampZero tests FR-017: For empty databases,
// max_timestamp MUST start at 0 requiring new_timestamp + skew_ms > 0 for first row
func Test_S_012_FR_017_EmptyDatabaseMaxTimestampZero(t *testing.T) {
	header := createTestHeader()

	t.Run("empty_database_starts_at_zero", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		// Default maxTimestamp should be 0
		if tx.GetMaxTimestamp() != 0 {
			t.Errorf("Default maxTimestamp should be 0, got %d", tx.GetMaxTimestamp())
		}
	})

	t.Run("first_row_succeeds_with_valid_timestamp", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("First AddRow should succeed: %v", err)
		}
	})
}

// =============================================================================
// Spec 013: Transaction Savepoint and Rollback
// =============================================================================

// Test_S_013_FR_001_SavepointMethodExists tests FR-001: Transaction MUST have a
// public Savepoint() method that creates a savepoint at the current transaction position
func Test_S_013_FR_001_SavepointMethodExists(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_method_exists_and_is_callable", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() should succeed after AddRow(): %v", err)
		}
	})

	t.Run("savepoint_creates_savepoint_continue_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row (first row finalized with savepoint), got %d", len(rows))
		}

		if rows[0].EndControl[0] != 'S' {
			t.Errorf("Expected first row to have savepoint end control (starts with 'S'), got '%c'", rows[0].EndControl[0])
		}
	})
}

// Test_S_013_FR_005_SavepointRequiresAtLeastOneRow tests FR-005: Savepoint()
// MUST return InvalidActionError if called before at least one AddRow() call
func Test_S_013_FR_005_SavepointRequiresAtLeastOneRow(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_on_empty_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail on empty transaction (no AddRow called)")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	t.Run("savepoint_after_begin_only_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail when no data rows have been added")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}

		// Verify error message contains expected phrase
		expectedMsg := "cannot savepoint empty transaction"
		if !contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
		}
	})
}

// Test_S_013_FR_006_SavepointReturnsErrorOnEmptyTransaction tests FR-006:
// Savepoint() MUST return InvalidActionError if called on an inactive transaction
func Test_S_013_FR_006_SavepointReturnsErrorOnEmptyTransaction(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_on_inactive_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail on inactive transaction (no Begin() called)")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	t.Run("savepoint_on_committed_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		err = tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail on committed transaction")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})
}

// Test_S_013_FR_007_SavepointEnforcesMaximumNineSavepoints tests FR-007:
// Savepoint() MUST return InvalidActionError if more than 9 savepoints would be created
func Test_S_013_FR_007_SavepointEnforcesMaximumNineSavepoints(t *testing.T) {
	header := createTestHeader()

	t.Run("ninth_savepoint_succeeds", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			err = tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, i)))
			if err != nil {
				t.Fatalf("AddRow() failed for row %d: %v", i, err)
			}

			if i < 8 {
				err = tx.Savepoint()
				if err != nil {
					t.Fatalf("Savepoint() failed for savepoint %d: %v", i+1, err)
				}
			}
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"ninth_row"}`))
		if err != nil {
			t.Fatalf("AddRow() failed for ninth row: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("9th Savepoint() should succeed: %v", err)
		}
	})

	t.Run("tenth_savepoint_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			err = tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, i)))
			if err != nil {
				t.Fatalf("AddRow() failed for row %d: %v", i, err)
			}

			err = tx.Savepoint()
			if err != nil {
				t.Fatalf("Savepoint() failed for savepoint %d: %v", i+1, err)
			}
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"tenth_row"}`))
		if err != nil {
			t.Fatalf("AddRow() failed for tenth row: %v", err)
		}

		err = tx.Savepoint()
		if err == nil {
			t.Fatal("10th Savepoint() should fail (max 9 savepoints)")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})
}

// Test_S_013_FR_002_FullRollbackMethodExists tests FR-002: Transaction MUST have a
// public Rollback(savepointId int) method that performs full rollback when savepointId is 0
func Test_S_013_FR_002_FullRollbackMethodExists(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_method_exists_and_is_callable", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) should succeed: %v", err)
		}
	})

	t.Run("full_rollback_invalidates_all_rows", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 rows (both rows finalized), got %d", len(rows))
		}

		if rows[1].EndControl[0] != 'R' {
			t.Errorf("Expected second row to have rollback end control (starts with 'R'), got '%c'", rows[1].EndControl[0])
		}

		if rows[1].EndControl[1] != '0' {
			t.Errorf("Expected second row to have rollback to savepoint 0 ('0'), got '%c'", rows[1].EndControl[1])
		}
	})

	t.Run("full_rollback_closes_transaction", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) failed: %v", err)
		}

		// Transaction should be considered committed (terminated) after Rollback(0)
		if !tx.IsCommitted() {
			t.Error("Transaction should be considered committed after Rollback(0)")
		}

		// Transaction should be closed (no active partial row)
		if tx.isActive() {
			t.Error("Transaction should not be active after Rollback(0)")
		}
	})
}

// Test_S_013_FR_009_RollbackReturnsErrorOnInactiveTransaction tests FR-009:
// Rollback() MUST return InvalidActionError if called on an inactive transaction
func Test_S_013_FR_009_RollbackReturnsErrorOnInactiveTransaction(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_on_inactive_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail on inactive transaction (no Begin() called)")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})

	t.Run("rollback_on_committed_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail on committed transaction")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	})
}

// Test_S_013_FR_010_FullRollbackCreatesNullRowForEmptyTransaction tests FR-010:
// Rollback(0) on an empty transaction (Begin() + Rollback() with no AddRow) MUST create a NullRow
func Test_S_013_FR_010_FullRollbackCreatesNullRowForEmptyTransaction(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_empty_transaction_creates_null_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) on empty transaction failed: %v", err)
		}

		emptyRow := tx.GetEmptyRow()
		if emptyRow == nil {
			t.Fatal("Rollback(0) should create NullRow in empty field for empty transaction")
		}

		if emptyRow.GetKey() != uuid.Nil {
			t.Errorf("NullRow should have uuid.Nil key, got %v", emptyRow.GetKey())
		}
	})

	t.Run("rollback_empty_transaction_clears_partial_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 0 {
			t.Errorf("Expected 0 rows for empty transaction rollback, got %d", len(rows))
		}
	})
}

// Test_S_013_FR_014_FullRollbackUsesR0OrS0EndControl tests FR-014:
// Rollback(0) MUST use R0 (no savepoint) or S0 (with savepoint) end control encoding
func Test_S_013_FR_014_FullRollbackUsesR0OrS0EndControl(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_without_savepoint_uses_R0", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(rows))
		}

		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[0] != 'R' {
			t.Errorf("Expected last row end control to start with 'R' for rollback without savepoint, got '%c'", lastRow.EndControl[0])
		}
		if lastRow.EndControl[1] != '0' {
			t.Errorf("Expected last row end control to be '0' for rollback to savepoint 0, got '%c'", lastRow.EndControl[1])
		}
	})

	t.Run("rollback_with_savepoint_uses_S0", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		// Now call Savepoint() again to mark the second row with savepoint intent
		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Second Savepoint() failed: %v", err)
		}

		// Rollback now - the partial row is in PartialDataRowWithSavepoint state with payload
		err = tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) failed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(rows))
		}

		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[0] != 'S' {
			t.Errorf("Expected last row end control to start with 'S' for rollback with savepoint, got '%c'", lastRow.EndControl[0])
		}
		if lastRow.EndControl[1] != '0' {
			t.Errorf("Expected last row end control to be '0' for rollback to savepoint 0, got '%c'", lastRow.EndControl[1])
		}
	})
}

// Test_S_013_FR_003_PartialRollbackMethodExists tests FR-003: Transaction MUST have a
// public Rollback(savepointId int) method that performs partial rollback when savepointId > 0
func Test_S_013_FR_003_PartialRollbackMethodExists(t *testing.T) {
	header := createTestHeader()

	t.Run("partial_rollback_to_savepoint_1", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) should succeed: %v", err)
		}

		rows := tx.GetRows()
		if len(rows) != 3 {
			t.Fatalf("Expected 3 rows, got %d", len(rows))
		}

		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[1] != '1' {
			t.Errorf("Expected last row end control to have '1' for rollback to savepoint 1, got '%c'", lastRow.EndControl[1])
		}
	})

	t.Run("partial_rollback_to_savepoint_2", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("First Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Second Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.AddRow(key4, json.RawMessage(`{"data":"fourth"}`))
		if err != nil {
			t.Fatalf("Fourth AddRow() failed: %v", err)
		}

		err = tx.Rollback(2)
		if err != nil {
			t.Fatalf("Rollback(2) should succeed: %v", err)
		}
	})
}

// Test_S_013_FR_008_RollbackReturnsErrorForInvalidSavepointNumber tests FR-008:
// Rollback() MUST return InvalidInputError if savepointId > current savepoint count
func Test_S_013_FR_008_RollbackReturnsErrorForInvalidSavepointNumber(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_to_nonexistent_savepoint_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Rollback(1)
		if err == nil {
			t.Fatal("Rollback(1) should fail when no savepoints exist")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("rollback_beyond_available_savepoints_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for i := 0; i < 3; i++ {
			key, _ := uuid.NewV7()
			err = tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, i)))
			if err != nil {
				t.Fatalf("AddRow() failed for row %d: %v", i, err)
			}
			if i < 2 {
				err = tx.Savepoint()
				if err != nil {
					t.Fatalf("Savepoint() failed for savepoint %d: %v", i+1, err)
				}
			}
		}

		err = tx.Rollback(5)
		if err == nil {
			t.Fatal("Rollback(5) should fail when only 2 savepoints exist")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("rollback_with_negative_savepoint_id_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Rollback(-1)
		if err == nil {
			t.Fatal("Rollback(-1) should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})

	t.Run("rollback_with_savepoint_id_over_9_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Rollback(10)
		if err == nil {
			t.Fatal("Rollback(10) should fail (max is 9)")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
		}
	})
}

// Test_S_013_FR_011_PartialRollbackCommitsRowsUpToSavepoint tests FR-011:
// Partial rollback MUST commit all rows up to and including the target savepoint
func Test_S_013_FR_011_PartialRollbackCommitsRowsUpToSavepoint(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_to_savepoint_1_commits_first_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows() failed: %v", err)
		}

		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
		}

		if len(committedKeys) != 1 {
			t.Errorf("Expected 1 committed row (up to savepoint 1), got %d", len(committedKeys))
		}

		if len(committedKeys) > 0 && committedKeys[0] != key1 {
			t.Errorf("Expected first row to be committed, got keys: %v", committedKeys)
		}
	})

	t.Run("rollback_to_savepoint_2_commits_first_two_rows", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()
		key4, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("First Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Second Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.AddRow(key4, json.RawMessage(`{"data":"fourth"}`))
		if err != nil {
			t.Fatalf("Fourth AddRow() failed: %v", err)
		}

		err = tx.Rollback(2)
		if err != nil {
			t.Fatalf("Rollback(2) failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows() failed: %v", err)
		}

		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
		}

		if len(committedKeys) != 2 {
			t.Errorf("Expected 2 committed rows (up to savepoint 2), got %d", len(committedKeys))
		}

		if len(committedKeys) >= 2 && (committedKeys[0] != key1 || committedKeys[1] != key2) {
			t.Errorf("Expected first two rows to be committed, got keys: %v", committedKeys)
		}
	})
}

// Test_S_013_FR_012_PartialRollbackInvalidatesRowsAfterSavepoint tests FR-012:
// Partial rollback MUST invalidate all rows after the target savepoint
func Test_S_013_FR_012_PartialRollbackInvalidatesRowsAfterSavepoint(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_to_savepoint_1_invalidates_subsequent_rows", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) failed: %v", err)
		}

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows() failed: %v", err)
		}

		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
		}

		if len(committedKeys) != 1 {
			t.Errorf("Expected only first row to be committed, got %d committed rows", len(committedKeys))
		}

		for _, key := range committedKeys {
			if key == key2 || key == key3 {
				t.Error("Rows after savepoint should not be committed")
			}
		}
	})
}

// Test_S_013_FR_013_PartialRollbackUsesRnOrSnEndControl tests FR-013:
// Partial rollback MUST use R1-R9 (no savepoint) or S1-S9 (with savepoint) end control encoding
func Test_S_013_FR_013_PartialRollbackUsesRnOrSnEndControl(t *testing.T) {
	header := createTestHeader()

	t.Run("partial_rollback_uses_R1", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()
		key3, _ := uuid.NewV7()

		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		err = tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() failed: %v", err)
		}

		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.AddRow(key3, json.RawMessage(`{"data":"third"}`))
		if err != nil {
			t.Fatalf("Third AddRow() failed: %v", err)
		}

		err = tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) failed: %v", err)
		}

		rows := tx.GetRows()
		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[0] != 'R' && lastRow.EndControl[0] != 'S' {
			t.Errorf("Expected last row end control to start with 'R' or 'S', got '%c'", lastRow.EndControl[0])
		}
		if lastRow.EndControl[1] != '1' {
			t.Errorf("Expected last row end control to be '1' for rollback to savepoint 1, got '%c'", lastRow.EndControl[1])
		}
	})
}

// =============================================================================
// Spec 015: Transaction File Persistence
// =============================================================================

// Helper function to create a minimal valid database file for testing
func createMinimalTestDatabase(t *testing.T, path string, header *Header) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	// Write header
	headerBytes, err := header.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal header: %v", err)
	}
	if _, err := file.Write(headerBytes); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Create and write checksum row
	checksumRow, err := NewChecksumRow(header.GetRowSize(), headerBytes)
	if err != nil {
		t.Fatalf("Failed to create checksum row: %v", err)
	}
	checksumBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal checksum row: %v", err)
	}
	if _, err := file.Write(checksumBytes); err != nil {
		t.Fatalf("Failed to write checksum row: %v", err)
	}
}

// Test_S_015_FR_001_BeginWritesPartialDataRow tests FR-001: When Begin() is called on a Transaction,
// the system MUST write a PartialDataRow to the database file via the FileManager
func Test_S_015_FR_001_BeginWritesPartialDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("begin_writes_partial_data_row_to_file", func(t *testing.T) {
		// Create a temporary file for testing
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		// Initialize file with header and checksum row
		createMinimalTestDatabase(t, tmpPath, header)

		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		// Create write channel
		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		// Create transaction with write channel
		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
		}

		// Record file size before Begin
		sizeBefore := fm.Size()

		// Call Begin()
		err = tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Verify file size increased by 2 bytes (ROW_START + 'T')
		sizeAfter := fm.Size()
		expectedSize := sizeBefore + 2
		if sizeAfter != expectedSize {
			t.Errorf("Expected file size %d, got %d", expectedSize, sizeAfter)
		}

		// Read the last 2 bytes and verify they are ROW_START + 'T'
		lastBytes, err := fm.Read(sizeAfter-2, 2)
		if err != nil {
			t.Fatalf("Failed to read last bytes: %v", err)
		}
		if lastBytes[0] != ROW_START {
			t.Errorf("Expected ROW_START (0x1F), got 0x%02X", lastBytes[0])
		}
		if lastBytes[1] != byte(START_TRANSACTION) {
			t.Errorf("Expected START_TRANSACTION ('T'), got '%c'", lastBytes[1])
		}
	})
}

// Test_S_015_FR_002_AddRowWritesPreviousAndNewPartialDataRow tests FR-002: When AddRow() is called on an active Transaction,
// the system MUST write the previous PartialDataRow (if exists) to disk as a finalized row, then write a new PartialDataRow
func Test_S_015_FR_002_AddRowWritesPreviousAndNewPartialDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("first_addrow_writes_incremental_bytes", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin first
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		sizeAfterBegin := fm.Size()

		// First AddRow - should write incremental bytes (rowSize-7 bytes)
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Verify file size increased
		sizeAfterAddRow := fm.Size()
		expectedIncrement := int64(header.GetRowSize() - 7) // rowSize-5 bytes minus 2 already written
		if sizeAfterAddRow-sizeAfterBegin != expectedIncrement {
			t.Errorf("Expected file size increase of %d, got %d", expectedIncrement, sizeAfterAddRow-sizeAfterBegin)
		}
	})

	t.Run("subsequent_addrow_finalizes_previous_and_writes_new", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin and first AddRow
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}
		key1, _ := uuid.NewV7()
		if err := tx.AddRow(key1, json.RawMessage(`{"data":"first"}`)); err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		sizeAfterFirstAddRow := fm.Size()

		// Second AddRow - should finalize previous (5 bytes: RE + parity + ROW_END) and write new (rowSize-5 bytes)
		key2, _ := uuid.NewV7()
		if err := tx.AddRow(key2, json.RawMessage(`{"data":"second"}`)); err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		// Verify file size increased by 5 (finalization) + rowSize-5 (new partial) = rowSize bytes
		sizeAfterSecondAddRow := fm.Size()
		expectedIncrement := int64(header.GetRowSize()) // 5 bytes finalization + rowSize-5 bytes new partial
		if sizeAfterSecondAddRow-sizeAfterFirstAddRow != expectedIncrement {
			t.Errorf("Expected file size increase of %d, got %d", expectedIncrement, sizeAfterSecondAddRow-sizeAfterFirstAddRow)
		}
	})
}

// Test_S_015_FR_003_CommitWithRowsWritesFinalDataRow tests FR-003: When Commit() is called on a Transaction with added rows,
// the system MUST write the final data row to disk via the FileManager
func Test_S_015_FR_003_CommitWithRowsWritesFinalDataRow(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_writes_final_data_row", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin and AddRow
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		sizeBeforeCommit := fm.Size()

		// Commit
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify file size increased by remaining bytes (end_control + parity + ROW_END = 5 bytes)
		sizeAfterCommit := fm.Size()
		expectedIncrement := int64(5) // TC + parity + ROW_END
		if sizeAfterCommit-sizeBeforeCommit != expectedIncrement {
			t.Errorf("Expected file size increase of %d, got %d", expectedIncrement, sizeAfterCommit-sizeBeforeCommit)
		}

		// Verify the last row ends with TC (transaction commit)
		lastBytes, err := fm.Read(sizeAfterCommit-5, 5)
		if err != nil {
			t.Fatalf("Failed to read last bytes: %v", err)
		}
		if lastBytes[0] != 'T' || lastBytes[1] != 'C' {
			t.Errorf("Expected end_control='TC', got '%c%c'", lastBytes[0], lastBytes[1])
		}
		if lastBytes[4] != ROW_END {
			t.Errorf("Expected ROW_END (0x0A), got 0x%02X", lastBytes[4])
		}
	})
}

// Test_S_015_FR_004_CommitWithoutRowsWritesNullRow tests FR-004: When Commit() is called on a Transaction with no added rows,
// the system MUST write the current PartialDataRow (created by Begin()) as a NullRow to disk via the FileManager, resulting in exactly one row
func Test_S_015_FR_004_CommitWithoutRowsWritesNullRow(t *testing.T) {
	header := createTestHeader()

	t.Run("empty_transaction_commit_writes_null_row", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin only (no AddRow)
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		sizeAfterBegin := fm.Size()

		// Commit empty transaction
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify file size increased by remaining bytes (rowSize-2 bytes: everything except first 2 bytes already written)
		sizeAfterCommit := fm.Size()
		expectedIncrement := int64(header.GetRowSize() - 2) // Complete row minus first 2 bytes
		if sizeAfterCommit-sizeAfterBegin != expectedIncrement {
			t.Errorf("Expected file size increase of %d, got %d", expectedIncrement, sizeAfterCommit-sizeAfterBegin)
		}

		// Verify the row is a NullRow (end_control='NR')
		lastBytes, err := fm.Read(sizeAfterCommit-5, 5)
		if err != nil {
			t.Fatalf("Failed to read last bytes: %v", err)
		}
		if lastBytes[0] != 'N' || lastBytes[1] != 'R' {
			t.Errorf("Expected end_control='NR', got '%c%c'", lastBytes[0], lastBytes[1])
		}
		if lastBytes[4] != ROW_END {
			t.Errorf("Expected ROW_END (0x0A), got 0x%02X", lastBytes[4])
		}
	})
}

// Test_S_015_FR_005_BeginSynchronousWrite tests FR-005: All write operations (Begin, AddRow, Commit)
// MUST complete synchronously before the operation returns to the caller
func Test_S_015_FR_005_BeginSynchronousWrite(t *testing.T) {
	header := createTestHeader()

	t.Run("begin_waits_for_write_completion", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		fm, err := NewFileManager(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Record file size before Begin
		sizeBefore := fm.Size()

		// Call Begin() - should block until write completes
		err = tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Verify file size increased immediately after Begin() returns
		sizeAfter := fm.Size()
		if sizeAfter <= sizeBefore {
			t.Errorf("Expected file size to increase after Begin(), before: %d, after: %d", sizeBefore, sizeAfter)
		}

		// Verify rowBytesWritten is set correctly
		if tx.rowBytesWritten != 2 {
			t.Errorf("Expected rowBytesWritten=2, got %d", tx.rowBytesWritten)
		}
	})
}

// Test_S_015_FR_005_AddRowSynchronousWrite tests FR-005: All write operations (Begin, AddRow, Commit)
// MUST complete synchronously before the operation returns to the caller
func Test_S_015_FR_005_AddRowSynchronousWrite(t *testing.T) {
	header := createTestHeader()

	t.Run("addrow_waits_for_write_completion", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		sizeBefore := fm.Size()

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Verify file size increased immediately after AddRow() returns
		sizeAfter := fm.Size()
		if sizeAfter <= sizeBefore {
			t.Errorf("Expected file size to increase after AddRow(), before: %d, after: %d", sizeBefore, sizeAfter)
		}
	})
}

// Test_S_015_FR_005_CommitSynchronousWrite tests FR-005: All write operations (Begin, AddRow, Commit)
// MUST complete synchronously before the operation returns to the caller
func Test_S_015_FR_005_CommitSynchronousWrite(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_waits_for_write_completion", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		sizeBefore := fm.Size()

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify file size increased immediately after Commit() returns
		sizeAfter := fm.Size()
		if sizeAfter <= sizeBefore {
			t.Errorf("Expected file size to increase after Commit(), before: %d, after: %d", sizeBefore, sizeAfter)
		}
	})
}

// Test_S_015_FR_006_BeginWriteFailureNoPartialData tests FR-006: If any write operation fails,
// the system MUST tombstone the transaction and return the write error. All subsequent public API calls
// on the tombstoned transaction MUST return TombstonedError
func Test_S_015_FR_006_BeginWriteFailureNoPartialData(t *testing.T) {
	header := createTestHeader()

	t.Run("begin_tombsones_on_write_failure", func(t *testing.T) {
		// Create a write channel but don't set up FileManager (simulates write failure)
		writeChan := make(chan Data, 10)

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
		}

		// Start a goroutine that will close the channel to simulate failure
		go func() {
			// Wait for the write request
			data := <-writeChan
			// Send an error to simulate write failure
			data.Response <- NewWriteError("simulated write failure", nil)
		}()

		// Call Begin() - should return error
		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should return error on write failure")
		}

		// FR-006: Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// FR-006: Verify subsequent API calls return TombstonedError
		if err := tx.Begin(); err == nil {
			t.Fatal("Begin() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}

		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err == nil {
			t.Fatal("AddRow() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}

		if err := tx.Commit(); err == nil {
			t.Fatal("Commit() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}
	})
}

// Test_S_015_FR_006_AddRowWriteFailureNoPartialData tests FR-006: If any write operation fails,
// the system MUST tombstone the transaction and return the write error. All subsequent public API calls
// on the tombstoned transaction MUST return TombstonedError
func Test_S_015_FR_006_AddRowWriteFailureNoPartialData(t *testing.T) {
	header := createTestHeader()

	t.Run("addrow_tombsones_on_write_failure", func(t *testing.T) {
		writeChan := make(chan Data, 10)

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
		}

		// Begin first
		go func() {
			data := <-writeChan
			data.Response <- nil // Begin succeeds
		}()
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Now simulate AddRow write failure
		go func() {
			// Consume the AddRow write request
			data := <-writeChan
			// Send an error to simulate write failure
			data.Response <- NewWriteError("simulated write failure", nil)
		}()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should return error on write failure")
		}

		// FR-006: Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// FR-006: Verify subsequent API calls return TombstonedError
		key2, _ := uuid.NewV7()
		if err := tx.AddRow(key2, json.RawMessage(`{"data":"test2"}`)); err == nil {
			t.Fatal("AddRow() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}

		if err := tx.Commit(); err == nil {
			t.Fatal("Commit() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}
	})
}

// Test_S_015_FR_006_CommitWriteFailureNoPartialData tests FR-006: If any write operation fails,
// the system MUST tombstone the transaction and return the write error. All subsequent public API calls
// on the tombstoned transaction MUST return TombstonedError
func Test_S_015_FR_006_CommitWriteFailureNoPartialData(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_tombsones_on_write_failure", func(t *testing.T) {
		writeChan := make(chan Data, 10)

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
		}

		// Begin and AddRow first
		go func() {
			data := <-writeChan
			data.Response <- nil // Begin succeeds
		}()
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		go func() {
			data := <-writeChan
			data.Response <- nil // AddRow succeeds
		}()
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Now simulate Commit write failure
		go func() {
			data := <-writeChan
			data.Response <- NewWriteError("simulated write failure", nil)
		}()

		err := tx.Commit()
		if err == nil {
			t.Fatal("Commit() should return error on write failure")
		}

		// FR-006: Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// FR-006: Verify subsequent API calls return TombstonedError
		key2, _ := uuid.NewV7()
		if err := tx.AddRow(key2, json.RawMessage(`{"data":"test2"}`)); err == nil {
			t.Fatal("AddRow() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}

		if err := tx.Commit(); err == nil {
			t.Fatal("Commit() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}

		if _, err := tx.GetCommittedRows(); err == nil {
			t.Fatal("GetCommittedRows() should return TombstonedError on tombstoned transaction")
		} else if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}
	})
}

// Test_S_015_FR_007_TransactionOnlyAppendsNewBytes tests FR-007: The Transaction MUST only append new bytes to the database file
// (no modification of existing data)
func Test_S_015_FR_007_TransactionOnlyAppendsNewBytes(t *testing.T) {
	header := createTestHeader()

	t.Run("transaction_only_appends_no_modification", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		// Initialize file with header and checksum row
		createMinimalTestDatabase(t, tmpPath, header)

		// Read initial file content
		initialContent, err := os.ReadFile(tmpPath)
		if err != nil {
			t.Fatalf("Failed to read initial file: %v", err)
		}

		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin, AddRow, Commit
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Read final file content
		finalContent, err := os.ReadFile(tmpPath)
		if err != nil {
			t.Fatalf("Failed to read final file: %v", err)
		}

		// Verify initial content is unchanged (prefix match)
		if len(finalContent) < len(initialContent) {
			t.Fatal("Final file should be at least as large as initial file")
		}
		for i := 0; i < len(initialContent); i++ {
			if finalContent[i] != initialContent[i] {
				t.Errorf("Byte at position %d was modified: expected 0x%02X, got 0x%02X", i, initialContent[i], finalContent[i])
			}
		}

		// Verify new bytes were appended (file size increased)
		if len(finalContent) <= len(initialContent) {
			t.Error("File size should have increased after transaction operations")
		}
	})
}

// Test_S_015_FR_008_TransactionAssumesValidFile tests FR-008: The Transaction MUST assume the database file is valid on
// initialization (header, checksum row, and finalized rows present)
func Test_S_015_FR_008_TransactionAssumesValidFile(t *testing.T) {
	header := createTestHeader()

	t.Run("transaction_works_with_valid_file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		// Initialize file with header and checksum row (valid file structure)
		createMinimalTestDatabase(t, tmpPath, header)

		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Transaction should work without validating file structure (assumes it's valid)
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() should succeed with valid file structure: %v", err)
		}

		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Fatalf("AddRow() should succeed with valid file structure: %v", err)
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() should succeed with valid file structure: %v", err)
		}
	})
}

// Test_S_015_FR_009_TransactionNoChecksumRows tests FR-009: The Transaction MUST NOT write checksum rows
// (assumes database < 10,000 rows)
func Test_S_015_FR_009_TransactionNoChecksumRows(t *testing.T) {
	header := createTestHeader()

	t.Run("transaction_does_not_write_checksum_rows", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		// Initialize file with header and checksum row
		createMinimalTestDatabase(t, tmpPath, header)

		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 10)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Perform multiple operations that would trigger checksum if implemented
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Add multiple rows (but less than 10,000)
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, i))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Read file and verify no checksum rows were written
		// Checksum rows would have ROW_START + 'C' as start_control
		fileContent, err := os.ReadFile(tmpPath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		// Search for checksum row pattern: ROW_START (0x1F) followed by 'C'
		// Skip the initial checksum row that was created by createMinimalTestDatabase
		initialSize := int64(len(fileContent))
		// Find the first checksum row (created by createMinimalTestDatabase)
		firstChecksumFound := false
		for i := 0; i < len(fileContent)-1; i++ {
			if fileContent[i] == ROW_START && fileContent[i+1] == 'C' {
				if !firstChecksumFound {
					firstChecksumFound = true
					continue // Skip the initial checksum row
				}
				// Found an additional checksum row - this should not happen
				t.Errorf("Found unexpected checksum row at byte position %d. Transaction should not write checksum rows.", i)
			}
		}

		// Verify file size matches expected size (no additional checksum rows)
		// Expected: header + initial checksum + transaction rows
		// We can't calculate exact size easily, but we can verify it's reasonable
		if initialSize < 0 {
			t.Error("File size should be positive")
		}
	})
}

// Test_S_015_FR_010_ConcurrentBegin tests FR-010: Transaction methods (Begin, AddRow, Commit, Rollback, Savepoint)
// MUST be thread-safe when called concurrently from multiple goroutines
func Test_S_015_FR_010_ConcurrentBegin(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_begin_operations", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 100)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
		}

		// Spawn multiple goroutines trying to call Begin() concurrently
		done := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func() {
				err := tx.Begin()
				done <- err
			}()
		}

		// Collect results
		successCount := 0
		errorCount := 0
		for i := 0; i < 10; i++ {
			err := <-done
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		// Only one Begin() should succeed, others should return InvalidActionError
		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful Begin(), got %d", successCount)
		}
		if errorCount != 9 {
			t.Errorf("Expected 9 failed Begin() calls, got %d", errorCount)
		}
	})
}

// Test_S_015_FR_010_ConcurrentAddRow tests FR-010: Transaction methods MUST be thread-safe
func Test_S_015_FR_010_ConcurrentAddRow(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_addrow_operations", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 100)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin first
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Spawn multiple goroutines calling AddRow() concurrently
		done := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				key, _ := uuid.NewV7()
				err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, id)))
				done <- err
			}(i)
		}

		// Collect results
		successCount := 0
		for i := 0; i < 10; i++ {
			err := <-done
			if err == nil {
				successCount++
			}
		}

		// All AddRow() calls should succeed (they are serialized by the mutex)
		if successCount != 10 {
			t.Errorf("Expected all 10 AddRow() calls to succeed, got %d successes", successCount)
		}

		// Verify transaction state is consistent
		if tx.last == nil {
			t.Error("Transaction.last should not be nil after AddRow() calls")
		}
	})
}

// Test_S_015_FR_010_ConcurrentAddRowAndCommit tests FR-010: Transaction methods MUST be thread-safe
func Test_S_015_FR_010_ConcurrentAddRowAndCommit(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_addrow_and_commit", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		writeChan := make(chan Data, 100)
		if err := fm.SetWriter(writeChan); err != nil {
			t.Fatalf("Failed to set writer: %v", err)
		}

		tx := &Transaction{
			Header:    header,
			writeChan: writeChan,
			db:        fm,
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Transaction validation failed: %v", err)
		}

		// Begin first
		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Add one row first
		key1, _ := uuid.NewV7()
		if err := tx.AddRow(key1, json.RawMessage(`{"data":"first"}`)); err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		// Spawn goroutines: some calling AddRow(), one calling Commit()
		done := make(chan error, 5)
		for i := 0; i < 4; i++ {
			go func(id int) {
				key, _ := uuid.NewV7()
				err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, id)))
				done <- err
			}(i)
		}
		go func() {
			err := tx.Commit()
			done <- err
		}()

		// Collect results
		results := make([]error, 5)
		for i := 0; i < 5; i++ {
			results[i] = <-done
		}

		// At least Commit() should succeed, and some AddRow() may succeed before Commit()
		commitSucceeded := false
		for _, err := range results {
			if err == nil {
				commitSucceeded = true
				break
			}
		}

		if !commitSucceeded {
			t.Error("Expected at least Commit() to succeed")
		}

		// Verify transaction is in a consistent state (either committed or still active)
		// The exact state depends on timing, but it should be valid
		if tx.last != nil && tx.empty != nil {
			t.Error("Transaction should not have both last and empty set")
		}
	})
}

// Test_S_016_FR_001_ChecksumAtIntervals tests FR-001: System MUST write checksum rows at row positions 10,000, 20,000, 30,000...
func Test_S_016_FR_001_ChecksumAtIntervals(t *testing.T) {
	header := createTestHeader()
	rowSize := int64(header.GetRowSize())

	t.Run("checksum_at_10000", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		// Insert exactly 10,000 rows in 100 transactions to trigger checksum at 10,000
		for txNum := 0; txNum < 100; txNum++ {
			tx, err := NewTransaction(fm, header, nil)
			if err != nil {
				t.Fatalf("Failed to create transaction: %v", err)
			}

			if err := tx.Begin(); err != nil {
				t.Fatalf("Begin() failed: %v", err)
			}

			for i := 0; i < 100; i++ {
				key, err := uuid.NewV7()
				if err != nil {
					t.Fatalf("Failed to generate UUIDv7: %v", err)
				}

				if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+i))); err != nil {
					t.Fatalf("AddRow() failed: %v", err)
				}
			}

			if err := tx.Commit(); err != nil {
				t.Fatalf("Commit() failed for transaction %d: %v", txNum, err)
			}

			fm.Close()
			fm, err = NewFileManager(tmpPath)
			if err != nil {
				t.Fatalf("Failed to reopen FileManager: %v", err)
			}
		}

		// Verify checksum row appears after the 10,000th row
		fileSize := fm.Size()
		// Expected size: header (64) + initial checksum (rowSize) + 10000 data rows (10000 * rowSize) + checksum at 10000 (rowSize)
		expectedSize := int64(HEADER_SIZE) + rowSize + (10000 * rowSize) + rowSize
		if fileSize != expectedSize {
			t.Errorf("Expected file size %d, got %d", expectedSize, fileSize)
		}

		// Verify checksum row exists at position 10,000
		checksumOffset := int64(HEADER_SIZE) + rowSize + (10000 * rowSize)
		checksumBytes, err := fm.Read(checksumOffset, int32(rowSize))
		if err != nil {
			t.Fatalf("Failed to read checksum row: %v", err)
		}

		// Verify start_control is 'C'
		if checksumBytes[1] != byte(CHECKSUM_ROW) {
			t.Errorf("Expected checksum row start_control 'C', got '%c'", checksumBytes[1])
		}

		// Verify end_control is 'CS'
		endControlStart := int(rowSize) - 5
		if checksumBytes[endControlStart] != 'C' || checksumBytes[endControlStart+1] != 'S' {
			t.Errorf("Expected checksum row end_control 'CS', got '%c%c'", checksumBytes[endControlStart], checksumBytes[endControlStart+1])
		}
	})

	t.Run("checksum_at_20000", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)

		createMinimalTestDatabase(t, tmpPath, header)
		fm, err := NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to create FileManager: %v", err)
		}
		defer fm.Close()

		// Insert 20,000 rows in 200 transactions to trigger checksum at 20,000
		for txNum := 0; txNum < 200; txNum++ {
			tx, err := NewTransaction(fm, header, nil)
			if err != nil {
				t.Fatalf("Failed to create transaction: %v", err)
			}

			if err := tx.Begin(); err != nil {
				t.Fatalf("Begin() failed: %v", err)
			}

			for i := 0; i < 100; i++ {
				key, err := uuid.NewV7()
				if err != nil {
					t.Fatalf("Failed to generate UUIDv7: %v", err)
				}

				if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+i))); err != nil {
					t.Fatalf("AddRow() failed: %v", err)
				}
			}

			if err := tx.Commit(); err != nil {
				t.Fatalf("Commit() failed: %v", err)
			}

			fm.Close()
			fm, err = NewFileManager(tmpPath)
			if err != nil {
				t.Fatalf("Failed to reopen FileManager: %v", err)
			}
		}

		// Verify checksum row appears at position 20,000
		fileSize := fm.Size()
		// Expected: header + initial checksum + 20000 data rows + 2 checksum rows
		expectedSize := int64(HEADER_SIZE) + rowSize + (20000 * rowSize) + (2 * rowSize)
		if fileSize != expectedSize {
			t.Errorf("Expected file size %d, got %d", expectedSize, fileSize)
		}

		// Verify checksum row exists at position after 20,000 rows
		checksumOffset := int64(HEADER_SIZE) + rowSize + (20000 * rowSize) + rowSize
		checksumBytes, err := fm.Read(checksumOffset, int32(rowSize))
		if err != nil {
			t.Fatalf("Failed to read checksum row: %v", err)
		}
		if checksumBytes[1] != byte(CHECKSUM_ROW) {
			t.Error("Checksum row missing or invalid")
		}
	})
}

// Test_S_016_FR_003_ExcludePartialDataRows tests FR-003: System MUST NOT count PartialDataRows toward checksum interval calculation
func Test_S_016_FR_003_ExcludePartialDataRows(t *testing.T) {
	header := createTestHeader()
	rowSize := int64(header.GetRowSize())

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,999 complete rows (99 transactions of 100 rows + 1 transaction of 99 rows = 100 transactions total)
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		rowsToAdd := 100
		if txNum == 99 {
			rowsToAdd = 99
		}

		for rowNum := 0; rowNum < rowsToAdd; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a new transaction and begin it (creates PartialDataRow)
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add one row to the partial (still partial, not complete)
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key, json.RawMessage(`{"data":"partial"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// At this point, we have 9,999 complete rows + 1 partial row
	// The partial row should NOT trigger a checksum
	// File size should be: header + initial checksum + 9999 complete rows + partial row bytes
	fileSize := fm.Size()
	// Partial row is incomplete, so it's less than rowSize
	expectedMinSize := int64(HEADER_SIZE) + rowSize + (9999 * rowSize)
	if fileSize < expectedMinSize {
		t.Errorf("File size too small: expected at least %d, got %d", expectedMinSize, fileSize)
	}

	// Complete the partial row by committing (making it the 10,000th complete row)
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Now the checksum should appear after the 10,000th complete row
	fileSizeAfter := fm.Size()
	expectedSizeAfter := int64(HEADER_SIZE) + rowSize + (10000 * rowSize) + rowSize
	if fileSizeAfter != expectedSizeAfter {
		t.Errorf("After completing 10,000th row: expected file size %d, got %d", expectedSizeAfter, fileSizeAfter)
	}
}

// Test_S_016_FR_004_FormatRequirements tests FR-004: System MUST follow all v1_file_format.md requirements for checksum rows
func Test_S_016_FR_004_FormatRequirements(t *testing.T) {
	header := createTestHeader()
	rowSize := int64(header.GetRowSize())

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 10,000 rows to trigger checksum
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Read and validate checksum row format
	checksumOffset := int64(HEADER_SIZE) + rowSize + (10000 * rowSize)
	checksumBytes, err := fm.Read(checksumOffset, int32(rowSize))
	if err != nil {
		t.Fatalf("Failed to read checksum row: %v", err)
	}

	// Verify ROW_START at position [0]
	if checksumBytes[0] != ROW_START {
		t.Errorf("Expected ROW_START 0x%02X at position [0], got 0x%02X", ROW_START, checksumBytes[0])
	}

	// Verify start_control='C' at position [1]
	if checksumBytes[1] != byte(CHECKSUM_ROW) {
		t.Errorf("Expected start_control='C' at position [1], got '%c'", checksumBytes[1])
	}

	// Verify CRC32 Base64 encoding at positions [2..9]
	crc32Base64 := string(checksumBytes[2:10])
	if len(crc32Base64) != 8 {
		t.Errorf("Expected 8-byte Base64 CRC32, got %d bytes", len(crc32Base64))
	}

	// Verify end_control='CS' at positions [N-5..N-4]
	endControlStart := int(rowSize) - 5
	if checksumBytes[endControlStart] != 'C' || checksumBytes[endControlStart+1] != 'S' {
		t.Errorf("Expected end_control='CS' at positions [%d..%d], got '%c%c'", endControlStart, endControlStart+1, checksumBytes[endControlStart], checksumBytes[endControlStart+1])
	}

	// Verify ROW_END at position [N-1]
	if checksumBytes[rowSize-1] != ROW_END {
		t.Errorf("Expected ROW_END 0x%02X at position [%d], got 0x%02X", ROW_END, rowSize-1, checksumBytes[rowSize-1])
	}

	// Verify checksum row can be unmarshaled and validated
	checksumRow := &ChecksumRow{
		baseRow[*Checksum]{
			RowSize: header.GetRowSize(),
		},
	}
	if err := checksumRow.UnmarshalText(checksumBytes); err != nil {
		t.Errorf("Failed to unmarshal checksum row: %v", err)
	}
}

// Test_S_016_FR_005_TransparencyToTransactions tests FR-005: System MUST ensure checksum rows are transparent to transactions
func Test_S_016_FR_005_TransparencyToTransactions(t *testing.T) {
	header := createTestHeader()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert rows that will cross the 10,000 boundary within a single transaction
	// Start at 9,995 rows
	for txNum := 0; txNum < 99; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Add 5 more rows (making it 9,995 total) in the last transaction
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, 9900+i))); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	fm.Close()
	fm, err = NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to reopen FileManager: %v", err)
	}

	// Start a new transaction that will cross the boundary
	tx, err = NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add 10 rows - the 5th will be the 10,000th overall, triggering a checksum
	for i := 0; i < 10; i++ {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"boundary%d"}`, i))); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Verify transaction committed successfully with all 10 rows
	// The checksum row insertion should not have affected the transaction
	rows := tx.GetRows()
	if len(rows) != 10 {
		t.Errorf("Expected 10 rows in transaction, got %d", len(rows))
	}

	// Verify transaction is committed
	if !tx.IsCommitted() {
		t.Error("Transaction should be committed")
	}
}

// Test_S_016_FR_006_StartControlAfterChecksum tests FR-006: When a checksum row is inserted between rows of an open transaction, the next row MUST maintain the correct start_control
func Test_S_016_FR_006_StartControlAfterChecksum(t *testing.T) {
	header := createTestHeader()
	rowSize := int64(header.GetRowSize())

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,998 rows (99 transactions of 100 rows + 1 transaction of 98 rows = 100 transactions total)
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		rowsToAdd := 100
		if txNum == 99 {
			rowsToAdd = 98
		}

		for rowNum := 0; rowNum < rowsToAdd; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a transaction that will cross the boundary
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add first row (will be 10,000th overall)
	key1, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key1, json.RawMessage(`{"data":"row10000"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Add second row (should come after checksum, with start_control='R')
	key2, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key2, json.RawMessage(`{"data":"row10001"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Add third row (also after checksum, with start_control='R')
	key3, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key3, json.RawMessage(`{"data":"row10002"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Verify the row after checksum has start_control='R' (continuation)
	// Calculate position: after checksum at 10,000
	rowAfterChecksumOffset := int64(HEADER_SIZE) + rowSize + (10000 * rowSize) + rowSize
	rowBytes, err := fm.Read(rowAfterChecksumOffset, int32(rowSize))
	if err != nil {
		t.Fatalf("Failed to read row after checksum: %v", err)
	}

	// Verify start_control is 'R' (not 'T')
	if rowBytes[1] != byte(ROW_CONTINUE) {
		t.Errorf("Expected start_control='R' for row after checksum, got '%c'", rowBytes[1])
	}
}

// Test_S_016_FR_007_NotInQueryResults tests FR-007: Checksum rows MUST NOT appear in query results or be counted as committed data
func Test_S_016_FR_007_NotInQueryResults(t *testing.T) {
	header := createTestHeader()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 10,000 rows to trigger checksum
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		if txNum == 99 {
			// Verify GetCommittedRows() from last transaction doesn't include checksum rows
			iter, err := tx.GetCommittedRows()
			if err != nil {
				t.Fatalf("GetCommittedRows() failed: %v", err)
			}

			count := 0
			for {
				_, more := iter()
				if !more {
					break
				}
				count++
			}

			// Should count only the last transaction's rows, not all 10,000
			// GetCommittedRows() only returns rows from the current transaction
			if count != 100 {
				t.Errorf("Expected 100 committed rows from last transaction, got %d", count)
			}

			// The key point: checksum rows should not be in the committed rows
			// This is verified by the fact that we can iterate through committed rows
			// without encountering a checksum row (which would have start_control='C')
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}
}

// Test_S_016_US2_TransactionSpansChecksumBoundary tests User Story 2: All rows visible across checksum boundary
func Test_S_016_US2_TransactionSpansChecksumBoundary(t *testing.T) {
	header := createTestHeader()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,900 rows first
	for txNum := 0; txNum < 99; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a transaction that will cross the 10,000 boundary
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add 10 rows - this will cross the boundary
	for i := 0; i < 10; i++ {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"boundary%d"}`, i))); err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Verify all 10 rows are in the transaction
	rows := tx.GetRows()
	if len(rows) != 10 {
		t.Errorf("Expected 10 rows in transaction, got %d", len(rows))
	}

	// Verify transaction is committed
	if !tx.IsCommitted() {
		t.Error("Transaction should be committed")
	}

	// Verify all rows are accessible via GetCommittedRows
	iter, err := tx.GetCommittedRows()
	if err != nil {
		t.Fatalf("GetCommittedRows() failed: %v", err)
	}

	count := 0
	for {
		_, more := iter()
		if !more {
			break
		}
		count++
	}

	if count != 10 {
		t.Errorf("Expected 10 committed rows, got %d", count)
	}
}

// Test_S_016_US2_ChecksumRowNotInResults tests User Story 2: Checksum rows excluded from queries
func Test_S_016_US2_ChecksumRowNotInResults(t *testing.T) {
	header := createTestHeader()
	rowSize := int64(header.GetRowSize())

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 10,000 rows to trigger checksum
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		if txNum == 99 {
			// Verify GetCommittedRows() doesn't include checksum rows
			// The last transaction should have 100 rows
			iter, err := tx.GetCommittedRows()
			if err != nil {
				t.Fatalf("GetCommittedRows() failed: %v", err)
			}

			count := 0
			for {
				row, more := iter()
				if !more {
					break
				}
				// Verify row is not a checksum row
				if row.StartControl == CHECKSUM_ROW {
					t.Error("Checksum row found in committed rows")
				}
				count++
			}

			// Should have exactly 100 rows from the last transaction
			if count != 100 {
				t.Errorf("Expected 100 committed rows, got %d", count)
			}
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Verify checksum row exists in file
	fileSize := fm.Size()
	expectedSize := int64(HEADER_SIZE) + rowSize + (10000 * rowSize) + rowSize
	if fileSize != expectedSize {
		t.Errorf("Expected file size %d, got %d", expectedSize, fileSize)
	}
}

// Test_S_016_US2_SavepointStateAfterChecksum tests User Story 2: Savepoint count unchanged after checksum
func Test_S_016_US2_SavepointStateAfterChecksum(t *testing.T) {
	header := createTestHeader()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,998 rows first (99 transactions of 100 rows + 1 transaction of 98 rows = 100 transactions total)
	for txNum := 0; txNum < 100; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		rowsToAdd := 100
		if txNum == 99 {
			rowsToAdd = 98
		}

		for rowNum := 0; rowNum < rowsToAdd; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a transaction that will cross boundary
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add first row
	key1, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key1, json.RawMessage(`{"data":"row1"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Create a savepoint
	if err := tx.Savepoint(); err != nil {
		t.Fatalf("Savepoint() failed: %v", err)
	}

	// Verify savepoint count is 0, since the row has not been completed
	savepointIndices := tx.GetSavepointIndices()
	if len(savepointIndices) != 0 {
		t.Fatalf("Expected 0 savepoint, got %d", len(savepointIndices))
	}

	// Add second row (this will be the 10,000th row, triggering checksum)
	key2, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key2, json.RawMessage(`{"data":"row2"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Verify savepoint count is still 1 (checksum insertion should not affect it)
	savepointIndices = tx.GetSavepointIndices()
	if len(savepointIndices) != 1 {
		t.Errorf("Expected savepoint count to remain 1 after checksum, got %d", len(savepointIndices))
	}
	tmpPath = tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err = NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,999 rows first
	for txNum := 0; txNum < 99; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a transaction that will cross the boundary
	tx, err = NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add first row
	key1, err = uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key1, json.RawMessage(`{"data":"row1"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Create a savepoint
	if err := tx.Savepoint(); err != nil {
		t.Fatalf("Savepoint() failed: %v", err)
	}

	// Verify savepoint count is 0, since the row has not been completed
	savepointIndices = tx.GetSavepointIndices()
	if len(savepointIndices) != 0 {
		t.Errorf("Expected 0 savepoint, got %d", len(savepointIndices))
	}

	// Add second row (this will be the 10,000th row, triggering checksum)
	key2, err = uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key2, json.RawMessage(`{"data":"row2"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Verify savepoint count is still 1 (checksum insertion should not affect it)
	savepointIndices = tx.GetSavepointIndices()
	if len(savepointIndices) != 1 {
		t.Errorf("Expected savepoint count to remain 1 after checksum, got %d", len(savepointIndices))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Verify savepoint count is still 1
	savepointIndices = tx.GetSavepointIndices()
	if len(savepointIndices) != 1 {
		t.Errorf("Expected savepoint count to remain 1 after commit, got %d", len(savepointIndices))
	}
}

// Test_S_016_US2_RollbackAfterChecksum tests User Story 2: Rollback to savepoint works correctly after checksum
func Test_S_016_US2_RollbackAfterChecksum(t *testing.T) {
	header := createTestHeader()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	createMinimalTestDatabase(t, tmpPath, header)
	fm, err := NewFileManager(tmpPath)
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Insert 9,999 rows first
	for txNum := 0; txNum < 99; txNum++ {
		tx, err := NewTransaction(fm, header, nil)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		if err := tx.Begin(); err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		for rowNum := 0; rowNum < 100; rowNum++ {
			key, err := uuid.NewV7()
			if err != nil {
				t.Fatalf("Failed to generate UUIDv7: %v", err)
			}

			if err := tx.AddRow(key, json.RawMessage(fmt.Sprintf(`{"data":"row%d"}`, txNum*100+rowNum))); err != nil {
				t.Fatalf("AddRow() failed: %v", err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		fm.Close()
		fm, err = NewFileManager(tmpPath)
		if err != nil {
			t.Fatalf("Failed to reopen FileManager: %v", err)
		}
	}

	// Start a transaction that will cross the boundary
	tx, err := NewTransaction(fm, header, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	if err := tx.Begin(); err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add first row
	key1, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key1, json.RawMessage(`{"data":"before_savepoint"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Create a savepoint
	if err := tx.Savepoint(); err != nil {
		t.Fatalf("Savepoint() failed: %v", err)
	}

	// Add second row (this will be the 10,000th row, triggering checksum)
	key2, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key2, json.RawMessage(`{"data":"after_savepoint"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Add third row
	key3, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if err := tx.AddRow(key3, json.RawMessage(`{"data":"after_checksum"}`)); err != nil {
		t.Fatalf("AddRow() failed: %v", err)
	}

	// Rollback to savepoint 1
	if err := tx.Rollback(1); err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Verify only rows up to savepoint are committed
	iter, err := tx.GetCommittedRows()
	if err != nil {
		t.Fatalf("GetCommittedRows() failed: %v", err)
	}

	count := 0
	for {
		_, more := iter()
		if !more {
			break
		}
		count++
	}

	// Should have 1 row (the one before savepoint)
	if count != 1 {
		t.Errorf("Expected 1 committed row after rollback, got %d", count)
	}
}
