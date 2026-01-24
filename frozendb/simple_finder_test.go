package frozendb

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// =============================================================================
// Mock DBFile Implementation for Unit Tests
// =============================================================================

type mockSimpleFinderDBFile struct {
	data []byte
	size int64
}

func (m *mockSimpleFinderDBFile) Read(start int64, size int32) ([]byte, error) {
	if start < 0 {
		return nil, NewInvalidInputError("start cannot be negative", nil)
	}
	if size <= 0 {
		return nil, NewInvalidInputError("size must be positive", nil)
	}
	end := start + int64(size)
	if end > int64(len(m.data)) {
		return nil, NewReadError("read beyond file size", nil)
	}
	return m.data[start:end], nil
}

func (m *mockSimpleFinderDBFile) Size() int64 {
	return m.size
}

func (m *mockSimpleFinderDBFile) Close() error {
	return nil
}

func (m *mockSimpleFinderDBFile) SetWriter(dataChan <-chan Data) error {
	return NewInvalidActionError("mock DBFile does not support writing", nil)
}

func (m *mockSimpleFinderDBFile) GetMode() string {
	return MODE_READ
}

// Helper to create a mock DBFile with given data
func newMockDBFile(data []byte) *mockSimpleFinderDBFile {
	return &mockSimpleFinderDBFile{
		data: data,
		size: int64(len(data)),
	}
}

// Helper to create a mock DBFile with header only
func newMockDBFileWithHeader(rowSize int32) *mockSimpleFinderDBFile {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   int(rowSize),
		skewMs:    0,
	}
	headerBytes, _ := header.MarshalText()
	return newMockDBFile(headerBytes)
}

// Helper to add a complete row to mock file data
func (m *mockSimpleFinderDBFile) appendRow(row []byte) {
	m.data = append(m.data, row...)
	m.size = int64(len(m.data))
}

// =============================================================================
// NewSimpleFinder Constructor Tests
// =============================================================================

func TestNewSimpleFinder_ValidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rowSize int32
	}{
		{"minimum row size 128", 128},
		{"standard row size 512", 512},
		{"standard row size 1024", 1024},
		{"maximum row size 65536", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbFile := newMockDBFileWithHeader(tt.rowSize)
			sf, err := NewSimpleFinder(dbFile, tt.rowSize)
			if err != nil {
				t.Fatalf("NewSimpleFinder failed: %v", err)
			}
			if sf == nil {
				t.Fatal("NewSimpleFinder returned nil without error")
			}
			if sf.rowSize != tt.rowSize {
				t.Errorf("rowSize = %d, want %d", sf.rowSize, tt.rowSize)
			}
			if sf.size != dbFile.Size() {
				t.Errorf("size = %d, want %d", sf.size, dbFile.Size())
			}
		})
	}
}

func TestNewSimpleFinder_InvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dbFile  DBFile
		rowSize int32
		wantErr string
	}{
		{
			name:    "nil dbFile",
			dbFile:  nil,
			rowSize: 512,
			wantErr: "dbFile cannot be nil",
		},
		{
			name:    "rowSize too small",
			dbFile:  newMockDBFileWithHeader(127),
			rowSize: 127,
			wantErr: "rowSize must be between 128 and 65536",
		},
		{
			name:    "rowSize too large",
			dbFile:  newMockDBFileWithHeader(65537),
			rowSize: 65537,
			wantErr: "rowSize must be between 128 and 65536",
		},
		{
			name:    "rowSize zero",
			dbFile:  newMockDBFileWithHeader(0),
			rowSize: 0,
			wantErr: "rowSize must be between 128 and 65536",
		},
		{
			name:    "rowSize negative",
			dbFile:  newMockDBFileWithHeader(-1),
			rowSize: -1,
			wantErr: "rowSize must be between 128 and 65536",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf, err := NewSimpleFinder(tt.dbFile, tt.rowSize)
			if err == nil {
				t.Error("NewSimpleFinder should return error")
			}
			if sf != nil {
				t.Error("NewSimpleFinder should return nil on error")
			}
			if _, ok := err.(*InvalidInputError); !ok {
				t.Errorf("expected InvalidInputError, got %T", err)
			}
		})
	}
}

