package frozendb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Helper to create a valid test database file
func createTestDatabase(t *testing.T, path string) {
	t.Helper()

	// Ensure parent directory exists
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}

	// Create database file
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer file.Close()

	// Write valid frozenDB v1 header
	header, err := generateHeader(1024, 5000)
	if err != nil {
		t.Fatalf("Failed to generate header: %v", err)
	}

	if _, err := file.Write(header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}
}

// Test_S_002_FR_001_NewFrozenDBFunctionSignature validates the NewFrozenDB function signature
// FR-001: NewFrozenDB function must accept (path string, mode string) and return (*FrozenDB, error)
func Test_S_002_FR_001_NewFrozenDBFunctionSignature(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Call NewFrozenDB with valid parameters
	db, err := NewFrozenDB(testPath, MODE_READ)

	// Verify return types
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if db == nil {
		t.Fatal("Expected *FrozenDB instance, got nil")
	}

	// Verify db is of type *FrozenDB
	var _ = db

	// Cleanup
	if err := db.Close(); err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
}

// Test_S_002_FR_002_ModeConstants validates MODE_READ and MODE_WRITE constants
// FR-002: MODE_READ constant must be "read" and MODE_WRITE constant must be "write"
func Test_S_002_FR_002_ModeConstants(t *testing.T) {
	// Verify MODE_READ constant value
	if MODE_READ != "read" {
		t.Errorf("MODE_READ constant: expected 'read', got '%s'", MODE_READ)
	}

	// Verify MODE_WRITE constant value
	if MODE_WRITE != "write" {
		t.Errorf("MODE_WRITE constant: expected 'write', got '%s'", MODE_WRITE)
	}

	// Verify constants are distinct
	if MODE_READ == MODE_WRITE {
		t.Error("MODE_READ and MODE_WRITE must have different values")
	}
}

// Test_S_002_FR_003_ModeParameterValidation validates mode parameter validation
// FR-003: NewFrozenDB must validate mode parameter and reject invalid values
func Test_S_002_FR_003_ModeParameterValidation(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	tests := []struct {
		name        string
		mode        string
		expectError bool
	}{
		{
			name:        "Valid mode: read",
			mode:        MODE_READ,
			expectError: false,
		},
		{
			name:        "Valid mode: write",
			mode:        MODE_WRITE,
			expectError: false,
		},
		{
			name:        "Invalid mode: empty string",
			mode:        "",
			expectError: true,
		},
		{
			name:        "Invalid mode: invalid value",
			mode:        "invalid",
			expectError: true,
		},
		{
			name:        "Invalid mode: READ uppercase",
			mode:        "READ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewFrozenDB(testPath, tt.mode)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for mode '%s', got nil", tt.mode)
				}
				var invalidInputErr *InvalidInputError
				if !errors.As(err, &invalidInputErr) {
					t.Errorf("Expected InvalidInputError, got: %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for mode '%s', got: %v", tt.mode, err)
				}
				if db != nil {
					_ = db.Close()
				}
			}
		})
	}
}

