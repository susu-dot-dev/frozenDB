package frozendb

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Mock DBFile Implementation for Get() Unit Tests
// =============================================================================

// mockGetDBFile is a specialized mock for testing Get() operations
type mockGetDBFile struct {
	data       []byte
	size       int64
	mode       string
	readCount  int
	readErrors map[int]error // Map of read count -> error to return
	isClosed   bool
	closeError error
	mu         sync.RWMutex
}

func newMockGetDBFile(data []byte, mode string) *mockGetDBFile {
	return &mockGetDBFile{
		data:       data,
		size:       int64(len(data)),
		mode:       mode,
		readErrors: make(map[int]error),
	}
}

func (m *mockGetDBFile) Read(start int64, size int32) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readCount++

	if m.isClosed {
		return nil, NewReadError("mock file is closed", nil)
	}

	// Check for injected error
	if err, exists := m.readErrors[m.readCount]; exists {
		return nil, err
	}

	if start < 0 {
		return nil, NewInvalidInputError("start cannot be negative", nil)
	}
	if size <= 0 {
		return nil, NewInvalidInputError("size must be positive", nil)
	}

	end := start + int64(size)
	if end > int64(len(m.data)) {
		return nil, NewReadError("read beyond file size", io.EOF)
	}

	return m.data[start:end], nil
}

func (m *mockGetDBFile) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

func (m *mockGetDBFile) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isClosed = true
	return m.closeError
}

func (m *mockGetDBFile) SetWriter(dataChan <-chan Data) error {
	return NewInvalidActionError("mock DBFile does not support writing", nil)
}

func (m *mockGetDBFile) GetMode() string {
	return m.mode
}

func (m *mockGetDBFile) WriterClosed() {
	// Mock implementation - return immediately (no writer to wait for)
}

func (m *mockGetDBFile) Subscribe(callback func() error) (func() error, error) {
	// Mock implementation - no-op subscription for read-only mock
	return func() error { return nil }, nil
}

// Helper to create a SimpleFinder with row emitter for testing
func newTestSimpleFinderForGet(dbFile DBFile, rowSize int32) (*SimpleFinder, error) {
	rowEmitter, err := NewRowEmitter(dbFile, int(rowSize))
	if err != nil {
		return nil, err
	}
	return NewSimpleFinder(dbFile, rowSize, rowEmitter)
}

// Simulate closing the file mid-operation
func (m *mockGetDBFile) simulateClose() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isClosed = true
}

// Inject read error at specific read count
func (m *mockGetDBFile) injectReadError(readNum int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readErrors[readNum] = err
}

// =============================================================================
// Helper Functions for Building Test Databases
// =============================================================================

func buildTestDatabase(rowSize int32, rows []testRow) ([]byte, []uuid.UUID, *Header) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   int(rowSize),
		skewMs:    5000,
	}

	headerBytes, _ := header.MarshalText()
	data := append([]byte(nil), headerBytes...)

	// Initial checksum row
	checksumRow := buildChecksumRow(rowSize, 0)
	data = append(data, checksumRow...)

	// Track keys for later reference
	var keys []uuid.UUID

	// Build rows
	for i, row := range rows {
		var rowBytes []byte

		switch row.rowType {
		case "data":
			key := uuid.Must(uuid.NewV7())
			keys = append(keys, key)
			rowBytes = buildDataRow(rowSize, key, row.value, row.startControl, row.endControl)

		case "null":
			rowBytes = buildNullRow(rowSize, row.startControl, row.endControl)

		case "checksum":
			rowBytes = buildChecksumRow(rowSize, uint32(i))

		case "partial":
			// Partial data row (incomplete)
			rowBytes = buildPartialDataRow(rowSize, row.bytesWritten)

		case "corrupt":
			rowBytes = make([]byte, rowSize)
			rowBytes[0] = 0xFF // Invalid sentinel
		}

		data = append(data, rowBytes...)
	}

	return data, keys, header
}

type testRow struct {
	rowType      string       // "data", "null", "checksum", "partial", "corrupt"
	value        string       // JSON value for data rows
	startControl StartControl // Start control byte
	endControl   EndControl   // End control bytes
	bytesWritten int          // For partial rows
}

func buildDataRow(rowSize int32, key uuid.UUID, value string, startControl StartControl, endControl EndControl) []byte {
	payload := &DataRowPayload{
		Key:   key,
		Value: json.RawMessage(value),
	}
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(rowSize),
			StartControl: startControl,
			EndControl:   endControl,
			RowPayload:   payload,
		},
	}
	bytes, _ := dataRow.MarshalText()
	return bytes
}

