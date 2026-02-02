package frozendb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func Test_S_014_FR_001_ReadMethodReturnsRawBytes(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	testData := []byte("FR-001 test data: raw bytes read verification")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	data, err := fm.Read(0, int32(len(testData)))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(data) != len(testData) {
		t.Errorf("Read returned wrong number of bytes: got %d, want %d", len(data), len(testData))
	}

	for i := range testData {
		if data[i] != testData[i] {
			t.Errorf("Read returned wrong byte at index %d: got %d, want %d", i, data[i], testData[i])
		}
	}

	if string(data) != string(testData) {
		t.Error("Read returned different data than written")
	}
}

func Test_S_014_FR_002_AllowConcurrentReadOperations(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fileSize := 1024 * 1024 // 1MB of test data
	testData := make([]byte, fileSize)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	const numReaders = 100
	const readsPerReader = 10
	var wg sync.WaitGroup
	errorChan := make(chan error, numReaders*readsPerReader)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < readsPerReader; j++ {
				start := int64((readerID + j) % (fileSize - 1000))
				size := 1000
				data, err := fm.Read(start, int32(size))
				if err != nil {
					errorChan <- err
					return
				}
				if len(data) != size {
					errorChan <- NewCorruptDatabaseError("read returned wrong size", nil)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		t.Errorf("Concurrent read failed: %v", err)
	}
}

func Test_S_014_FR_003_EnforceExclusiveWriteAccess(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan1 := make(chan Data, 1)
	err = fm.SetWriter(dataChan1)
	if err != nil {
		t.Fatalf("First SetWriter failed: %v", err)
	}

	dataChan2 := make(chan Data, 1)
	err = fm.SetWriter(dataChan2)
	if err == nil {
		t.Error("Second SetWriter should have failed but succeeded")
	}

	if _, ok := err.(*InvalidActionError); !ok {
		t.Errorf("Expected InvalidActionError, got %T", err)
	}

	close(dataChan1)
}

func Test_S_014_FR_004_SetWriterReturnsErrorIfWriterActive(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 1)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	secondChan := make(chan Data, 1)
	err = fm.SetWriter(secondChan)
	if err == nil {
		t.Error("SetWriter should return error when writer is already active")
	}

	if err != nil {
		if _, ok := err.(*InvalidActionError); !ok {
			t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
		}
	}

	close(dataChan)
}

func Test_S_014_FR_005_AcceptWritesThroughDataChannel(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	testData := []byte("TEST_WRITE_DATA")
	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    testData,
		Response: responseChan,
	}

	err = <-responseChan
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	close(dataChan)
}

func Test_S_014_FR_006_DataPayloadContainsResponseChannel(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	responseChan := make(chan error, 1)
	data := Data{
		Bytes:    []byte("TEST"),
		Response: responseChan,
	}

	if data.Response == nil {
		t.Error("Data payload response channel should not be nil")
	}

	dataChan <- data
	err = <-responseChan
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	close(dataChan)
}

func Test_S_014_FR_007_MaintainThreadSafeAccess(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fileSize := 64 * 1024 // 64KB of test data
	testData := make([]byte, fileSize)
	for i := range testData {
		testData[i] = byte(i)
	}
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	const numGoroutines = 50
	const readsPerGoroutine = 100
	var wg sync.WaitGroup
	accessCount := make(chan int, numGoroutines*readsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				start := int64(goroutineID * j % (fileSize - 64))
				data, err := fm.Read(start, 64)
				if err != nil {
					t.Errorf("Thread-safe read failed for goroutine %d: %v", goroutineID, err)
					return
				}
				_ = data
				accessCount <- goroutineID
			}
		}(i)
	}

	wg.Wait()
	close(accessCount)

	totalAccesses := len(accessCount)
	expectedAccesses := numGoroutines * readsPerGoroutine
	if totalAccesses != expectedAccesses {
		t.Errorf("Not all thread-safe accesses completed: got %d, want %d", totalAccesses, expectedAccesses)
	}
}

func Test_S_014_FR_008_ReadOperationsAccessStableData(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	stableData := []byte("STABLE_DATA_1234567890")
	if _, err := tmpFile.Write(stableData); err != nil {
		t.Fatalf("Failed to write stable data: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	var wg sync.WaitGroup
	readsMatch := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := fm.Read(0, int32(len(stableData)))
			if err != nil {
				t.Errorf("Read failed during stable data check: %v", err)
				return
			}
			if string(data) == string(stableData) {
				readsMatch <- true
			} else {
				readsMatch <- false
			}
		}()
	}

	wg.Wait()
	close(readsMatch)

	consistentReads := 0
	totalReads := 0
	for match := range readsMatch {
		totalReads++
		if match {
			consistentReads++
		}
	}

	if totalReads == 0 {
		t.Fatal("No reads completed")
	}

	if consistentReads != totalReads {
		t.Errorf("Not all reads accessed stable data: %d/%d reads were consistent", consistentReads, totalReads)
	}
}

