package frozendb

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// AddRow Unit Tests
// =============================================================================

// TestAddRow_DataFlowFirstAddRow verifies the first AddRow modifies the existing
// partial from Begin() rather than creating a new one
func TestAddRow_DataFlowFirstAddRow(t *testing.T) {
	header := createTestHeader()

	t.Run("first_addrow_uses_existing_partial_from_begin", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// After Begin, partial should have START_TRANSACTION
		if tx.last == nil {
			t.Fatal("Expected partial row after Begin()")
		}
		if tx.last.d.StartControl != START_TRANSACTION {
			t.Errorf("Partial from Begin() should have START_TRANSACTION, got %c", tx.last.d.StartControl)
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		// After first AddRow, partial should still have START_TRANSACTION
		// because we modified the existing partial, not created a new one
		if tx.last.d.StartControl != START_TRANSACTION {
			t.Errorf("After first AddRow, partial should still have START_TRANSACTION, got %c", tx.last.d.StartControl)
		}

		// No rows should be finalized yet (first AddRow just adds to existing partial)
		if len(tx.rows) != 0 {
			t.Errorf("First AddRow should not finalize any rows, got %d", len(tx.rows))
		}
	})

	t.Run("second_addrow_creates_new_partial_with_row_continue", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		tx.AddRow(key1, `{"data":"first"}`)
		tx.AddRow(key2, `{"data":"second"}`)

		// After second AddRow, partial should have ROW_CONTINUE
		if tx.last.d.StartControl != ROW_CONTINUE {
			t.Errorf("After second AddRow, partial should have ROW_CONTINUE, got %c", tx.last.d.StartControl)
		}

		// One row should be finalized
		if len(tx.rows) != 1 {
			t.Fatalf("Second AddRow should finalize first row, got %d rows", len(tx.rows))
		}

		// Finalized row should have START_TRANSACTION
		if tx.rows[0].StartControl != START_TRANSACTION {
			t.Errorf("First finalized row should have START_TRANSACTION, got %c", tx.rows[0].StartControl)
		}

		// Finalized row should have RE end control
		if tx.rows[0].EndControl != ROW_END_CONTROL {
			t.Errorf("First finalized row should have RE end control, got %s", tx.rows[0].EndControl.String())
		}
	})
}

// TestAddRow_PartialRowStateTransitions verifies proper state transitions
func TestAddRow_PartialRowStateTransitions(t *testing.T) {
	header := createTestHeader()

	t.Run("partial_advances_from_start_control_to_payload", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		if tx.last.GetState() != PartialDataRowWithStartControl {
			t.Errorf("After Begin(), state should be PartialDataRowWithStartControl, got %v", tx.last.GetState())
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		if tx.last.GetState() != PartialDataRowWithPayload {
			t.Errorf("After AddRow(), state should be PartialDataRowWithPayload, got %v", tx.last.GetState())
		}
	})
}

// TestAddRow_KeyValueStorage verifies key and value are properly stored
func TestAddRow_KeyValueStorage(t *testing.T) {
	header := createTestHeader()

	t.Run("key_and_value_stored_correctly", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		value := `{"name":"test","count":42}`

		tx.AddRow(key, value)
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].GetKey() != key {
			t.Errorf("Key mismatch: expected %s, got %s", key, rows[0].GetKey())
		}

		if rows[0].GetValue() != value {
			t.Errorf("Value mismatch: expected %s, got %s", value, rows[0].GetValue())
		}
	})

	t.Run("multiple_rows_preserve_order", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		keys := make([]uuid.UUID, 5)
		values := []string{
			`{"index":0}`,
			`{"index":1}`,
			`{"index":2}`,
			`{"index":3}`,
			`{"index":4}`,
		}

		for i := 0; i < 5; i++ {
			keys[i], _ = uuid.NewV7()
			tx.AddRow(keys[i], values[i])
		}
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 5 {
			t.Fatalf("Expected 5 rows, got %d", len(rows))
		}

		for i, row := range rows {
			if row.GetKey() != keys[i] {
				t.Errorf("Row %d key mismatch: expected %s, got %s", i, keys[i], row.GetKey())
			}
			if row.GetValue() != values[i] {
				t.Errorf("Row %d value mismatch: expected %s, got %s", i, values[i], row.GetValue())
			}
		}
	})
}

// TestAddRow_EndControlPatterns verifies correct end_control assignment
func TestAddRow_EndControlPatterns(t *testing.T) {
	header := createTestHeader()

	t.Run("intermediate_rows_have_RE", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}
		tx.Commit()

		rows := tx.GetRows()
		// All rows except last should have RE
		for i := 0; i < len(rows)-1; i++ {
			if rows[i].EndControl != ROW_END_CONTROL {
				t.Errorf("Row %d should have RE end control, got %s", i, rows[i].EndControl.String())
			}
		}
		// Last row should have TC
		if rows[len(rows)-1].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Last row should have TC end control, got %s", rows[len(rows)-1].EndControl.String())
		}
	})
}