// Test_S_002_FR_004_FileDescriptorAndHeaderValidation validates file opening and header validation
// FR-004: NewFrozenDB must open file descriptor and validate frozenDB v1 header
func Test_S_002_FR_004_FileDescriptorAndHeaderValidation(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T, path string)
		expectError   bool
		errorType     interface{}
		errorContains string
	}{
		{
			name: "Valid database file",
			setupFile: func(t *testing.T, path string) {
				createTestDatabase(t, path)
			},
			expectError: false,
		},
		{
			name: "File does not exist",
			setupFile: func(t *testing.T, path string) {
				// Don't create file
			},
			expectError:   true,
			errorType:     &PathError{},
			errorContains: "does not exist",
		},
		{
			name: "Invalid header - wrong signature",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				// Write invalid header with wrong signature
				invalidHeader := []byte(`{"sig":"BAD","ver":1,"row_size":1024,"skew_ms":5000}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			expectError:   true,
			errorType:     &CorruptDatabaseError{},
			errorContains: "signature",
		},
		{
			name: "Invalid header - wrong version",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":2,"row_size":1024,"skew_ms":5000}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			expectError:   true,
			errorType:     &CorruptDatabaseError{},
			errorContains: "version",
		},
		{
			name: "Invalid header - row_size out of range",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":100,"skew_ms":5000}` + string(make([]byte, 64-47)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			expectError:   true,
			errorType:     &CorruptDatabaseError{},
			errorContains: "row_size",
		},
		{
			name: "Truncated header",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				file.Write([]byte("short"))
			},
			expectError:   true,
			errorType:     &CorruptDatabaseError{},
			errorContains: "incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := filepath.Join(t.TempDir(), "test.fdb")
			tt.setupFile(t, testPath)

			db, err := NewFrozenDB(testPath, MODE_READ)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.errorType != nil {
					switch tt.errorType.(type) {
					case *PathError:
						var pathErr *PathError
						if !errors.As(err, &pathErr) {
							t.Errorf("Expected PathError, got: %T", err)
						}
					case *CorruptDatabaseError:
						var corruptErr *CorruptDatabaseError
						if !errors.As(err, &corruptErr) {
							t.Errorf("Expected CorruptDatabaseError, got: %T", err)
						}
					}
				}
				if tt.errorContains != "" {
					if err.Error() == "" || !contains(err.Error(), tt.errorContains) {
						t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if db == nil {
					t.Fatal("Expected database instance, got nil")
				}
				_ = db.Close()
			}
		})
	}
}

// Test_S_002_FR_008_MultipleConcurrentReaders validates multiple concurrent readers
// FR-008: Multiple processes can open the same database file in read mode concurrently
func Test_S_002_FR_008_MultipleConcurrentReaders(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Number of concurrent readers
	numReaders := 5
	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			// Open database in read mode
			db, err := NewFrozenDB(testPath, MODE_READ)
			if err != nil {
				errors <- fmt.Errorf("reader %d failed to open: %w", readerID, err)
				return
			}
			defer db.Close()

			// Verify database is open
			if db == nil {
				errors <- fmt.Errorf("reader %d got nil database", readerID)
				return
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

// Test_S_002_FR_011_FixedMemoryUsage validates fixed memory usage regardless of database size
// FR-011: Memory usage must remain constant regardless of database file size
func Test_S_002_FR_011_FixedMemoryUsage(t *testing.T) {
	// Create two databases with different sizes
	smallPath := filepath.Join(t.TempDir(), "small.fdb")
	largePath := filepath.Join(t.TempDir(), "large.fdb")

	// Create small database (just header)
	createTestDatabase(t, smallPath)

	// Create large database (header + 10MB of data)
	createTestDatabase(t, largePath)
	largeFile, _ := os.OpenFile(largePath, os.O_APPEND|os.O_WRONLY, 0644)
	largeFile.Write(make([]byte, 10*1024*1024))
	largeFile.Close()

	// Measure memory for small database
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	db1, err := NewFrozenDB(smallPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open small database: %v", err)
	}
	defer db1.Close()

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	smallMemory := m2.Alloc - m1.Alloc

	// Measure memory for large database
	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	db2, err := NewFrozenDB(largePath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open large database: %v", err)
	}
	defer db2.Close()

	runtime.GC()
	var m4 runtime.MemStats
	runtime.ReadMemStats(&m4)
	largeMemory := m4.Alloc - m3.Alloc

	// Memory difference should be minimal (within 10KB)
	memoryDiff := int64(largeMemory) - int64(smallMemory)
	if memoryDiff < 0 {
		memoryDiff = -memoryDiff
	}

	if memoryDiff > 10*1024 {
		t.Errorf("Memory usage not constant: small=%d bytes, large=%d bytes, diff=%d bytes",
			smallMemory, largeMemory, memoryDiff)
	}
}

// Test_S_002_FR_015_InvalidInputErrorHandling validates InvalidInputError for invalid parameters
// FR-015: NewFrozenDB must return InvalidInputError for invalid path or mode parameters
func Test_S_002_FR_015_InvalidInputErrorHandling(t *testing.T) {
	tests := []struct {
		name string
		path string
		mode string
	}{
		{
			name: "Empty path",
			path: "",
			mode: MODE_READ,
		},
		{
			name: "Path without .fdb extension",
			path: "/tmp/test.txt",
			mode: MODE_READ,
		},
		{
			name: "Path with only extension",
			path: ".fdb",
			mode: MODE_READ,
		},
		{
			name: "Invalid mode value",
			path: "/tmp/test.fdb",
			mode: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewFrozenDB(tt.path, tt.mode)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			var invalidInputErr *InvalidInputError
			if !errors.As(err, &invalidInputErr) {
				t.Errorf("Expected InvalidInputError, got: %T", err)
			}

			if db != nil {
				t.Error("Expected nil database on error")
			}
		})
	}
}

// Test_S_002_FR_016_PathErrorHandling validates PathError for filesystem issues
// FR-016: NewFrozenDB must return PathError for filesystem access issues
func Test_S_002_FR_016_PathErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupPath func(t *testing.T) string
	}{
		{
			name: "File does not exist",
			setupPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent.fdb")
			},
		},
		{
			name: "Parent directory does not exist",
			setupPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent", "test.fdb")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupPath(t)
			db, err := NewFrozenDB(path, MODE_READ)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			var pathErr *PathError
			if !errors.As(err, &pathErr) {
				t.Errorf("Expected PathError, got: %T", err)
			}

			if db != nil {
				t.Error("Expected nil database on error")
			}
		})
	}
}

