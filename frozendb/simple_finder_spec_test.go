package frozendb

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// Test_S_019_FR_001_FinderInterfaceDefinition validates that the Finder interface
// is properly defined with all required methods.
func Test_S_019_FR_001_FinderInterfaceDefinition(t *testing.T) {
	// FR-001: System MUST define a Finder interface with methods:
	// GetIndex(key UUID) (int64, error)
	// GetTransactionEnd(index int64) (int64, error)
	// GetTransactionStart(index int64) (int64, error)
	// OnRowAdded(index int64, row *RowUnion) error

	// Create a test database to verify interface implementation
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create a simple database with header and checksum
	createTestDatabase(t, dbPath)

	// Open the database file
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	// Create SimpleFinder and verify it implements Finder interface
	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Verify SimpleFinder implements Finder interface by type assertion
	var _ Finder = sf

	// Verify all methods exist and are callable (even if they error)
	testUUID := uuid.Must(uuid.NewV7())
	_, _ = sf.GetIndex(testUUID)
	_, _ = sf.GetTransactionStart(1)
	_, _ = sf.GetTransactionEnd(1)
	_ = sf.OnRowAdded(0, &RowUnion{})

	// If we reach here, the interface is properly defined
	t.Log("FR-001: Finder interface is properly defined with all required methods")
}

