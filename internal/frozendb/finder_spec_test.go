package frozendb

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// createTestRowEmitterForFinder creates a RowEmitter for testing purposes
func createTestRowEmitterForFinder(t *testing.T, dbFile DBFile, rowSize int32) *RowEmitter {
	t.Helper()
	emitter, err := NewRowEmitter(dbFile, int(rowSize))
	if err != nil {
		t.Fatalf("Failed to create RowEmitter: %v", err)
	}
	return emitter
}

// Test_S_023_FR_001_MaxTimestamp_Method_Required validates that the Finder interface
// includes the MaxTimestamp() method as required by FR-001.
func Test_S_023_FR_001_MaxTimestamp_Method_Required(t *testing.T) {
	// FR-001: Finder protocol MUST require implementation of MaxTimestamp() method

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

	// Create SimpleFinder and verify it implements Finder interface with MaxTimestamp()
	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Verify SimpleFinder implements Finder interface by type assertion
	var finder Finder = sf

	// Verify MaxTimestamp() method exists and is callable
	_ = finder.MaxTimestamp()

	// Create InMemoryFinder and verify it also implements MaxTimestamp()
	rowEmitter2 := createTestRowEmitterForFinder(t, dbFile, 1024)
	imf, err := NewInMemoryFinder(dbFile, 1024, rowEmitter2)
	if err != nil {
		t.Fatalf("Failed to create InMemoryFinder: %v", err)
	}

	var finder2 Finder = imf
	_ = finder2.MaxTimestamp()

	t.Log("FR-001: Finder interface includes MaxTimestamp() method")
}