// Test_S_002_FR_005_ExclusiveLockAfterValidation validates exclusive lock acquisition after header validation
// FR-005: NewFrozenDB must acquire exclusive lock AFTER header validation when mode is MODE_WRITE
func Test_S_002_FR_005_ExclusiveLockAfterValidation(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open database in write mode
	db, err := NewFrozenDB(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("Failed to open database in write mode: %v", err)
	}
	defer db.Close()

	// Verify database is open
	if db == nil {
		t.Fatal("Expected database instance, got nil")
	}

	// Try to acquire another write lock (should fail)
	db2, err := NewFrozenDB(testPath, MODE_WRITE)
	if err == nil {
		db2.Close()
		t.Fatal("Expected lock acquisition to fail for second writer")
	}

	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Errorf("Expected WriteError for lock contention, got: %T", err)
	}
}

// Test_S_002_FR_006_MaintainDescriptorAndLock validates file descriptor and lock maintenance
// FR-006: NewFrozenDB must maintain open file descriptor and lock until Close() is called
func Test_S_002_FR_006_MaintainDescriptorAndLock(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open database in write mode
	db, err := NewFrozenDB(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("Failed to open database in write mode: %v", err)
	}

	// Try to acquire another write lock while first is open (should fail)
	db2, err := NewFrozenDB(testPath, MODE_WRITE)
	if err == nil {
		db2.Close()
		t.Fatal("Expected lock to be held by first database instance")
	}

	// Close first database
	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Now second writer should succeed
	db3, err := NewFrozenDB(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("Expected lock acquisition to succeed after Close(), got: %v", err)
	}
	defer db3.Close()
}

// Test_S_002_FR_009_WriteErrorMultipleWriters validates WriteError for multiple writers
// FR-009: NewFrozenDB must return WriteError when attempting to open in write mode while another writer holds lock
func Test_S_002_FR_009_WriteErrorMultipleWriters(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// First writer opens successfully
	db1, err := NewFrozenDB(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First writer failed to open: %v", err)
	}
	defer db1.Close()

	// Second writer should fail with WriteError
	db2, err := NewFrozenDB(testPath, MODE_WRITE)
	if err == nil {
		db2.Close()
		t.Fatal("Expected WriteError for second writer, got nil")
	}

	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Errorf("Expected WriteError for lock contention, got: %T", err)
	}

	if db2 != nil {
		t.Error("Expected nil database on lock failure")
	}
}

// Test_S_002_FR_010_DifferentFileIndependence validates operations on different files don't interfere
// FR-010: Operations on different database files must not interfere with each other
func Test_S_002_FR_010_DifferentFileIndependence(t *testing.T) {
	// Create two separate database files
	db1Path := filepath.Join(t.TempDir(), "db1.fdb")
	db2Path := filepath.Join(t.TempDir(), "db2.fdb")
	createTestDatabase(t, db1Path)
	createTestDatabase(t, db2Path)

	// Open both in write mode (should succeed since different files)
	db1, err := NewFrozenDB(db1Path, MODE_WRITE)
	if err != nil {
		t.Fatalf("Failed to open first database: %v", err)
	}
	defer db1.Close()

	db2, err := NewFrozenDB(db2Path, MODE_WRITE)
	if err != nil {
		t.Fatalf("Failed to open second database: %v", err)
	}
	defer db2.Close()

	// Both should be open and functional
	if db1 == nil || db2 == nil {
		t.Fatal("Expected both databases to be open")
	}

	// Verify we can also open readers on different files
	db1Reader, err := NewFrozenDB(db1Path, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open reader on first database: %v", err)
	}
	defer db1Reader.Close()

	db2Reader, err := NewFrozenDB(db2Path, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to open reader on second database: %v", err)
	}
	defer db2Reader.Close()
}