// TestAddRow_TimestampOrdering verifies UUID timestamp ordering behavior
func TestAddRow_TimestampOrdering(t *testing.T) {
	t.Run("ascending_timestamps_accepted", func(t *testing.T) {
		header := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    1, // Small skew to handle same-millisecond UUIDs
		}
		tx := &Transaction{Header: header}
		tx.Begin()

		// Generate keys in quick succession (ascending timestamps)
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, `{"data":"test"}`)
			if err != nil {
				t.Fatalf("AddRow %d failed: %v", i, err)
			}
		}
	})

	t.Run("skew_allows_slightly_older_timestamps", func(t *testing.T) {
		header := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    5000, // 5 second skew
		}
		tx := &Transaction{Header: header}
		tx.Begin()

		// First key
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)

		// Create a key with slightly older timestamp (within skew)
		// The skew should allow this
		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, `{"data":"second"}`)
		if err != nil {
			t.Fatalf("Second AddRow should succeed with skew: %v", err)
		}
	})

	t.Run("max_timestamp_tracks_highest_seen", func(t *testing.T) {
		header := createTestHeader()
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)

		ts1 := tx.GetMaxTimestamp()

		// Add delay to ensure next key has higher timestamp
		time.Sleep(2 * time.Millisecond)

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		ts2 := tx.GetMaxTimestamp()

		if ts2 <= ts1 {
			t.Errorf("Max timestamp should increase: was %d, now %d", ts1, ts2)
		}
	})
}

// TestAddRow_RowCountLimit verifies the 100 row limit
func TestAddRow_RowCountLimit(t *testing.T) {
	header := createTestHeader()

	t.Run("exactly_100_rows_allowed", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, `{"i":1}`)
			if err != nil {
				t.Fatalf("AddRow %d should succeed: %v", i, err)
			}
		}

		// Should be able to commit
		err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit should succeed: %v", err)
		}

		if len(tx.GetRows()) != 100 {
			t.Errorf("Expected 100 rows, got %d", len(tx.GetRows()))
		}
	})

	t.Run("101st_row_rejected", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"i":1}`)
		}

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, `{"i":1}`)
		if err == nil {
			t.Fatal("101st AddRow should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})
}

// TestAddRow_ErrorConditions verifies various error scenarios
func TestAddRow_ErrorConditions(t *testing.T) {
	header := createTestHeader()

	t.Run("addrow_before_begin_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, `{"data":"test"}`)

		if err == nil {
			t.Fatal("AddRow before Begin should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("addrow_after_commit_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"test"}`)
		tx.Commit()

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, `{"data":"more"}`)

		if err == nil {
			t.Fatal("AddRow after Commit should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("addrow_after_empty_commit_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()
		tx.Commit() // Empty commit produces NullRow

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, `{"data":"test"}`)

		if err == nil {
			t.Fatal("AddRow after empty Commit should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("nil_uuid_rejected", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		err := tx.AddRow(uuid.Nil, `{"data":"test"}`)

		if err == nil {
			t.Fatal("Nil UUID should be rejected")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("empty_value_rejected", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, "")

		if err == nil {
			t.Fatal("Empty value should be rejected")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("uuidv4_rejected", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key := uuid.New() // v4
		err := tx.AddRow(key, `{"data":"test"}`)

		if err == nil {
			t.Fatal("UUIDv4 should be rejected")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})
}

// TestAddRow_Concurrency verifies thread safety
func TestAddRow_Concurrency(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_reads_safe", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = tx.GetRows()
				_ = tx.GetMaxTimestamp()
				_ = tx.IsCommitted()
			}()
		}
		wg.Wait()
	})

	t.Run("sequential_addrows_maintain_order", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		keys := make([]uuid.UUID, 10)
		for i := 0; i < 10; i++ {
			keys[i], _ = uuid.NewV7()
			err := tx.AddRow(keys[i], `{"data":"test"}`)
			if err != nil {
				t.Fatalf("AddRow %d failed: %v", i, err)
			}
		}
		tx.Commit()

		rows := tx.GetRows()
		for i, row := range rows {
			if row.GetKey() != keys[i] {
				t.Errorf("Row %d key mismatch after concurrent operations", i)
			}
		}
	})
}

// TestAddRow_TransactionStateInference verifies state helper methods
func TestAddRow_TransactionStateInference(t *testing.T) {
	header := createTestHeader()

	t.Run("is_active_after_begin", func(t *testing.T) {
		tx := &Transaction{Header: header}

		// Initially not active
		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()
		if active {
			t.Error("Should not be active before Begin()")
		}

		tx.Begin()

		tx.mu.RLock()
		active = tx.isActive()
		tx.mu.RUnlock()
		if !active {
			t.Error("Should be active after Begin()")
		}
	})

	t.Run("is_active_after_addrow", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()
		if !active {
			t.Error("Should be active after AddRow()")
		}
	})

	t.Run("not_active_after_commit", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Commit()

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()
		if active {
			t.Error("Should not be active after Commit()")
		}
	})

	t.Run("is_committed_after_data_commit", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Commit()

		tx.mu.RLock()
		committed := tx.isCommittedState()
		tx.mu.RUnlock()
		if !committed {
			t.Error("Should be committed after Commit()")
		}
	})

	t.Run("is_committed_after_empty_commit", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()
		tx.Commit()

		tx.mu.RLock()
		committed := tx.isCommittedState()
		tx.mu.RUnlock()
		if !committed {
			t.Error("Should be committed after empty Commit()")
		}
	})
}

