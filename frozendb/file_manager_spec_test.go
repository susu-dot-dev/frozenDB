package frozendb

import (
	"os"
	"sync"
	"testing"
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

	data, err := fm.Read(0, len(testData))
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
				data, err := fm.Read(start, size)
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
			data, err := fm.Read(0, len(stableData))
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

	fm.file.Close()
	fm.file = nil

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
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T: %v", err, err)
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

	fm.file.Close()
	fm.file = nil

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
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError, got %T: %v", err, err)
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

		data, err := fm.Read(0, size)
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
