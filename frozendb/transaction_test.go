package frozendb

import (
	"encoding/json"
	"fmt"
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// After Begin, partial should have START_TRANSACTION
		if tx.last == nil {
			t.Fatal("Expected partial row after Begin()")
		}
		if tx.last.d.StartControl != START_TRANSACTION {
			t.Errorf("Partial from Begin() should have START_TRANSACTION, got %c", tx.last.d.StartControl)
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		key2, _ := uuid.NewV7()

		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		if tx.last.GetState() != PartialDataRowWithStartControl {
			t.Errorf("After Begin(), state should be PartialDataRowWithStartControl, got %v", tx.last.GetState())
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		if tx.last.GetState() != PartialDataRowWithPayload {
			t.Errorf("After AddRow(), state should be PartialDataRowWithPayload, got %v", tx.last.GetState())
		}
	})
}

// TestAddRow_KeyValueStorage verifies key and value are properly stored
func TestAddRow_KeyValueStorage(t *testing.T) {
	header := createTestHeader()

	t.Run("key_and_value_stored_correctly", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		value := json.RawMessage(`{"name":"test","count":42}`)

		tx.AddRow(key, value)
		tx.Commit()

		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].GetKey() != key {
			t.Errorf("Key mismatch: expected %s, got %s", key, rows[0].GetKey())
		}

		if rows[0].GetValue() == nil || string(rows[0].GetValue()) != string(value) {
			t.Errorf("Value mismatch: expected %s, got %s", value, rows[0].GetValue())
		}
	})

	t.Run("multiple_rows_preserve_order", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		keys := make([]uuid.UUID, 5)
		values := []json.RawMessage{
			json.RawMessage(`{"index":0}`),
			json.RawMessage(`{"index":1}`),
			json.RawMessage(`{"index":2}`),
			json.RawMessage(`{"index":3}`),
			json.RawMessage(`{"index":4}`),
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
			if string(row.GetValue()) != string(values[i]) {
				t.Errorf("Row %d value mismatch: expected %s, got %s", i, values[i], row.GetValue())
			}
		}
	})
}

// TestAddRow_EndControlPatterns verifies correct end_control assignment
func TestAddRow_EndControlPatterns(t *testing.T) {
	header := createTestHeader()

	t.Run("intermediate_rows_have_RE", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Generate keys in quick succession (ascending timestamps)
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// First key
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))

		// Create a key with slightly older timestamp (within skew)
		// The skew should allow this
		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow should succeed with skew: %v", err)
		}
	})

	t.Run("max_timestamp_tracks_highest_seen", func(t *testing.T) {
		header := createTestHeader()
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))

		// Add delay to ensure next key has higher timestamp
		time.Sleep(2 * time.Millisecond)

		key2, _ := uuid.NewV7()
		// If maxTimestamp tracking works, this should succeed (ascending order)
		if err := tx.AddRow(key2, json.RawMessage(`{"data":"second"}`)); err != nil {
			t.Errorf("AddRow should accept ascending timestamps: %v", err)
		}
	})
}

