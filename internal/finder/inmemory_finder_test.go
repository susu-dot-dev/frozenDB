package finder

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestNewInMemoryFinder_InvalidInputs(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbFile, _ := NewDBFile(path, MODE_READ)
	defer dbFile.Close()

	tests := []struct {
		name    string
		dbFile  DBFile
		rowSize int32
	}{
		{"nil dbFile", nil, 1024},
		{"rowSize 127", dbFile, 127},
		{"rowSize 65537", dbFile, 65537},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewInMemoryFinder(tt.dbFile, tt.rowSize)
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
	f, err := NewInMemoryFinder(dbFile, confRowSize)
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
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbAddDataRow(t, path, uuidFromTS(1), `{}`)
	f, cleanup := inmemoryFinderFactory(t, path, confRowSize)
	defer cleanup()

	ru := &RowUnion{DataRow: &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      int(confRowSize),
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload:   &DataRowPayload{Key: uuidFromTS(2), Value: json.RawMessage(`{}`)},
		},
	}}

	err := f.OnRowAdded(0, nil)
	if err == nil {
		t.Fatal("OnRowAdded(0, nil) expected error")
	}
	var e *InvalidInputError
	if !errors.As(err, &e) {
		t.Errorf("expected InvalidInputError, got %T", err)
	}

	err = f.OnRowAdded(1, ru)
	if err == nil {
		t.Fatal("OnRowAdded(1, ru) when expected 2: expected error")
	}

	err = f.OnRowAdded(4, ru)
	if err == nil {
		t.Fatal("OnRowAdded(4, ru) when expected 2: expected error")
	}
}

func TestInMemoryFinder_OnRowAdded_ChecksumRow(t *testing.T) {
	dir := t.TempDir()
	path := setupCreate(t, dir, 0)
	dbAddDataRow(t, path, uuidFromTS(1), `{}`)
	dbFile, _ := NewDBFile(path, MODE_READ)
	f, err := NewInMemoryFinder(dbFile, confRowSize)
	if err != nil {
		dbFile.Close()
		t.Fatalf("NewInMemoryFinder: %v", err)
	}
	defer dbFile.Close()

	cs := &RowUnion{ChecksumRow: &ChecksumRow{}}
	if err := f.OnRowAdded(2, cs); err != nil {
		t.Fatalf("OnRowAdded(2, ChecksumRow): %v", err)
	}
	idx, err := f.GetIndex(uuidFromTS(1))
	if err != nil {
		t.Fatalf("GetIndex after OnRowAdded ChecksumRow: %v", err)
	}
	if idx != 1 {
		t.Errorf("GetIndex = %d, want 1", idx)
	}
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
	f, _ := NewInMemoryFinder(dbFile, confRowSize)
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
	f, err := NewInMemoryFinder(dbFile, rowSize)
	if err != nil {
		dbFile.Close()
		b.Fatalf("NewInMemoryFinder: %v", err)
	}
	return f, func() { _ = dbFile.Close() }
}
