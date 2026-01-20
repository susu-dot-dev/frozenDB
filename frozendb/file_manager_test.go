package frozendb

import (
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileManager_ReadBoundaryConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupFile   func(t *testing.T) *os.File
		start       int64
		size        int
		wantErrType *InvalidInputError
	}{
		{
			name: "read at file start",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				data := []byte("START")
				tmpFile.Write(data)
				tmpFile.Close()
				return tmpFile
			},
			start:       0,
			size:        5,
			wantErrType: nil,
		},
		{
			name: "read at middle of file",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				data := []byte("0123456789ABCDEF")
				tmpFile.Write(data)
				tmpFile.Close()
				return tmpFile
			},
			start:       5,
			size:        5,
			wantErrType: nil,
		},
		{
			name: "read at file end boundary",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				data := []byte("END")
				tmpFile.Write(data)
				tmpFile.Close()
				return tmpFile
			},
			start:       1,
			size:        2,
			wantErrType: nil,
		},
		{
			name: "read with negative start returns error",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				tmpFile.Close()
				return tmpFile
			},
			start:       -1,
			size:        10,
			wantErrType: &InvalidInputError{},
		},
		{
			name: "read with zero size returns error",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				tmpFile.Close()
				return tmpFile
			},
			start:       0,
			size:        0,
			wantErrType: &InvalidInputError{},
		},
		{
			name: "read with negative size returns error",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				tmpFile.Close()
				return tmpFile
			},
			start:       0,
			size:        -5,
			wantErrType: &InvalidInputError{},
		},
		{
			name: "read beyond file size returns error",
			setupFile: func(t *testing.T) *os.File {
				tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				data := []byte("SMALL")
				tmpFile.Write(data)
				tmpFile.Close()
				return tmpFile
			},
			start:       0,
			size:        100,
			wantErrType: &InvalidInputError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := tt.setupFile(t)
			defer os.Remove(tmpFile.Name())

			fm, err := NewFileManager(tmpFile.Name())
			if err != nil && tt.wantErrType == nil {
				t.Fatalf("Failed to create FileManager: %v", err)
			}
			if fm != nil {
				defer fm.Close()
			}

			_, err = fm.Read(tt.start, tt.size)

			if tt.wantErrType != nil {
				if err == nil {
					t.Errorf("Read() expected error, got nil")
				} else if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Read() expected InvalidInputError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Read() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestFileManager_ReadFromClosedFileManager(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("TEST DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

	fm.Close()

	_, err = fm.Read(0, 4)
	if err == nil {
		t.Error("Read from closed FileManager should return error")
	}
}

func TestFileManager_Size(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("X")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	size := fm.Size()
	if size != 1 {
		t.Errorf("Size() = %d, want 1", size)
	}
}

func TestFileManager_IsTombstoned(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("X")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	if fm.IsTombstoned() {
		t.Error("IsTombstoned() = true, want false")
	}
}

func TestFileManager_CloseIdempotent(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("X")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

	err1 := fm.Close()
	err2 := fm.Close()

	if err1 != nil {
		t.Errorf("First Close() returned error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second Close() returned error: %v", err2)
	}
}

func TestFileManager_NewFileManagerValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{
			name:     "empty file path returns error",
			filePath: "",
			wantErr:  true,
		},
		{
			name:     "nonexistent file returns error",
			filePath: "/tmp/nonexistent_frozendb_file_12345.fdb",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFileManager(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileManager_SetWriterExclusivity(t *testing.T) {
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

	dataChan := make(chan Data, 1)
	err = fm.SetWriter(dataChan)
	if err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	secondChan := make(chan Data, 1)
	err = fm.SetWriter(secondChan)
	if err == nil {
		t.Error("Second SetWriter should fail")
	}

	if _, ok := err.(*InvalidActionError); !ok {
		t.Errorf("Expected InvalidActionError, got %T", err)
	}

	close(dataChan)
}

func TestFileManager_SetWriterClosedFileManager(t *testing.T) {
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

	fm.Close()

	dataChan := make(chan Data, 1)
	err = fm.SetWriter(dataChan)
	if err == nil {
		t.Error("SetWriter on closed FileManager should fail")
	}
}

func TestFileManager_WriteDataAndVerify(t *testing.T) {
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

	testData := []byte("TEST_DATA_12345")
	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    testData,
		Response: responseChan,
	}

	err = <-responseChan
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	close(dataChan)

	readData, err := fm.Read(7, len(testData))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if string(readData) != string(testData) {
		t.Errorf("Read data = %q, want %q", string(readData), string(testData))
	}
}

func TestFileManager_WriteSequence(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	tmpFile.WriteString("START")
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

	writes := []string{"A", "B", "C", "D", "E"}
	for _, w := range writes {
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte(w),
			Response: responseChan,
		}
		err = <-responseChan
		if err != nil {
			t.Errorf("Write %s failed: %v", w, err)
		}
	}

	close(dataChan)

	expectedSize := int64(5 + 5)
	size := fm.Size()
	if size != expectedSize {
		t.Errorf("Size = %d, want %d", size, expectedSize)
	}
}

func TestFileManager_TombstoneOnWriteError(t *testing.T) {
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

	// Close the underlying file to force a write error
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

	if !fm.IsTombstoned() {
		t.Error("FileManager should be tombstoned after write error")
	}

	close(dataChan)
}

// =============================================================================
// Concurrency and Race Condition Tests
// =============================================================================

func TestFileManager_ConcurrentReadsWhileWriting(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	initialData := make([]byte, 10000)
	for i := range initialData {
		initialData[i] = byte(i % 256)
	}
	if _, err := tmpFile.Write(initialData); err != nil {
		t.Fatalf("Failed to write initial data: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 100)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	var wg sync.WaitGroup
	readErrors := make(chan error, 1000)
	writeErrors := make(chan error, 100)
	stopReaders := make(chan struct{})

	// Start readers that continuously read initial data
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for {
				select {
				case <-stopReaders:
					return
				default:
					offset := int64((readerID * 100) % 9000)
					data, err := fm.Read(offset, 100)
					if err != nil {
						readErrors <- err
						return
					}
					// Verify data integrity
					for j, b := range data {
						expected := byte((int(offset) + j) % 256)
						if b != expected {
							readErrors <- NewCorruptDatabaseError("data mismatch", nil)
							return
						}
					}
				}
			}
		}(i)
	}

	// Perform writes concurrently
	for i := 0; i < 50; i++ {
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte("WRITE_DATA_"),
			Response: responseChan,
		}
		if err := <-responseChan; err != nil {
			writeErrors <- err
		}
	}

	close(stopReaders)
	wg.Wait()
	close(dataChan)
	close(readErrors)
	close(writeErrors)

	for err := range readErrors {
		t.Errorf("Read error during concurrent access: %v", err)
	}
	for err := range writeErrors {
		t.Errorf("Write error during concurrent access: %v", err)
	}
}

func TestFileManager_ConcurrentSizeReads(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("INITIAL_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 100)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	var wg sync.WaitGroup
	var lastObservedSize int64

	// Writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			responseChan := make(chan error, 1)
			dataChan <- Data{
				Bytes:    []byte("X"),
				Response: responseChan,
			}
			<-responseChan
		}
	}()

	// Size readers - size should be monotonically increasing
	sizeErrors := make(chan string, 100)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prevSize := int64(0)
			for j := 0; j < 200; j++ {
				size := fm.Size()
				if size < prevSize {
					sizeErrors <- "size decreased"
				}
				prevSize = size
				atomic.StoreInt64(&lastObservedSize, size)
				runtime.Gosched()
			}
		}()
	}

	wg.Wait()
	close(dataChan)
	close(sizeErrors)

	for err := range sizeErrors {
		t.Errorf("Size consistency error: %s", err)
	}

	finalSize := fm.Size()
	expectedSize := int64(12 + 100) // "INITIAL_DATA" + 100 "X"
	if finalSize != expectedSize {
		t.Errorf("Final size = %d, want %d", finalSize, expectedSize)
	}
}