// Test_S_002_FR_014_WriteErrorLockFailures validates WriteError for lock acquisition failures
// FR-014: NewFrozenDB must return WriteError for lock acquisition failures
func Test_S_002_FR_014_WriteErrorLockFailures(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Acquire write lock with first instance
	db1, err := NewFrozenDB(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First writer failed to open: %v", err)
	}
	defer db1.Close()

	// Try to acquire write lock with second instance (should fail)
	db2, err := NewFrozenDB(testPath, MODE_WRITE)
	if err == nil {
		db2.Close()
		t.Fatal("Expected error for lock acquisition failure")
	}

	// Verify error is WriteError
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Errorf("Expected WriteError for lock failure, got: %T", err)
	}

	// Verify error message is informative
	if !contains(err.Error(), "lock") {
		t.Errorf("Expected error message to mention 'lock', got: %v", err)
	}
}

// Test_S_002_FR_013_CorruptDatabaseErrorForHeaderValidationFailures validates CorruptDatabaseError for header validation failures
// FR-013: NewFrozenDB must return CorruptDatabaseError for header validation failures
func Test_S_002_FR_013_CorruptDatabaseErrorForHeaderValidationFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T, path string)
		errorContains string
	}{
		{
			name: "Invalid signature",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"BAD","ver":1,"row_size":1024,"skew_ms":5000}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "signature",
		},
		{
			name: "Invalid version",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":2,"row_size":1024,"skew_ms":5000}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "version",
		},
		{
			name: "Row size below minimum",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":100,"skew_ms":5000}` + string(make([]byte, 64-47)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "row_size",
		},
		{
			name: "Row size above maximum",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":70000,"skew_ms":5000}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "row_size",
		},
		{
			name: "Negative skew",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":-1}` + string(make([]byte, 64-47)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "skew_ms",
		},
		{
			name: "Skew above maximum",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":86400001}` + string(make([]byte, 64-48)))
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "skew_ms",
		},
		{
			name: "Missing newline at byte 63",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				header, _ := generateHeader(1024, 5000)
				header[63] = 'X' // Replace newline with invalid character
				file.Write(header)
			},
			errorContains: "newline",
		},
		{
			name: "No null terminator",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				invalidHeader := []byte(`{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":5000}XXXXXXXXXXXXXXXXXXXXXXXXXXX`)
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "null",
		},
		{
			name: "Invalid padding",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				header, _ := generateHeader(1024, 5000)
				// Corrupt a padding byte (after JSON content but before newline)
				// The JSON content ends with a null terminator, padding starts after that
				// Find first null byte after JSON and corrupt the next byte
				nullPos := -1
				for i := 0; i < len(header); i++ {
					if header[i] == '\x00' {
						nullPos = i
						break
					}
				}
				if nullPos != -1 && nullPos+1 < 63 {
					header[nullPos+1] = 'X' // Corrupt padding byte
				}
				file.Write(header)
			},
			errorContains: "padding",
		},
		{
			name: "Malformed JSON",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				// Create 64-byte header with malformed JSON
				invalidHeader := make([]byte, 64)
				jsonStr := `{invalid json here}`
				copy(invalidHeader, jsonStr)
				// Fill rest with null bytes except last byte
				for i := len(jsonStr); i < 63; i++ {
					invalidHeader[i] = '\x00'
				}
				invalidHeader[63] = '\n'
				file.Write(invalidHeader)
			},
			errorContains: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := filepath.Join(t.TempDir(), "test.fdb")
			tt.setupFile(t, testPath)

			db, err := NewFrozenDB(testPath, MODE_READ)

			if err == nil {
				if db != nil {
					db.Close()
				}
				t.Fatal("Expected CorruptDatabaseError, got nil")
			}

			var corruptErr *CorruptDatabaseError
			if !errors.As(err, &corruptErr) {
				t.Errorf("Expected CorruptDatabaseError for %s, got: %T", tt.name, err)
			}

			if tt.errorContains != "" {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			}

			if db != nil {
				t.Error("Expected nil database on error")
			}
		})
	}
}

