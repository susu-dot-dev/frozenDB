package frozendb

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func inmemoryFinderFactory(t *testing.T, path string, rowSize int32) (Finder, func()) {
	t.Helper()
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	f, err := NewInMemoryFinder(dbFile, rowSize)
	if err != nil {
		_ = dbFile.Close()
		t.Fatalf("NewInMemoryFinder: %v", err)
	}
	return f, func() { _ = dbFile.Close() }
}

// Test_S_021_FR_001_InMemoryFinderImplementation verifies that the system provides
// an InMemoryFinder implementation of the Finder interface.
func Test_S_021_FR_001_InMemoryFinderImplementation(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	defer dbFile.Close()
	f, err := NewInMemoryFinder(dbFile, confRowSize)
	if err != nil {
		t.Fatalf("NewInMemoryFinder: %v", err)
	}
	var _ Finder = f
	_, err = f.GetIndex(uuidFromTS(1))
	if err == nil {
		t.Error("GetIndex on empty DB should return KeyNotFoundError")
	}
}

// Test_S_021_FR_002_UUIDIndexMap verifies that InMemoryFinder maintains a map of
// UUID key to row index for O(1) GetIndex operations.
func Test_S_021_FR_002_UUIDIndexMap(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbAddDataRow(t, path, uuidFromTS(10), `{"v":1}`)
	dbAddDataRow(t, path, uuidFromTS(20), `{"v":2}`)
	dbAddDataRow(t, path, uuidFromTS(30), `{"v":3}`)
	finder, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()
	tests := []struct {
		key   uuid.UUID
		index int64
	}{
		{uuidFromTS(10), 1},
		{uuidFromTS(20), 2},
		{uuidFromTS(30), 3},
	}
	for _, tt := range tests {
		idx, err := finder.GetIndex(tt.key)
		if err != nil {
			t.Errorf("GetIndex(%v): %v", tt.key, err)
			continue
		}
		if idx != tt.index {
			t.Errorf("GetIndex(%v) = %d, want %d", tt.key, idx, tt.index)
		}
	}
}

// Test_S_021_FR_003_TransactionBoundaryMaps verifies that InMemoryFinder maintains
// transaction boundary indices for O(1) GetTransactionStart and GetTransactionEnd.
func Test_S_021_FR_003_TransactionBoundaryMaps(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	tx, db := openAndBegin(t, path)
	_ = tx.AddRow(uuidFromTS(1), json.RawMessage(`{}`))
	_ = tx.AddRow(uuidFromTS(2), json.RawMessage(`{}`))
	_ = tx.AddRow(uuidFromTS(3), json.RawMessage(`{}`))
	_ = tx.Commit()
	_ = db.Close()
	finder, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()
	start, err := finder.GetTransactionStart(2)
	if err != nil {
		t.Fatalf("GetTransactionStart(2): %v", err)
	}
	if start != 1 {
		t.Errorf("GetTransactionStart(2) = %d, want 1", start)
	}
	end, err := finder.GetTransactionEnd(2)
	if err != nil {
		t.Fatalf("GetTransactionEnd(2): %v", err)
	}
	if end != 3 {
		t.Errorf("GetTransactionEnd(2) = %d, want 3", end)
	}
}

// Test_S_021_FR_004_IndexUpdatesOnRowAddition verifies that InMemoryFinder updates
// its internal index when new rows are committed via OnRowAdded.
func Test_S_021_FR_004_IndexUpdatesOnRowAddition(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbAddDataRow(t, path, uuidFromTS(1), `{}`)
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	finder, err := NewInMemoryFinder(dbFile, confRowSize)
	if err != nil {
		dbFile.Close()
		t.Fatalf("NewInMemoryFinder: %v", err)
	}
	defer dbFile.Close()
	idx, _ := finder.GetIndex(uuidFromTS(1))
	if idx != 1 {
		t.Fatalf("GetIndex before OnRowAdded: got %d, want 1", idx)
	}
	ru := &RowUnion{DataRow: &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(confRowSize),
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload:   &DataRowPayload{Key: uuidFromTS(2), Value: json.RawMessage(`{}`)},
		},
	}}
	if err := finder.OnRowAdded(2, ru); err != nil {
		t.Fatalf("OnRowAdded(2, row): %v", err)
	}
	idx, err = finder.GetIndex(uuidFromTS(2))
	if err != nil {
		t.Fatalf("GetIndex(uuidFromTS(2)) after OnRowAdded: %v", err)
	}
	if idx != 2 {
		t.Errorf("GetIndex after OnRowAdded = %d, want 2", idx)
	}
}

// Test_S_021_FR_006_ConformanceTestPass verifies that InMemoryFinder passes all
// finder_conformance_test scenarios.
func Test_S_021_FR_006_ConformanceTestPass(t *testing.T) {
	RunFinderConformance(t, inmemoryFinderFactory)
}

// Test_S_021_FR_007_ThreadSafeAccess verifies that InMemoryFinder maintains
// thread-safe access for concurrent Get* method calls.
func Test_S_021_FR_007_ThreadSafeAccess(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	keys := []int{1, 2, 3, 4, 5}
	for _, ts := range keys {
		dbAddDataRow(t, path, uuidFromTS(ts), `{}`)
	}
	finder, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, ts := range keys {
				_, _ = finder.GetIndex(uuidFromTS(ts))
				_, _ = finder.GetTransactionStart(int64(ts))
				_, _ = finder.GetTransactionEnd(int64(ts))
			}
		}()
	}
	wg.Wait()
}