func TestNewSimpleFinder_InitializesSize(t *testing.T) {
	t.Parallel()

	// Test that SimpleFinder initializes with current DBFile size
	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Add some rows to the file
	for i := 0; i < 5; i++ {
		row := make([]byte, rowSize)
		row[0] = ROW_START
		row[1] = 'T'
		dbFile.appendRow(row)
	}

	sf, err := NewSimpleFinder(dbFile, rowSize)
	if err != nil {
		t.Fatalf("NewSimpleFinder failed: %v", err)
	}

	expectedSize := HEADER_SIZE + (5 * int64(rowSize))
	if sf.size != expectedSize {
		t.Errorf("SimpleFinder size = %d, want %d", sf.size, expectedSize)
	}
}

// =============================================================================
// GetIndex Tests
// =============================================================================

func TestGetIndex_EmptyDatabase(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Add initial checksum row
	checksumRow := createMockChecksumRow(rowSize)
	dbFile.appendRow(checksumRow)

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	testKey := uuid.Must(uuid.NewV7())
	_, err := sf.GetIndex(testKey)
	if err == nil {
		t.Error("GetIndex should return error for empty database")
	}
	if _, ok := err.(*KeyNotFoundError); !ok {
		t.Errorf("expected KeyNotFoundError, got %T", err)
	}
}

func TestGetIndex_SingleRow(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Add checksum row at index 0
	checksumRow := createMockChecksumRow(rowSize)
	dbFile.appendRow(checksumRow)

	// Add data row at index 1
	testKey := uuid.Must(uuid.NewV7())
	dataRow := createMockDataRow(rowSize, testKey, `{"value":"test"}`)
	dbFile.appendRow(dataRow)

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	index, err := sf.GetIndex(testKey)
	if err != nil {
		t.Errorf("GetIndex failed: %v", err)
	}
	if index != 1 {
		t.Errorf("GetIndex returned %d, want 1", index)
	}
}

func TestGetIndex_MultipleRows(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Add checksum row
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add multiple data rows
	keys := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		keys[i] = uuid.Must(uuid.NewV7())
		dataRow := createMockDataRow(rowSize, keys[i], fmt.Sprintf(`{"index":%d}`, i))
		dbFile.appendRow(dataRow)
	}

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Test finding each key
	for i, key := range keys {
		index, err := sf.GetIndex(key)
		if err != nil {
			t.Errorf("GetIndex failed for key %d: %v", i, err)
		}
		expectedIndex := int64(i + 1) // +1 for checksum at index 0
		if index != expectedIndex {
			t.Errorf("GetIndex returned %d, want %d", index, expectedIndex)
		}
	}
}

func TestGetIndex_SkipsNonDataRows(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Index 0: Checksum row
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Index 1: Data row with target key
	targetKey := uuid.Must(uuid.NewV7())
	dbFile.appendRow(createMockDataRow(rowSize, targetKey, `{"value":"target"}`))

	// Index 2: Null row
	dbFile.appendRow(createMockNullRow(rowSize))

	// Index 3: Another data row
	otherKey := uuid.Must(uuid.NewV7())
	dbFile.appendRow(createMockDataRow(rowSize, otherKey, `{"value":"other"}`))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Should find target key at index 1
	index, err := sf.GetIndex(targetKey)
	if err != nil {
		t.Errorf("GetIndex failed: %v", err)
	}
	if index != 1 {
		t.Errorf("GetIndex returned %d, want 1", index)
	}

	// Should find other key at index 3 (skipping null row at 2)
	index, err = sf.GetIndex(otherKey)
	if err != nil {
		t.Errorf("GetIndex failed: %v", err)
	}
	if index != 3 {
		t.Errorf("GetIndex returned %d, want 3", index)
	}
}

func TestGetIndex_ReturnsFirstMatch(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add same key twice
	duplicateKey := uuid.Must(uuid.NewV7())
	dbFile.appendRow(createMockDataRow(rowSize, duplicateKey, `{"instance":"first"}`))
	dbFile.appendRow(createMockDataRow(rowSize, duplicateKey, `{"instance":"second"}`))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	index, err := sf.GetIndex(duplicateKey)
	if err != nil {
		t.Errorf("GetIndex failed: %v", err)
	}
	if index != 1 {
		t.Errorf("GetIndex should return first match, got index %d", index)
	}
}