func Test_S_014_FR_009_TrackCurrentFileEndPosition(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	initialSize := fm.Size()
	if initialSize != 7 {
		t.Errorf("Initial size = %d, want 7", initialSize)
	}

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	testData := []byte("追加データ")
	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    testData,
		Response: responseChan,
	}

	err = <-responseChan
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	newSize := fm.Size()
	expectedSize := initialSize + int64(len(testData))
	if newSize != expectedSize {
		t.Errorf("Size after write = %d, want %d", newSize, expectedSize)
	}

	close(dataChan)
}

func Test_S_014_FR_010_WriteOperationsAppendInOrder(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	writeOrder := []string{"FIRST", "SECOND", "THIRD"}

	for i, data := range writeOrder {
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte(data),
			Response: responseChan,
		}
		err = <-responseChan
		if err != nil {
			t.Errorf("Write %d failed: %v", i, err)
		}
	}

	close(dataChan)

	readData, err := fm.Read(7, 16)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	combined := string(readData)
	expectedCombined := "FIRSTSECONDTHIRD"
	if combined != expectedCombined {
		t.Errorf("Written data = %q, want %q", combined, expectedCombined)
	}
}

func Test_S_014_FR_011_ReturnErrorsOnCorruptionDetection(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	file := fm.file.Load().(*os.File)
	file.Close()
	// Use Close() to properly set the sentinel value
	fm.Close()

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte("TEST"),
		Response: responseChan,
	}

	err = <-responseChan
	if err == nil {
		t.Error("Write should have failed after file was closed")
	}

	if err != nil {
		if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}
	}

	close(dataChan)
}

func Test_S_014_FR_012_SignalWriteErrorsViaResponseChannels(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 10)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	file := fm.file.Load().(*os.File)
	file.Close()
	// Use Close() to properly set the sentinel value
	fm.Close()

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte("TEST"),
		Response: responseChan,
	}

	err = <-responseChan
	if err == nil {
		t.Error("Write should have failed")
	}

	if err != nil {
		if _, ok := err.(*TombstonedError); !ok {
			t.Errorf("Expected TombstonedError, got %T: %v", err, err)
		}
	}

	close(dataChan)
}

func Test_S_014_FR_013_NoArtificialReadSizeLimits(t *testing.T) {
	t.Parallel()

	sizesToTest := []int{
		1,
		64,
		256,
		1024,
		4096,
		16384,
		65536,
		262144,
		1048576, // 1MB
		4194304, // 4MB
	}

	for _, size := range sizesToTest {
		testData := make([]byte, size)
		for i := range testData {
			testData[i] = byte(i % 256)
		}

		tmpFile2, err := os.CreateTemp("", "frozendb_test_*.fdb")
		if err != nil {
			t.Fatalf("Failed to create temp file for size %d: %v", size, err)
		}

		if _, err := tmpFile2.Write(testData); err != nil {
			tmpFile2.Close()
			os.Remove(tmpFile2.Name())
			t.Fatalf("Failed to write test data for size %d: %v", size, err)
		}
		tmpFile2.Close()

		fm, err := NewFileManager(tmpFile2.Name())
		if err != nil {
			os.Remove(tmpFile2.Name())
			t.Fatalf("Failed to create FileManager for size %d: %v", size, err)
		}

		data, err := fm.Read(0, int32(size))
		fm.Close()
		os.Remove(tmpFile2.Name())

		if err != nil {
			t.Errorf("Read failed for size %d: %v", size, err)
			continue
		}

		if len(data) != size {
			t.Errorf("Read returned wrong size for size %d: got %d, want %d", size, len(data), size)
		}
	}
}

// Test_S_017_FR_001_NewDBFileConstructor tests FR-001: A new constructor function NewDBFile(path string, mode string)
// MUST be added to create DBFile instances with appropriate mode configuration for read-only access with no file locking
func Test_S_017_FR_001_NewDBFileConstructor(t *testing.T) {

	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Test: NewDBFile with read mode should succeed
	t.Run("read_mode_succeeds", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_READ)
		if err != nil {
			t.Fatalf("NewDBFile with read mode failed: %v", err)
		}
		defer dbFile.Close()

		// Verify GetMode() returns "read"
		if dbFile.GetMode() != MODE_READ {
			t.Errorf("GetMode() = %q, want %q", dbFile.GetMode(), MODE_READ)
		}

		// Verify read operations work
		data, err := dbFile.Read(0, 64)
		if err != nil {
			t.Errorf("Read() failed: %v", err)
		}
		if len(data) != 64 {
			t.Errorf("Read() returned %d bytes, want 64", len(data))
		}
	})

	// Test: NewDBFile with write mode should succeed
	t.Run("write_mode_succeeds", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_WRITE)
		if err != nil {
			t.Fatalf("NewDBFile with write mode failed: %v", err)
		}
		defer dbFile.Close()

		// Verify GetMode() returns "write"
		if dbFile.GetMode() != MODE_WRITE {
			t.Errorf("GetMode() = %q, want %q", dbFile.GetMode(), MODE_WRITE)
		}

		// Verify read operations work
		data, err := dbFile.Read(0, 64)
		if err != nil {
			t.Errorf("Read() failed: %v", err)
		}
		if len(data) != 64 {
			t.Errorf("Read() returned %d bytes, want 64", len(data))
		}
	})

	// Test: Invalid mode should return error
	t.Run("invalid_mode_fails", func(t *testing.T) {
		_, err := NewDBFile(testPath, "invalid")
		if err == nil {
			t.Error("NewDBFile with invalid mode should have failed")
		}
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError, got %T", err)
		}
	})
}