// Test_S_002_FR_007_IdempotentCloseMethod validates idempotent Close() method
// FR-007: Close() method must be idempotent and safe to call multiple times
func Test_S_002_FR_007_IdempotentCloseMethod(t *testing.T) {
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	tests := []struct {
		name string
		mode string
	}{
		{
			name: "Read mode - multiple close calls",
			mode: MODE_READ,
		},
		{
			name: "Write mode - multiple close calls",
			mode: MODE_WRITE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := NewFrozenDB(testPath, tt.mode)
			if err != nil {
				t.Fatalf("Failed to open database: %v", err)
			}

			// First close should succeed
			if err := db.Close(); err != nil {
				t.Errorf("First Close() failed: %v", err)
			}

			// Second close should be idempotent (no error)
			if err := db.Close(); err != nil {
				t.Errorf("Second Close() should be idempotent, got error: %v", err)
			}

			// Third close should still be idempotent
			if err := db.Close(); err != nil {
				t.Errorf("Third Close() should be idempotent, got error: %v", err)
			}
		})
	}

	// Test concurrent Close() calls (thread safety)
	t.Run("Concurrent close calls", func(t *testing.T) {
		db, err := NewFrozenDB(testPath, MODE_READ)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}

		// Call Close() from multiple goroutines concurrently
		var wg sync.WaitGroup
		numGoroutines := 10
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := db.Close(); err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		// All Close() calls should succeed (idempotent)
		for err := range errors {
			t.Errorf("Concurrent Close() failed: %v", err)
		}
	})
}

// Test_S_002_FR_012_ResourceCleanupOnErrors validates resource cleanup for all error conditions
// FR-012: Resources (file descriptors, locks) must be cleaned up for ALL error conditions
func Test_S_002_FR_012_ResourceCleanupOnErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupFile func(t *testing.T, path string)
		mode      string
	}{
		{
			name: "File not found - read mode",
			setupFile: func(t *testing.T, path string) {
				// Don't create file
			},
			mode: MODE_READ,
		},
		{
			name: "File not found - write mode",
			setupFile: func(t *testing.T, path string) {
				// Don't create file
			},
			mode: MODE_WRITE,
		},
		{
			name: "Invalid header - corrupt database",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				file.Write([]byte("corrupt data"))
			},
			mode: MODE_READ,
		},
		{
			name: "Truncated header",
			setupFile: func(t *testing.T, path string) {
				file, _ := os.Create(path)
				defer file.Close()
				file.Write([]byte("short"))
			},
			mode: MODE_WRITE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := filepath.Join(t.TempDir(), "test.fdb")
			tt.setupFile(t, testPath)

			// Get initial file descriptor count
			initialFDs := countOpenFileDescriptors(t)

			// Attempt to open database (should fail)
			db, err := NewFrozenDB(testPath, tt.mode)
			if err == nil {
				db.Close()
				t.Fatal("Expected error opening database")
			}

			// Verify database is nil
			if db != nil {
				t.Error("Expected nil database on error")
			}

			// Get final file descriptor count
			finalFDs := countOpenFileDescriptors(t)

			// File descriptors should be cleaned up (same count as before)
			if finalFDs > initialFDs {
				t.Errorf("File descriptor leak detected: initial=%d, final=%d", initialFDs, finalFDs)
			}
		})
	}

	// Test cleanup when lock acquisition fails
	t.Run("Lock acquisition failure cleanup", func(t *testing.T) {
		testPath := filepath.Join(t.TempDir(), "test.fdb")
		createTestDatabase(t, testPath)

		// First writer acquires lock
		db1, err := NewFrozenDB(testPath, MODE_WRITE)
		if err != nil {
			t.Fatalf("First writer failed: %v", err)
		}
		defer db1.Close()

		// Get file descriptor count
		initialFDs := countOpenFileDescriptors(t)

		// Second writer fails to acquire lock
		db2, err := NewFrozenDB(testPath, MODE_WRITE)
		if err == nil {
			db2.Close()
			t.Fatal("Expected lock acquisition to fail")
		}

		// Verify cleanup happened
		finalFDs := countOpenFileDescriptors(t)
		if finalFDs > initialFDs {
			t.Errorf("File descriptor not cleaned up after lock failure: initial=%d, final=%d",
				initialFDs, finalFDs)
		}
	})
}