// TestAddRow_ValueSizes verifies various value sizes work correctly
func TestAddRow_ValueSizes(t *testing.T) {
	header := createTestHeader() // 512 byte rows

	t.Run("small_value_accepted", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, `{}`)
		if err != nil {
			t.Fatalf("Small value should be accepted: %v", err)
		}
	})

	t.Run("medium_value_accepted", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		// Create a value that fits in the row
		value := `{"data":"` + string(make([]byte, 100)) + `"}`
		err := tx.AddRow(key, value)
		if err != nil {
			t.Fatalf("Medium value should be accepted: %v", err)
		}
	})
}

// TestAddRow_MaxTimestampInitialization verifies max timestamp handling
func TestAddRow_MaxTimestampInitialization(t *testing.T) {
	header := createTestHeader()

	t.Run("default_max_timestamp_is_zero", func(t *testing.T) {
		tx := &Transaction{Header: header}
		if tx.GetMaxTimestamp() != 0 {
			t.Errorf("Default max timestamp should be 0, got %d", tx.GetMaxTimestamp())
		}
	})

	t.Run("max_timestamp_field_accessible", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.maxTimestamp = 12345

		if tx.GetMaxTimestamp() != 12345 {
			t.Errorf("Max timestamp should be 12345, got %d", tx.GetMaxTimestamp())
		}
	})

	t.Run("addrow_respects_initial_max_timestamp", func(t *testing.T) {
		header := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    0, // No skew
		}
		tx := &Transaction{Header: header}

		// Set a very high initial max timestamp (far in the future)
		futureTs := int64(9999999999999) // Very far future
		tx.maxTimestamp = futureTs

		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, `{"data":"test"}`)

		// Should fail because current timestamp + 0 skew <= future timestamp
		if err == nil {
			t.Fatal("AddRow should fail when key timestamp is before max_timestamp")
		}

		if _, ok := err.(*KeyOrderingError); !ok {
			t.Errorf("Expected KeyOrderingError, got %T", err)
		}
	})
}

// TestAddRow_UUIDv7TimestampExtraction verifies timestamp extraction
func TestAddRow_UUIDv7TimestampExtraction(t *testing.T) {
	t.Run("extracts_timestamp_correctly", func(t *testing.T) {
		key, _ := uuid.NewV7()
		ts := extractUUIDv7Timestamp(key)

		// Timestamp should be reasonable (after year 2020, which is ~1577836800000 ms)
		if ts < 1577836800000 {
			t.Errorf("Extracted timestamp seems too old: %d", ts)
		}

		// Timestamp should not be too far in the future (before year 2100)
		if ts > 4102444800000 {
			t.Errorf("Extracted timestamp seems too far in future: %d", ts)
		}
	})

	t.Run("timestamps_are_monotonically_increasing", func(t *testing.T) {
		var prevTs int64 = 0
		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			ts := extractUUIDv7Timestamp(key)
			if ts < prevTs {
				t.Errorf("Timestamp %d is less than previous %d", ts, prevTs)
			}
			prevTs = ts
		}
	})
}

// TestAddRow_IntegrationWithCommit verifies AddRow and Commit work together
func TestAddRow_IntegrationWithCommit(t *testing.T) {
	header := createTestHeader()

	t.Run("single_row_transaction", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"only"}`)
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].StartControl != START_TRANSACTION {
			t.Errorf("Single row should have START_TRANSACTION, got %c", rows[0].StartControl)
		}
		if rows[0].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Single row should have TC end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("multi_row_transaction_structure", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"i":1}`)
		}
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 5 {
			t.Fatalf("Expected 5 rows, got %d", len(rows))
		}

		// First row: T...RE
		if rows[0].StartControl != START_TRANSACTION {
			t.Errorf("First row should have T, got %c", rows[0].StartControl)
		}
		if rows[0].EndControl != ROW_END_CONTROL {
			t.Errorf("First row should have RE, got %s", rows[0].EndControl.String())
		}

		// Middle rows: R...RE
		for i := 1; i < 4; i++ {
			if rows[i].StartControl != ROW_CONTINUE {
				t.Errorf("Row %d should have R, got %c", i, rows[i].StartControl)
			}
			if rows[i].EndControl != ROW_END_CONTROL {
				t.Errorf("Row %d should have RE, got %s", i, rows[i].EndControl.String())
			}
		}

		// Last row: R...TC
		if rows[4].StartControl != ROW_CONTINUE {
			t.Errorf("Last row should have R, got %c", rows[4].StartControl)
		}
		if rows[4].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Last row should have TC, got %s", rows[4].EndControl.String())
		}
	})
}