// Test_S_017_FR_004_ConcurrentReadersAllowed tests FR-004: Multiple concurrent readers MUST be allowed
// to access the same file simultaneously
func Test_S_017_FR_004_ConcurrentReadersAllowed(t *testing.T) {

	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	const numReaders = 50
	const readsPerReader = 10
	var wg sync.WaitGroup
	errorChan := make(chan error, numReaders*readsPerReader)

	// Open multiple readers concurrently
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			dbFile, err := NewDBFile(testPath, MODE_READ)
			if err != nil {
				errorChan <- fmt.Errorf("reader %d: NewDBFile failed: %w", readerID, err)
				return
			}
			defer dbFile.Close()

			// Verify mode
			if dbFile.GetMode() != MODE_READ {
				errorChan <- fmt.Errorf("reader %d: GetMode() = %q, want %q", readerID, dbFile.GetMode(), MODE_READ)
				return
			}

			// Perform multiple reads
			for j := 0; j < readsPerReader; j++ {
				_, err := dbFile.Read(0, 64)
				if err != nil {
					errorChan <- fmt.Errorf("reader %d, read %d: Read() failed: %w", readerID, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent read failed: %v", err)
	}
}

// Test_S_017_FR_009_NonExistentFileReadMode tests FR-009: Read mode attempts on non-existent files
// MUST follow existing error handling patterns in open.go (return PathError for non-existent files)
func Test_S_017_FR_009_NonExistentFileReadMode(t *testing.T) {

	nonExistentPath := filepath.Join(t.TempDir(), "nonexistent.fdb")

	_, err := NewDBFile(nonExistentPath, MODE_READ)
	if err == nil {
		t.Fatal("NewDBFile with non-existent file should have failed")
	}

	// Verify it returns PathError
	if _, ok := err.(*PathError); !ok {
		t.Errorf("Expected PathError for non-existent file, got %T: %v", err, err)
	}

	// Verify error message indicates file doesn't exist
	if !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("Error message should indicate file doesn't exist, got: %v", err)
	}
}

// Test_S_017_FR_002_NewDBFileWriteMode tests FR-002: The NewDBFile(path string, mode string) constructor
// MUST support read-write mode with exclusive file locking when mode parameter indicates write access
func Test_S_017_FR_002_NewDBFileWriteMode(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Test: NewDBFile with write mode should succeed and acquire lock
	dbFile, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("NewDBFile with write mode failed: %v", err)
	}
	defer dbFile.Close()

	// Verify GetMode() returns "write"
	if dbFile.GetMode() != MODE_WRITE {
		t.Errorf("GetMode() = %q, want %q", dbFile.GetMode(), MODE_WRITE)
	}

	// Verify read operations work
	data, err := dbFile.Read(0, 64)
	if err != nil {
		t.Errorf("Read() failed: %v", err)
	}
	if len(data) != 64 {
		t.Errorf("Read() returned %d bytes, want 64", len(data))
	}

	// Verify write operations work (SetWriter should succeed)
	dataChan := make(chan Data, 1)
	err = dbFile.SetWriter(dataChan)
	if err != nil {
		t.Errorf("SetWriter() failed: %v", err)
	}
	close(dataChan)
}

// Test_S_017_FR_003_OSLevelFileLocking tests FR-003: File locking MUST use OS-level flocks
// to ensure cross-process coordination
func Test_S_017_FR_003_OSLevelFileLocking(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open first writer - should succeed
	dbFile1, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First NewDBFile with write mode failed: %v", err)
	}

	// Attempt to open second writer in same process - should fail with lock error
	_, err = NewDBFile(testPath, MODE_WRITE)
	if err == nil {
		t.Error("Second NewDBFile with write mode should have failed due to lock")
		dbFile1.Close()
		return
	}

	// Verify it returns WriteError
	if _, ok := err.(*WriteError); !ok {
		t.Errorf("Expected WriteError for locked file, got %T: %v", err, err)
	}

	dbFile1.Close()
}

// Test_S_017_FR_005_SingleWriterOnly tests FR-005: Only one writer MUST be allowed
// to access a file at any given time
func Test_S_017_FR_005_SingleWriterOnly(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open first writer
	dbFile1, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First NewDBFile with write mode failed: %v", err)
	}
	defer dbFile1.Close()

	// Verify first writer can set writer channel
	dataChan1 := make(chan Data, 1)
	err = dbFile1.SetWriter(dataChan1)
	if err != nil {
		t.Fatalf("First SetWriter failed: %v", err)
	}

	// Attempt to open second writer - should fail
	_, err = NewDBFile(testPath, MODE_WRITE)
	if err == nil {
		t.Error("Second NewDBFile with write mode should have failed")
		return
	}

	// Verify it returns WriteError
	if _, ok := err.(*WriteError); !ok {
		t.Errorf("Expected WriteError, got %T: %v", err, err)
	}

	close(dataChan1)
}