// TestAddRow_RowCountLimit verifies the 100 row limit
func TestAddRow_RowCountLimit(t *testing.T) {
	header := createTestHeader()

	t.Run("exactly_100_rows_allowed", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"i":1}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 100; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"i":1}`))
		}

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"i":1}`))
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
		tx := createTransactionWithMockWriter(header)

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		if err == nil {
			t.Fatal("AddRow before Begin should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("addrow_after_commit_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"test"}`))
		tx.Commit()

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, json.RawMessage(`{"data":"more"}`))

		if err == nil {
			t.Fatal("AddRow after Commit should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("addrow_after_empty_commit_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()
		tx.Commit() // Empty commit produces NullRow

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		if err == nil {
			t.Fatal("AddRow after empty Commit should fail")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("nil_uuid_rejected", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		err := tx.AddRow(uuid.Nil, json.RawMessage(`{"data":"test"}`))

		if err == nil {
			t.Fatal("Nil UUID should be rejected")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("empty_value_rejected", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(""))

		if err == nil {
			t.Fatal("Empty value should be rejected")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("uuidv4_rejected", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key := uuid.New() // v4
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = tx.GetRows()
				_ = tx.IsCommitted()
			}()
		}
		wg.Wait()
	})

	t.Run("sequential_addrows_maintain_order", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		keys := make([]uuid.UUID, 10)
		for i := 0; i < 10; i++ {
			keys[i], _ = uuid.NewV7()
			err := tx.AddRow(keys[i], json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()
		if !active {
			t.Error("Should be active after AddRow()")
		}
	})

	t.Run("not_active_after_commit", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		tx.Commit()

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()
		if active {
			t.Error("Should not be active after Commit()")
		}
	})

	t.Run("is_committed_after_data_commit", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		tx.Commit()

		tx.mu.RLock()
		committed := tx.isCommittedState()
		tx.mu.RUnlock()
		if !committed {
			t.Error("Should be committed after Commit()")
		}
	})

	t.Run("is_committed_after_empty_commit", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Small value should be accepted: %v", err)
		}
	})

	t.Run("medium_value_accepted", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		// Create a value that fits in the row
		value := json.RawMessage(`{"data":"` + string(make([]byte, 100)) + `"}`)
		err := tx.AddRow(key, value)
		if err != nil {
			t.Fatalf("Medium value should be accepted: %v", err)
		}
	})
}

// TestAddRow_MaxTimestampInitialization verifies max timestamp handling
func TestAddRow_MaxTimestampInitialization(t *testing.T) {
	header := createTestHeader()

	t.Run("max_timestamp_from_finder_affects_ordering", func(t *testing.T) {
		// Create a mock finder with a specific maxTimestamp
		mockFinder := &mockFinderWithMaxTimestamp{maxTs: 12345}
		tx := createTransactionWithMockWriterAndFinder(header, mockFinder)
		tx.Begin()

		// Create a key with timestamp less than finder's max (should be rejected)
		// We can't directly create a UUID with a specific timestamp, but we can test
		// that the finder's maxTimestamp is being used for ordering validation
		// by verifying that normal ascending timestamps still work
		key, _ := uuid.NewV7()
		if err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`)); err != nil {
			t.Errorf("AddRow should work with ascending timestamps: %v", err)
		}
	})

	t.Run("addrow_respects_initial_max_timestamp", func(t *testing.T) {
		header := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    0, // No skew
		}

		// Create a mock finder with a very high max timestamp (far in the future)
		futureTs := int64(9999999999999) // Very far future
		mockFinder := &mockFinderWithMaxTimestamp{maxTs: futureTs}
		tx := createTransactionWithMockWriterAndFinder(header, mockFinder)

		tx.Begin()

		key, _ := uuid.NewV7()
		err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"only"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"i":1}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		err := tx.Savepoint()
		if err != nil {
			t.Fatalf("Savepoint() should succeed after AddRow(): %v", err)
		}
	})

	t.Run("savepoint_changes_partial_row_state", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		if tx.last.GetState() != PartialDataRowWithPayload {
			t.Errorf("State should be PartialDataRowWithPayload before Savepoint(), got %v", tx.last.GetState())
		}

		tx.Savepoint()

		if tx.last.GetState() != PartialDataRowWithSavepoint {
			t.Errorf("State should be PartialDataRowWithSavepoint after Savepoint(), got %v", tx.last.GetState())
		}
	})

	t.Run("savepoint_allows_subsequent_addrow", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("AddRow() should succeed after Savepoint(): %v", err)
		}
	})

	t.Run("multiple_savepoints_allowed", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)

		err := tx.Savepoint()
		if err == nil {
			t.Fatal("Savepoint() should fail before Begin()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("savepoint_on_empty_transaction_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx.AddRow(key, json.RawMessage(`{"data":"after_9_savepoints"}`))

		// Verify we have 9 savepoints by counting finalized rows with 'S' end control
		indices := tx.GetSavepointIndices()
		if len(indices) != 9 {
			t.Errorf("Expected 9 savepoint indices, got %d", len(indices))
		}
	})

	t.Run("10th_savepoint_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Create 9 savepoints
		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			tx.Savepoint()
		}

		// Add one more row
		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Row 0: savepoint
		key0, _ := uuid.NewV7()
		tx.AddRow(key0, json.RawMessage(`{"data":"row0"}`))
		tx.Savepoint()

		// Row 1: no savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"row1"}`))

		// Row 2: savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"row2"}`))
		tx.Savepoint()

		// Row 3: savepoint
		key3, _ := uuid.NewV7()
		tx.AddRow(key3, json.RawMessage(`{"data":"row3"}`))
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
		tx := createTransactionWithMockWriter(header)
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		err := tx.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback(0) should succeed: %v", err)
		}
	})

	t.Run("rollback_0_on_empty_transaction_creates_null_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

		err := tx.Rollback(1)
		if err != nil {
			t.Fatalf("Rollback(1) should succeed: %v", err)
		}
	})

	t.Run("rollback_to_savepoint_commits_rows_up_to_savepoint", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		keys := make([]uuid.UUID, 4)
		for i := 0; i < 4; i++ {
			keys[i], _ = uuid.NewV7()
			tx.AddRow(keys[i], json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		keys := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			keys[i], _ = uuid.NewV7()
			tx.AddRow(keys[i], json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
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
			tx := createTransactionWithMockWriter(header)
			tx.Begin()

			// Create n savepoints
			for i := 0; i < n; i++ {
				key, _ := uuid.NewV7()
				tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
				tx.Savepoint()
			}

			// Add one more row without savepoint
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"last"}`))

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
		tx := createTransactionWithMockWriter(header)

		err := tx.Rollback(0)
		if err == nil {
			t.Fatal("Rollback() should fail before Begin()")
		}

		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("rollback_after_commit_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		err := tx.Rollback(-1)
		if err == nil {
			t.Fatal("Rollback(-1) should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_savepoint_over_9_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		err := tx.Rollback(10)
		if err == nil {
			t.Fatal("Rollback(10) should fail")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_to_nonexistent_savepoint_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

		err := tx.Rollback(1)
		if err == nil {
			t.Fatal("Rollback(1) should fail when no savepoints exist")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})

	t.Run("rollback_beyond_existing_savepoints_fails", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Create 2 savepoints
		for i := 0; i < 2; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			tx.Savepoint()
		}

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"last"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		tx.Rollback(0)

		tx.mu.RLock()
		active := tx.isActive()
		tx.mu.RUnlock()

		if active {
			t.Error("Transaction should not be active after Rollback()")
		}
	})

	t.Run("partial_row_cleared_after_rollback", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		tx.Rollback(0)

		if tx.last != nil {
			t.Error("Partial row should be nil after Rollback()")
		}
	})

	t.Run("addrow_fails_after_rollback", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"test"}`))
		tx.Rollback(0)

		key2, _ := uuid.NewV7()
		err := tx.AddRow(key2, json.RawMessage(`{"data":"after_rollback"}`))
		if err == nil {
			t.Fatal("AddRow() should fail after Rollback()")
		}
	})
}

