package frozendb

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// createTestRowEmitterForInMemoryTests creates a RowEmitter for testing purposes
func createTestRowEmitterForInMemoryTests(t *testing.T, dbFile DBFile, rowSize int32) *RowEmitter {
	t.Helper()
	emitter, err := NewRowEmitter(dbFile, int(rowSize))
	if err != nil {
		t.Fatalf("Failed to create RowEmitter: %v", err)
	}
	return emitter
}

func TestNewInMemoryFinder_InvalidInputs(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbFile, _ := NewDBFile(path, MODE_READ)
	defer dbFile.Close()

	// Create a valid RowEmitter for tests that have valid dbFile and rowSize
	validRowEmitter := createTestRowEmitterForInMemoryTests(t, dbFile, 1024)

	tests := []struct {
		name       string
		dbFile     DBFile
		rowSize    int32
		rowEmitter *RowEmitter
	}{
		{"nil dbFile", nil, 1024, nil},
		{"rowSize 127", dbFile, 127, validRowEmitter},
		{"rowSize 65537", dbFile, 65537, validRowEmitter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInMemoryFinder(tt.dbFile, tt.rowSize, tt.rowEmitter)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var e *InvalidInputError
			if !errors.As(err, &e) {
				t.Errorf("expected InvalidInputError, got %T", err)
			}
		})
	}
}

func TestNewInMemoryFinder_ValidInputs(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile: %v", err)
	}
	defer dbFile.Close()
	f, err := NewInMemoryFinder(dbFile, confRowSize, createTestRowEmitterForInMemoryTests(t, dbFile, confRowSize))
	if err != nil {
		t.Fatalf("NewInMemoryFinder: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil finder")
	}
	if f.rowSize != confRowSize {
		t.Errorf("rowSize = %d, want %d", f.rowSize, confRowSize)
	}
}

func TestInMemoryFinder_GetIndex_InvalidUUID(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	f, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()

	_, err := f.GetIndex(uuid.Nil)
	if err == nil {
		t.Fatal("GetIndex(uuid.Nil) expected error")
	}
	var e *InvalidInputError
	if !errors.As(err, &e) {
		t.Errorf("expected InvalidInputError, got %T", err)
	}

	_, err = f.GetIndex(uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))
	if err == nil {
		t.Fatal("GetIndex(non-v7) expected error")
	}
	if !errors.As(err, &e) {
		t.Errorf("expected InvalidInputError, got %T", err)
	}
}

func TestInMemoryFinder_GetTransactionStart_ChecksumRow(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	f, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()

	_, err := f.GetTransactionStart(0)
	if err == nil {
		t.Fatal("GetTransactionStart(0) expected error for checksum row")
	}
	var e *InvalidInputError
	if !errors.As(err, &e) {
		t.Errorf("expected InvalidInputError, got %T", err)
	}
}

func TestInMemoryFinder_OnRowAdded_Validation(t *testing.T) {
	t.Skip("OnRowAdded is no longer public API - internal onRowAdded is now called via RowEmitter subscriptions")
}

func TestInMemoryFinder_OnRowAdded_ChecksumRow(t *testing.T) {
	t.Skip("OnRowAdded is no longer public API - internal onRowAdded is now called via RowEmitter subscriptions")
}

func TestInMemoryFinder_ConcurrentGets(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	for i := 1; i <= 10; i++ {
		dbAddDataRow(t, path, uuidFromTS(i), `{}`)
	}
	f, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ts := 1; ts <= 10; ts++ {
				idx, err := f.GetIndex(uuidFromTS(ts))
				if err != nil || idx < 0 {
					t.Errorf("GetIndex: err=%v idx=%d", err, idx)
				}
				_, _ = f.GetTransactionStart(idx)
				_, _ = f.GetTransactionEnd(idx)
			}
		}()
	}
	wg.Wait()
}

func BenchmarkInMemoryFinder_GetIndex(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bm.fdb")
	setupCreateB(b, dir, path)
	for i := 1; i <= 1000; i++ {
		dbAddDataRowB(b, path, uuidFromTS(i), `{}`)
	}
	dbFile, _ := NewDBFile(path, MODE_READ)
	rowEmitter, err := NewRowEmitter(dbFile, int(confRowSize))
	if err != nil {
		b.Fatalf("NewRowEmitter: %v", err)
	}
	f, _ := NewInMemoryFinder(dbFile, confRowSize, rowEmitter)
	defer dbFile.Close()
	key := uuidFromTS(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.GetIndex(key)
	}
}

func BenchmarkInMemoryFinder_GetTransactionStart(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bm.fdb")
	setupCreateB(b, dir, path)
	for i := 1; i <= 1000; i++ {
		dbAddDataRowB(b, path, uuidFromTS(i), `{}`)
	}
	f, cleanup := inmemoryFinderFactoryB(b, path, confRowSize)
	defer cleanup()
	idx := int64(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.GetTransactionStart(idx)
	}
}

func BenchmarkInMemoryFinder_GetTransactionEnd(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bm.fdb")
	setupCreateB(b, dir, path)
	for i := 1; i <= 1000; i++ {
		dbAddDataRowB(b, path, uuidFromTS(i), `{}`)
	}
	f, cleanup := inmemoryFinderFactoryB(b, path, confRowSize)
	defer cleanup()
	idx := int64(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.GetTransactionEnd(idx)
	}
}

func setupCreateB(b *testing.B, dir, path string) {
	b.Helper()
	setupMockSyscalls(false, false)
	b.Cleanup(restoreRealSyscalls)
	b.Setenv("SUDO_USER", MOCK_USER)
	b.Setenv("SUDO_UID", MOCK_UID)
	b.Setenv("SUDO_GID", MOCK_GID)
	if err := Create(CreateConfig{path: path, rowSize: confRowSize, skewMs: confSkewMs}); err != nil {
		b.Fatalf("Create: %v", err)
	}
}

func dbAddDataRowB(b *testing.B, path string, key uuid.UUID, value string) {
	b.Helper()
	db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		b.Fatalf("NewFrozenDB: %v", err)
	}
	defer db.Close()
	tx, _ := db.BeginTx()
	_ = tx.AddRow(key, json.RawMessage(value))
	_ = tx.Commit()
}

func inmemoryFinderFactoryB(b *testing.B, path string, rowSize int32) (Finder, func()) {
	b.Helper()
	dbFile, err := NewDBFile(path, MODE_READ)
	if err != nil {
		b.Fatalf("NewDBFile: %v", err)
	}
	rowEmitter, err := NewRowEmitter(dbFile, int(rowSize))
	if err != nil {
		dbFile.Close()
		b.Fatalf("NewRowEmitter: %v", err)
	}
	f, err := NewInMemoryFinder(dbFile, rowSize, rowEmitter)
	if err != nil {
		dbFile.Close()
		b.Fatalf("NewInMemoryFinder: %v", err)
	}
	return f, func() { _ = dbFile.Close() }
}