// Test_S_017_FR_008_LockReleaseOnClose tests FR-008: File locks MUST be properly released
// when DBFile is closed
func Test_S_017_FR_008_LockReleaseOnClose(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open first writer
	dbFile1, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First NewDBFile with write mode failed: %v", err)
	}

	// Verify second writer fails while first is open
	_, err = NewDBFile(testPath, MODE_WRITE)
	if err == nil {
		t.Error("Second NewDBFile should have failed while first is open")
		dbFile1.Close()
		return
	}

	// Close first writer
	err = dbFile1.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Now second writer should succeed (lock released)
	dbFile2, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Errorf("Second NewDBFile should succeed after first closed, got: %v", err)
		return
	}
	defer dbFile2.Close()
}

// Test_S_017_FR_010_NonBlockingLockAcquisition tests FR-010: Write mode MUST use only
// non-blocking lock acquisition to fail fast if another process has the file locked
func Test_S_017_FR_010_NonBlockingLockAcquisition(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open first writer
	dbFile1, err := NewDBFile(testPath, MODE_WRITE)
	if err != nil {
		t.Fatalf("First NewDBFile with write mode failed: %v", err)
	}
	defer dbFile1.Close()

	// Attempt to open second writer - should fail immediately (non-blocking)
	start := time.Now()
	_, err = NewDBFile(testPath, MODE_WRITE)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Second NewDBFile should have failed")
		return
	}

	// Verify it returns WriteError
	if _, ok := err.(*WriteError); !ok {
		t.Errorf("Expected WriteError, got %T: %v", err, err)
	}

	// Verify it failed fast (non-blocking) - should be < 50ms per SC-004
	if elapsed > 50*time.Millisecond {
		t.Errorf("Lock acquisition should be non-blocking and fast (<50ms), took %v", elapsed)
	}
}

// Test_S_017_FR_006_RefactorOpenFunctions tests FR-006: open.go functions MUST be refactored
// to use DBFile interface instead of direct os.File operations
func Test_S_017_FR_006_RefactorOpenFunctions(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Test: NewDBFile should return DBFile interface
	t.Run("NewDBFile_returns_DBFile", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_READ)
		if err != nil {
			t.Fatalf("NewDBFile failed: %v", err)
		}
		defer dbFile.Close()

		// Verify it implements DBFile interface
		if dbFile == nil {
			t.Fatal("NewDBFile returned nil")
		}

		// Verify GetMode() works (DBFile interface method)
		mode := dbFile.GetMode()
		if mode != MODE_READ {
			t.Errorf("GetMode() = %q, want %q", mode, MODE_READ)
		}

		// Verify Read() works (DBFile interface method)
		data, err := dbFile.Read(0, 64)
		if err != nil {
			t.Errorf("Read() failed: %v", err)
		}
		if len(data) != 64 {
			t.Errorf("Read() returned %d bytes, want 64", len(data))
		}
	})

	// Test: validateDatabaseFile should accept DBFile interface
	t.Run("validateDatabaseFile_accepts_DBFile", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_READ)
		if err != nil {
			t.Fatalf("NewDBFile failed: %v", err)
		}
		defer dbFile.Close()

		// validateDatabaseFile should work with DBFile
		header, err := validateDatabaseFile(dbFile)
		if err != nil {
			t.Fatalf("validateDatabaseFile failed: %v", err)
		}

		if header == nil {
			t.Fatal("validateDatabaseFile returned nil header")
		}

		// Verify header is valid
		if err := header.Validate(); err != nil {
			t.Errorf("Header validation failed: %v", err)
		}
	})

	// Test: NewFrozenDB should use DBFile internally (indirect test via behavior)
	t.Run("NewFrozenDB_uses_DBFile", func(t *testing.T) {
		// Open in read mode
		db, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
		if err != nil {
			t.Fatalf("NewFrozenDB failed: %v", err)
		}
		defer db.Close()

		// Verify it works correctly (if it uses DBFile, it should work)
		if db.file == nil {
			t.Fatal("FrozenDB.file is nil")
		}

		// Verify file implements DBFile interface by calling GetMode()
		mode := db.file.GetMode()
		if mode != MODE_READ {
			t.Errorf("file.GetMode() = %q, want %q", mode, MODE_READ)
		}
	})
}