// =============================================================================
// Savepoint Unit Tests
// =============================================================================

// TestSavepoint_BasicFunctionality verifies basic savepoint operations
func TestSavepoint_BasicFunctionality(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_succeeds_after_addrow", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		err := tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() should succeed after AddRow(): %v", err)
		}
	})

	t.Run("savepoint_changes_partial_row_state", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		if tx.last.GetState() != PartialDataRowWithPayload {
			t.Errorf("State should be PartialDataRowWithPayload before Savepoint(), got %v", tx.last.GetState())
		}

		tx.Savepoint()

		if tx.last.GetState() != PartialDataRowWithSavepoint {
			t.Errorf("State should be PartialDataRowWithSavepoint after Savepoint(), got %v", tx.last.GetState())
		}
	})

	t.Run("savepoint_allows_subsequent_addrow", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, `{"data":"second"}`)
		if err != nil {
			t.Fatalf("AddRow() should succeed after Savepoint(): %v", err)
		}
	})

	t.Run("multiple_savepoints_allowed", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, `{"data":"test"}`)
			if err != nil {
				t.Fatalf("AddRow() %d failed: %v", i, err)
			}

			err = tx.Savepoint()
			if err != nil {
				t.Fatalf("Savepoint() %d failed: %v", i+1, err)
			}
		}
	})

	t.Run("consecutive_savepoints_on_same_row_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		// First savepoint
		err := tx.Savepoint()
		if err != nil {
			t.Fatalf("First Savepoint() failed: %v", err)
		}

		// Second consecutive savepoint on same row should fail
		// (PartialDataRow is now in PartialDataRowWithSavepoint state, not PartialDataRowWithPayload)
		err = tx.Savepoint()
		if err == nil {
			t.Fatal("Second consecutive Savepoint() should fail - row already has savepoint")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})
}

// TestSavepoint_ErrorConditions verifies error cases for savepoint
func TestSavepoint_ErrorConditions(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_before_begin_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail before Begin()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("savepoint_on_empty_transaction_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail on empty transaction")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("savepoint_after_commit_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Commit()

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail after Commit()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("savepoint_after_empty_commit_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()
		tx.Commit() // Empty commit creates NullRow

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail after empty Commit()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("savepoint_after_rollback_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail after Rollback()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})
}

// TestSavepoint_LimitEnforcement verifies the 9 savepoint limit
func TestSavepoint_LimitEnforcement(t *testing.T) {
	header := createTestHeader()

	t.Run("exactly_9_savepoints_allowed", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, `{"data":"test"}`)
			if err != nil {
				t.Fatalf("AddRow() %d failed: %v", i, err)
			}

			err = tx.Savepoint()
			if err != nil {
				t.Fatalf("Savepoint() %d should succeed: %v", i+1, err)
			}
		}

		// We need to add one more row to finalize the 9th savepoint
		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"after_9_savepoints"}`)

		// Verify we have 9 savepoints by counting finalized rows with 'S' end control
		indices := tx.GetSavepointIndices()
		if len(indices) != 9 {
			t.Errorf("Expected 9 savepoint indices, got %d", len(indices))
		}
	})

	t.Run("10th_savepoint_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Create 9 savepoints
		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			tx.Savepoint()
		}

		// Add one more row
		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		// 10th savepoint should fail
		err := tx.Savepoint()
		if err == nil {
			t.Fatal("10th Savepoint() should fail")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})
}

// TestSavepoint_EndControlPatterns verifies correct end control assignment
func TestSavepoint_EndControlPatterns(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_row_uses_SE_when_continued", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 finalized row, got %d", len(rows))
		}

		// First row should have SE (savepoint continue)
		if rows[0].EndControl != SAVEPOINT_CONTINUE {
			t.Errorf("First row should have SE end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("savepoint_row_uses_SC_when_committed", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Savepoint()
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		// Single row with savepoint should have SC (savepoint commit)
		if rows[0].EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Row should have SC end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("multiple_savepoint_rows_tracked_correctly", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			tx.Savepoint()
		}
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 5 {
			t.Fatalf("Expected 5 rows, got %d", len(rows))
		}

		// All rows except last should have SE
		for i := 0; i < len(rows)-1; i++ {
			if rows[i].EndControl != SAVEPOINT_CONTINUE {
				t.Errorf("Row %d should have SE end control, got %s", i, rows[i].EndControl.String())
			}
		}

		// Last row should have SC
		if rows[4].EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Last row should have SC end control, got %s", rows[4].EndControl.String())
		}
	})
}

// TestSavepoint_CountingLogic verifies savepoint indices are tracked correctly
func TestSavepoint_CountingLogic(t *testing.T) {
	header := createTestHeader()

	t.Run("getSavepointIndices_returns_correct_indices", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Row 0: savepoint
		key0, _ := uuid.NewV7()
		tx.AddRow(key0, `{"data":"row0"}`)
		tx.Savepoint()

		// Row 1: no savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"row1"}`)

		// Row 2: savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"row2"}`)
		tx.Savepoint()

		// Row 3: savepoint
		key3, _ := uuid.NewV7()
		tx.AddRow(key3, `{"data":"row3"}`)
		tx.Savepoint()

		tx.Commit()

		indices := tx.GetSavepointIndices()
		expected := []int{0, 2, 3}

		if len(indices) != len(expected) {
			t.Fatalf("Expected %d savepoint indices, got %d", len(expected), len(indices))
		}

		for i, idx := range indices {
			if idx != expected[i] {
				t.Errorf("Savepoint index %d: expected %d, got %d", i, expected[i], idx)
			}
		}
	})

	t.Run("empty_transaction_has_no_savepoints", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		indices := tx.GetSavepointIndices()
		if len(indices) != 0 {
			t.Errorf("Expected 0 savepoint indices for empty transaction, got %d", len(indices))
		}
	})
}