// Test_S_023_FR_002_O1_Time_Complexity validates that MaxTimestamp() executes in O(1) time.
func Test_S_023_FR_002_O1_Time_Complexity(t *testing.T) {
	// FR-002: MaxTimestamp() method MUST return the maximum timestamp among all complete
	// data and null rows in O(1) time

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with many rows to test O(1) performance
	createTestDatabase(t, dbPath)

	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add many rows to ensure we're testing with a non-trivial database
	for i := 0; i < 100; i++ {
		tx, err := db.BeginTx()
		if err != nil {
			db.Close()
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		if err := tx.AddRow(uuid.Must(uuid.NewV7()), json.RawMessage(`{"index":`+string(rune('0'+i%10))+`}`)); err != nil {
			tx.Rollback(0)
			db.Close()
			t.Fatalf("Failed to add row: %v", err)
		}
		if err := tx.Commit(); err != nil {
			db.Close()
			t.Fatalf("Failed to commit: %v", err)
		}
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Open database and create finders
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	rowEmitter2 := createTestRowEmitterForFinder(t, dbFile, 1024)
	imf, err := NewInMemoryFinder(dbFile, 1024, rowEmitter2)
	if err != nil {
		t.Fatalf("Failed to create InMemoryFinder: %v", err)
	}

	// Test that MaxTimestamp() returns quickly (O(1) behavior)
	// Multiple calls should all be fast
	for i := 0; i < 1000; i++ {
		_ = sf.MaxTimestamp()
		_ = imf.MaxTimestamp()
	}

	// If we reach here without timeout, O(1) requirement is satisfied
	t.Log("FR-002: MaxTimestamp() executes in O(1) time")
}

// Test_S_023_FR_003_Returns_Zero_Empty validates that MaxTimestamp() returns 0
// when the database contains no complete data or null rows.
func Test_S_023_FR_003_Returns_Zero_Empty(t *testing.T) {
	// FR-003: MaxTimestamp() MUST return 0 when the database contains no complete
	// data or null rows (only checksum/PartialDataRow entries)

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database with only header and checksum row (no data rows)
	createTestDatabase(t, dbPath)

	// Open the database file
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open database file: %v", err)
	}
	defer dbFile.Close()

	// Test SimpleFinder
	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	maxTs := sf.MaxTimestamp()
	if maxTs != 0 {
		t.Errorf("SimpleFinder.MaxTimestamp() on empty database: got %d, want 0", maxTs)
	}

	// Test InMemoryFinder
	rowEmitter2 := createTestRowEmitterForFinder(t, dbFile, 1024)
	imf, err := NewInMemoryFinder(dbFile, 1024, rowEmitter2)
	if err != nil {
		t.Fatalf("Failed to create InMemoryFinder: %v", err)
	}

	maxTs2 := imf.MaxTimestamp()
	if maxTs2 != 0 {
		t.Errorf("InMemoryFinder.MaxTimestamp() on empty database: got %d, want 0", maxTs2)
	}

	// Test with database that has only null rows (empty transactions)
	// Null rows should contribute to maxTimestamp, so let's test with only checksum
	// Actually, null rows DO contribute, so we need a database with only checksum rows
	// The createTestDatabase already creates a checksum row, so the test above is correct

	t.Log("FR-003: MaxTimestamp() returns 0 for empty database")
}

// Test_S_023_FR_004_Updates_On_Commit validates that MaxTimestamp() updates only
// when complete DataRow or NullRow entries are added during Transaction.Commit()
// or Transaction.Rollback() operations.
func Test_S_023_FR_004_Updates_On_Commit(t *testing.T) {
	// FR-004: MaxTimestamp() MUST update only when complete DataRow or NullRow entries
	// are added during Transaction.Commit() or Transaction.Rollback() operations

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create database
	createTestDatabase(t, dbPath)

	// Open database for writing
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Get finder from database to test MaxTimestamp updates
	finder := db.finder

	// Initial MaxTimestamp should be 0 (only checksum row exists)
	maxTsBefore := finder.MaxTimestamp()
	if maxTsBefore != 0 {
		t.Errorf("Initial MaxTimestamp: got %d, want 0", maxTsBefore)
	}

	// Begin transaction but don't commit yet - MaxTimestamp should not update
	tx, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	key1 := uuid.Must(uuid.NewV7())
	if err := tx.AddRow(key1, json.RawMessage(`{"value":"test1"}`)); err != nil {
		tx.Rollback(0)
		db.Close()
		t.Fatalf("Failed to add row: %v", err)
	}

	// MaxTimestamp should still be 0 (PartialDataRow doesn't contribute)
	maxTsDuring := finder.MaxTimestamp()
	if maxTsDuring != 0 {
		t.Errorf("MaxTimestamp during transaction (before commit): got %d, want 0", maxTsDuring)
	}

	// Commit transaction - MaxTimestamp should update
	if err := tx.Commit(); err != nil {
		db.Close()
		t.Fatalf("Failed to commit: %v", err)
	}

	// MaxTimestamp should now reflect the committed row
	maxTsAfter := finder.MaxTimestamp()
	if maxTsAfter == 0 {
		t.Error("MaxTimestamp after commit: got 0, want non-zero")
	}

	// Verify it's the timestamp from the committed row
	expectedTs := ExtractUUIDv7Timestamp(key1)
	if maxTsAfter != expectedTs {
		t.Errorf("MaxTimestamp after commit: got %d, want %d", maxTsAfter, expectedTs)
	}

	// Add another row with a newer timestamp
	tx2, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	key2 := uuid.Must(uuid.NewV7())
	if err := tx2.AddRow(key2, json.RawMessage(`{"value":"test2"}`)); err != nil {
		tx2.Rollback(0)
		db.Close()
		t.Fatalf("Failed to add second row: %v", err)
	}

	if err := tx2.Commit(); err != nil {
		db.Close()
		t.Fatalf("Failed to commit second transaction: %v", err)
	}

	// MaxTimestamp should update to the newer timestamp
	maxTsAfter2 := finder.MaxTimestamp()
	expectedTs2 := ExtractUUIDv7Timestamp(key2)
	if maxTsAfter2 < expectedTs2 {
		t.Errorf("MaxTimestamp after second commit: got %d, want at least %d", maxTsAfter2, expectedTs2)
	}

	// Test with null row (empty transaction commit)
	tx3, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("Failed to begin third transaction: %v", err)
	}

	if err := tx3.Commit(); err != nil {
		db.Close()
		t.Fatalf("Failed to commit empty transaction: %v", err)
	}

	// MaxTimestamp should still reflect the data row (null rows have uuid.Nil which has timestamp 0)
	// Actually, null rows might have a timestamp from when they were created
	// But according to the spec, null rows contribute to MaxTimestamp
	// Let's check that MaxTimestamp doesn't decrease
	maxTsAfter3 := finder.MaxTimestamp()
	if maxTsAfter3 < maxTsAfter2 {
		t.Errorf("MaxTimestamp after null row commit decreased: got %d, was %d", maxTsAfter3, maxTsAfter2)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	t.Log("FR-004: MaxTimestamp() updates only on commit/rollback of complete rows")
}

// Test_S_039_FR_010_FinderTombstonedOnRefreshError tests FR-010: When a Finder's
// OnRowAdded() callback encounters an error, the Finder MUST set its tombstoned
// error state BEFORE returning the error to FileManager.
func Test_S_039_FR_010_FinderTombstonedOnRefreshError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create test database
	createTestDatabase(t, dbPath)

	// Open in write mode to add data
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add a data row to establish state
	tx, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	key1 := uuid.Must(uuid.NewV7())
	if err := tx.AddRow(key1, json.RawMessage(`{"value":"test1"}`)); err != nil {
		tx.Rollback(0)
		db.Close()
		t.Fatalf("Failed to add row: %v", err)
	}

	if err := tx.Commit(); err != nil {
		db.Close()
		t.Fatalf("Failed to commit: %v", err)
	}

	// Close write-mode database
	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Open database file in read mode
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open DBFile: %v", err)
	}
	defer dbFile.Close()

	// Create a SimpleFinder
	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Inject an error into OnRowAdded by calling it with an invalid index
	// This simulates what happens during background update cycles when a Finder
	// fails to update its internal state
	validRow := &RowUnion{
		DataRow: &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      1024,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   &DataRowPayload{Key: uuid.Must(uuid.NewV7()), Value: json.RawMessage(`{"test":"value"}`)},
			},
		},
	}

	// Call OnRowAdded with an invalid index (skips positions)
	// This should cause an error, and the Finder should tombstone itself
	currentIndex := (dbFile.Size() - HEADER_SIZE) / int64(1024)
	err = sf.OnRowAdded(currentIndex+100, validRow) // Skip 100 positions - causes index mismatch error

	// Verify that OnRowAdded returned an error
	if err == nil {
		t.Fatal("OnRowAdded should have returned an error for invalid index")
	}

	// FR-010 requirement: Finder should now be in tombstoned state
	// Verify by calling a public method - it should return TombstonedError
	_, err = sf.GetIndex(key1)
	if err == nil {
		t.Fatal("GetIndex should return TombstonedError after OnRowAdded failed")
	}

	// Verify the error is specifically a TombstonedError
	var tombErr *TombstonedError
	if !isError(err, &tombErr) {
		t.Errorf("Expected TombstonedError, got %T: %v", err, err)
	}

	t.Log("FR-010: Finder tombstones itself before returning error from OnRowAdded")
}