// Test_S_024_FR_001_WriterStateCleared tests FR-001: System MUST ensure all pending writes are processed
// by DBFile and writer state is cleared before Commit() or Rollback() returns
func Test_S_024_FR_001_WriterStateCleared(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("INITIAL")
	tmpFile.Close()

	fm, err := NewDBFile(tmpFile.Name(), MODE_WRITE)
	if err != nil {
		t.Fatalf("Failed to create DBFile: %v", err)
	}
	defer fm.Close()

	// Test: Verify WriterClosed() method exists and works correctly
	t.Run("WriterClosed_method_exists", func(t *testing.T) {
		// WriterClosed() should exist and return immediately when no writer is set
		// (no error, just returns)
		fm.WriterClosed()
	})

	t.Run("WriterClosed_blocks_until_writer_complete", func(t *testing.T) {
		// Set up a writer with data to process
		dataChan := make(chan Data, 10)
		err := fm.SetWriter(dataChan)
		if err != nil {
			t.Fatalf("SetWriter failed: %v", err)
		}

		// Send some data to ensure writer is active
		testData := []byte("TEST_DATA_FOR_WRITER")
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    testData,
			Response: responseChan,
		}

		// Wait for the write to complete
		err = <-responseChan
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Close the channel to signal writer to finish
		close(dataChan)

		// Now WriterClosed() should block until writer is done
		start := time.Now()
		fm.WriterClosed()
		elapsed := time.Since(start)

		// Verify it took some time (writer had to finish processing)
		// This is a rough check - the exact timing depends on the system
		if elapsed == 0 {
			t.Logf("WriterClosed() completed very quickly (%v), this might be normal on fast systems", elapsed)
		}

		// After WriterClosed() completes, writer state should be clear
		// Try to set a new writer - should succeed
		newDataChan := make(chan Data, 10)
		err = fm.SetWriter(newDataChan)
		if err != nil {
			t.Errorf("SetWriter should succeed after WriterClosed(), got: %v", err)
		}

		close(newDataChan)
	})

	t.Run("WriterClosed_returns_immediately_in_read_mode", func(t *testing.T) {
		// Create a DBFile in read mode
		readFm, err := NewDBFile(tmpFile.Name(), MODE_READ)
		if err != nil {
			t.Fatalf("Failed to create read mode DBFile: %v", err)
		}
		defer readFm.Close()

		// WriterClosed() should return immediately in read mode (no error)
		start := time.Now()
		readFm.WriterClosed()
		elapsed := time.Since(start)
		if elapsed > 10*time.Millisecond {
			t.Errorf("WriterClosed() should return immediately in read mode, took %v", elapsed)
		}
	})
}

// Test_S_039_FR_001_FileWatcherCreatedOnlyInReadMode tests FR-001: File watcher MUST be created only when mode==MODE_READ
// and watcher MUST listen only for fsnotify.Write events
func Test_S_039_FR_001_FileWatcherCreatedOnlyInReadMode(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Test: Read mode should create watcher
	t.Run("read_mode_creates_watcher", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_READ)
		if err != nil {
			t.Fatalf("NewDBFile with read mode failed: %v", err)
		}
		defer dbFile.Close()

		// Verify mode is read
		if dbFile.GetMode() != MODE_READ {
			t.Errorf("GetMode() = %q, want %q", dbFile.GetMode(), MODE_READ)
		}

		// Verify watcher field exists and is non-nil
		fm, ok := dbFile.(*FileManager)
		if !ok {
			t.Fatalf("dbFile is not *FileManager, got %T", dbFile)
		}

		if fm.watcher == nil {
			t.Error("watcher field should be non-nil in read mode")
		}
	})

	// Test: Write mode should NOT create watcher
	t.Run("write_mode_does_not_create_watcher", func(t *testing.T) {
		dbFile, err := NewDBFile(testPath, MODE_WRITE)
		if err != nil {
			t.Fatalf("NewDBFile with write mode failed: %v", err)
		}
		defer dbFile.Close()

		// Verify mode is write
		if dbFile.GetMode() != MODE_WRITE {
			t.Errorf("GetMode() = %q, want %q", dbFile.GetMode(), MODE_WRITE)
		}

		// Verify watcher field is nil in write mode
		fm, ok := dbFile.(*FileManager)
		if !ok {
			t.Fatalf("dbFile is not *FileManager, got %T", dbFile)
		}

		if fm.watcher != nil {
			t.Error("watcher field should be nil in write mode")
		}
	})
}

// Test_S_039_FR_001_WatcherListensOnlyForWriteEvents tests FR-001: Watcher MUST listen only for fsnotify.Write events
// (not Chmod, Rename, etc.)
func Test_S_039_FR_001_WatcherListensOnlyForWriteEvents(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile with read mode failed: %v", err)
	}
	defer dbFile.Close()

	// Get FileManager to access internal fields
	fm, ok := dbFile.(*FileManager)
	if !ok {
		t.Fatalf("dbFile is not *FileManager, got %T", dbFile)
	}

	if fm.watcher == nil {
		t.Fatal("watcher should be non-nil in read mode")
	}

	// Track callback invocations
	var callbackCount atomic.Int32
	_, err = dbFile.Subscribe(func() error {
		callbackCount.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Record initial size and callback count
	initialSize := dbFile.Size()
	initialCount := callbackCount.Load()

	// Perform Write operation - should trigger callback
	file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("Failed to open file for write: %v", err)
	}
	testData := []byte("TEST_DATA_WRITE")
	if _, err := file.Write(testData); err != nil {
		file.Close()
		t.Fatalf("Failed to write test data: %v", err)
	}
	file.Close()

	// Wait for watcher to process Write event
	time.Sleep(100 * time.Millisecond)

	// Verify callback was invoked after Write
	if callbackCount.Load() <= initialCount {
		t.Error("Callback should have been invoked after Write event")
	}

	// Verify size was updated
	if dbFile.Size() <= initialSize {
		t.Error("File size should have increased after Write event")
	}

	// Perform Chmod operation - should NOT trigger callback
	beforeChmodCount := callbackCount.Load()
	if err := os.Chmod(testPath, 0644); err != nil {
		t.Fatalf("Failed to chmod file: %v", err)
	}

	// Wait to ensure no spurious callbacks
	time.Sleep(100 * time.Millisecond)

	// Verify callback was NOT invoked after Chmod
	if callbackCount.Load() != beforeChmodCount {
		t.Error("Callback should NOT have been invoked after Chmod event")
	}
}