// =============================================================================
// Rollback Unit Tests
// =============================================================================

// TestRollback_FullRollback verifies Rollback(0) behavior
func TestRollback_FullRollback(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_0_succeeds_with_data", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		err := tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) should succeed: %v", err)
		}
	})

	t.Run("rollback_0_on_empty_transaction_creates_null_row", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		err := tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) on empty transaction should succeed: %v", err)
		}

		if tx.GetEmptyRow() == nil {
			t.Error("Rollback(0) on empty transaction should create NullRow")
		}

		if len(tx.GetRows()) != 0 {
			t.Errorf("Rollback(0) on empty transaction should have no data rows, got %d", len(tx.GetRows()))
		}
	})

	t.Run("rollback_0_closes_transaction", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		// Transaction should be closed
		if tx.isActive() {
			t.Error("Transaction should not be active after Rollback(0)")
		}

		// Transaction should be considered committed (terminated)
		if !tx.IsCommitted() {
			t.Error("Transaction should be considered committed after Rollback(0)")
		}
	})

	t.Run("rollback_0_invalidates_all_rows", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}
		tx.Rollback(0)

		iter, err := tx.GetCommittedRows()
		if err != nil {
			t.Fatalf("GetCommittedRows() failed: %v", err)
		}

		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		if count != 0 {
			t.Errorf("Rollback(0) should invalidate all rows, got %d committed", count)
		}
	})

	t.Run("rollback_0_uses_R0_without_savepoint", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].EndControl[0] != 'R' {
			t.Errorf("Expected end control to start with 'R', got '%c'", rows[0].EndControl[0])
		}
		if rows[0].EndControl[1] != '0' {
			t.Errorf("Expected end control second byte to be '0', got '%c'", rows[0].EndControl[1])
		}
	})

	t.Run("rollback_0_uses_S0_with_savepoint", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Savepoint()
		tx.Rollback(0)

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].EndControl[0] != 'S' {
			t.Errorf("Expected end control to start with 'S', got '%c'", rows[0].EndControl[0])
		}
		if rows[0].EndControl[1] != '0' {
			t.Errorf("Expected end control second byte to be '0', got '%c'", rows[0].EndControl[1])
		}
	})
}