func buildNullRow(rowSize int32, startControl StartControl, endControl EndControl) []byte {
	// Use valid NullRow UUID with timestamp 0 (empty database)
	nullRowUUID := CreateNullRowUUID(0)
	payload := &NullRowPayload{
		Key: nullRowUUID,
	}
	nullRow := &NullRow{
		baseRow[*NullRowPayload]{
			RowSize:      int(rowSize),
			StartControl: startControl,
			EndControl:   endControl,
			RowPayload:   payload,
		},
	}
	bytes, err := nullRow.MarshalText()
	if err != nil {
		panic(fmt.Sprintf("buildNullRow: failed to marshal: %v", err))
	}
	return bytes
}

func buildChecksumRow(rowSize int32, value uint32) []byte {
	checksum := Checksum(value)
	checksumRow := &ChecksumRow{
		baseRow[*Checksum]{
			RowSize:      int(rowSize),
			StartControl: CHECKSUM_ROW,
			EndControl:   CHECKSUM_ROW_CONTROL,
			RowPayload:   &checksum,
		},
	}
	bytes, _ := checksumRow.MarshalText()
	return bytes
}

func buildPartialDataRow(rowSize int32, bytesWritten int) []byte {
	partial := make([]byte, bytesWritten)
	if bytesWritten >= 1 {
		partial[0] = ROW_START
	}
	if bytesWritten >= 2 {
		partial[1] = byte(START_TRANSACTION)
	}
	return partial
}

// =============================================================================
// Get() Basic Functionality Tests
// =============================================================================

func TestGet_ValidInputs(t *testing.T) {
	t.Run("get_single_committed_row", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"name":"test","count":42}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}

		if result["name"] != "test" {
			t.Errorf("name = %v, want test", result["name"])
		}
		if result["count"] != float64(42) {
			t.Errorf("count = %v, want 42", result["count"])
		}
	})

	t.Run("get_with_struct_destination", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"name":"alice","age":30}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var person Person
		err := db.Get(keys[0], &person)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}

		if person.Name != "alice" {
			t.Errorf("Name = %s, want alice", person.Name)
		}
		if person.Age != 30 {
			t.Errorf("Age = %d, want 30", person.Age)
		}
	})

	t.Run("get_multiple_rows_in_transaction", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: ROW_END_CONTROL},
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: ROW_END_CONTROL},
			{rowType: "data", value: `{"id":3}`, startControl: ROW_CONTINUE, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// All three keys should be retrievable
		for i, key := range keys {
			var result map[string]interface{}
			err := db.Get(key, &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
			if result["id"] != float64(i+1) {
				t.Errorf("key[%d] id = %v, want %d", i, result["id"], i+1)
			}
		}
	})
}

// =============================================================================
// Get() Invalid Input Tests
// =============================================================================

func TestGet_InvalidInputs(t *testing.T) {
	t.Run("nil_uuid_key", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"test":"data"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, _, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(uuid.Nil, &result)
		if err == nil {
			t.Fatal("Get() should fail with nil UUID")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("expected InvalidInputError, got %T", err)
		}
	})

	t.Run("nil_value_destination", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"test":"data"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		err := db.Get(keys[0], nil)
		if err == nil {
			t.Fatal("Get() should fail with nil value destination")
		}

		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("expected InvalidInputError, got %T", err)
		}
	})

	t.Run("non_pointer_value", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"test":"data"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Pass non-pointer struct (json.Unmarshal will fail)
		var result map[string]interface{}
		err := db.Get(keys[0], result)
		if err == nil {
			t.Fatal("Get() should fail with non-pointer value")
		}

		// json.Unmarshal will return an error, which should be wrapped in InvalidDataError
		if _, ok := err.(*InvalidDataError); !ok {
			t.Errorf("expected InvalidDataError for json.Unmarshal error, got %T", err)
		}
	})
}

// =============================================================================
// Get() Key Not Found Tests
// =============================================================================