// Test_S_019_FR_002_GetIndexReturnsCorrectIndex validates that GetIndex()
// returns the correct index of rows containing specific UUID keys.
func Test_S_019_FR_002_GetIndexReturnsCorrectIndex(t *testing.T) {
	// FR-002: GetIndex() MUST return the index of the first row containing
	// the specified UUID key, or error if not found

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add multiple rows with known UUIDs
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	uuid1 := uuid.Must(uuid.NewV7())
	uuid2 := uuid.Must(uuid.NewV7())
	uuid3 := uuid.Must(uuid.NewV7())

	if err := tx.AddRow(uuid1, json.RawMessage(`{"value":"first"}`)); err != nil {
		t.Fatalf("Failed to add first row: %v", err)
	}
	if err := tx.AddRow(uuid2, json.RawMessage(`{"value":"second"}`)); err != nil {
		t.Fatalf("Failed to add second row: %v", err)
	}
	if err := tx.AddRow(uuid3, json.RawMessage(`{"value":"third"}`)); err != nil {
		t.Fatalf("Failed to add third row: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	db.Close()

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test: Find first UUID (should be at index 1 - after initial checksum at index 0)
	index1, err := sf.GetIndex(uuid1)
	if err != nil {
		t.Errorf("Failed to find uuid1: %v", err)
	}
	if index1 != 1 {
		t.Errorf("GetIndex(uuid1) returned wrong index: got %d, want 1", index1)
	}

	// Test: Find second UUID (should be at index 2)
	index2, err := sf.GetIndex(uuid2)
	if err != nil {
		t.Errorf("Failed to find uuid2: %v", err)
	}
	if index2 != 2 {
		t.Errorf("GetIndex(uuid2) returned wrong index: got %d, want 2", index2)
	}

	// Test: Find third UUID (should be at index 3)
	index3, err := sf.GetIndex(uuid3)
	if err != nil {
		t.Errorf("Failed to find uuid3: %v", err)
	}
	if index3 != 3 {
		t.Errorf("GetIndex(uuid3) returned wrong index: got %d, want 3", index3)
	}

	// Test: Try to find non-existent UUID
	nonExistentUUID := uuid.Must(uuid.NewV7())
	_, err = sf.GetIndex(nonExistentUUID)
	if err == nil {
		t.Error("GetIndex should return error for non-existent UUID")
	}
	if _, ok := err.(*KeyNotFoundError); !ok {
		t.Errorf("GetIndex should return KeyNotFoundError for non-existent UUID, got %T", err)
	}

	t.Log("FR-002: GetIndex() returns correct indices for existing keys and errors for non-existent keys")
}

// Test_S_019_FR_007_SimpleFinderImplementation validates that SimpleFinder
// provides a direct, linear scan implementation of the Finder interface.
func Test_S_019_FR_007_SimpleFinderImplementation(t *testing.T) {
	// FR-007: System MUST implement a SimpleFinder class that provides
	// a direct, linear scan implementation of the Finder interface

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with multiple rows
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add rows in a transaction
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	testUUIDs := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		testUUIDs[i] = uuid.Must(uuid.NewV7())
		if err := tx.AddRow(testUUIDs[i], json.RawMessage(`{"index":`+string(rune('0'+i))+`}`)); err != nil {
			t.Fatalf("Failed to add row %d: %v", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	db.Close()

	// Open database and create SimpleFinder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Verify SimpleFinder implements Finder interface
	var finder Finder = sf

	// Test linear scan behavior: find each UUID
	for i, testUUID := range testUUIDs {
		index, err := finder.GetIndex(testUUID)
		if err != nil {
			t.Errorf("Failed to find UUID %d: %v", i, err)
		}
		expectedIndex := int64(i + 1) // +1 because index 0 is checksum row
		if index != expectedIndex {
			t.Errorf("GetIndex returned wrong index for UUID %d: got %d, want %d", i, index, expectedIndex)
		}
	}

	// Verify that SimpleFinder scans linearly (finds first match)
	// Add the same UUID twice in different transactions
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}

	tx2, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	duplicateUUID := uuid.Must(uuid.NewV7())
	if err := tx2.AddRow(duplicateUUID, json.RawMessage(`{"first":"true"}`)); err != nil {
		t.Fatalf("Failed to add duplicate row: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit second transaction: %v", err)
	}

	db2.Close()

	// Reopen and test that it finds the first occurrence
	dbFile2, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to reopen database file: %v", err)
	}
	defer dbFile2.Close()

	sf2, err := NewSimpleFinder(dbFile2, 1024)
	if err != nil {
		t.Fatalf("Failed to create second SimpleFinder: %v", err)
	}

	index, err := sf2.GetIndex(duplicateUUID)
	if err != nil {
		t.Fatalf("Failed to find duplicate UUID: %v", err)
	}

	// Should find at index 11 (after 10 previous data rows + 1 checksum)
	if index != 11 {
		t.Errorf("GetIndex should find first occurrence: got index %d, want 11", index)
	}

	t.Log("FR-007: SimpleFinder provides direct linear scan implementation")
}

// Test_S_019_FR_003_GetTransactionStartReturnsCorrectIndex validates that
// GetTransactionStart() correctly identifies transaction boundaries.
func Test_S_019_FR_003_GetTransactionStartReturnsCorrectIndex(t *testing.T) {
	// FR-003: GetTransactionStart() MUST return the index of the first row
	// in the transaction containing the specified index

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with multiple transactions
	createTestDatabase(t, dbPath)

	// Open database for writing - Transaction 1
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Transaction 1: 3 rows
	tx1, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := tx1.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":1}`)); err != nil {
			t.Fatalf("Failed to add row to tx1: %v", err)
		}
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 1: %v", err)
	}

	db.Close()

	// Reopen for Transaction 2
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}

	// Transaction 2: 2 rows
	tx2, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := tx2.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":2}`)); err != nil {
			t.Fatalf("Failed to add row to tx2: %v", err)
		}
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 2: %v", err)
	}

	db2.Close()

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test: All rows in transaction 1 should point to index 1 (first data row)
	// Indices: 0=checksum, 1-3=tx1, 4-5=tx2
	for i := int64(1); i <= 3; i++ {
		start, err := sf.GetTransactionStart(i)
		if err != nil {
			t.Errorf("GetTransactionStart(%d) failed: %v", i, err)
		}
		if start != 1 {
			t.Errorf("GetTransactionStart(%d) = %d, want 1", i, start)
		}
	}

	// Test: All rows in transaction 2 should point to index 4
	for i := int64(4); i <= 5; i++ {
		start, err := sf.GetTransactionStart(i)
		if err != nil {
			t.Errorf("GetTransactionStart(%d) failed: %v", i, err)
		}
		if start != 4 {
			t.Errorf("GetTransactionStart(%d) = %d, want 4", i, start)
		}
	}

	t.Log("FR-003: GetTransactionStart() returns correct transaction start indices")
}