func TestFileManager_WriterChannelCloseDuringWrite(t *testing.T) {
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
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Write some data
	for i := 0; i < 5; i++ {
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte("DATA"),
			Response: responseChan,
		}
		<-responseChan
	}

	// Close channel
	close(dataChan)

	// Allow goroutine to process
	time.Sleep(10 * time.Millisecond)

	// Should be able to set a new writer after channel closes
	newChan := make(chan Data, 1)
	err = fm.SetWriter(newChan)
	if err != nil {
		t.Errorf("SetWriter after channel close failed: %v", err)
	}
	close(newChan)
}

func TestFileManager_RapidSetWriterAfterClose(t *testing.T) {
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

	// Rapidly create and close writers
	for i := 0; i < 10; i++ {
		dataChan := make(chan Data, 1)
		err := fm.SetWriter(dataChan)
		if err != nil {
			t.Errorf("Iteration %d: SetWriter failed: %v", i, err)
			continue
		}

		// Do one write
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte("X"),
			Response: responseChan,
		}
		<-responseChan

		// Close and wait for goroutine to finish
		close(dataChan)
		time.Sleep(5 * time.Millisecond)
	}

	expectedSize := int64(7 + 10) // "INITIAL" + 10 "X"
	if fm.Size() != expectedSize {
		t.Errorf("Size = %d, want %d", fm.Size(), expectedSize)
	}
}