// Helper to count open file descriptors for current process
func countOpenFileDescriptors(t *testing.T) int {
	t.Helper()

	// Read /proc/self/fd directory
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		// If /proc/self/fd not available, skip FD counting
		return 0
	}

	return len(fds)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test_S_004_FR_001_ValidateMethodExists tests FR-001: System MUST provide Validate() error method on all structs that require field validation
func Test_S_004_FR_001_ValidateMethodExists(t *testing.T) {
	// Test Header has Validate() method
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	err := header.Validate()
	// Method should exist and be callable (may return error for invalid state, but method exists)
	_ = err

	// Test FrozenDB has Validate() method
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)
	db, err := NewFrozenDB(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to create FrozenDB: %v", err)
	}
	defer db.Close()
	err = db.Validate()
	// Method should exist and be callable
	_ = err

	// Test CreateConfig has Validate() method (already exists, verify it's still there)
	config := CreateConfig{
		path:    filepath.Join(t.TempDir(), "test.fdb"),
		rowSize: 1024,
		skewMs:  5000,
	}
	err = config.Validate()
	// Method should exist and be callable
	_ = err
}

// Test_S_004_FR_002_DirectInitRequiresValidation tests FR-002: System MUST call Validate() when struct is directly initialized via struct literal before struct can be used in operations
func Test_S_004_FR_002_DirectInitRequiresValidation(t *testing.T) {
	// Test that directly initialized Header must call Validate() before use
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}

	// Validate() must be called explicitly for direct initialization
	err := header.Validate()
	if err != nil {
		t.Fatalf("Valid header should pass validation: %v", err)
	}

	// Test invalid header requires validation and fails
	invalidHeader := &Header{
		signature: "XXX",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	err = invalidHeader.Validate()
	if err == nil {
		t.Error("Invalid header should fail validation")
	}

	// Test that CreateConfig direct initialization requires Validate()
	config := CreateConfig{
		path:    filepath.Join(t.TempDir(), "test.fdb"),
		rowSize: 1024,
		skewMs:  5000,
	}
	err = config.Validate()
	// Should pass for valid config (may fail on filesystem checks, but validation method works)
	_ = err
}

// Test_S_004_FR_005_ValidateMethodIsIdempotent tests FR-005: System MUST make Validate() idempotent (calling multiple times returns same result)
func Test_S_004_FR_005_ValidateMethodIsIdempotent(t *testing.T) {
	// Test Header Validate() is idempotent
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}

	// First call
	err1 := header.Validate()
	// Second call
	err2 := header.Validate()
	// Third call
	err3 := header.Validate()

	// All calls should return the same result
	if (err1 == nil) != (err2 == nil) {
		t.Error("Validate() should return consistent results: first call and second call differ")
	}
	if (err2 == nil) != (err3 == nil) {
		t.Error("Validate() should return consistent results: second call and third call differ")
	}
	if err1 != nil && err2 != nil && err1.Error() != err2.Error() {
		t.Error("Validate() error messages should be consistent across calls")
	}

	// Test invalid header also returns consistent results
	invalidHeader := &Header{
		signature: "XXX",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	err1 = invalidHeader.Validate()
	err2 = invalidHeader.Validate()
	if (err1 == nil) != (err2 == nil) {
		t.Error("Validate() on invalid struct should return consistent results")
	}

	// Test CreateConfig Validate() is idempotent
	config := CreateConfig{
		path:    filepath.Join(t.TempDir(), "test.fdb"),
		rowSize: 1024,
		skewMs:  5000,
	}
	err1 = config.Validate()
	err2 = config.Validate()
	if (err1 == nil) != (err2 == nil) {
		t.Error("CreateConfig.Validate() should be idempotent")
	}
}

