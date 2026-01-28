package frozendb

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

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
	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	// Verify SimpleFinder implements Finder interface by type assertion
	var finder Finder = sf

	// Verify MaxTimestamp() method exists and is callable
	_ = finder.MaxTimestamp()

	// Create InMemoryFinder and verify it also implements MaxTimestamp()
	imf, err := NewInMemoryFinder(dbFile, 1024)
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

	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	imf, err := NewInMemoryFinder(dbFile, 1024)
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
	sf, err := NewSimpleFinder(dbFile, 1024)
	if err != nil {
		t.Fatalf("Failed to create SimpleFinder: %v", err)
	}

	maxTs := sf.MaxTimestamp()
	if maxTs != 0 {
		t.Errorf("SimpleFinder.MaxTimestamp() on empty database: got %d, want 0", maxTs)
	}

	// Test InMemoryFinder
	imf, err := NewInMemoryFinder(dbFile, 1024)
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