func TestFileManager_MultipleWriteResponseChannels(t *testing.T) {
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

	dataChan := make(chan Data, 100)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Queue up many writes before waiting for responses
	responseChans := make([]chan error, 50)
	for i := 0; i < 50; i++ {
		responseChans[i] = make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte("X"),
			Response: responseChans[i],
		}
	}

	// Wait for all responses in order
	for i, respChan := range responseChans {
		select {
		case err := <-respChan:
			if err != nil {
				t.Errorf("Write %d failed: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("Write %d timed out", i)
		}
	}

	close(dataChan)
}

func TestFileManager_ReadWhileTombstoned(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("TEST_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 1)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Force tombstone by closing file and attempting write
	fm.file.Close()
	fm.file = nil

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte("X"),
		Response: responseChan,
	}
	<-responseChan

	if !fm.IsTombstoned() {
		t.Fatal("Expected tombstoned state")
	}

	// Reads should fail when tombstoned
	_, err = fm.Read(0, 4)
	if err == nil {
		t.Error("Read should fail when tombstoned")
	}
	if _, ok := err.(*InvalidActionError); !ok {
		t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
	}

	close(dataChan)
}

func TestFileManager_SetWriterWhileTombstoned(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("TEST_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 1)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Force tombstone
	fm.file.Close()
	fm.file = nil

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte("X"),
		Response: responseChan,
	}
	<-responseChan
	close(dataChan)

	// Wait for writerLoop to exit
	time.Sleep(10 * time.Millisecond)

	// SetWriter should fail when tombstoned
	newChan := make(chan Data, 1)
	err = fm.SetWriter(newChan)
	if err == nil {
		t.Error("SetWriter should fail when tombstoned")
	}
	if _, ok := err.(*InvalidActionError); !ok {
		t.Errorf("Expected InvalidActionError, got %T: %v", err, err)
	}
}

func TestFileManager_ConcurrentCloseAttempts(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("TEST_DATA")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Multiple concurrent close attempts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fm.Close(); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestFileManager_WriteOrderPreserved(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 100)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Write numbered data
	for i := 0; i < 100; i++ {
		responseChan := make(chan error, 1)
		data := []byte{byte(i)}
		dataChan <- Data{
			Bytes:    data,
			Response: responseChan,
		}
		if err := <-responseChan; err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
	}

	close(dataChan)

	// Verify order
	result, err := fm.Read(0, 100)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for i, b := range result {
		if b != byte(i) {
			t.Errorf("Byte at %d = %d, want %d", i, b, i)
		}
	}
}

func TestFileManager_ReadExactlyAtFileEnd(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testData := []byte("EXACTLY10!")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Read exactly to the end
	data, err := fm.Read(0, 10)
	if err != nil {
		t.Errorf("Read to exact end failed: %v", err)
	}
	if string(data) != string(testData) {
		t.Errorf("Data = %q, want %q", string(data), string(testData))
	}

	// Read starting from last byte
	data, err = fm.Read(9, 1)
	if err != nil {
		t.Errorf("Read last byte failed: %v", err)
	}
	if data[0] != '!' {
		t.Errorf("Last byte = %c, want !", data[0])
	}

	// Read one byte past end should fail
	_, err = fm.Read(10, 1)
	if err == nil {
		t.Error("Read past end should fail")
	}
}

func TestFileManager_Int64OverflowProtection(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("TEST")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Test overflow protection in Read
	_, err = fm.Read(9223372036854775807, 100) // MaxInt64
	if err == nil {
		t.Error("Read with overflow should fail")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
	}
}

func TestFileManager_EmptyWrite(t *testing.T) {
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

	initialSize := fm.Size()

	dataChan := make(chan Data, 1)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Write empty data
	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte{},
		Response: responseChan,
	}
	if err := <-responseChan; err != nil {
		t.Errorf("Empty write failed: %v", err)
	}

	close(dataChan)

	// Size should be unchanged
	if fm.Size() != initialSize {
		t.Errorf("Size changed after empty write: %d -> %d", initialSize, fm.Size())
	}
}

func TestFileManager_LargeWriteData(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 1)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Write 10MB of data
	largeData := make([]byte, 10*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    largeData,
		Response: responseChan,
	}

	select {
	case err := <-responseChan:
		if err != nil {
			t.Errorf("Large write failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Large write timed out")
	}

	close(dataChan)

	if fm.Size() != int64(len(largeData)) {
		t.Errorf("Size = %d, want %d", fm.Size(), len(largeData))
	}

	// Verify data integrity
	readData, err := fm.Read(0, len(largeData))
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for i := 0; i < len(largeData); i += 1000000 {
		if readData[i] != largeData[i] {
			t.Errorf("Data mismatch at %d: got %d, want %d", i, readData[i], largeData[i])
		}
	}
}