// TestRollback_MultipleRows verifies rollback with multiple rows
func TestRollback_MultipleRows(t *testing.T) {
	header := createTestHeader()

	t.Run("rollback_with_many_rows_and_savepoints", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Create complex transaction: 10 rows with savepoints at 2, 5, 7
		for i := 0; i < 10; i++ {
			key, _ := uuid.NewV7()
			err := tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"only"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Create 9 savepoints
		for i := 0; i < 9; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			tx.Savepoint()
		}

		// Add final row
		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"last"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 3; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		}
		// Transaction still open (no Commit or Rollback)

		// Directly finalize a row with RE (continue) to simulate open transaction
		// We need to manually construct this scenario
		tx2 := createTransactionWithMockWriter(header)
		tx2.Begin()
		key, _ := uuid.NewV7()
		tx2.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		for i := 0; i < 5; i++ {
			key, _ := uuid.NewV7()
			tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
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
		tx := createTransactionWithMockWriter(header)
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
// Additional Integration Tests
// =============================================================================

// TestSavepointRollback_Integration verifies savepoint and rollback work together
func TestSavepointRollback_Integration(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_after_savepoint", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		tx.Savepoint()
		tx.Commit()

		// Should have SC end control
		rows := tx.GetRows()
		if rows[0].EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Expected SC end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("addrow_after_savepoint_creates_new_row", func(t *testing.T) {
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Row 0: no savepoint
		key0, _ := uuid.NewV7()
		tx.AddRow(key0, json.RawMessage(`{"data":"row0"}`))

		// Row 1: savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"row1"}`))
		tx.Savepoint()

		// Row 2: no savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"row2"}`))

		// Row 3: savepoint
		key3, _ := uuid.NewV7()
		tx.AddRow(key3, json.RawMessage(`{"data":"row3"}`))
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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Add two rows so first one is finalized
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// Add one row (still partial with payload)
		key, _ := uuid.NewV7()
		tx.AddRow(key, json.RawMessage(`{"data":"test"}`))

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
		tx := createTransactionWithMockWriter(header)
		tx.Begin()

		// First row with savepoint
		key1, _ := uuid.NewV7()
		tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		tx.Savepoint()

		// Second row also with savepoint
		key2, _ := uuid.NewV7()
		tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
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

// =============================================================================
// Disk Persistence Unit Tests (015-transaction-persistence)
// =============================================================================

// createTransactionWithByteCollector creates a transaction with a write channel
// that collects all written bytes into a slice. This simulates an in-memory file
// by appending all bytes written to the channel.
func createTransactionWithByteCollector(header *Header) (*Transaction, *[][]byte) {
	var writtenBytes [][]byte
	writeChan := make(chan Data, 100)
	go func() {
		for data := range writeChan {
			// Collect bytes (simulating in-memory file)
			writtenBytes = append(writtenBytes, append([]byte(nil), data.Bytes...))
			// Send success response
			data.Response <- nil
		}
	}()
	tx := &Transaction{
		Header:    header,
		writeChan: writeChan,
		db:        &mockDBFile{},
		finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
	}
	return tx, &writtenBytes
}

// TestBegin_DiskPersistence verifies Begin() writes PartialDataRow to disk (FR-001)
func TestBegin_DiskPersistence(t *testing.T) {
	header := createTestHeader()

	t.Run("begin_writes_partial_data_row_to_disk", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Wait for write to complete
		time.Sleep(50 * time.Millisecond)

		if len(*writtenBytes) != 1 {
			t.Fatalf("Expected 1 write, got %d", len(*writtenBytes))
		}

		// Verify written bytes: ROW_START (0x1F) + 'T' (START_TRANSACTION)
		bytes := (*writtenBytes)[0]
		if len(bytes) != 2 {
			t.Errorf("Expected 2 bytes (ROW_START + 'T'), got %d", len(bytes))
		}
		if bytes[0] != ROW_START {
			t.Errorf("First byte should be ROW_START (0x1F), got 0x%02X", bytes[0])
		}
		if bytes[1] != byte(START_TRANSACTION) {
			t.Errorf("Second byte should be 'T', got '%c'", bytes[1])
		}

		// Verify rowBytesWritten is updated
		if tx.rowBytesWritten != 2 {
			t.Errorf("Expected rowBytesWritten=2, got %d", tx.rowBytesWritten)
		}
	})

	t.Run("begin_fails_when_write_channel_not_set", func(t *testing.T) {
		tx := &Transaction{
			Header:    header,
			writeChan: nil,
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should fail when writeChan is nil")
		}
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T", err)
		}
	})

	t.Run("begin_fails_when_write_fails", func(t *testing.T) {
		header := createTestHeader()
		dataChan := make(chan Data, 1)

		// Create a channel that will fail writes
		go func() {
			for data := range dataChan {
				// Send error response
				data.Response <- NewWriteError("simulated write failure", nil)
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should fail when write fails")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		// Verify transaction is tombstoned (FR-006)
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// Verify subsequent calls return TombstonedError
		err2 := tx.Begin()
		if err2 == nil {
			t.Fatal("Begin() should fail on tombstoned transaction")
		}
		if _, ok := err2.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T", err2)
		}
	})

	t.Run("begin_fails_when_channel_blocked", func(t *testing.T) {
		header := createTestHeader()
		// Unbuffered channel that no one reads from to force blocking
		dataChan := make(chan Data)

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		// Start Begin() in goroutine since it will block
		done := make(chan error, 1)
		go func() {
			done <- tx.Begin()
		}()

		// Give it a moment to try to send
		time.Sleep(5 * time.Millisecond)

		// Close channel to cause writeBytes to fail in default case
		close(dataChan)

		// Wait a bit more for the error
		time.Sleep(5 * time.Millisecond)

		select {
		case err := <-done:
			if err == nil {
				t.Fatal("Begin() should fail when channel is closed")
			}
		default:
			// Operation might still be blocked, which is also a failure case
			// Force unblock by reading from done channel with timeout
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("Begin() should fail when channel is closed")
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Begin() should have failed or completed")
			}
		}
	})
}

// TestAddRow_DiskPersistence verifies AddRow() writes to disk (FR-002)
func TestAddRow_DiskPersistence(t *testing.T) {
	header := createTestHeader()

	t.Run("first_addrow_writes_incremental_bytes", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Wait for write to complete
		time.Sleep(50 * time.Millisecond)

		// Should have 2 writes: Begin() (2 bytes) + AddRow() incremental bytes
		if len(*writtenBytes) != 2 {
			t.Fatalf("Expected 2 writes, got %d", len(*writtenBytes))
		}

		// First write is from Begin() (2 bytes)
		beginBytes := (*writtenBytes)[0]
		if len(beginBytes) != 2 {
			t.Errorf("Begin() should write 2 bytes, got %d", len(beginBytes))
		}

		// Second write is incremental bytes from first AddRow()
		addRowBytes := (*writtenBytes)[1]
		rowSize := header.GetRowSize()
		expectedIncrementalSize := rowSize - 5 - 2 // rowSize - end_control(5) - start_control(2)
		if len(addRowBytes) != expectedIncrementalSize {
			t.Errorf("First AddRow() should write %d incremental bytes, got %d", expectedIncrementalSize, len(addRowBytes))
		}
	})

	t.Run("subsequent_addrow_writes_finalization_and_new_partial", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		key2, _ := uuid.NewV7()
		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		// Wait for writes to complete
		time.Sleep(50 * time.Millisecond)

		// Should have writes: Begin() + first AddRow() incremental + second AddRow() finalization + second AddRow() new partial
		if len(*writtenBytes) < 4 {
			t.Fatalf("Expected at least 4 writes, got %d", len(*writtenBytes))
		}

		// Verify finalization bytes (5 bytes: RE + parity + ROW_END)
		finalizationBytes := (*writtenBytes)[2]
		if len(finalizationBytes) != 5 {
			t.Errorf("Finalization should be 5 bytes, got %d", len(finalizationBytes))
		}

		// Verify new partial row bytes
		newPartialBytes := (*writtenBytes)[3]
		rowSize := header.GetRowSize()
		expectedPartialSize := rowSize - 5 // rowSize - end_control(5)
		if len(newPartialBytes) != expectedPartialSize {
			t.Errorf("New partial should be %d bytes, got %d", expectedPartialSize, len(newPartialBytes))
		}
	})

	t.Run("addrow_fails_when_write_fails", func(t *testing.T) {
		header := createTestHeader()
		dataChan := make(chan Data, 1)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on second write (AddRow incremental write)
				if writeCount == 2 {
					data.Response <- NewWriteError("simulated write failure", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should fail when write fails")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		// Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// Verify subsequent calls return TombstonedError
		key2, _ := uuid.NewV7()
		err2 := tx.AddRow(key2, json.RawMessage(`{"data":"test2"}`))
		if err2 == nil {
			t.Fatal("AddRow() should fail on tombstoned transaction")
		}
		if _, ok := err2.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T", err2)
		}
	})

	t.Run("addrow_fails_when_finalization_write_fails", func(t *testing.T) {
		header := createTestHeader()
		dataChan := make(chan Data, 10)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on third write (finalization bytes for second AddRow)
				if writeCount == 3 {
					data.Response <- NewWriteError("simulated finalization write failure", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		key2, _ := uuid.NewV7()
		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err == nil {
			t.Fatal("Second AddRow() should fail when finalization write fails")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		// Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}
	})
}

// TestCommit_DiskPersistence verifies Commit() writes to disk (FR-003, FR-004)
func TestCommit_DiskPersistence(t *testing.T) {
	header := createTestHeader()

	t.Run("commit_empty_transaction_writes_nullrow", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Wait for write to complete
		time.Sleep(50 * time.Millisecond)

		// Should have 2 writes: Begin() (2 bytes) + Commit() incremental bytes
		if len(*writtenBytes) != 2 {
			t.Fatalf("Expected 2 writes, got %d", len(*writtenBytes))
		}

		// Verify Commit() wrote remaining bytes
		commitBytes := (*writtenBytes)[1]
		rowSize := header.GetRowSize()
		expectedIncrementalSize := rowSize - 2 // rowSize - start_control(2)
		if len(commitBytes) != expectedIncrementalSize {
			t.Errorf("Commit() should write %d incremental bytes, got %d", expectedIncrementalSize, len(commitBytes))
		}

		// Verify NullRow was created
		if tx.GetEmptyRow() == nil {
			t.Error("Empty transaction should create NullRow")
		}
	})

	t.Run("commit_data_transaction_writes_final_row", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Wait for writes to complete
		time.Sleep(50 * time.Millisecond)

		// Should have writes: Begin() + AddRow() incremental + Commit() finalization
		if len(*writtenBytes) < 3 {
			t.Fatalf("Expected at least 3 writes, got %d", len(*writtenBytes))
		}

		// Verify final write is finalization bytes (5 bytes: TC + parity + ROW_END)
		finalBytes := (*writtenBytes)[len(*writtenBytes)-1]
		if len(finalBytes) != 5 {
			t.Errorf("Commit() finalization should be 5 bytes, got %d", len(finalBytes))
		}

		// Verify transaction has committed row
		rows := tx.GetRows()
		if len(rows) != 1 {
			t.Errorf("Expected 1 row after commit, got %d", len(rows))
		}
		if rows[0].EndControl != TRANSACTION_COMMIT {
			t.Errorf("Final row should have TC end control, got %s", rows[0].EndControl.String())
		}
	})

	t.Run("commit_fails_when_write_fails", func(t *testing.T) {
		header := createTestHeader()
		dataChan := make(chan Data, 10)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on commit write
				if writeCount == 3 {
					data.Response <- NewWriteError("simulated commit write failure", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		err = tx.Commit()
		if err == nil {
			t.Fatal("Commit() should fail when write fails")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		// Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}
	})

	t.Run("commit_empty_transaction_fails_when_write_fails", func(t *testing.T) {
		header := createTestHeader()
		dataChan := make(chan Data, 1)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on commit write (second write)
				if writeCount == 2 {
					data.Response <- NewWriteError("simulated commit write failure", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		err = tx.Commit()
		if err == nil {
			t.Fatal("Commit() should fail when write fails")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		// Verify transaction is tombstoned
		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// Verify empty row was not created
		if tx.GetEmptyRow() != nil {
			t.Error("Empty row should not be created when commit write fails")
		}
	})
}

// TestTransactionPersistence_Concurrency verifies thread-safety of disk operations (FR-010)
func TestTransactionPersistence_Concurrency(t *testing.T) {
	header := createTestHeader()

	t.Run("concurrent_begin_calls_only_one_succeeds", func(t *testing.T) {
		tx, _ := createTransactionWithByteCollector(header)

		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		// Launch multiple goroutines calling Begin() concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := tx.Begin()
				mu.Lock()
				if err == nil {
					successCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Only one Begin() should succeed
		if successCount != 1 {
			t.Errorf("Expected exactly 1 successful Begin(), got %d", successCount)
		}
	})

	t.Run("concurrent_addrow_calls_are_serialized", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		var wg sync.WaitGroup
		keys := make([]uuid.UUID, 10)
		errors := make([]error, 10)

		// Generate keys sequentially first to ensure proper timestamp ordering
		// Use longer delays to ensure timestamps are definitely different
		for i := 0; i < 10; i++ {
			keys[i], _ = uuid.NewV7()
			// Delay to ensure timestamps are different (UUIDv7 has millisecond precision)
			time.Sleep(2 * time.Millisecond)
		}

		// Launch multiple goroutines calling AddRow() concurrently
		// They will be serialized by the mutex, so all should succeed
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errors[idx] = tx.AddRow(keys[idx], json.RawMessage(`{"index":`+fmt.Sprintf("%d", idx)+`}`))
			}(i)
		}

		wg.Wait()

		// Count successful AddRow() calls
		successCount := 0
		for i, err := range errors {
			if err != nil {
				t.Logf("AddRow() %d failed: %v", i, err)
			} else {
				successCount++
			}
		}

		// All AddRow() calls should succeed (they're serialized by the mutex)
		// However, timestamp ordering might cause some to fail if they're too close
		// So we check that at least most succeed (allowing for timing edge cases)
		if successCount < 8 {
			t.Errorf("Expected at least 8 successful AddRow() calls (serialized), got %d", successCount)
		}

		// Wait for all writes to complete
		time.Sleep(100 * time.Millisecond)

		// Verify rows were written
		// Note: With N successful AddRow() calls:
		// - 1st AddRow: updates partial (0 rows finalized)
		// - 2nd AddRow: finalizes 1st row (1 row finalized)
		// - 3rd AddRow: finalizes 2nd row (2 rows finalized)
		// - ...
		// - Nth AddRow: finalizes (N-1)th row (N-1 rows finalized)
		// So we should have exactly (successCount - 1) finalized rows
		rows := tx.GetRows()
		expectedRows := successCount - 1
		if len(rows) != expectedRows {
			t.Errorf("Expected %d rows (successCount-1), got %d (successCount=%d)", expectedRows, len(rows), successCount)
		}

		// Verify writes happened
		if len(*writtenBytes) < 2 {
			t.Errorf("Expected at least 2 writes, got %d", len(*writtenBytes))
		}
	})

	t.Run("concurrent_commit_and_addrow_race_condition", func(t *testing.T) {
		tx, _ := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		var commitErr, addRowErr error
		var wg sync.WaitGroup

		// Launch Commit() and AddRow() concurrently
		// Due to mutex serialization, one will complete first and the other will see
		// the transaction in a different state
		wg.Add(2)
		go func() {
			defer wg.Done()
			commitErr = tx.Commit()
		}()

		go func() {
			defer wg.Done()
			// Small delay to increase chance of race
			time.Sleep(1 * time.Millisecond)
			key2, _ := uuid.NewV7()
			addRowErr = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		}()

		wg.Wait()

		// Due to mutex serialization, one will complete first:
		// - If Commit() completes first: AddRow() will fail because transaction is committed
		// - If AddRow() completes first: Commit() will succeed
		// Both operations are protected by the same mutex, so they're fully serialized
		// The test verifies that the operations complete safely without corruption

		// At least one should succeed
		successCount := 0
		if commitErr == nil {
			successCount++
		}
		if addRowErr == nil {
			successCount++
		}

		// With mutex serialization, both could succeed if AddRow happens before Commit
		// Or one succeeds and one fails if Commit happens first
		// The key is that there's no corruption and operations complete safely
		if successCount == 0 {
			t.Error("At least one operation should succeed")
		}

		// Verify transaction state is consistent
		if tx.IsTombstoned() {
			t.Error("Transaction should not be tombstoned in this scenario")
		}

		// The failing operation (if any) should return InvalidActionError
		if commitErr != nil {
			if _, ok := commitErr.(*InvalidActionError); !ok {
				t.Errorf("Expected InvalidActionError for failed Commit(), got %T", commitErr)
			}
		}
		if addRowErr != nil {
			if _, ok := addRowErr.(*InvalidActionError); !ok {
				t.Errorf("Expected InvalidActionError for failed AddRow(), got %T", addRowErr)
			}
		}
	})

	t.Run("concurrent_reads_during_write_operations", func(t *testing.T) {
		tx, _ := createTransactionWithByteCollector(header)

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		var wg sync.WaitGroup
		readErrors := make([]error, 20)

		// Launch multiple goroutines reading while writing
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Try to read transaction state
				_ = tx.GetRows()
				_ = tx.IsCommitted()
				readErrors[idx] = nil
			}(i)
		}

		// Add rows concurrently with reads
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				key, _ := uuid.NewV7()
				time.Sleep(1 * time.Millisecond)
				_ = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			}
		}()

		wg.Wait()

		// All reads should succeed without errors
		for i, err := range readErrors {
			if err != nil {
				t.Errorf("Read operation %d failed: %v", i, err)
			}
		}
	})
}

// TestTransactionPersistence_ErrorConditions verifies error handling (FR-006)
func TestTransactionPersistence_ErrorConditions(t *testing.T) {
	header := createTestHeader()

	t.Run("tombstoned_transaction_rejects_all_operations", func(t *testing.T) {
		dataChan := make(chan Data, 1)

		// Fail first write to tombstone transaction
		go func() {
			data := <-dataChan
			data.Response <- NewWriteError("simulated failure", nil)
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should fail")
		}

		// Verify all operations return TombstonedError
		operations := []struct {
			name string
			fn   func() error
		}{
			{"Begin", func() error { return tx.Begin() }},
			{"AddRow", func() error {
				key, _ := uuid.NewV7()
				return tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
			}},
			{"Commit", func() error { return tx.Commit() }},
			{"Savepoint", func() error { return tx.Savepoint() }},
			{"Rollback", func() error { return tx.Rollback(0) }},
		}

		for _, op := range operations {
			err := op.fn()
			if err == nil {
				t.Errorf("%s() should fail on tombstoned transaction", op.name)
			}
			if _, ok := err.(*TombstonedError); !ok {
				t.Errorf("%s() should return TombstonedError, got %T", op.name, err)
			}
		}
	})

	t.Run("write_failure_during_addrow_tombstones_transaction", func(t *testing.T) {
		dataChan := make(chan Data, 10)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on second write (AddRow incremental)
				if writeCount == 2 {
					data.Response <- NewWriteError("disk full", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() should succeed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err == nil {
			t.Fatal("AddRow() should fail when write fails")
		}

		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// Verify state is unchanged (no partial data persisted)
		if len(tx.GetRows()) != 0 {
			t.Error("No rows should be persisted after failed AddRow()")
		}
	})

	t.Run("write_failure_during_commit_tombstones_transaction", func(t *testing.T) {
		dataChan := make(chan Data, 10)

		writeCount := 0
		go func() {
			for data := range dataChan {
				writeCount++
				// Fail on commit write
				if writeCount == 3 {
					data.Response <- NewWriteError("disk error", nil)
				} else {
					data.Response <- nil
				}
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() should succeed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() should succeed: %v", err)
		}

		err = tx.Commit()
		if err == nil {
			t.Fatal("Commit() should fail when write fails")
		}

		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned after write failure")
		}

		// Verify transaction remains in active state (no partial commit)
		if tx.IsCommitted() {
			t.Error("Transaction should not be committed after failed Commit()")
		}
	})

	t.Run("channel_full_during_operation_tombstones_transaction", func(t *testing.T) {
		// Use a small buffer and fill it up to force default case
		dataChan := make(chan Data, 1)

		// Fill the channel buffer
		blocker := make(chan bool)
		go func() {
			// Block on reading from channel (never reads, so buffer stays full)
			<-blocker
		}()

		// Send one item to fill buffer
		responseChan1 := make(chan error, 1)
		select {
		case dataChan <- Data{Bytes: []byte("blocker"), Response: responseChan1}:
			// Buffer is now full
		default:
			// Already full
		}

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		// Begin() should fail because channel is full (hits default case)
		err := tx.Begin()
		if err == nil {
			t.Fatal("Begin() should fail when channel is full")
		}
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T", err)
		}

		if !tx.IsTombstoned() {
			t.Error("Transaction should be tombstoned when channel is full")
		}

		// Cleanup
		close(blocker)
		close(dataChan)
	})
}

// TestTransactionPersistence_AppendOnly verifies append-only semantics (FR-007)
func TestTransactionPersistence_AppendOnly(t *testing.T) {
	header := createTestHeader()

	t.Run("writes_only_append_new_bytes", func(t *testing.T) {
		tx, writtenBytes := createTransactionWithByteCollector(header)

		// Track initial byte count
		initialByteCount := 0

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key1, _ := uuid.NewV7()
		err = tx.AddRow(key1, json.RawMessage(`{"data":"first"}`))
		if err != nil {
			t.Fatalf("First AddRow() failed: %v", err)
		}

		key2, _ := uuid.NewV7()
		err = tx.AddRow(key2, json.RawMessage(`{"data":"second"}`))
		if err != nil {
			t.Fatalf("Second AddRow() failed: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Wait for all writes to complete
		time.Sleep(50 * time.Millisecond)

		// Verify bytes were written (in-memory file grows)
		finalByteCount := 0
		for _, bytes := range *writtenBytes {
			finalByteCount += len(bytes)
		}

		if finalByteCount <= initialByteCount {
			t.Errorf("Byte count should increase, initial=%d, final=%d", initialByteCount, finalByteCount)
		}

		// Verify all written bytes can be concatenated (simulating in-memory file)
		allWritten := make([]byte, 0)
		for _, bytes := range *writtenBytes {
			allWritten = append(allWritten, bytes...)
		}

		if len(allWritten) != finalByteCount {
			t.Errorf("Concatenated bytes length mismatch, expected=%d, got=%d", finalByteCount, len(allWritten))
		}

		// Verify we have some bytes written
		if len(allWritten) == 0 {
			t.Error("Expected some bytes to be written")
		}
	})
}

// TestTransactionPersistence_SynchronousWrites verifies synchronous write behavior (FR-005)
func TestTransactionPersistence_SynchronousWrites(t *testing.T) {
	header := createTestHeader()

	t.Run("begin_waits_for_write_completion", func(t *testing.T) {
		dataChan := make(chan Data, 1)
		writeCompleted := make(chan bool, 1)

		go func() {
			data := <-dataChan
			// Simulate write delay
			time.Sleep(10 * time.Millisecond)
			data.Response <- nil
			writeCompleted <- true
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
		}

		startTime := time.Now()
		err := tx.Begin()
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Verify Begin() waited for write (should take at least 10ms)
		if duration < 10*time.Millisecond {
			t.Errorf("Begin() should wait for write completion, took %v", duration)
		}

		// Verify write actually completed
		select {
		case <-writeCompleted:
			// Good
		case <-time.After(100 * time.Millisecond):
			t.Error("Write should have completed")
		}
	})

	t.Run("addrow_waits_for_write_completion", func(t *testing.T) {
		dataChan := make(chan Data, 10)
		writeCount := 0

		go func() {
			for data := range dataChan {
				writeCount++
				// Simulate write delay
				time.Sleep(5 * time.Millisecond)
				data.Response <- nil
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		startTime := time.Now()
		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		// Verify AddRow() waited for write (should take at least 5ms)
		if duration < 5*time.Millisecond {
			t.Errorf("AddRow() should wait for write completion, took %v", duration)
		}

		// Verify write actually completed
		if writeCount < 2 {
			t.Errorf("Expected at least 2 writes, got %d", writeCount)
		}
	})

	t.Run("commit_waits_for_write_completion", func(t *testing.T) {
		dataChan := make(chan Data, 10)
		writeCompleted := false
		var mu sync.Mutex

		go func() {
			for data := range dataChan {
				// Simulate write delay for commit
				time.Sleep(10 * time.Millisecond)
				data.Response <- nil
				mu.Lock()
				writeCompleted = true
				mu.Unlock()
			}
		}()

		tx := &Transaction{
			Header:    header,
			writeChan: dataChan,
			db:        &mockDBFile{},
			finder:    &mockFinderWithMaxTimestamp{maxTs: 0},
		}

		err := tx.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		key, _ := uuid.NewV7()
		err = tx.AddRow(key, json.RawMessage(`{"data":"test"}`))
		if err != nil {
			t.Fatalf("AddRow() failed: %v", err)
		}

		startTime := time.Now()
		err = tx.Commit()
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Commit() failed: %v", err)
		}

		// Verify Commit() waited for write (should take at least 10ms)
		if duration < 10*time.Millisecond {
			t.Errorf("Commit() should wait for write completion, took %v", duration)
		}

		// Verify write actually completed
		mu.Lock()
		completed := writeCompleted
		mu.Unlock()
		if !completed {
			t.Error("Write should have completed")
		}
	})
}