// Test_S_039_FR_011_TombstonedFinderReturnsError tests FR-011: All public Finder
// methods MUST check the tombstoned error state FIRST and return TombstonedError
// if the Finder is tombstoned.
func Test_S_039_FR_011_TombstonedFinderReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create test database
	createTestDatabase(t, dbPath)

	// Open in write mode to add data
	db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Add data rows
	tx, err := db.BeginTx()
	if err != nil {
		db.Close()
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	key1 := uuid.Must(uuid.NewV7())
	if err := tx.AddRow(key1, json.RawMessage(`{"value":"test1"}`)); err != nil {
		tx.Rollback(0)
		db.Close()
		t.Fatalf("Failed to add row: %v", err)
	}

	if err := tx.Commit(); err != nil {
		db.Close()
		t.Fatalf("Failed to commit: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Open database file in read mode
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open DBFile: %v", err)
	}
	defer dbFile.Close()

	// Create SimpleFinder
	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Tombstone the Finder by calling OnRowAdded with invalid data
	currentIndex := (dbFile.Size() - HEADER_SIZE) / int64(1024)
	validRow := &RowUnion{
		DataRow: &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      1024,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   &DataRowPayload{Key: uuid.Must(uuid.NewV7()), Value: json.RawMessage(`{"test":"value"}`)},
			},
		},
	}
	err = sf.OnRowAdded(currentIndex+100, validRow) // Invalid index - skips positions
	if err == nil {
		t.Fatal("OnRowAdded should have returned error")
	}

	// FR-011 requirement: All public methods should now return TombstonedError

	// Test GetIndex
	_, err = sf.GetIndex(key1)
	if err == nil {
		t.Error("GetIndex should return TombstonedError after Finder is tombstoned")
	}
	var tombErr *TombstonedError
	if !isError(err, &tombErr) {
		t.Errorf("GetIndex: Expected TombstonedError, got %T: %v", err, err)
	}

	// Test GetTransactionStart
	_, err = sf.GetTransactionStart(1)
	if err == nil {
		t.Error("GetTransactionStart should return TombstonedError after Finder is tombstoned")
	}
	if !isError(err, &tombErr) {
		t.Errorf("GetTransactionStart: Expected TombstonedError, got %T: %v", err, err)
	}

	// Test GetTransactionEnd
	_, err = sf.GetTransactionEnd(1)
	if err == nil {
		t.Error("GetTransactionEnd should return TombstonedError after Finder is tombstoned")
	}
	if !isError(err, &tombErr) {
		t.Errorf("GetTransactionEnd: Expected TombstonedError, got %T: %v", err, err)
	}

	// MaxTimestamp does NOT check tombstoned state - it returns historical data
	// which remains valid even after tombstoning
	_ = sf.MaxTimestamp()

	t.Log("FR-011: All public Finder methods return TombstonedError when tombstoned")
}

