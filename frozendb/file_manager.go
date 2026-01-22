package frozendb

import (
	"errors"
	"math"
	"os"
	"sync/atomic"
)

type Data struct {
	Bytes    []byte
	Response chan<- error
}

type FileManager struct {
	file         atomic.Value // stores *os.File (nil after Close())
	writeChannel atomic.Value // stores <-chan Data (nil when no writer)
	currentSize  atomic.Uint64
}

func NewFileManager(filePath string) (*FileManager, error) {
	if filePath == "" {
		return nil, NewInvalidInputError("file path cannot be empty", nil)
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, NewPathError("failed to open file", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, NewPathError("failed to stat file", err)
	}

	fm := &FileManager{}
	fm.file.Store(file)
	fm.writeChannel.Store((<-chan Data)(nil))
	fm.currentSize.Store(uint64(fileInfo.Size()))

	return fm, nil
}

func (fm *FileManager) Read(start int64, size int32) ([]byte, error) {
	if start < 0 {
		return nil, NewInvalidInputError("start offset cannot be negative", nil)
	}
	if size <= 0 {
		return nil, NewInvalidInputError("size must be positive", nil)
	}
	// Guaranteed not to overflow because start is int64 and size is int32
	// Thus, the max value is MAX_INT64 + MAX_INT32 < MAX_UINT64
	if uint64(start)+uint64(size) > fm.currentSize.Load() {
		return nil, NewInvalidInputError("read exceeds file size", nil)
	}

	data := make([]byte, size)
	file, err := fm.getFile()
	if err != nil {
		return nil, err
	}
	_, err = file.ReadAt(data, start)
	if err != nil {
		// If there's a race, and Close() is called before the read, detect that and wrap the correct frozendDB error
		if errors.Is(err, os.ErrClosed) {
			return nil, NewTombstonedError("file manager is closed", err)
		}
		return nil, NewCorruptDatabaseError("failed to read from file", err)
	}

	return data, nil
}

func (fm *FileManager) Size() int64 {
	return int64(fm.currentSize.Load())
}

func (fm *FileManager) Close() error {
	file := fm.file.Load().(*os.File)
	if file != nil && fm.file.CompareAndSwap(file, (*os.File)(nil)) {
		// First time Close() was called, and also we won any race calling Close() multiple times
		_ = file.Close()
	}
	return nil
}

func (fm *FileManager) SetWriter(dataChan <-chan Data) error {
	// The writer channel is allowed to be set when the FileManager is closed
	// In that case, any writes will fail
	if !fm.writeChannel.CompareAndSwap((<-chan Data)(nil), dataChan) {
		return NewInvalidActionError("writer already active", nil)
	}
	go fm.writerLoop(dataChan)

	return nil
}

// writerLoop drains the provided channel, writing data to the file.
// The channel is passed as a parameter to ensure this goroutine drains
// the channel it was started with, even if fm.writeChannel changes.
func (fm *FileManager) writerLoop(dataChan <-chan Data) {
	defer fm.writeChannel.Store((<-chan Data)(nil))
	for data := range dataChan {
		err := fm.processWrite(data.Bytes)
		data.Response <- err
		if err != nil {
			return
		}
	}
}

// processWrite handles a single write operation.
// Returns an error if the write fails or would overflow; on error, the file is tombstoned.
func (fm *FileManager) processWrite(bytes []byte) error {
	// There is still a potential race condition with this function and Close()
	// because Close() can be called at any point after it was checked above.
	// However, in that case, either the last write will slip through the covers
	// or the file will be closed, and file.Write() will fail from the OS perspective
	// processWrite is the only function that can write to the file, and since there can only be
	// writerLoop, only this function can change the currentSize. Thus, we can read the currentSize
	// and know it won't be changed from under us.
	currentSize := fm.currentSize.Load()
	appendSize := uint64(len(bytes))
	if appendSize > math.MaxInt64 || appendSize+currentSize > math.MaxInt64 {
		return NewInvalidInputError("write would overflow file size", nil)
	}

	file, err := fm.getFile()
	if err != nil {
		return err
	}
	// The file is opened with O_APPEND, so Write will append to the end of the file
	_, writeErr := file.Write(bytes)
	if writeErr != nil {
		_ = fm.Close()
		if errors.Is(writeErr, os.ErrClosed) {
			return NewTombstonedError("file manager is closed", writeErr)
		}
		return NewWriteError("failed to write data", writeErr)
	}
	fm.currentSize.Add(appendSize)
	return nil
}

func (fm *FileManager) getFile() (*os.File, error) {
	file := fm.file.Load().(*os.File)
	if file == nil {
		return nil, NewTombstonedError("file manager is closed", os.ErrClosed)
	}
	return file, nil
}