func TestGetIndex_InvalidInputs(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)
	dbFile.appendRow(createMockChecksumRow(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	tests := []struct {
		name    string
		key     uuid.UUID
		wantErr string
	}{
		{
			name:    "nil UUID",
			key:     uuid.Nil,
			wantErr: "key cannot be uuid.Nil",
		},
		{
			name:    "non-UUIDv7",
			key:     uuid.Must(uuid.NewRandom()),
			wantErr: "must be UUIDv7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sf.GetIndex(tt.key)
			if err == nil {
				t.Error("GetIndex should return error")
			}
			if _, ok := err.(*InvalidInputError); !ok {
				t.Errorf("expected InvalidInputError, got %T", err)
			}
		})
	}
}

func TestGetIndex_CorruptRowReturnsError(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add a corrupt row (invalid format)
	corruptRow := make([]byte, rowSize)
	corruptRow[0] = 0xFF // Invalid ROW_START
	dbFile.appendRow(corruptRow)

	// Add valid row after corrupt one
	targetKey := uuid.Must(uuid.NewV7())
	dbFile.appendRow(createMockDataRow(rowSize, targetKey, `{"value":"valid"}`))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Should return CorruptDatabaseError when encountering corrupt row
	_, err := sf.GetIndex(targetKey)
	if err == nil {
		t.Error("GetIndex should return error for corrupt row")
	}
	if _, ok := err.(*CorruptDatabaseError); !ok {
		t.Errorf("expected CorruptDatabaseError, got %T", err)
	}
}

// =============================================================================
// PartialDataRow Handling Tests
// =============================================================================

func TestGetIndex_WithPartialDataRowAtEnd(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Add checksum row at index 0
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add complete data row at index 1
	completeKey := uuid.Must(uuid.NewV7())
	dbFile.appendRow(createMockDataRow(rowSize, completeKey, `{"value":"complete"}`))

	// Add partial data row (incomplete bytes at end - simulates in-progress transaction)
	partialBytes := make([]byte, 50) // Less than full row size
	partialBytes[0] = ROW_START
	partialBytes[1] = 'T'
	dbFile.appendRow(partialBytes)

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Should find the complete row
	index, err := sf.GetIndex(completeKey)
	if err != nil {
		t.Errorf("GetIndex failed: %v", err)
	}
	if index != 1 {
		t.Errorf("GetIndex returned %d, want 1", index)
	}

	// Verify totalRows calculation excludes partial row
	sf.mu.Lock()
	confirmedSize := sf.size
	sf.mu.Unlock()
	totalRows := (confirmedSize - HEADER_SIZE) / int64(rowSize)
	expectedRows := int64(2) // checksum + 1 complete data row (partial excluded by division)
	if totalRows != expectedRows {
		t.Errorf("totalRows = %d, want %d (partial row should be excluded)", totalRows, expectedRows)
	}
}

func TestNewSimpleFinder_DatabaseEndsWithPartialDataRow(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Simulate a database file that ended mid-transaction
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add a few complete rows
	for i := 0; i < 3; i++ {
		key := uuid.Must(uuid.NewV7())
		dbFile.appendRow(createMockDataRow(rowSize, key, fmt.Sprintf(`{"row":%d}`, i)))
	}

	// Add partial data row at end (Begin() was called but not committed)
	partialBytes := make([]byte, 100)
	partialBytes[0] = ROW_START
	partialBytes[1] = 'T'
	dbFile.appendRow(partialBytes)

	// Create SimpleFinder - should handle partial row gracefully
	sf, err := NewSimpleFinder(dbFile, rowSize)
	if err != nil {
		t.Fatalf("NewSimpleFinder failed: %v", err)
	}

	// Verify size includes partial bytes but operations exclude them
	// Header (64) + checksum row (256) + 3 data rows (3*256) + partial (100)
	expectedFileSize := HEADER_SIZE + int64(rowSize) + (3 * int64(rowSize)) + int64(len(partialBytes))
	if sf.size != expectedFileSize {
		t.Errorf("sf.size = %d, want %d", sf.size, expectedFileSize)
	}

	// Verify operations work correctly (partial row is excluded)
	testKey := uuid.Must(uuid.NewV7())
	_, err = sf.GetIndex(testKey)
	if _, ok := err.(*KeyNotFoundError); !ok {
		t.Errorf("expected KeyNotFoundError, got %T", err)
	}
}

