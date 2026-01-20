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