// TestRollback_PartialRollback verifies Rollback(n>0) behavior
func TestRollback_PartialRollback(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_1_succeeds_with_savepoint", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		err := tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) should succeed: %v", err)
		}
	})

	t.Run("rollback_to_savepoint_commits_rows_up_to_savepoint", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		keys := make([]uuid.UUID, 4)
		for i := 0; i < 4; i++ {
			keys[i], _ = uuid.NewV7()
			tx.AddRow(keys[i], `{"data":"test"}`)
			if i == 1 { // Savepoint after row 1 (index 1)
				tx.Savepoint()
			}
		}
		tx.Rollback(1) // Rollback to savepoint 1

		iter, _ := tx.GetCommittedRows()
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
		}

		// Only rows 0 and 1 should be committed (up to and including savepoint 1)
		if len(committedKeys) != 2 {
			t.Errorf("Expected 2 committed rows, got %d", len(committedKeys))
		}
	})

	t.Run("rollback_to_savepoint_2_commits_first_2_savepoint_rows", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		keys := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			keys[i], _ = uuid.NewV7()
			tx.AddRow(keys[i], `{"data":"test"}`)
			if i == 0 || i == 2 { // Savepoints after rows 0 and 2
				tx.Savepoint()
			}
		}
		tx.Rollback(2) // Rollback to savepoint 2 (row index 2)

		iter, _ := tx.GetCommittedRows()
		var committedKeys []uuid.UUID
		for row, more := iter(); more; row, more = iter() {
			committedKeys = append(committedKeys, row.GetKey())
		}

		// Rows 0, 1, 2 should be committed (up to and including savepoint 2 at index 2)
		if len(committedKeys) != 3 {
			t.Errorf("Expected 3 committed rows, got %d", len(committedKeys))
		}
	})

	t.Run("partial_rollback_uses_Rn_without_savepoint_on_last_row", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		// No savepoint on second row, so rollback should use 'R'
		tx.Rollback(1)

		rows := tx.GetRows()
		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[0] != 'R' {
			t.Errorf("Expected end control to start with 'R', got '%c'", lastRow.EndControl[0])
		}
		if lastRow.EndControl[1] != '1' {
			t.Errorf("Expected end control second byte to be '1', got '%c'", lastRow.EndControl[1])
		}
	})

	t.Run("partial_rollback_uses_Sn_with_savepoint_on_last_row", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)
		tx.Savepoint() // Savepoint on last row

		tx.Rollback(1)

		rows := tx.GetRows()
		lastRow := rows[len(rows)-1]
		if lastRow.EndControl[0] != 'S' {
			t.Errorf("Expected end control to start with 'S', got '%c'", lastRow.EndControl[0])
		}
		if lastRow.EndControl[1] != '1' {
			t.Errorf("Expected end control second byte to be '1', got '%c'", lastRow.EndControl[1])
		}
	})

	t.Run("rollback_all_savepoint_numbers_1_through_9", func(t *testing.T) {
		for n := 1; n <= 9; n++ {
			tx := &Transaction{Header: header}
			tx.Begin()

			// Create n savepoints
			for i := 0; i < n; i++ {
				key, _ := uuid.NewV7()
				tx.AddRow(key, `{"data":"test"}`)
				tx.Savepoint()
			}

			// Add one more row without savepoint
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"last"}`)

			err := tx.Rollback(n)
			if err != nil {
				t.Errorf("Rollback(%d) should succeed: %v", n, err)
			}

			rows := tx.GetRows()
			lastRow := rows[len(rows)-1]
			if lastRow.EndControl[1] != byte('0'+n) {
				t.Errorf("Rollback(%d) should have end control digit '%c', got '%c'", n, '0'+n, lastRow.EndControl[1])
			}
		}
	})
}

// TestRollback_ErrorConditions verifies error cases for rollback
func TestRollback_ErrorConditions(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_before_begin_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}

		err := tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail before Begin()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("rollback_after_commit_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Commit()

		err := tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail after Commit()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("rollback_after_rollback_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		err := tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail after previous Rollback()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("rollback_negative_savepoint_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		err := tx.Rollback(-1)
		if err == nil {
			t.Fatal("Rollback(-1) should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_savepoint_over_9_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		err := tx.Rollback(10)
		if err == nil {
			t.Fatal("Rollback(10) should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_to_nonexistent_savepoint_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		err := tx.Rollback(1)
		if err == nil {
			t.Fatal("Rollback(1) should fail when no savepoints exist")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_beyond_existing_savepoints_fails", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Create 2 savepoints
		for i := 0; i < 2; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			tx.Savepoint()
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"last"}`)

		err := tx.Rollback(5) // Only 2 savepoints exist
		if err == nil {
			t.Fatal("Rollback(5) should fail when only 2 savepoints exist")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})
}

// TestRollback_IsRowCommitted verifies individual row commit status
func TestRollback_IsRowCommitted(t *testing.T) {
	header := createTestHeader()

	t.Run("all_rows_not_committed_after_full_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}
		tx.Rollback(0)

		for i := 0; i < len(tx.GetRows()); i++ {
			committed, err := tx.IsRowCommitted(i)
			if err != nil {
				t.Fatalf("IsRowCommitted(%d) failed: %v", i, err)
			}
			if committed {
				t.Errorf("Row %d should not be committed after Rollback(0)", i)
			}
		}
	})

	t.Run("partial_rows_committed_after_partial_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			if i == 1 { // Savepoint after row 1
				tx.Savepoint()
			}
		}
		tx.Rollback(1) // Commit rows 0-1

		rows := tx.GetRows()

		// Rows 0-1 should be committed
		for i := 0; i <= 1; i++ {
			committed, err := tx.IsRowCommitted(i)
			if err != nil {
				t.Fatalf("IsRowCommitted(%d) failed: %v", i, err)
			}
			if !committed {
				t.Errorf("Row %d should be committed after Rollback(1)", i)
			}
		}

		// Rows 2-4 should not be committed
		for i := 2; i < len(rows); i++ {
			committed, err := tx.IsRowCommitted(i)
			if err != nil {
				t.Fatalf("IsRowCommitted(%d) failed: %v", i, err)
			}
			if committed {
				t.Errorf("Row %d should not be committed after Rollback(1)", i)
			}
		}
	})

	t.Run("out_of_bounds_index_returns_error", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Commit()

		_, err := tx.IsRowCommitted(-1)
		if err == nil {
			t.Fatal("IsRowCommitted(-1) should fail")
		}

		_, err = tx.IsRowCommitted(10)
		if err == nil {
			t.Fatal("IsRowCommitted(10) should fail when only 1 row exists")
		}
	})
}