func TestGetTransactionBoundaries_WithPartialDataRow(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Index 0: Checksum
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Index 1-2: Complete transaction
	dbFile.appendRow(createMockTransactionStartRow(rowSize))
	dbFile.appendRow(createMockTransactionEndRow(rowSize))

	// Partial row at end
	partialBytes := make([]byte, 80)
	partialBytes[0] = ROW_START
	partialBytes[1] = 'T'
	dbFile.appendRow(partialBytes)

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// GetTransactionStart/End should work on complete transaction
	start, err := sf.GetTransactionStart(1)
	if err != nil {
		t.Errorf("GetTransactionStart failed: %v", err)
	}
	if start != 1 {
		t.Errorf("GetTransactionStart = %d, want 1", start)
	}

	end, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd failed: %v", err)
	}
	if end != 2 {
		t.Errorf("GetTransactionEnd = %d, want 2", end)
	}

	// Trying to access index 3 (where partial would be) should fail
	_, err = sf.GetTransactionStart(3)
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("expected InvalidInputError for out of bounds, got %T", err)
	}
}

// =============================================================================
// GetTransactionStart Tests
// =============================================================================

func TestGetTransactionStart_SingleRowTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))
	dbFile.appendRow(createMockSingleRowTransaction(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	start, err := sf.GetTransactionStart(1)
	if err != nil {
		t.Errorf("GetTransactionStart failed: %v", err)
	}
	if start != 1 {
		t.Errorf("GetTransactionStart = %d, want 1", start)
	}
}

func TestGetTransactionStart_MultiRowTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Transaction spanning indices 1-3
	dbFile.appendRow(createMockTransactionStartRow(rowSize))    // Index 1: start_control='T', end_control='RE'
	dbFile.appendRow(createMockTransactionContinueRow(rowSize)) // Index 2: start_control='R', end_control='RE'
	dbFile.appendRow(createMockTransactionEndRow(rowSize))      // Index 3: start_control='R', end_control='TC'

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// All rows in transaction should point to start at index 1
	for i := int64(1); i <= 3; i++ {
		start, err := sf.GetTransactionStart(i)
		if err != nil {
			t.Errorf("GetTransactionStart(%d) failed: %v", i, err)
		}
		if start != 1 {
			t.Errorf("GetTransactionStart(%d) = %d, want 1", i, start)
		}
	}
}

func TestGetTransactionStart_SkipsChecksumRows(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	// Index 0: Checksum
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Index 1: Transaction start
	dbFile.appendRow(createMockTransactionStartRow(rowSize))

	// Index 2: Another checksum (for testing)
	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Index 3: Transaction continue (should scan backward past checksum)
	dbFile.appendRow(createMockTransactionContinueRow(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Calling on index 3 should skip checksum at 2 and find start at 1
	start, err := sf.GetTransactionStart(3)
	if err != nil {
		t.Errorf("GetTransactionStart failed: %v", err)
	}
	if start != 1 {
		t.Errorf("GetTransactionStart = %d, want 1", start)
	}
}

func TestGetTransactionStart_InvalidInputs(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))
	dbFile.appendRow(createMockSingleRowTransaction(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	tests := []struct {
		name    string
		index   int64
		wantErr string
	}{
		{"negative index", -1, "index cannot be negative"},
		{"out of bounds", 100, "out of bounds"},
		{"checksum row", 0, "checksum row"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sf.GetTransactionStart(tt.index)
			if err == nil {
				t.Error("GetTransactionStart should return error")
			}
			if _, ok := err.(*InvalidInputError); !ok {
				t.Errorf("expected InvalidInputError, got %T", err)
			}
		})
	}
}

// =============================================================================
// GetTransactionEnd Tests
// =============================================================================

func TestGetTransactionEnd_SingleRowTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))
	dbFile.appendRow(createMockSingleRowTransaction(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	end, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd failed: %v", err)
	}
	if end != 1 {
		t.Errorf("GetTransactionEnd = %d, want 1", end)
	}
}