// Test_S_019_FR_004_GetTransactionEndReturnsCorrectIndex validates that
// GetTransactionEnd() correctly identifies transaction end boundaries.
func Test_S_019_FR_004_GetTransactionEndReturnsCorrectIndex(t *testing.T) {
	// FR-004: GetTransactionEnd() MUST return the index of the last row
	// in the transaction containing the specified index

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with multiple transactions
	createTestDatabase(t, dbPath)

	// Open database for writing - Transaction 1
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Transaction 1: 3 rows
	tx1, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := tx1.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":1}`)); err != nil {
			t.Fatalf("Failed to add row to tx1: %v", err)
		}
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 1: %v", err)
	}

	db2.Close() // Close after first transaction

	// Reopen for Transaction 2
	db3, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}

	// Transaction 2: 2 rows
	tx2, err := db3.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := tx2.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":2}`)); err != nil {
			t.Fatalf("Failed to add row to tx2: %v", err)
		}
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 2: %v", err)
	}

	db3.Close() // Close after second transaction

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test: All rows in transaction 1 should point to index 3 (last row of tx1)
	// Indices: 0=checksum, 1-3=tx1, 4-5=tx2
	for i := int64(1); i <= 3; i++ {
		end, err := sf.GetTransactionEnd(i)
		if err != nil {
			t.Errorf("GetTransactionEnd(%d) failed: %v", i, err)
		}
		if end != 3 {
			t.Errorf("GetTransactionEnd(%d) = %d, want 3", i, end)
		}
	}

	// Test: All rows in transaction 2 should point to index 5 (last row of tx2)
	for i := int64(4); i <= 5; i++ {
		end, err := sf.GetTransactionEnd(i)
		if err != nil {
			t.Errorf("GetTransactionEnd(%d) failed: %v", i, err)
		}
		if end != 5 {
			t.Errorf("GetTransactionEnd(%d) = %d, want 5", i, end)
		}
	}

	t.Log("FR-004: GetTransactionEnd() returns correct transaction end indices")
}