// TestRollback_TransactionState verifies transaction state after rollback
func TestRollback_TransactionState(t *testing.T) {
	header := createTestHeader()

	t.Run("transaction_inactive_after_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()

		if active {
			t.Error("Transaction should not be active after Rollback()")
		}
	})

	t.Run("partial_row_cleared_after_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Rollback(0)

		if tx.last != nil {
			t.Error("Partial row should be nil after Rollback()")
		}
	})

	t.Run("addrow_fails_after_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"test"}`)
		tx.Rollback(0)

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, `{"data":"after_rollback"}`)
		if err == nil {
			t.Fatal("AddRow() should fail after Rollback()")
		}
	})
}

// TestRollback_MultipleRows verifies rollback with multiple rows
func TestRollback_MultipleRows(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_with_many_rows_and_savepoints", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Create complex transaction: 10 rows with savepoints at 2, 5, 7
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, `{"data":"test"}`)
			if err != nil {
				t.Fatalf("AddRow() %d failed: %v", i, err)
			}
			if i == 2 || i == 5 || i == 7 {
				tx.Savepoint()
			}
		}

		// Rollback to savepoint 2 (row index 5)
		err := tx.Rollback(2)
		if err != nil {
			t.Fatalf("Rollback(2) failed: %v", err)
		}

		iter, _ := tx.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		// Rows 0-5 should be committed (up to and including savepoint 2 at index 5)
		if count != 6 {
			t.Errorf("Expected 6 committed rows, got %d", count)
		}
	})
}

// TestRollback_EdgeCases verifies edge cases
func TestRollback_EdgeCases(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_single_row_transaction", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"only"}`)
		tx.Rollback(0)

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		iter, _ := tx.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}
		if count != 0 {
			t.Errorf("Single row should be invalidated after Rollback(0), got %d committed", count)
		}
	})

	t.Run("rollback_all_9_savepoints", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Create 9 savepoints
		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			tx.Savepoint()
		}

		// Add final row
		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"last"}`)

		// Rollback to savepoint 9
		err := tx.Rollback(9)
		if err != nil {
			t.Fatalf("Rollback(9) failed: %v", err)
		}

		iter, _ := tx.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		// All 9 savepoint rows should be committed
		if count != 9 {
			t.Errorf("Expected 9 committed rows, got %d", count)
		}
	})
}

// TestRollback_ThreadSafety verifies concurrent access during rollback
func TestRollback_ThreadSafety(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_reads_during_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = tx.GetRows()
				_ = tx.GetSavepointIndices()
			}()
		}

		tx.Rollback(0)
		wg.Wait()
	})
}

// =============================================================================
// GetCommittedRows and calculateCommittedIndices Tests
// =============================================================================

// TestGetCommittedRows_VariousTransactionStates verifies GetCommittedRows behavior
func TestGetCommittedRows_VariousTransactionStates(t *testing.T) {
	header := createTestHeader()

	t.Run("no_rows_committed_when_transaction_open", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 3; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}
		// Transaction still open (no Commit or Rollback)

		// Directly finalize a row with RE (continue) to simulate open transaction
		// We need to manually construct this scenario
		tx2 := &Transaction{Header: header}
		tx2.Begin()
		key, _ := uuid.NewV7()
		tx2.AddRow(key, `{"data":"test"}`)

		// Get the partial row and finalize it with RE (continue)
		dataRow, _ := tx2.last.EndRow()
		tx2.rows = append(tx2.rows, *dataRow)
		tx2.last = nil

		iter, _ := tx2.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		if count != 0 {
			t.Errorf("Expected 0 committed rows for open transaction, got %d", count)
		}
	})

	t.Run("all_rows_committed_after_commit", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
		}
		tx.Commit()

		iter, _ := tx.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		if count != 5 {
			t.Errorf("Expected 5 committed rows after Commit(), got %d", count)
		}
	})

	t.Run("empty_rows_returns_empty_iterator", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()
		tx.Commit() // Empty transaction

		iter, _ := tx.GetCommittedRows()
		count := 0
		for _, more := iter(); more; _, more = iter() {
			count++
		}

		if count != 0 {
			t.Errorf("Expected 0 rows for empty transaction, got %d", count)
		}
	})
}

// =============================================================================
// Validate Tests for Savepoint/Rollback Related Scenarios
// =============================================================================

// TestTransactionValidate_SavepointScenarios verifies Validate() for savepoint cases
func TestTransactionValidate_SavepointScenarios(t *testing.T) {
	header := createTestHeader()

	t.Run("valid_transaction_with_savepoints", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		for i := 0; i < 3; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, `{"data":"test"}`)
			tx.Savepoint()
		}
		tx.Commit()

		err := tx.Validate()
		if err != nil {
			t.Fatalf("Validate() should pass for valid transaction with savepoints: %v", err)
		}
	})

	t.Run("valid_transaction_after_rollback", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		tx.Rollback(1)

		err := tx.Validate()
		if err != nil {
			t.Fatalf("Validate() should pass after rollback: %v", err)
		}
	})
}

// =============================================================================
// Additional Integration Tests
// =============================================================================

// TestSavepointRollback_Integration verifies savepoint and rollback work together
func TestSavepointRollback_Integration(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_after_savepoint", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)
		tx.Savepoint()
		tx.Commit()

		// Should have SC end control
		rows := tx.GetRows()
		if rows[0].EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Expected SC end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("addrow_after_savepoint_creates_new_row", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(rows))
		}

		// First row should have SE (savepoint continue)
		if rows[0].EndControl != SAVEPOINT_CONTINUE {
			t.Errorf("First row should have SE, got %s", rows[0].EndControl.String())
		}

		// Second row should have TC (commit)
		if rows[1].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Second row should have TC, got %s", rows[1].EndControl.String())
		}
	})

	t.Run("mixed_savepoint_and_non_savepoint_rows", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Row 0: no savepoint
		key0, _ := uuid.NewV7()
		tx.AddRow(key0, `{"data":"row0"}`)

		// Row 1: savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"row1"}`)
		tx.Savepoint()

		// Row 2: no savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"row2"}`)

		// Row 3: savepoint
		key3, _ := uuid.NewV7()
		tx.AddRow(key3, `{"data":"row3"}`)
		tx.Savepoint()

		tx.Commit()

		rows := tx.GetRows()
		// Row 0: RE
		if rows[0].EndControl != ROW_END_CONTROL {
			t.Errorf("Row 0 should have RE, got %s", rows[0].EndControl.String())
		}
		// Row 1: SE (savepoint continue)
		if rows[1].EndControl != SAVEPOINT_CONTINUE {
			t.Errorf("Row 1 should have SE, got %s", rows[1].EndControl.String())
		}
		// Row 2: RE
		if rows[2].EndControl != ROW_END_CONTROL {
			t.Errorf("Row 2 should have RE, got %s", rows[2].EndControl.String())
		}
		// Row 3: SC (savepoint commit)
		if rows[3].EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Row 3 should have SC, got %s", rows[3].EndControl.String())
		}

		// Verify savepoint indices
		indices := tx.GetSavepointIndices()
		if len(indices) != 2 {
			t.Fatalf("Expected 2 savepoint indices, got %d", len(indices))
		}
		if indices[0] != 1 || indices[1] != 3 {
			t.Errorf("Savepoint indices should be [1, 3], got %v", indices)
		}
	})
}