func TestGetTransactionEnd_MultiRowTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Transaction spanning indices 1-3
	dbFile.appendRow(createMockTransactionStartRow(rowSize))
	dbFile.appendRow(createMockTransactionContinueRow(rowSize))
	dbFile.appendRow(createMockTransactionEndRow(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// All rows in transaction should point to end at index 3
	for i := int64(1); i <= 3; i++ {
		end, err := sf.GetTransactionEnd(i)
		if err != nil {
			t.Errorf("GetTransactionEnd(%d) failed: %v", i, err)
		}
		if end != 3 {
			t.Errorf("GetTransactionEnd(%d) = %d, want 3", i, end)
		}
	}
}

func TestGetTransactionEnd_NullRowTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))
	dbFile.appendRow(createMockNullRow(rowSize)) // Index 1: NullRow ends transaction

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	end, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd failed: %v", err)
	}
	if end != 1 {
		t.Errorf("GetTransactionEnd = %d, want 1", end)
	}
}

func TestGetTransactionEnd_ActiveTransaction(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Transaction with only start and continue, no end
	dbFile.appendRow(createMockTransactionStartRow(rowSize))
	dbFile.appendRow(createMockTransactionContinueRow(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	_, err := sf.GetTransactionEnd(1)
	if err == nil {
		t.Error("GetTransactionEnd should return error for active transaction")
	}
	if _, ok := err.(*TransactionActiveError); !ok {
		t.Errorf("expected TransactionActiveError, got %T", err)
	}
}

func TestGetTransactionEnd_SkipsChecksumRows(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))         // Index 0
	dbFile.appendRow(createMockTransactionStartRow(rowSize)) // Index 1
	dbFile.appendRow(createMockChecksumRow(rowSize))         // Index 2
	dbFile.appendRow(createMockTransactionEndRow(rowSize))   // Index 3

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	end, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd failed: %v", err)
	}
	if end != 3 {
		t.Errorf("GetTransactionEnd = %d, want 3 (should skip checksum at 2)", end)
	}
}

func TestGetTransactionEnd_InvalidInputs(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))
	dbFile.appendRow(createMockSingleRowTransaction(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	tests := []struct {
		name  string
		index int64
	}{
		{"negative index", -1},
		{"out of bounds", 100},
		{"checksum row", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sf.GetTransactionEnd(tt.index)
			if err == nil {
				t.Error("GetTransactionEnd should return error")
			}
			if _, ok := err.(*InvalidInputError); !ok {
				t.Errorf("expected InvalidInputError, got %T", err)
			}
		})
	}
}

// =============================================================================
// OnRowAdded Tests
// =============================================================================

func TestOnRowAdded_UpdatesSize(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)
	dbFile.appendRow(createMockChecksumRow(rowSize))

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	initialSize := sf.size

	// Add a row
	key := uuid.Must(uuid.NewV7())
	row := &RowUnion{
		DataRow: &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      int(rowSize),
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: `{"test":"data"}`,
				},
			},
		},
	}

	err := sf.OnRowAdded(1, row)
	if err != nil {
		t.Errorf("OnRowAdded failed: %v", err)
	}

	expectedSize := initialSize + int64(rowSize)
	if sf.size != expectedSize {
		t.Errorf("size = %d, want %d", sf.size, expectedSize)
	}
}

func TestOnRowAdded_ValidatesIndex(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)
	sf, _ := NewSimpleFinder(dbFile, rowSize)

	row := &RowUnion{
		DataRow: &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      int(rowSize),
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   uuid.Must(uuid.NewV7()),
					Value: `{"test":"data"}`,
				},
			},
		},
	}

	tests := []struct {
		name    string
		index   int64
		wantErr bool
	}{
		{"correct index 0", 0, false},
		{"skipped index", 5, true},
		{"negative index", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset SimpleFinder
			sf.size = HEADER_SIZE

			err := sf.OnRowAdded(tt.index, row)
			if tt.wantErr && err == nil {
				t.Error("OnRowAdded should return error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("OnRowAdded failed: %v", err)
			}
		})
	}
}

func TestOnRowAdded_RejectsNilRow(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)
	sf, _ := NewSimpleFinder(dbFile, rowSize)

	err := sf.OnRowAdded(0, nil)
	if err == nil {
		t.Error("OnRowAdded should reject nil row")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("expected InvalidInputError, got %T", err)
	}
}