func TestGet_KeyNotFound(t *testing.T) {
	t.Run("empty_database", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{} // No data rows
		data, _, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		randomKey := uuid.Must(uuid.NewV7())
		var result map[string]interface{}
		err := db.Get(randomKey, &result)
		if err == nil {
			t.Fatal("Get() should fail on empty database")
		}

		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError, got %T", err)
		}
	})

	t.Run("key_not_in_database", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, _, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		differentKey := uuid.Must(uuid.NewV7())
		var result map[string]interface{}
		err := db.Get(differentKey, &result)
		if err == nil {
			t.Fatal("Get() should fail for non-existent key")
		}

		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError, got %T", err)
		}
	})
}

// =============================================================================
// Get() Transaction State Tests
// =============================================================================

func TestGet_TransactionStates(t *testing.T) {
	t.Run("committed_transaction_TC", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"committed"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}
		if result["status"] != "committed" {
			t.Errorf("status = %v, want committed", result["status"])
		}
	})

	t.Run("committed_with_savepoint_SC", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"savepoint_commit"}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}
		if result["status"] != "savepoint_commit" {
			t.Errorf("status = %v, want savepoint_commit", result["status"])
		}
	})

	t.Run("full_rollback_R0", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"rolled_back"}`, startControl: START_TRANSACTION, endControl: EndControl{'R', '0'}},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail for fully rolled back transaction")
		}

		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError, got %T", err)
		}
	})

	t.Run("full_rollback_S0", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"rolled_back"}`, startControl: START_TRANSACTION, endControl: EndControl{'S', '0'}},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail for fully rolled back savepoint transaction")
		}

		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError, got %T", err)
		}
	})

	t.Run("active_transaction_RE", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"active"}`, startControl: START_TRANSACTION, endControl: ROW_END_CONTROL},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail for active transaction")
		}

		// GetTransactionEnd returns TransactionActiveError, which Get converts to KeyNotFoundError
		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError for active transaction, got %T", err)
		}
	})

	t.Run("active_savepoint_SE", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"active_savepoint"}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail for active savepoint transaction")
		}

		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError for active savepoint, got %T", err)
		}
	})
}

// =============================================================================
// Get() Partial Rollback Tests
// =============================================================================

func TestGet_PartialRollback(t *testing.T) {
	t.Run("partial_rollback_R1_first_row_visible", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			// Transaction with savepoint at row 0
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE}, // Savepoint 1
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: EndControl{'R', '1'}},    // Rollback to savepoint 1
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// First row (before savepoint 1) should be visible
		var result1 map[string]interface{}
		err := db.Get(keys[0], &result1)
		if err != nil {
			t.Fatalf("Get(key[0]) failed: %v", err)
		}
		if result1["id"] != float64(1) {
			t.Errorf("key[0] id = %v, want 1", result1["id"])
		}

		// Second row (after savepoint 1) should NOT be visible
		var result2 map[string]interface{}
		err = db.Get(keys[1], &result2)
		if err == nil {
			t.Fatal("Get(key[1]) should fail for row after rollback point")
		}
		if _, ok := err.(*KeyNotFoundError); !ok {
			t.Errorf("expected KeyNotFoundError, got %T", err)
		}
	})

	t.Run("partial_rollback_R2_multiple_savepoints", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE}, // Savepoint 1
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: SAVEPOINT_CONTINUE},      // Savepoint 2
			{rowType: "data", value: `{"id":3}`, startControl: ROW_CONTINUE, endControl: EndControl{'R', '2'}},    // Rollback to savepoint 2
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Rows 0 and 1 should be visible (up to and including savepoint 2)
		for i := 0; i < 2; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
			if result["id"] != float64(i+1) {
				t.Errorf("key[%d] id = %v, want %d", i, result["id"], i+1)
			}
		}

		// Row 2 should NOT be visible
		var result3 map[string]interface{}
		err := db.Get(keys[2], &result3)
		if err == nil {
			t.Fatal("Get(key[2]) should fail for row after rollback point")
		}
	})

	t.Run("partial_rollback_S3_with_S_prefix", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE},
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: SAVEPOINT_CONTINUE},
			{rowType: "data", value: `{"id":3}`, startControl: ROW_CONTINUE, endControl: SAVEPOINT_CONTINUE},
			{rowType: "data", value: `{"id":4}`, startControl: ROW_CONTINUE, endControl: EndControl{'S', '3'}}, // Rollback to savepoint 3 with S prefix
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// First 3 rows should be visible
		for i := 0; i < 3; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
		}

		// Fourth row should NOT be visible
		var result4 map[string]interface{}
		err := db.Get(keys[3], &result4)
		if err == nil {
			t.Fatal("Get(key[3]) should fail for row after savepoint")
		}
	})

	t.Run("partial_rollback_with_checksum_rows", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE},
			{rowType: "checksum", value: "", startControl: CHECKSUM_ROW, endControl: CHECKSUM_ROW_CONTROL}, // Checksum row in middle
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: EndControl{'S', '1'}},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// First row should be visible (checksum rows are skipped)
		var result1 map[string]interface{}
		err := db.Get(keys[0], &result1)
		if err != nil {
			t.Fatalf("Get(key[0]) failed: %v", err)
		}

		// Second row should NOT be visible (after savepoint)
		var result2 map[string]interface{}
		err = db.Get(keys[1], &result2)
		if err == nil {
			t.Fatal("Get(key[1]) should fail")
		}
	})
}

// =============================================================================
// Get() Error Handling Tests
// =============================================================================

func TestGet_ErrorHandling(t *testing.T) {
	t.Run("corrupt_transaction_end_row", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: ROW_END_CONTROL},
			{rowType: "corrupt"}, // Corrupt end row
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail with corrupt transaction end row")
		}

		// Should get CorruptDatabaseError when parsing end row
		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("expected CorruptDatabaseError, got %T", err)
		}
	})

	t.Run("invalid_json_in_value", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{invalid json}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail with invalid JSON")
		}

		if _, ok := err.(*InvalidDataError); !ok {
			t.Errorf("expected InvalidDataError, got %T", err)
		}
	})

	t.Run("disk_read_error_during_get", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		// Inject read error on 2nd read (first read is during initialization, second is GetIndex)
		// Initialization reads: row 0 (checksum) = read #1, row 1 (data) = read #2
		// Get() will do: GetIndex reads row 1 = read #3, readRowAtIndex reads row 1 = read #4
		// So inject error on read #3 (during GetIndex in Get())
		dbFile.injectReadError(3, NewReadError("simulated disk failure", nil))

		finder, finderErr := newTestSimpleFinderForGet(dbFile, rowSize)
		if finderErr != nil {
			t.Fatalf("NewSimpleFinder failed: %v", finderErr)
		}

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail with disk read error")
		}

		if _, ok := err.(*ReadError); !ok {
			t.Errorf("expected ReadError, got %T", err)
		}
	})

	t.Run("file_closed_during_operation", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Close file before Get
		dbFile.simulateClose()

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail with closed file")
		}

		if _, ok := err.(*ReadError); !ok {
			t.Errorf("expected ReadError, got %T", err)
		}
	})

	t.Run("target_row_is_null_row", func(t *testing.T) {
		rowSize := int32(512)

		// Manually construct a scenario where finder returns index to a NullRow
		// This shouldn't happen in practice, but tests robustness
		header := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   int(rowSize),
			skewMs:    5000,
		}

		headerBytes, _ := header.MarshalText()
		data := append([]byte(nil), headerBytes...)
		data = append(data, buildChecksumRow(rowSize, 0)...)
		data = append(data, buildNullRow(rowSize, START_TRANSACTION, NULL_ROW_CONTROL)...)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Directly attempt to read row at index 1 (NullRow)
		// We can't easily test this through Get() since GetIndex won't return NullRow index
		// But we can test readAndUnmarshalRow directly
		var result map[string]interface{}
		err := db.readAndUnmarshalRow(1, &result)
		if err == nil {
			t.Fatal("readAndUnmarshalRow should fail for NullRow")
		}

		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("expected CorruptDatabaseError, got %T", err)
		}
	})
}

// =============================================================================
// Get() Concurrency Tests
// =============================================================================

func TestGet_Concurrency(t *testing.T) {
	t.Run("concurrent_get_calls_same_key", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"counter":100}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		const numGoroutines = 50
		const numOps = 20

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*numOps)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numOps; j++ {
					var result map[string]interface{}
					err := db.Get(keys[0], &result)
					if err != nil {
						errors <- err
						return
					}
					if result["counter"] != float64(100) {
						errors <- fmt.Errorf("counter = %v, want 100", result["counter"])
						return
					}
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("concurrent Get failed: %v", err)
		}
	})

	t.Run("concurrent_get_calls_different_keys", func(t *testing.T) {
		rowSize := int32(512)
		rows := make([]testRow, 100)
		for i := 0; i < 100; i++ {
			endControl := ROW_END_CONTROL
			if i == 99 {
				endControl = TRANSACTION_COMMIT
			}
			startControl := ROW_CONTINUE
			if i == 0 {
				startControl = START_TRANSACTION
			}
			rows[i] = testRow{
				rowType:      "data",
				value:        fmt.Sprintf(`{"index":%d}`, i),
				startControl: startControl,
				endControl:   endControl,
			}
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*10)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					keyIdx := (goroutineID*10 + j) % len(keys)
					var result map[string]interface{}
					err := db.Get(keys[keyIdx], &result)
					if err != nil {
						errors <- err
						return
					}
					expectedIndex := float64(keyIdx)
					if result["index"] != expectedIndex {
						errors <- fmt.Errorf("key[%d] index = %v, want %v", keyIdx, result["index"], expectedIndex)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("concurrent Get failed: %v", err)
		}
	})

	t.Run("get_during_file_close", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"test":"data"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Goroutines performing Get operations
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					var result map[string]interface{}
					err := db.Get(keys[0], &result)
					if err != nil {
						// Expected: some will fail after close
						if _, ok := err.(*ReadError); !ok {
							errors <- fmt.Errorf("unexpected error type: %T", err)
						}
					}
					time.Sleep(1 * time.Millisecond)
				}
			}()
		}

		// Close file after a delay
		time.Sleep(10 * time.Millisecond)
		dbFile.simulateClose()

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// =============================================================================
// Get() With Partial Data Row Tests
// =============================================================================

func TestGet_WithPartialDataRow(t *testing.T) {
	t.Run("partial_row_at_end_of_file", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"status":"complete"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
			{rowType: "partial", bytesWritten: 50}, // Incomplete row at end
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Should still be able to Get the complete row
		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}
		if result["status"] != "complete" {
			t.Errorf("status = %v, want complete", result["status"])
		}
	})

	t.Run("get_operations_ignore_partial_row", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: ROW_END_CONTROL},
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: TRANSACTION_COMMIT},
			{rowType: "partial", bytesWritten: 30},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Both complete rows should be accessible
		for i := 0; i < 2; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
		}
	})
}

// =============================================================================
// Get() Complex Scenarios
// =============================================================================

func TestGet_ComplexScenarios(t *testing.T) {
	t.Run("multiple_transactions_with_different_states", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			// Transaction 1: Committed
			{rowType: "data", value: `{"tx":1}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},

			// Transaction 2: Rolled back
			{rowType: "data", value: `{"tx":2}`, startControl: START_TRANSACTION, endControl: EndControl{'R', '0'}},

			// Transaction 3: Committed
			{rowType: "data", value: `{"tx":3}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Transaction 1 should be visible
		var result1 map[string]interface{}
		err := db.Get(keys[0], &result1)
		if err != nil {
			t.Errorf("Get(tx1) failed: %v", err)
		}

		// Transaction 2 should NOT be visible (rolled back)
		var result2 map[string]interface{}
		err = db.Get(keys[1], &result2)
		if err == nil {
			t.Error("Get(tx2) should fail for rolled back transaction")
		}

		// Transaction 3 should be visible
		var result3 map[string]interface{}
		err = db.Get(keys[2], &result3)
		if err != nil {
			t.Errorf("Get(tx3) failed: %v", err)
		}
	})

	t.Run("large_transaction_with_savepoints", func(t *testing.T) {
		rowSize := int32(512)
		rows := make([]testRow, 0)

		// Create transaction with 50 rows and savepoints at rows 10, 20, 30
		for i := 0; i < 50; i++ {
			startControl := ROW_CONTINUE
			if i == 0 {
				startControl = START_TRANSACTION
			}

			endControl := ROW_END_CONTROL
			if i == 10 || i == 20 || i == 30 {
				endControl = SAVEPOINT_CONTINUE
			}
			if i == 49 {
				// Rollback to savepoint 2 (row 20)
				endControl = EndControl{'R', '2'}
			}

			rows = append(rows, testRow{
				rowType:      "data",
				value:        fmt.Sprintf(`{"row":%d}`, i),
				startControl: startControl,
				endControl:   endControl,
			})
		}

		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Rows 0-20 should be visible (up to and including savepoint 2)
		for i := 0; i <= 20; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
			if result["row"] != float64(i) {
				t.Errorf("key[%d] row = %v, want %d", i, result["row"], i)
			}
		}

		// Rows 21-49 should NOT be visible
		for i := 21; i < 50; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err == nil {
				t.Errorf("Get(key[%d]) should fail (after savepoint)", i)
			}
		}
	})

	t.Run("missing_savepoint_in_partial_rollback", func(t *testing.T) {
		// This tests error handling when rollback references non-existent savepoint
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: EndControl{'R', '5'}}, // References savepoint 5 that doesn't exist
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		var result map[string]interface{}
		err := db.Get(keys[0], &result)
		if err == nil {
			t.Fatal("Get() should fail with missing savepoint")
		}

		if _, ok := err.(*CorruptDatabaseError); !ok {
			t.Errorf("expected CorruptDatabaseError for missing savepoint, got %T", err)
		}
	})

	t.Run("alternating_savepoints_and_regular_rows", func(t *testing.T) {
		rowSize := int32(512)
		rows := []testRow{
			{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE}, // Savepoint 1
			{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: ROW_END_CONTROL},
			{rowType: "data", value: `{"id":3}`, startControl: ROW_CONTINUE, endControl: SAVEPOINT_CONTINUE}, // Savepoint 2
			{rowType: "data", value: `{"id":4}`, startControl: ROW_CONTINUE, endControl: ROW_END_CONTROL},
			{rowType: "data", value: `{"id":5}`, startControl: ROW_CONTINUE, endControl: EndControl{'S', '2'}}, // Rollback to savepoint 2
		}
		data, keys, header := buildTestDatabase(rowSize, rows)

		dbFile := newMockGetDBFile(data, MODE_READ)
		finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

		db := &FrozenDB{
			file:   dbFile,
			header: header,
			finder: finder,
		}

		// Rows 0-2 should be visible (up to savepoint 2)
		for i := 0; i < 3; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err != nil {
				t.Errorf("Get(key[%d]) failed: %v", i, err)
			}
		}

		// Rows 3-4 should NOT be visible
		for i := 3; i < 5; i++ {
			var result map[string]interface{}
			err := db.Get(keys[i], &result)
			if err == nil {
				t.Errorf("Get(key[%d]) should fail", i)
			}
		}
	})
}

// =============================================================================
// Get() Benchmark Tests
// =============================================================================

func BenchmarkGet_SingleRow(b *testing.B) {
	rowSize := int32(512)
	rows := []testRow{
		{rowType: "data", value: `{"benchmark":"data"}`, startControl: START_TRANSACTION, endControl: TRANSACTION_COMMIT},
	}
	data, keys, header := buildTestDatabase(rowSize, rows)

	dbFile := newMockGetDBFile(data, MODE_READ)
	finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

	db := &FrozenDB{
		file:   dbFile,
		header: header,
		finder: finder,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		_ = db.Get(keys[0], &result)
	}
}

func BenchmarkGet_LargeTransaction(b *testing.B) {
	rowSize := int32(512)
	rows := make([]testRow, 100)
	for i := 0; i < 100; i++ {
		startControl := ROW_CONTINUE
		if i == 0 {
			startControl = START_TRANSACTION
		}
		endControl := ROW_END_CONTROL
		if i == 99 {
			endControl = TRANSACTION_COMMIT
		}
		rows[i] = testRow{
			rowType:      "data",
			value:        fmt.Sprintf(`{"index":%d}`, i),
			startControl: startControl,
			endControl:   endControl,
		}
	}
	data, keys, header := buildTestDatabase(rowSize, rows)

	dbFile := newMockGetDBFile(data, MODE_READ)
	finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

	db := &FrozenDB{
		file:   dbFile,
		header: header,
		finder: finder,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyIdx := i % len(keys)
		var result map[string]interface{}
		_ = db.Get(keys[keyIdx], &result)
	}
}

func BenchmarkGet_WithSavepoints(b *testing.B) {
	rowSize := int32(512)
	rows := []testRow{
		{rowType: "data", value: `{"id":1}`, startControl: START_TRANSACTION, endControl: SAVEPOINT_CONTINUE},
		{rowType: "data", value: `{"id":2}`, startControl: ROW_CONTINUE, endControl: SAVEPOINT_CONTINUE},
		{rowType: "data", value: `{"id":3}`, startControl: ROW_CONTINUE, endControl: EndControl{'S', '1'}},
	}
	data, keys, header := buildTestDatabase(rowSize, rows)

	dbFile := newMockGetDBFile(data, MODE_READ)
	finder, _ := newTestSimpleFinderForGet(dbFile, rowSize)

	db := &FrozenDB{
		file:   dbFile,
		header: header,
		finder: finder,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		_ = db.Get(keys[0], &result)
	}
}