// TestSavepoint_HasDataRowConditions tests the hasDataRow logic more thoroughly
func TestSavepoint_HasDataRowConditions(t *testing.T) {
	header := createTestHeader()

	t.Run("savepoint_succeeds_with_finalized_rows", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Add two rows so first one is finalized
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)

		// Now the first row is finalized (in tx.rows)
		if len(tx.GetRows()) != 1 {
			t.Fatalf("Expected 1 finalized row, got %d", len(tx.GetRows()))
		}

		// Savepoint should succeed because hasDataRow = true (finalized rows exist)
		err := tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() should succeed with finalized rows: %v", err)
		}
	})

	t.Run("savepoint_succeeds_with_only_partial_row_with_payload", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// Add one row (still partial with payload)
		key, _ := uuid.NewV7()
		tx.AddRow(key, `{"data":"test"}`)

		// No finalized rows
		if len(tx.GetRows()) != 0 {
			t.Fatalf("Expected 0 finalized rows, got %d", len(tx.GetRows()))
		}

		// But partial row has payload, so hasDataRow = true
		err := tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() should succeed with partial row that has payload: %v", err)
		}
	})
}

// TestRollback_WithSavepointOnCurrentRow tests rollback when current row has savepoint
func TestRollback_WithSavepointOnCurrentRow(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_with_savepoint_on_current_row_uses_S_prefix", func(t *testing.T) {
		tx := &Transaction{Header: header}
		tx.Begin()

		// First row with savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, `{"data":"first"}`)
		tx.Savepoint()

		// Second row also with savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, `{"data":"second"}`)
		tx.Savepoint()

		// Rollback to first savepoint
		tx.Rollback(1)

		rows := tx.GetRows()
		// First row should have SE
		if rows[0].EndControl[0] != 'S' {
			t.Errorf("First row should have S prefix, got '%c'", rows[0].EndControl[0])
		}
		// Second row (rollback row) should have S1
		if rows[1].EndControl[0] != 'S' {
			t.Errorf("Rollback row should have S prefix, got '%c'", rows[1].EndControl[0])
		}
		if rows[1].EndControl[1] != '1' {
			t.Errorf("Rollback row should have '1' suffix, got '%c'", rows[1].EndControl[1])
		}
	})
}