// Test_S_039_FR_002_InitialFileSizeCapturedBeforeWatcher tests FR-002: Initial file size MUST be captured
// during DBFile creation before watcher starts
func Test_S_039_FR_002_InitialFileSizeCapturedBeforeWatcher(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Get initial file info
	fileInfo, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	expectedSize := fileInfo.Size()

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Verify Size() returns initial file size
	actualSize := dbFile.Size()
	if actualSize != expectedSize {
		t.Errorf("Size() = %d, want %d (initial file size)", actualSize, expectedSize)
	}

	// Verify size is consistent even before any writes
	for i := 0; i < 10; i++ {
		if dbFile.Size() != expectedSize {
			t.Errorf("Size() changed before any writes: got %d, want %d", dbFile.Size(), expectedSize)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Test_S_039_FR_003_ZeroTimingGapsDuringInitialization tests FR-003: Zero timing gaps where writes could be missed
// during initialization. This test creates a database during active writes and verifies all written keys are retrievable.
func Test_S_039_FR_003_ZeroTimingGapsDuringInitialization(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Start background writer that continuously writes
	stopWriter := make(chan struct{})
	writerDone := make(chan struct{})
	var writtenSizes []int64

	go func() {
		defer close(writerDone)
		file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Errorf("Writer: Failed to open file: %v", err)
			return
		}
		defer file.Close()

		for i := 0; ; i++ {
			select {
			case <-stopWriter:
				return
			default:
				data := fmt.Sprintf("WRITE_%04d_", i)
				if _, err := file.Write([]byte(data)); err != nil {
					t.Errorf("Writer: Failed to write: %v", err)
					return
				}
				// Force sync to ensure write is visible
				file.Sync()

				// Record size after write
				info, err := os.Stat(testPath)
				if err == nil {
					writtenSizes = append(writtenSizes, info.Size())
				}

				// Small delay to allow overlap with reader initialization
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Give writer time to start
	time.Sleep(20 * time.Millisecond)

	// Open read-mode database DURING active writes (testing initialization window)
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		close(stopWriter)
		<-writerDone
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Record size immediately after open
	initialSeenSize := dbFile.Size()

	// Continue writing for a bit more
	time.Sleep(50 * time.Millisecond)

	// Stop writer
	close(stopWriter)
	<-writerDone

	// Get final file size from OS
	finalInfo, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	finalActualSize := finalInfo.Size()

	// Wait for watcher to catch up
	time.Sleep(200 * time.Millisecond)

	// Verify DBFile.Size() matches final actual size
	finalSeenSize := dbFile.Size()
	if finalSeenSize != finalActualSize {
		t.Errorf("Final Size() = %d, want %d (actual file size)", finalSeenSize, finalActualSize)
	}

	// Critical check: Verify we didn't miss any size increases
	// The watcher should have seen ALL writes that happened after initialization
	if finalSeenSize < initialSeenSize {
		t.Errorf("Size decreased: initial=%d, final=%d", initialSeenSize, finalSeenSize)
	}

	// Verify we captured writes that happened during initialization
	if initialSeenSize == finalActualSize {
		// This is fine - we captured everything including gap writes
		t.Logf("Initial size already matched final: %d", initialSeenSize)
	} else {
		// Watcher caught up - verify no size regressions
		for i, size := range writtenSizes {
			if dbFile.Size() < size {
				t.Errorf("Size regression detected: written size[%d]=%d, but Size()=%d", i, size, dbFile.Size())
				break
			}
		}
	}
}

// Test_S_039_FR_004_OnlyOneUpdateCycleActiveAtATime tests FR-004: Only one update cycle MUST be active at a time.
// This test triggers rapid file modifications and verifies that only one update cycle executes at a time
// using instrumented callbacks to detect concurrent execution.
func Test_S_039_FR_004_OnlyOneUpdateCycleActiveAtATime(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Track concurrent callback executions
	var activeCallbacks atomic.Int32
	var maxConcurrent atomic.Int32
	var callbackCount atomic.Int32

	// Register callback that detects concurrent execution
	_, err = dbFile.Subscribe(func() error {
		// Increment active count
		active := activeCallbacks.Add(1)
		callbackCount.Add(1)

		// Track maximum concurrent executions
		for {
			current := maxConcurrent.Load()
			if active <= current || maxConcurrent.CompareAndSwap(current, active) {
				break
			}
		}

		// Simulate some work to increase chance of detecting concurrency
		time.Sleep(10 * time.Millisecond)

		// Decrement active count
		activeCallbacks.Add(-1)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Trigger rapid file modifications
	for i := 0; i < 20; i++ {
		file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("Failed to open file for write: %v", err)
		}
		data := fmt.Sprintf("RAPID_WRITE_%04d", i)
		if _, err := file.Write([]byte(data)); err != nil {
			file.Close()
			t.Fatalf("Failed to write: %v", err)
		}
		file.Close()

		// Small delay between writes to allow watcher to process
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for all callbacks to complete
	time.Sleep(500 * time.Millisecond)

	// Verify: Maximum concurrent should be 1 (serialization)
	if maxConcurrent.Load() > 1 {
		t.Errorf("Concurrent update cycles detected: max concurrent = %d, want 1", maxConcurrent.Load())
	}

	// Verify: At least some callbacks were invoked
	if callbackCount.Load() == 0 {
		t.Error("No callbacks invoked, file watcher may not be working")
	}
}

// Test_S_039_FR_005_SizeUpdateBeforeCallbacks tests FR-005: File size update MUST complete before
// subscriber callbacks are invoked. This verifies that callbacks see the updated size.
func Test_S_039_FR_005_SizeUpdateBeforeCallbacks(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Record initial size
	initialSize := dbFile.Size()

	// Track sizes seen in callback
	var sizesSeenInCallback []int64
	var mu sync.Mutex

	// Register callback that records current size
	_, err = dbFile.Subscribe(func() error {
		mu.Lock()
		sizesSeenInCallback = append(sizesSeenInCallback, dbFile.Size())
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Perform writes to trigger callbacks
	testData := []byte("SIZE_UPDATE_TEST_DATA")
	for i := 0; i < 5; i++ {
		file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("Failed to open file for write: %v", err)
		}
		if _, err := file.Write(testData); err != nil {
			file.Close()
			t.Fatalf("Failed to write: %v", err)
		}
		file.Close()

		// Wait for watcher to process
		time.Sleep(100 * time.Millisecond)
	}

	// Verify: All sizes seen in callback should be greater than initial size
	mu.Lock()
	defer mu.Unlock()

	if len(sizesSeenInCallback) == 0 {
		t.Fatal("No callbacks invoked")
	}

	for i, size := range sizesSeenInCallback {
		if size <= initialSize {
			t.Errorf("Callback[%d] saw size %d, which is not greater than initial size %d", i, size, initialSize)
		}

		// Each callback should see a size >= previous callback
		if i > 0 && size < sizesSeenInCallback[i-1] {
			t.Errorf("Callback[%d] saw size decrease: %d -> %d", i, sizesSeenInCallback[i-1], size)
		}
	}
}

// Test_S_039_FR_005_NoCallbacksWhenSizeUnchanged tests FR-005: Callbacks MUST NOT be invoked
// when file size is unchanged (metadata-only updates).
func Test_S_039_FR_005_NoCallbacksWhenSizeUnchanged(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Record initial size
	initialSize := dbFile.Size()

	// Track callback invocations
	var callbackCount atomic.Int32

	// Register callback
	_, err = dbFile.Subscribe(func() error {
		callbackCount.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for any pending callbacks to complete
	time.Sleep(100 * time.Millisecond)
	initialCallbackCount := callbackCount.Load()

	// Trigger metadata-only update (chmod)
	if err := os.Chmod(testPath, 0644); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Wait to see if spurious callbacks occur
	time.Sleep(200 * time.Millisecond)

	// Verify: Size unchanged
	if dbFile.Size() != initialSize {
		t.Errorf("Size changed unexpectedly: %d -> %d", initialSize, dbFile.Size())
	}

	// Verify: No additional callbacks invoked
	finalCallbackCount := callbackCount.Load()
	if finalCallbackCount != initialCallbackCount {
		t.Errorf("Callbacks invoked despite no size change: count changed from %d to %d", initialCallbackCount, finalCallbackCount)
	}

	// Now perform actual write to verify watcher is still working
	file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("Failed to open file for write: %v", err)
	}
	if _, err := file.Write([]byte("VERIFY_WATCHER_WORKS")); err != nil {
		file.Close()
		t.Fatalf("Failed to write: %v", err)
	}
	file.Close()

	// Wait for callback
	time.Sleep(200 * time.Millisecond)

	// Verify: Callback was invoked after real write
	if callbackCount.Load() <= finalCallbackCount {
		t.Error("Callback should have been invoked after write with size change")
	}
}

// Test_S_039_FR_006_CallbacksInRegistrationOrder tests FR-006: Callbacks MUST be invoked
// in registration order.
func Test_S_039_FR_006_CallbacksInRegistrationOrder(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Wait for watcher to settle
	time.Sleep(100 * time.Millisecond)

	// Track callback invocation order
	var invocationOrder []int
	var mu sync.Mutex
	var recordingStarted atomic.Bool
	var recordedCount atomic.Int32

	// Register multiple callbacks in order
	for i := 1; i <= 5; i++ {
		callbackID := i
		_, err := dbFile.Subscribe(func() error {
			// Only record the FIRST 5 callbacks after we start recording
			if !recordingStarted.Load() {
				return nil
			}
			if recordedCount.Load() >= 5 {
				return nil
			}
			mu.Lock()
			if len(invocationOrder) < 5 {
				invocationOrder = append(invocationOrder, callbackID)
				recordedCount.Add(1)
			}
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe %d failed: %v", i, err)
		}
	}

	// Additional sleep to drain any pending file system events
	time.Sleep(200 * time.Millisecond)

	// Start recording
	recordingStarted.Store(true)

	// Clear any spurious invocations
	mu.Lock()
	invocationOrder = invocationOrder[:0]
	mu.Unlock()

	// Trigger update cycle
	file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("Failed to open file for write: %v", err)
	}
	if _, err := file.Write([]byte("ORDER_TEST_DATA")); err != nil {
		file.Close()
		t.Fatalf("Failed to write: %v", err)
	}
	file.Close()

	// Wait for callbacks to complete
	time.Sleep(200 * time.Millisecond)

	// Verify: Callbacks invoked in registration order
	mu.Lock()
	defer mu.Unlock()

	expectedOrder := []int{1, 2, 3, 4, 5}
	if len(invocationOrder) != len(expectedOrder) {
		t.Fatalf("Expected %d callbacks, got %d: %v", len(expectedOrder), len(invocationOrder), invocationOrder)
	}

	for i, callbackID := range invocationOrder {
		if callbackID != expectedOrder[i] {
			t.Errorf("Callback order mismatch at position %d: got %d, want %d", i, callbackID, expectedOrder[i])
		}
	}
}

// Test_S_039_FR_006_FirstCallbackErrorStopsProcessing tests FR-006: When a callback returns error,
// FileManager MUST stop invoking remaining callbacks.
func Test_S_039_FR_006_FirstCallbackErrorStopsProcessing(t *testing.T) {
	// Create a test database file
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in read mode
	dbFile, err := NewDBFile(testPath, MODE_READ)
	if err != nil {
		t.Fatalf("NewDBFile failed: %v", err)
	}
	defer dbFile.Close()

	// Wait for watcher to settle and any pending callbacks to complete
	time.Sleep(200 * time.Millisecond)

	// Track which callbacks were invoked
	var invoked []int
	var mu sync.Mutex
	var recordingStarted atomic.Bool

	// Register callback 1 - succeeds
	_, err = dbFile.Subscribe(func() error {
		if !recordingStarted.Load() {
			return nil
		}
		mu.Lock()
		invoked = append(invoked, 1)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	// Register callback 2 - returns error
	_, err = dbFile.Subscribe(func() error {
		if !recordingStarted.Load() {
			return nil
		}
		mu.Lock()
		invoked = append(invoked, 2)
		mu.Unlock()
		// Return error only after recording
		return NewInvalidInputError("callback 2 error", nil)
	})
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	// Register callback 3 - should NOT be invoked
	_, err = dbFile.Subscribe(func() error {
		if !recordingStarted.Load() {
			return nil
		}
		mu.Lock()
		invoked = append(invoked, 3)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe 3 failed: %v", err)
	}

	// Additional sleep to drain any pending file system events from the initial setup
	// This prevents race conditions where callbacks are invoked before recording starts
	time.Sleep(200 * time.Millisecond)

	// Start recording
	recordingStarted.Store(true)

	// Clear any spurious invocations that might have occurred during setup
	mu.Lock()
	invoked = invoked[:0]
	mu.Unlock()

	// Trigger update cycle
	file, err := os.OpenFile(testPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("Failed to open file for write: %v", err)
	}
	if _, err := file.Write([]byte("ERROR_TEST_DATA")); err != nil {
		file.Close()
		t.Fatalf("Failed to write: %v", err)
	}
	file.Close()

	// Wait for callbacks to complete
	time.Sleep(200 * time.Millisecond)

	// Verify: Only callbacks 1 and 2 were invoked (callback 3 was NOT invoked)
	mu.Lock()
	defer mu.Unlock()

	expectedInvoked := []int{1, 2}
	if len(invoked) != len(expectedInvoked) {
		t.Fatalf("Expected %d callbacks invoked, got %d: %v", len(expectedInvoked), len(invoked), invoked)
	}

	for i, callbackID := range invoked {
		if callbackID != expectedInvoked[i] {
			t.Errorf("Invoked callback mismatch at position %d: got %d, want %d", i, callbackID, expectedInvoked[i])
		}
	}

	// Verify callback 3 was NOT invoked
	for _, callbackID := range invoked {
		if callbackID == 3 {
			t.Error("Callback 3 should NOT have been invoked after callback 2 returned error")
		}
	}
}

// NOTE: FR-009 is tested by Test_S_039_FR_006_FirstCallbackErrorStopsProcessing above.
// FR-006 and FR-009 have identical requirements: when a callback returns error,
// FileManager MUST stop invoking remaining callbacks. The FR-006 test validates this
// behavior comprehensively, so a separate FR-009 test would be redundant.