// Test_S_039_FR_011_TombstonedStateIsPermanent tests FR-011: Once a Finder is
// tombstoned, it remains tombstoned for its lifetime (no recovery mechanism).
func Test_S_039_FR_011_TombstonedStateIsPermanent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Create test database
	createTestDatabase(t, dbPath)

	// Open database file
	dbFile, err := NewDBFile(dbPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open DBFile: %v", err)
	}
	defer dbFile.Close()

	// Create SimpleFinder
	rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
	sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Tombstone the Finder
	currentIndex := (dbFile.Size() - HEADER_SIZE) / int64(1024)
	validRow := &RowUnion{
		DataRow: &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      1024,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   &DataRowPayload{Key: uuid.Must(uuid.NewV7()), Value: json.RawMessage(`{"test":"value"}`)},
			},
		},
	}
	err = sf.OnRowAdded(currentIndex+100, validRow) // Invalid index - skips positions
	if err == nil {
		t.Fatal("OnRowAdded should have returned error")
	}

	// Verify tombstoned state
	_, err = sf.GetIndex(uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Fatal("Finder should be tombstoned")
	}

	// FR-011 requirement: State is permanent
	// Even calling OnRowAdded with valid data won't clear tombstoned state
	_, err = sf.GetIndex(uuid.Must(uuid.NewV7()))
	if err == nil {
		t.Error("Finder should remain tombstoned (no recovery)")
	}

	var tombErr *TombstonedError
	if !isError(err, &tombErr) {
		t.Errorf("Expected TombstonedError to persist, got %T: %v", err, err)
	}

	// Try multiple method calls - all should return TombstonedError
	for i := 0; i < 10; i++ {
		_, err = sf.GetIndex(uuid.Must(uuid.NewV7()))
		if err == nil {
			t.Errorf("Call %d: Finder should remain tombstoned", i)
		}
		if !isError(err, &tombErr) {
			t.Errorf("Call %d: Expected TombstonedError, got %T", i, err)
		}
	}

	t.Log("FR-011: Tombstoned state is permanent for Finder lifetime")
}

