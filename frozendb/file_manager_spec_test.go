package frozendb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