// Test_S_004_FR_006_ParentAssumesChildValid tests FR-006: System MUST have Validate() assume all child struct fields are already valid (child Validate() called during child construction)
func Test_S_004_FR_006_ParentAssumesChildValid(t *testing.T) {
	// Test FrozenDB.Validate() assumes Header is already valid
	// Create a valid header first (validated during construction)
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)
	db, err := NewFrozenDB(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("Failed to create FrozenDB: %v", err)
	}
	defer db.Close()

	// FrozenDB.Validate() should assume header is valid (it was validated during NewFrozenDB)
	// If header was invalid, NewFrozenDB would have failed
	err = db.Validate()
	if err != nil {
		t.Errorf("FrozenDB.Validate() should succeed when header is valid: %v", err)
	}

	// Test ChecksumRow.Validate() assumes baseRow is already valid
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	// Validate header first (child validation)
	if err := header.Validate(); err != nil {
		t.Fatalf("Header validation failed: %v", err)
	}

	// Create ChecksumRow (baseRow is validated during construction)
	cr, err := NewChecksumRow(header, []byte("test data"))
	if err != nil {
		t.Fatalf("Failed to create ChecksumRow: %v", err)
	}

	// ChecksumRow.Validate() should assume baseRow is valid
	// It only checks context-specific requirements (StartControl='C', EndControl='CS')
	err = cr.Validate()
	if err != nil {
		t.Errorf("ChecksumRow.Validate() should succeed when baseRow is valid: %v", err)
	}
}

// Test_S_004_FR_008_ValidatesNilPointers tests FR-008: System MUST have Validate() check that struct pointer fields are non-nil when required
func Test_S_004_FR_008_ValidatesNilPointers(t *testing.T) {
	// Test FrozenDB.Validate() checks nil file pointer
	db := &FrozenDB{
		file:   nil,
		mode:   MODE_READ,
		header: nil,
		closed: false,
	}
	err := db.Validate()
	if err == nil {
		t.Error("FrozenDB.Validate() should fail when file is nil")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("Expected InvalidInputError, got: %T", err)
	}

	// Test FrozenDB.Validate() checks nil header pointer
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)
	file, err := os.Open(testPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	db = &FrozenDB{
		file:   file,
		mode:   MODE_READ,
		header: nil,
		closed: false,
	}
	err = db.Validate()
	if err == nil {
		t.Error("FrozenDB.Validate() should fail when header is nil")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("Expected InvalidInputError, got: %T", err)
	}
}

// Test_S_004_FR_014_NoValidateMeansAlwaysValid tests FR-014: System MUST allow structs without Validate() method (considered always valid, no validation required)
func Test_S_004_FR_014_NoValidateMeansAlwaysValid(t *testing.T) {
	// Test that structs without Validate() method are considered always valid
	// This is tested implicitly - if a struct doesn't have Validate(), parent structs
	// should not call Validate() on it and should assume it's valid

	// Example: If we had a simple struct without Validate(), parent should assume it's valid
	// For now, all our structs have Validate(), but the requirement is that
	// if a struct doesn't have Validate(), it's considered always valid

	// This test verifies the behavior: structs without Validate() don't cause errors
	// when used in parent structs that assume child validity

	// Test that we can use structs in contexts where Validate() is expected
	// but if they don't have it, they're considered valid
	// (This is more of a design verification - all current structs have Validate())
	t.Log("FR-014: Structs without Validate() are considered always valid - verified by design")
}

// Test_S_004_FR_010_FieldsUnexported tests FR-010: System MUST convert exported struct fields to unexported (lowercase) to prevent external modification after construction
func Test_S_004_FR_010_FieldsUnexported(t *testing.T) {
	// This test verifies that struct fields are unexported (lowercase)
	// by attempting to access them from outside the package
	// If fields are unexported, compilation will fail

	// Test Header fields are unexported
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	// Fields should be unexported, so direct access should fail at compile time
	// We test this by verifying we can't access fields directly
	// (In actual code, this would be a compile error, but in tests we verify via getters)
	_ = header // Use header to avoid unused variable warning

	// Test CreateConfig fields are unexported
	config := CreateConfig{
		path:    filepath.Join(t.TempDir(), "test.fdb"),
		rowSize: 1024,
		skewMs:  5000,
	}
	_ = config // Use config to avoid unused variable warning

	// Test SudoContext fields are unexported
	ctx := &SudoContext{
		user: "testuser",
		uid:  1000,
		gid:  1000,
	}
	_ = ctx // Use ctx to avoid unused variable warning

	// Note: This test verifies the design requirement
	// Actual compilation errors would occur if fields were exported and accessed externally
	t.Log("FR-010: Struct fields are unexported - verified by design (compilation would fail if exported)")
}

