package frozendb

import (
	"math"
	"os"
	"sync"
)

type Data struct {
	Bytes    []byte
	Response chan<- error
}

type FileManager struct {
	filePath     string
	file         *os.File
	mutex        sync.RWMutex
	writeChannel <-chan Data
	currentSize  int64
	tombstone    bool
	closed       bool
}

func NewFileManager(filePath string) (*FileManager, error) {
	if filePath == "" {
		return nil, NewInvalidInputError("file path cannot be empty", nil)
	}

	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return nil, NewPathError("failed to open file", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, NewPathError("failed to stat file", err)
	}

	fm := &FileManager{
		filePath:    filePath,
		file:        file,
		currentSize: fileInfo.Size(),
		tombstone:   false,
		closed:      false,
	}

	return fm, nil
}

func (fm *FileManager) Read(start int64, size int) ([]byte, error) {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()

	if fm.closed {
		return nil, NewInvalidActionError("file manager is closed", nil)
	}
	if fm.tombstone {
		return nil, NewInvalidActionError("file manager is tombstoned", nil)
	}

	if start < 0 {
		return nil, NewInvalidInputError("start offset cannot be negative", nil)
	}
	if size <= 0 {
		return nil, NewInvalidInputError("size must be positive", nil)
	}
	size64 := int64(size)
	if size64 > math.MaxInt64-start {
		return nil, NewInvalidInputError("read range overflows int64", nil)
	}
	if start+size64 > fm.currentSize {
		return nil, NewInvalidInputError("read exceeds file size", nil)
	}

	data := make([]byte, size)
	_, err := fm.file.ReadAt(data, start)
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to read from file", err)
	}

	return data, nil
}

func (fm *FileManager) Size() int64 {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.currentSize
}

func (fm *FileManager) IsTombstoned() bool {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.tombstone
}

func (fm *FileManager) Close() error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if fm.closed {
		return nil
	}

	fm.closed = true

	if fm.file != nil {
		fileErr := fm.file.Close()
		fm.file = nil
		if fileErr != nil {
			return NewWriteError("failed to close file", fileErr)
		}
	}

	return nil
}

func (fm *FileManager) SetWriter(dataChan <-chan Data) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if fm.closed {
		return NewInvalidActionError("file manager is closed", nil)
	}
	if fm.tombstone {
		return NewInvalidActionError("file manager is tombstoned", nil)
	}
	if fm.writeChannel != nil {
		return NewInvalidActionError("writer already active", nil)
	}

	fm.writeChannel = dataChan
	go fm.writerLoop(dataChan)

	return nil
}

// writerLoop drains the provided channel, writing data to the file.
// The channel is passed as a parameter to ensure this goroutine drains
// the channel it was started with, even if fm.writeChannel changes.
func (fm *FileManager) writerLoop(dataChan <-chan Data) {
	for data := range dataChan {
		err := fm.processWrite(data.Bytes)
		data.Response <- err
		if err != nil {
			return
		}
	}

	fm.mutex.Lock()
	fm.writeChannel = nil
	fm.mutex.Unlock()
}

// processWrite handles a single write operation.
// Returns an error if the write fails or would overflow; on error, the file is tombstoned.
func (fm *FileManager) processWrite(bytes []byte) error {
	dataLen := int64(len(bytes))

	// RLock for validation and the write operation itself
	writeOffset, err := func() (int64, error) {
		fm.mutex.RLock()
		defer fm.mutex.RUnlock()

		if dataLen > math.MaxInt64-fm.currentSize {
			return 0, NewInvalidInputError("write would overflow file size", nil)
		}
		if fm.file == nil {
			return 0, NewWriteError("file is closed", nil)
		}
		return fm.currentSize, nil
	}()
	if err != nil {
		fm.setTombstone()
		return err
	}

	_, writeErr := fm.file.WriteAt(bytes, writeOffset)
	if writeErr != nil {
		fm.setTombstone()
		return NewWriteError("failed to write data", writeErr)
	}

	// Exclusive lock only for updating currentSize
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	fm.currentSize += dataLen
	return nil
}

func (fm *FileManager) setTombstone() {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	fm.tombstone = true
}