func TestOnRowAdded_SequentialIndexing(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)
	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Add rows sequentially
	for i := int64(0); i < 10; i++ {
		row := &RowUnion{
			DataRow: &DataRow{
				baseRow[*DataRowPayload]{
					RowSize:      int(rowSize),
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload: &DataRowPayload{
						Key:   uuid.Must(uuid.NewV7()),
						Value: fmt.Sprintf(`{"index":%d}`, i),
					},
				},
			},
		}

		err := sf.OnRowAdded(i, row)
		if err != nil {
			t.Errorf("OnRowAdded(%d) failed: %v", i, err)
		}

		expectedSize := HEADER_SIZE + ((i + 1) * int64(rowSize))
		if sf.size != expectedSize {
			t.Errorf("after index %d, size = %d, want %d", i, sf.size, expectedSize)
		}
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestSimpleFinder_ConcurrentReads(t *testing.T) {
	t.Parallel()

	rowSize := int32(256)
	dbFile := newMockDBFileWithHeader(rowSize)

	dbFile.appendRow(createMockChecksumRow(rowSize))

	// Add 100 rows
	keys := make([]uuid.UUID, 100)
	for i := 0; i < 100; i++ {
		keys[i] = uuid.Must(uuid.NewV7())
		dbFile.appendRow(createMockDataRow(rowSize, keys[i], fmt.Sprintf(`{"i":%d}`, i)))
	}

	sf, _ := NewSimpleFinder(dbFile, rowSize)

	// Concurrent GetIndex calls
	const numGoroutines = 10
	const numOpsPerGoroutine = 100

	errChan := make(chan error, numGoroutines*numOpsPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		go func() {
			for op := 0; op < numOpsPerGoroutine; op++ {
				keyIdx := op % len(keys)
				index, err := sf.GetIndex(keys[keyIdx])
				if err != nil {
					errChan <- err
					continue
				}
				expectedIndex := int64(keyIdx + 1) // +1 for checksum
				if index != expectedIndex {
					errChan <- fmt.Errorf("wrong index: got %d, want %d", index, expectedIndex)
				}
			}
		}()
	}

	// Collect any errors
	for i := 0; i < numGoroutines*numOpsPerGoroutine; i++ {
		select {
		case err := <-errChan:
			t.Errorf("concurrent operation failed: %v", err)
		default:
			// No error
		}
	}
}

// =============================================================================
// Helper Functions for Creating Mock Rows
// =============================================================================

func createMockChecksumRow(rowSize int32) []byte {
	checksum := Checksum(0)
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

func createMockDataRow(rowSize int32, key uuid.UUID, value string) []byte {
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(rowSize),
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}
	bytes, _ := dataRow.MarshalText()
	return bytes
}

func createMockNullRow(rowSize int32) []byte {
	nullRow := &NullRow{
		baseRow[*NullRowPayload]{
			RowSize:      int(rowSize),
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload: &NullRowPayload{
				Key: uuid.Nil,
			},
		},
	}
	bytes, _ := nullRow.MarshalText()
	return bytes
}

func createMockSingleRowTransaction(rowSize int32) []byte {
	key := uuid.Must(uuid.NewV7())
	return createMockDataRow(rowSize, key, `{"single":"row"}`)
}

func createMockTransactionStartRow(rowSize int32) []byte {
	key := uuid.Must(uuid.NewV7())
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(rowSize),
			StartControl: START_TRANSACTION,
			EndControl:   ROW_END_CONTROL, // RE = transaction continues
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: `{"start":"true"}`,
			},
		},
	}
	bytes, _ := dataRow.MarshalText()
	return bytes
}

func createMockTransactionContinueRow(rowSize int32) []byte {
	key := uuid.Must(uuid.NewV7())
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(rowSize),
			StartControl: ROW_CONTINUE,
			EndControl:   ROW_END_CONTROL, // RE = transaction continues
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: `{"continue":"true"}`,
			},
		},
	}
	bytes, _ := dataRow.MarshalText()
	return bytes
}

func createMockTransactionEndRow(rowSize int32) []byte {
	key := uuid.Must(uuid.NewV7())
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(rowSize),
			StartControl: ROW_CONTINUE,
			EndControl:   TRANSACTION_COMMIT, // TC = transaction commit
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: `{"end":"true"}`,
			},
		},
	}
	bytes, _ := dataRow.MarshalText()
	return bytes
}