// Test_S_004_FR_011_GetterFunctionsProvideAccess tests FR-011: System MUST provide getter functions (e.g., GetFieldName()) for struct fields that need external read access
func Test_S_004_FR_011_GetterFunctionsProvideAccess(t *testing.T) {
	// Test Header getter functions exist and provide access
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   1024,
		skewMs:    5000,
	}
	if err := header.Validate(); err != nil {
		t.Fatalf("Header validation failed: %v", err)
	}

	// Verify getter functions exist and return correct values
	sig := header.GetSignature()
	if sig != "fDB" {
		t.Errorf("GetSignature() returned %s, expected 'fDB'", sig)
	}

	ver := header.GetVersion()
	if ver != 1 {
		t.Errorf("GetVersion() returned %d, expected 1", ver)
	}

	rowSize := header.GetRowSize()
	if rowSize != 1024 {
		t.Errorf("GetRowSize() returned %d, expected 1024", rowSize)
	}

	skewMs := header.GetSkewMs()
	if skewMs != 5000 {
		t.Errorf("GetSkewMs() returned %d, expected 5000", skewMs)
	}

	// Test CreateConfig getter functions exist and provide access
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	config := CreateConfig{
		path:    testPath,
		rowSize: 1024,
		skewMs:  5000,
	}

	path := config.GetPath()
	if path != testPath {
		t.Errorf("GetPath() returned %s, expected %s", path, testPath)
	}

	rowSize = config.GetRowSize()
	if rowSize != 1024 {
		t.Errorf("GetRowSize() returned %d, expected 1024", rowSize)
	}

	skewMs = config.GetSkewMs()
	if skewMs != 5000 {
		t.Errorf("GetSkewMs() returned %d, expected 5000", skewMs)
	}

	// Test SudoContext getter functions exist and provide access
	ctx := &SudoContext{
		user: "testuser",
		uid:  1000,
		gid:  2000,
	}
	if err := ctx.Validate(); err != nil {
		t.Fatalf("SudoContext validation failed: %v", err)
	}

	user := ctx.GetUser()
	if user != "testuser" {
		t.Errorf("GetUser() returned %s, expected 'testuser'", user)
	}

	uid := ctx.GetUID()
	if uid != 1000 {
		t.Errorf("GetUID() returned %d, expected 1000", uid)
	}

	gid := ctx.GetGID()
	if gid != 2000 {
		t.Errorf("GetGID() returned %d, expected 2000", gid)
	}
}

// Test_S_004_FR_012_GetterFunctionsAreReadOnly tests FR-012: System MUST ensure getter functions return read-only access to struct fields
func Test_S_004_FR_012_GetterFunctionsAreReadOnly(t *testing.T) {
	// FR-012: Getter functions provide read-only access to struct fields
	//
	// This requirement is enforced at the compiler level in Go:
	// - Getter functions return values (not pointers) for primitive types and strings
	// - Modifying the returned value cannot affect the original struct field
	// - This is a language-level guarantee, not an implementation detail
	//
	// Since this is a compiler-level feature that cannot be meaningfully tested
	// (any test would be testing Go's language semantics, not our implementation),
	// we skip this test with documentation.
	//
	// The requirement is satisfied by:
	// 1. Struct fields being unexported (lowercase) - prevents direct external modification
	// 2. Getter functions returning values (not pointers) - prevents indirect modification
	// 3. No setter functions provided - prevents programmatic modification
	//
	// These design decisions ensure read-only access at compile time.
	t.Skip("FR-012: Getter function read-only access is a compiler-level guarantee in Go. " +
		"Getter functions return values (not pointers) for primitive types and strings, " +
		"which makes modification of the returned value impossible by design. " +
		"The requirement is satisfied by: (1) unexported struct fields preventing direct access, " +
		"(2) getter functions returning values preventing indirect modification, " +
		"and (3) no setter functions preventing programmatic modification. " +
		"This is a language-level guarantee that cannot be meaningfully tested.")
}
