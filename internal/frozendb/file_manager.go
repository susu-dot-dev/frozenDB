package frozendb

import (
	"errors"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

type Data struct {
	Bytes    []byte
	Response chan<- error
}

// DBFile defines interface for file operations, enabling mock implementations for testing.
// This interface is used by Transaction to read rows and calculate checksums.
//
// Methods:
//   - Read: Reads bytes from file at specified offset
//   - Size: Returns current file size in bytes
//   - Close: Closes the file
//   - SetWriter: Sets write channel for appending data
//   - GetMode: Returns the access mode ("read" or "write")
//   - WriterClosed: Waits for writer goroutine to complete (returns immediately in read mode)
type DBFile interface {
	Read(start int64, size int32) ([]byte, error)
	Size() int64
	Close() error
	SetWriter(dataChan <-chan Data) error
	GetMode() string
	WriterClosed()
}

type FileManager struct {
	file         atomic.Value // stores *os.File (nil after Close())
	writeChannel atomic.Value // stores <-chan Data (nil when no writer)
	writerWg     sync.WaitGroup
	currentSize  atomic.Uint64
	mode         string // Access mode: "read" or "write"
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

// NewDBFile creates a new DBFile instance with specified access mode.
// Parameters:
//   - path: Filesystem path to frozenDB database file
//   - mode: Access mode - MODE_READ for read-only, MODE_WRITE for read-write
//
// Returns:
//   - DBFile: Interface implementation configured with mode-specific behavior
//   - error: InvalidInputError, PathError, or WriteError
func NewDBFile(path string, mode string) (DBFile, error) {
	// Validate mode
	if mode != MODE_READ && mode != MODE_WRITE {
		return nil, NewInvalidInputError("mode must be 'read' or 'write'", nil)
	}

	// Validate path extension
	if !strings.HasSuffix(path, FILE_EXTENSION) || len(path) <= len(FILE_EXTENSION) {
		return nil, NewInvalidInputError("path must have .fdb extension", nil)
	}

	var flags int
	if mode == MODE_READ {
		flags = os.O_RDONLY
	} else {
		flags = os.O_RDWR | os.O_APPEND
	}

	// Open file
	file, err := os.OpenFile(path, flags, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewPathError("database file does not exist", err)
		}
		if os.IsPermission(err) {
			return nil, NewPathError("permission denied to access database file", err)
		}
		return nil, NewPathError("failed to open database file", err)
	}

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, NewPathError("failed to stat file", err)
	}

	// Create FileManager
	fm := &FileManager{
		mode: mode,
	}
	fm.file.Store(file)
	fm.writeChannel.Store((<-chan Data)(nil))
	fm.currentSize.Store(uint64(fileInfo.Size()))

	// Acquire lock if write mode
	if mode == MODE_WRITE {
		lockMode := syscall.LOCK_EX | syscall.LOCK_NB
		err = syscall.Flock(int(file.Fd()), lockMode)
		if err != nil {
			_ = file.Close()
			if err == syscall.EWOULDBLOCK {
				return nil, NewWriteError("another process has the database locked", err)
			}
			return nil, NewWriteError("failed to acquire file lock", err)
		}
	}

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

func (fm *FileManager) GetMode() string {
	return fm.mode
}

// WriterClosed waits for the writer goroutine to complete.
// This method blocks until the writer finishes processing all queued writes and clears
// the writer state, ensuring transaction completion is atomic.
// If the DBFile is in read mode, returns immediately without waiting.
func (fm *FileManager) WriterClosed() {
	// If in read mode, return immediately (no writer to wait for)
	if fm.mode == MODE_READ {
		return
	}

	// Wait for writer goroutine to complete
	fm.writerWg.Wait()
}

func (fm *FileManager) Close() error {
	file := fm.file.Load().(*os.File)
	if file != nil && fm.file.CompareAndSwap(file, (*os.File)(nil)) {
		// Release lock if in write mode
		if fm.mode == MODE_WRITE {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		}
		// First time Close() was called, and also we won any race calling Close() multiple times
		_ = file.Close()
	}
	return nil
}

func (fm *FileManager) SetWriter(dataChan <-chan Data) error {
	// Check if in read mode
	if fm.mode == MODE_READ {
		return NewInvalidActionError("cannot set writer on read-mode DBFile", nil)
	}

	// Reject nil channel - outside callers cannot pass nil
	// The only way to reset the channel is for the writerLoop to finish
	// (when the channel is closed by the caller)
	if dataChan == nil {
		return NewInvalidActionError("dataChan cannot be nil", nil)
	}

	if !fm.writeChannel.CompareAndSwap((<-chan Data)(nil), dataChan) {
		return NewInvalidActionError("writer already active", nil)
	}

	// Increment wait group before starting goroutine
	// The writer channel is allowed to be set when the FileManager is closed
	// In that case, any writes will fail from getFile() or Write() calls
	fm.writerWg.Add(1)
	go fm.writerLoop(dataChan)

	return nil
}

// writerLoop drains the provided channel, writing data to the file.
// The channel is passed as a parameter to ensure this goroutine drains
// the channel it was started with, even if fm.writeChannel changes.
func (fm *FileManager) writerLoop(dataChan <-chan Data) {
	defer func() {
		fm.writeChannel.Store((<-chan Data)(nil))
		fm.writerWg.Done()
	}()

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
