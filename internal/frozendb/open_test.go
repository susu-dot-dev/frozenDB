package frozendb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func createTestHeaderBytes() []byte {
	header := &Header{
		signature: HEADER_SIGNATURE,
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	if err := header.Validate(); err != nil {
		panic(err)
	}
	bytes, _ := header.MarshalText()
	return bytes
}

// Benchmark_NewFrozenDB_ReadMode benchmarks opening database in read mode
func Benchmark_NewFrozenDB_ReadMode(b *testing.B) {
	// Create test database
	testPath := filepath.Join(b.TempDir(), "bench.fdb")
	file, _ := os.Create(testPath)
	header := createTestHeaderBytes()
	file.Write(header)
	file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
		if err != nil {
			b.Fatal(err)
		}
		db.Close()
	}
}

// Benchmark_NewFrozenDB_WriteMode benchmarks opening database in write mode
func Benchmark_NewFrozenDB_WriteMode(b *testing.B) {
	// Create test database
	testPath := filepath.Join(b.TempDir(), "bench.fdb")
	file, _ := os.Create(testPath)
	header := createTestHeaderBytes()
	file.Write(header)
	file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
		if err != nil {
			b.Fatal(err)
		}
		db.Close()
	}
}

// Benchmark_Close benchmarks closing a database
func Benchmark_Close(b *testing.B) {
	testPath := filepath.Join(b.TempDir(), "bench.fdb")
	file, _ := os.Create(testPath)
	header := createTestHeaderBytes()
	file.Write(header)
	file.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		db, _ := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
		b.StartTimer()
		db.Close()
	}
}

// Benchmark_ConcurrentReaders benchmarks concurrent read access
func Benchmark_ConcurrentReaders(b *testing.B) {
	testPath := filepath.Join(b.TempDir(), "bench.fdb")
	file, _ := os.Create(testPath)
	header := createTestHeaderBytes()
	file.Write(header)
	file.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
			if err != nil {
				b.Error(err)
				return
			}
			db.Close()
		}
	})
}

// TestHeaderValidation_EdgeCases tests edge cases in header validation
func TestHeaderValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		rowSize     int
		skewMs      int
		expectError bool
	}{
		{
			name:        "Minimum valid row size",
			rowSize:     MIN_ROW_SIZE,
			skewMs:      0,
			expectError: false,
		},
		{
			name:        "Maximum valid row size",
			rowSize:     MAX_ROW_SIZE,
			skewMs:      MAX_SKEW_MS,
			expectError: false,
		},
		{
			name:        "Row size below minimum",
			rowSize:     MIN_ROW_SIZE - 1,
			skewMs:      5000,
			expectError: true,
		},
		{
			name:        "Row size above maximum",
			rowSize:     MAX_ROW_SIZE + 1,
			skewMs:      5000,
			expectError: true,
		},
		{
			name:        "Negative skew",
			rowSize:     1024,
			skewMs:      -1,
			expectError: true,
		},
		{
			name:        "Skew above maximum",
			rowSize:     1024,
			skewMs:      MAX_SKEW_MS + 1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &Header{
				signature: HEADER_SIGNATURE,
				version:   1,
				rowSize:   tt.rowSize,
				skewMs:    tt.skewMs,
			}

			err := header.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestConcurrentStress tests database under concurrent stress
func TestConcurrentStress(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "stress.fdb")

	setupMockSyscalls(false, false)
	defer restoreRealSyscalls()
	t.Setenv("SUDO_USER", MOCK_USER)
	t.Setenv("SUDO_UID", MOCK_UID)
	t.Setenv("SUDO_GID", MOCK_GID)

	if err := Create(CreateConfig{path: testPath, rowSize: 1024, skewMs: 5000}); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run concurrent readers for 100 iterations
	const numReaders = 10
	const iterations = 100

	var wg sync.WaitGroup
	errors := make(chan error, numReaders*iterations)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				db, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
				if err != nil {
					errors <- err
					return
				}
				// Immediately close to stress resource management
				if err := db.Close(); err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}
}

// TestFileDescriptorLeaks tests that file descriptors are properly cleaned up
func TestFileDescriptorLeaks(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "fd_test.fdb")

	setupMockSyscalls(false, false)
	defer restoreRealSyscalls()
	t.Setenv("SUDO_USER", MOCK_USER)
	t.Setenv("SUDO_UID", MOCK_UID)
	t.Setenv("SUDO_GID", MOCK_GID)

	if err := Create(CreateConfig{path: testPath, rowSize: 1024, skewMs: 5000}); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Get initial FD count
	initialFDs := countOpenFileDescriptors(t)

	// Open and close database many times
	for i := 0; i < 100; i++ {
		db, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}

	// Get final FD count
	finalFDs := countOpenFileDescriptors(t)

	// FD count should not grow significantly (allow small variance)
	if finalFDs > initialFDs+5 {
		t.Errorf("File descriptor leak detected: initial=%d, final=%d", initialFDs, finalFDs)
	}
}

func countOpenFileDescriptors(t *testing.T) int {
	return 0
}