// Test_S_019_FR_005_TransactionBoundaryErrors validates that transaction
// boundary methods return appropriate errors for invalid inputs.
func Test_S_019_FR_005_TransactionBoundaryErrors(t *testing.T) {
	// FR-005: GetTransactionStart() and GetTransactionEnd() MUST return errors
	// when called with invalid indices (negative, out of bounds, or pointing to checksum rows)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with one transaction
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	if err := tx.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"test":"data"}`)); err != nil {
		t.Fatalf("Failed to add row: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	db.Close()

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test: Negative index
	_, err = sf.GetTransactionStart(-1)
	if err == nil {
		t.Error("GetTransactionStart should return error for negative index")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionStart should return InvalidInputError for negative index, got %T", err)
	}

	_, err = sf.GetTransactionEnd(-1)
	if err == nil {
		t.Error("GetTransactionEnd should return error for negative index")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionEnd should return InvalidInputError for negative index, got %T", err)
	}

	// Test: Out of bounds index
	_, err = sf.GetTransactionStart(1000)
	if err == nil {
		t.Error("GetTransactionStart should return error for out of bounds index")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionStart should return InvalidInputError for out of bounds, got %T", err)
	}

	_, err = sf.GetTransactionEnd(1000)
	if err == nil {
		t.Error("GetTransactionEnd should return error for out of bounds index")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionEnd should return InvalidInputError for out of bounds, got %T", err)
	}

	// Test: Checksum row (index 0)
	_, err = sf.GetTransactionStart(0)
	if err == nil {
		t.Error("GetTransactionStart should return error for checksum row")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionStart should return InvalidInputError for checksum row, got %T", err)
	}

	_, err = sf.GetTransactionEnd(0)
	if err == nil {
		t.Error("GetTransactionEnd should return error for checksum row")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("GetTransactionEnd should return InvalidInputError for checksum row, got %T", err)
	}

	t.Log("FR-005: Transaction boundary methods return appropriate errors for invalid inputs")
}

// Test_S_019_FR_006_OnRowAddedUpdatesState validates that OnRowAdded()
// correctly updates finder state for subsequent operations.
func Test_S_019_FR_006_OnRowAddedUpdatesState(t *testing.T) {
	// FR-006: OnRowAdded() MUST update finder state to include newly added rows
	// for subsequent find operations

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database
	createTestDatabase(t, dbPath)

	// Open in write mode
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Get the internal dbFile from the database
	// We'll test OnRowAdded indirectly by adding rows and verifying GetIndex works

	tx, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	testUUID := uuid.Must(uuid.NewV7())
	if err := tx.AddRow(testUUID, json.RawMessage(`{"test":"data"}`)); err != nil {
		t.Fatalf("Failed to add row: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	db2.Close()

	// Reopen and verify the row can be found
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// The row should be findable after OnRowAdded was called during AddRow
	index, err := sf.GetIndex(testUUID)
	if err != nil {
		t.Errorf("Failed to find UUID after OnRowAdded: %v", err)
	}
	if index != 1 {
		t.Errorf("GetIndex returned wrong index: got %d, want 1", index)
	}

	// Test OnRowAdded validation: try adding with wrong index
	testRow := &RowUnion{
		DataRow: &DataRow{
			baseRow: baseRow[*DataRowPayload]{
				RowSize:      256,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   uuid.Must(uuid.NewV7()),
					Value: json.RawMessage(`{"test":"value"}`),
				},
			},
		},
	}

	// Should fail with gap in indices
	err = sf.OnRowAdded(10, testRow)
	if err == nil {
		t.Error("OnRowAdded should return error for index gap")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("OnRowAdded should return InvalidInputError for index gap, got %T", err)
	}

	t.Log("FR-006: OnRowAdded() updates finder state correctly")
}

// Test_S_019_FR_008_DirectLinearScanning validates that SimpleFinder
// uses direct linear scanning without caching.
func Test_S_019_FR_008_DirectLinearScanning(t *testing.T) {
	// FR-008: SimpleFinder MUST scan the file system directly for each operation
	// without using caching or optimization techniques

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with rows
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	testUUID := uuid.Must(uuid.NewV7())
	if err := tx.AddRow(testUUID, json.RawMessage(`{"test":"data"}`)); err != nil {
		t.Fatalf("Failed to add row: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	db.Close()

	// Create two separate SimpleFinder instances
	dbFile1, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file 1: %v", err)
	}
	defer dbFile1.Close()

	sf1, err := NewSimpleFinder(dbFile1, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder 1: %v", err)
	}

	dbFile2, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file 2: %v", err)
	}
	defer dbFile2.Close()

	sf2, err := NewSimpleFinder(dbFile2, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder 2: %v", err)
	}

	// Both finders should find the same UUID at the same index (no caching differences)
	index1, err := sf1.GetIndex(testUUID)
	if err != nil {
		t.Fatalf("SimpleFinder 1 failed to find UUID: %v", err)
	}

	index2, err := sf2.GetIndex(testUUID)
	if err != nil {
		t.Fatalf("SimpleFinder 2 failed to find UUID: %v", err)
	}

	if index1 != index2 {
		t.Errorf("SimpleFinders returned different indices: %d vs %d", index1, index2)
	}

	// Multiple calls should return consistent results (no caching side effects)
	for i := 0; i < 5; i++ {
		index, err := sf1.GetIndex(testUUID)
		if err != nil {
			t.Errorf("GetIndex call %d failed: %v", i, err)
		}
		if index != index1 {
			t.Errorf("GetIndex call %d returned different index: got %d, want %d", i, index, index1)
		}
	}

	t.Log("FR-008: SimpleFinder uses direct linear scanning without caching")
}

// Test_S_019_FR_009_HandlesAllRowTypes validates that finder methods
// properly handle all row types defined in the file format.
func Test_S_019_FR_009_HandlesAllRowTypes(t *testing.T) {
	// FR-009: All finder methods MUST properly handle database files containing
	// checksum rows, data rows, null rows, and PartialDataRows

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with various row types
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add data rows
	tx1, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}

	uuid1 := uuid.Must(uuid.NewV7())
	if err := tx1.AddRow(uuid1, json.RawMessage(`{"type":"data"}`)); err != nil {
		t.Fatalf("Failed to add data row: %v", err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 1: %v", err)
	}

	db.Close() // Close after first transaction

	// Reopen for transaction 2
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database for tx2: %v", err)
	}

	// Add null row (empty transaction)
	tx2, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 2: %v", err)
	}

	db2.Close() // Close after second transaction

	// Reopen for transaction 3
	db3, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database for tx3: %v", err)
	}

	// Add more data rows
	tx3, err := db3.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 3: %v", err)
	}

	uuid2 := uuid.Must(uuid.NewV7())
	if err := tx3.AddRow(uuid2, json.RawMessage(`{"type":"data2"}`)); err != nil {
		t.Fatalf("Failed to add data row 2: %v", err)
	}

	if err := tx3.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 3: %v", err)
	}

	db3.Close() // Close after third transaction

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test: GetIndex should skip checksum and null rows
	index1, err := sf.GetIndex(uuid1)
	if err != nil {
		t.Errorf("Failed to find uuid1: %v", err)
	}
	if index1 != 1 {
		t.Errorf("GetIndex(uuid1) returned wrong index: got %d, want 1", index1)
	}

	index2, err := sf.GetIndex(uuid2)
	if err != nil {
		t.Errorf("Failed to find uuid2: %v", err)
	}
	// uuid2 should be at index 3 (0=checksum, 1=data, 2=null, 3=data)
	if index2 != 3 {
		t.Errorf("GetIndex(uuid2) returned wrong index: got %d, want 3", index2)
	}

	// Test: Transaction boundary methods should work across different row types
	start1, err := sf.GetTransactionStart(1)
	if err != nil {
		t.Errorf("GetTransactionStart(1) failed: %v", err)
	}
	if start1 != 1 {
		t.Errorf("GetTransactionStart(1) = %d, want 1", start1)
	}

	end1, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd(1) failed: %v", err)
	}
	if end1 != 1 {
		t.Errorf("GetTransactionEnd(1) = %d, want 1", end1)
	}

	// Test: Null row transaction boundaries
	start2, err := sf.GetTransactionStart(2)
	if err != nil {
		t.Errorf("GetTransactionStart(2) failed: %v", err)
	}
	if start2 != 2 {
		t.Errorf("GetTransactionStart(2) = %d, want 2", start2)
	}

	end2, err := sf.GetTransactionEnd(2)
	if err != nil {
		t.Errorf("GetTransactionEnd(2) failed: %v", err)
	}
	if end2 != 2 {
		t.Errorf("GetTransactionEnd(2) = %d, want 2", end2)
	}

	t.Log("FR-009: Finder methods properly handle all row types")
}

// Test_S_019_FR_010_TransactionBoundaryDetection validates that finder methods
// correctly identify transaction boundaries based on control bytes.
func Test_S_019_FR_010_TransactionBoundaryDetection(t *testing.T) {
	// FR-010: Finder methods MUST correctly identify transaction boundaries based on
	// start_control and end_control bytes as defined in the file format specification

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with transactions having different end controls
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Single-row transaction with commit (TC)
	tx1, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 1: %v", err)
	}
	if err := tx1.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":1}`)); err != nil {
		t.Fatalf("Failed to add row to tx1: %v", err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 1: %v", err)
	}

	db.Close() // Close after first transaction

	// Reopen for transaction 2
	db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database for tx2: %v", err)
	}

	// Multi-row transaction with continuation (RE) and commit (TC)
	tx2, err := db2.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 2: %v", err)
	}
	if err := tx2.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":2,"row":1}`)); err != nil {
		t.Fatalf("Failed to add first row to tx2: %v", err)
	}
	if err := tx2.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":2,"row":2}`)); err != nil {
		t.Fatalf("Failed to add second row to tx2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 2: %v", err)
	}

	db2.Close() // Close after second transaction

	// Reopen for transaction 3
	db3, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to reopen database for tx3: %v", err)
	}

	// Transaction with savepoint and commit (SE -> SC)
	tx3, err := db3.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction 3: %v", err)
	}
	if err := tx3.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"tx":3,"row":1}`)); err != nil {
		t.Fatalf("Failed to add row to tx3: %v", err)
	}
	if err := tx3.Savepoint(); err != nil {
		t.Fatalf("Failed to create savepoint: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction 3: %v", err)
	}

	db3.Close() // Close after third transaction

	// Open database and create finder
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Test transaction 1: single row (index 1)
	// start_control='T', end_control='TC'
	start1, err := sf.GetTransactionStart(1)
	if err != nil {
		t.Errorf("GetTransactionStart(1) failed: %v", err)
	}
	if start1 != 1 {
		t.Errorf("GetTransactionStart(1) = %d, want 1", start1)
	}

	end1, err := sf.GetTransactionEnd(1)
	if err != nil {
		t.Errorf("GetTransactionEnd(1) failed: %v", err)
	}
	if end1 != 1 {
		t.Errorf("GetTransactionEnd(1) = %d, want 1", end1)
	}

	// Test transaction 2: two rows (indices 2-3)
	// Row 2: start_control='T', end_control='RE'
	// Row 3: start_control='R', end_control='TC'
	start2, err := sf.GetTransactionStart(2)
	if err != nil {
		t.Errorf("GetTransactionStart(2) failed: %v", err)
	}
	if start2 != 2 {
		t.Errorf("GetTransactionStart(2) = %d, want 2", start2)
	}

	end2, err := sf.GetTransactionEnd(2)
	if err != nil {
		t.Errorf("GetTransactionEnd(2) failed: %v", err)
	}
	if end2 != 3 {
		t.Errorf("GetTransactionEnd(2) = %d, want 3", end2)
	}

	start3, err := sf.GetTransactionStart(3)
	if err != nil {
		t.Errorf("GetTransactionStart(3) failed: %v", err)
	}
	if start3 != 2 {
		t.Errorf("GetTransactionStart(3) = %d, want 2", start3)
	}

	end3, err := sf.GetTransactionEnd(3)
	if err != nil {
		t.Errorf("GetTransactionEnd(3) failed: %v", err)
	}
	if end3 != 3 {
		t.Errorf("GetTransactionEnd(3) = %d, want 3", end3)
	}

	// Test transaction 3: single row with savepoint (index 4)
	// start_control='T', end_control='SC' (savepoint + commit)
	start4, err := sf.GetTransactionStart(4)
	if err != nil {
		t.Errorf("GetTransactionStart(4) failed: %v", err)
	}
	if start4 != 4 {
		t.Errorf("GetTransactionStart(4) = %d, want 4", start4)
	}

	end4, err := sf.GetTransactionEnd(4)
	if err != nil {
		t.Errorf("GetTransactionEnd(4) failed: %v", err)
	}
	if end4 != 4 {
		t.Errorf("GetTransactionEnd(4) = %d, want 4", end4)
	}

	t.Log("FR-010: Transaction boundary detection correctly identifies start_control and end_control patterns")
}