func TestFileManager_WriteErrorStopsLoop(t *testing.T) {
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
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	// Queue multiple writes
	responses := make([]chan error, 5)
	for i := 0; i < 5; i++ {
		responses[i] = make(chan error, 1)
	}

	// Close file to cause write errors
	fm.file.Close()
	fm.file = nil

	for i := 0; i < 5; i++ {
		dataChan <- Data{
			Bytes:    []byte("X"),
			Response: responses[i],
		}
	}

	// First write should fail
	err = <-responses[0]
	if err == nil {
		t.Error("First write should have failed")
	}

	// Writer loop exits on error, so channel won't be drained
	// Give time for any processing
	time.Sleep(20 * time.Millisecond)

	// The file manager should be tombstoned
	if !fm.IsTombstoned() {
		t.Error("FileManager should be tombstoned after write error")
	}

	close(dataChan)
}

func TestFileManager_SizeUpdatesAreAtomic(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("START")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 1000)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	var wg sync.WaitGroup
	sizeIncreases := int32(0)

	// Reader goroutine checking sizes
	wg.Add(1)
	go func() {
		defer wg.Done()
		prevSize := fm.Size()
		for i := 0; i < 10000; i++ {
			size := fm.Size()
			if size > prevSize {
				atomic.AddInt32(&sizeIncreases, 1)
				prevSize = size
			} else if size < prevSize {
				t.Errorf("Size decreased: %d -> %d", prevSize, size)
			}
		}
	}()

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			responseChan := make(chan error, 1)
			dataChan <- Data{
				Bytes:    []byte("XX"),
				Response: responseChan,
			}
			<-responseChan
		}
	}()

	wg.Wait()
	close(dataChan)

	expectedFinalSize := int64(5 + 500*2)
	if fm.Size() != expectedFinalSize {
		t.Errorf("Final size = %d, want %d", fm.Size(), expectedFinalSize)
	}
}

func TestFileManager_ReadAfterMultipleWrites(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("")
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	dataChan := make(chan Data, 10)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	writes := []string{"FIRST", "SECOND", "THIRD", "FOURTH", "FIFTH"}
	offsets := make([]int64, len(writes))
	offset := int64(0)

	for i, w := range writes {
		offsets[i] = offset
		responseChan := make(chan error, 1)
		dataChan <- Data{
			Bytes:    []byte(w),
			Response: responseChan,
		}
		if err := <-responseChan; err != nil {
			t.Fatalf("Write %d failed: %v", i, err)
		}
		offset += int64(len(w))
	}

	close(dataChan)

	// Read each write back
	for i, w := range writes {
		data, err := fm.Read(offsets[i], len(w))
		if err != nil {
			t.Errorf("Read %d failed: %v", i, err)
			continue
		}
		if string(data) != w {
			t.Errorf("Read %d = %q, want %q", i, string(data), w)
		}
	}
}

func TestFileManager_BlockedWriterDoesNotBlockReaders(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testData := make([]byte, 1000)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	// Create unbuffered channel to create back-pressure
	dataChan := make(chan Data)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	readDone := make(chan bool, 1)

	// Reader should not be blocked by writer
	go func() {
		for i := 0; i < 100; i++ {
			_, err := fm.Read(0, 100)
			if err != nil {
				t.Errorf("Read failed: %v", err)
				return
			}
		}
		readDone <- true
	}()

	// Wait for reads to complete (should be fast since writer is blocked)
	select {
	case <-readDone:
		// Success - reads completed despite blocked writer
	case <-time.After(5 * time.Second):
		t.Error("Reads blocked by pending writer")
	}

	close(dataChan)
}

func TestFileManager_ZeroSizeFile(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "frozendb_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	fm, err := NewFileManager(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create FileManager: %v", err)
	}
	defer fm.Close()

	if fm.Size() != 0 {
		t.Errorf("Size = %d, want 0", fm.Size())
	}

	// Any read should fail
	_, err = fm.Read(0, 1)
	if err == nil {
		t.Error("Read from empty file should fail")
	}

	// But writes should work
	dataChan := make(chan Data, 1)
	if err := fm.SetWriter(dataChan); err != nil {
		t.Fatalf("SetWriter failed: %v", err)
	}

	responseChan := make(chan error, 1)
	dataChan <- Data{
		Bytes:    []byte("FIRST"),
		Response: responseChan,
	}
	if err := <-responseChan; err != nil {
		t.Errorf("Write to empty file failed: %v", err)
	}

	close(dataChan)

	if fm.Size() != 5 {
		t.Errorf("Size after write = %d, want 5", fm.Size())
	}
}