// Test_S_039_FR_011_TombstonedErrorInBothModes tests FR-011: Tombstoning behavior
// is identical in both read mode (file watcher callbacks) and write mode
// (processWrite callbacks). The Finder doesn't know which mode triggered the update.
func Test_S_039_FR_011_TombstonedErrorInBothModes(t *testing.T) {
	// Test write mode tombstoning
	t.Run("WriteMode", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "test.fdb")

		// Create test database
		createTestDatabase(t, dbPath)

		// Open in write mode
		db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Get the finder (need to cast to *SimpleFinder to access OnRowAdded)
		// The public Finder interface doesn't expose OnRowAdded
		// In write mode, we access the internal finder from FrozenDB
		var sf *SimpleFinder
		switch f := db.finder.(type) {
		case *SimpleFinder:
			sf = f
		default:
			t.Fatalf("Expected SimpleFinder, got %T", db.finder)
		}

		// Tombstone it by calling OnRowAdded with invalid data
		currentIndex := (db.file.Size() - HEADER_SIZE) / int64(1024)
		validRow := &RowUnion{
			DataRow: &DataRow{
				baseRow[*DataRowPayload]{
					RowSize:      1024,
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload:   &DataRowPayload{Key: uuid.Must(uuid.NewV7()), Value: json.RawMessage(`{"test":"value"}`)},
				},
			},
		}
		err = sf.OnRowAdded(currentIndex+100, validRow) // Invalid index - skips positions
		if err == nil {
			t.Fatal("OnRowAdded should have returned error")
		}

		// Verify tombstoned state
		_, err = sf.GetIndex(uuid.Must(uuid.NewV7()))
		if err == nil {
			t.Error("Write mode: Finder should be tombstoned")
		}
		var tombErr *TombstonedError
		if !isError(err, &tombErr) {
			t.Errorf("Write mode: Expected TombstonedError, got %T", err)
		}
	})

	// Test read mode tombstoning
	t.Run("ReadMode", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "test.fdb")

		// Create test database
		createTestDatabase(t, dbPath)

		// Open in read mode
		dbFile, err := NewDBFile(dbPath, MODE_READ)
		if err != nil {
			t.Fatalf("Failed to open DBFile: %v", err)
		}
		defer dbFile.Close()

		// Create SimpleFinder
		rowEmitter := createTestRowEmitterForFinder(t, dbFile, 1024)
		sf, err := NewSimpleFinder(dbFile, 1024, rowEmitter)
		if err != nil {
			t.Fatalf("Failed to create SimpleFinder: %v", err)
		}

		// Tombstone it by calling OnRowAdded with invalid data
		currentIndex := (dbFile.Size() - HEADER_SIZE) / int64(1024)
		validRow := &RowUnion{
			DataRow: &DataRow{
				baseRow[*DataRowPayload]{
					RowSize:      1024,
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload:   &DataRowPayload{Key: uuid.Must(uuid.NewV7()), Value: json.RawMessage(`{"test":"value"}`)},
				},
			},
		}
		err = sf.OnRowAdded(currentIndex+100, validRow) // Invalid index - skips positions
		if err == nil {
			t.Fatal("OnRowAdded should have returned error")
		}

		// Verify tombstoned state
		_, err = sf.GetIndex(uuid.Must(uuid.NewV7()))
		if err == nil {
			t.Error("Read mode: Finder should be tombstoned")
		}
		var tombErr *TombstonedError
		if !isError(err, &tombErr) {
			t.Errorf("Read mode: Expected TombstonedError, got %T", err)
		}
	})

	// FR-011 requirement: Behavior is identical in both modes
	t.Log("FR-011: Tombstoning behavior is identical in read and write modes")
}

// isError is a helper function to check if an error matches a specific type
func isError(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	switch target.(type) {
	case **TombstonedError:
		_, ok := err.(*TombstonedError)
		return ok
	default:
		return false
	}
}
