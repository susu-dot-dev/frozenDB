package frozendb

import (
	"fmt"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

// WatcherOps defines the interface for file system watcher operations.
// This interface enables dependency injection for testing without real fsnotify.
// Production code uses realWatcherOps which wraps fsnotify.Watcher.
type WatcherOps interface {
	// NewWatcher creates a new file system watcher
	NewWatcher() (WatcherInstance, error)
}

// WatcherInstance defines the interface for an active file system watcher.
// Abstracts fsnotify.Watcher to enable testing with mocks.
type WatcherInstance interface {
	// Add starts watching the named file or directory
	Add(name string) error

	// Close removes all watches and closes the watcher
	Close() error

	// Events returns the event channel
	Events() <-chan fsnotify.Event

	// Errors returns the error channel
	Errors() <-chan error
}

// realWatcherOps implements WatcherOps using real fsnotify package
type realWatcherOps struct{}

// NewWatcher creates a real fsnotify.Watcher
func (r *realWatcherOps) NewWatcher() (WatcherInstance, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &realWatcherInstance{w: w}, nil
}

// realWatcherInstance wraps fsnotify.Watcher to implement WatcherInstance
type realWatcherInstance struct {
	w *fsnotify.Watcher
}

func (r *realWatcherInstance) Add(name string) error {
	return r.w.Add(name)
}

func (r *realWatcherInstance) Close() error {
	return r.w.Close()
}

func (r *realWatcherInstance) Events() <-chan fsnotify.Event {
	return r.w.Events
}

func (r *realWatcherInstance) Errors() <-chan error {
	return r.w.Errors
}

// FileWatcher monitors a database file for changes and notifies the parent Finder
// of new rows appended to the file. Used internally by InMemoryFinder and
// BinarySearchFinder when opened in read-mode.
//
// Architecture:
//   - Uses fsnotify (inotify on Linux) for file system event notifications
//   - Runs background goroutine (watchLoop) to process events
//   - Processes batches of rows on Write events to handle rapid appends efficiently
//   - Implements kickstart mechanism to catch rows written during Finder initialization
//
// Thread Safety:
//   - lastProcessedSize uses atomic operations for lock-free updates
//   - Callbacks (onRowAdded, onError) must be thread-safe in parent Finder
type FileWatcher struct {
	watcher           WatcherInstance              // File system watcher instance
	lastProcessedSize atomic.Int64                 // Last byte position processed from file
	dbFile            DBFile                       // Database file interface for reading
	onRowAdded        func(int64, *RowUnion) error // Callback to notify parent Finder
	onError           func(error)                  // Callback to notify parent of errors
	rowSize           int32                        // Fixed row size from header
	dbFilePath        string                       // Path to database file
}

// NewFileWatcher creates and initializes a FileWatcher using fsnotify for file monitoring.
// Launches background goroutine to process file change events.
//
// Parameters:
//   - dbFilePath: Filesystem path to database file
//   - dbFile: Database file interface for reading
//   - onRowAdded: Callback function to notify parent Finder of new rows
//   - onError: Callback function to notify parent Finder of errors
//   - rowSize: Fixed row size from header
//   - initialSize: Initial file size at start of parent Finder's scan
//   - watcherOps: WatcherOps interface for DI (use nil for production fsnotify)
//
// Returns:
//   - *FileWatcher: Fully initialized watcher with goroutine running
//   - error: Error if fsnotify initialization fails
//
// The FileWatcher immediately performs a "kickstart" to process any rows written
// between initialSize capture and watcher startup, preventing initialization races.
func NewFileWatcher(dbFilePath string, dbFile DBFile, onRowAdded func(int64, *RowUnion) error,
	onError func(error), rowSize int32, initialSize int64, watcherOps WatcherOps) (*FileWatcher, error) {

	// Use real fsnotify if no mock provided
	if watcherOps == nil {
		watcherOps = &realWatcherOps{}
	}

	// Create fsnotify watcher
	watcher, err := watcherOps.NewWatcher()
	if err != nil {
		return nil, NewInternalError("failed to create fsnotify watcher", err)
	}

	// Add database file to watch list
	if err := watcher.Add(dbFilePath); err != nil {
		_ = watcher.Close() // Best effort cleanup
		return nil, NewInternalError(fmt.Sprintf("failed to add watch for %s", dbFilePath), err)
	}

	fw := &FileWatcher{
		watcher:    watcher,
		dbFile:     dbFile,
		onRowAdded: onRowAdded,
		onError:    onError,
		rowSize:    rowSize,
		dbFilePath: dbFilePath,
	}

	// Set lastProcessedSize to initialSize (where Finder's scan stopped)
	fw.lastProcessedSize.Store(initialSize)

	// Launch background goroutine to monitor events
	go fw.watchLoop()

	return fw, nil
}

// Close stops the file watcher and releases resources.
// Closes the fsnotify watcher, which closes event channels and causes
// the background goroutine to exit cleanly.
//
// Returns:
//   - error: Error if watcher cleanup fails, nil if successful
//
// Thread Safety: Safe for concurrent calls; idempotent
func (fw *FileWatcher) Close() error {
	if fw.watcher == nil {
		return nil
	}

	if err := fw.watcher.Close(); err != nil {
		return NewInternalError("failed to close file watcher", err)
	}

	return nil
}

// watchLoop is the background goroutine that monitors fsnotify event channels.
// Runs until Close() is called (channels are closed by fsnotify).
//
// Event Processing:
//   - Write events: Call processBatch() to read and process new rows
//   - Error events: Call onError callback and exit
//   - Channel close: Exit cleanly (normal shutdown)
//
// Kickstart Mechanism:
//   - Immediately after starting, checks if file grew since initialSize
//   - Processes any gap between initialSize and current size
//   - Prevents missing rows written during Finder initialization window
func (fw *FileWatcher) watchLoop() {
	// Phase 3: Kickstart - Process any rows written during initialization
	// This handles the race window between Finder scan and watcher startup
	currentSize := fw.dbFile.Size()
	lastProcessed := fw.lastProcessedSize.Load()

	if currentSize > lastProcessed {
		// Gap detected - process rows written during initialization
		if err := fw.processBatch(); err != nil {
			fw.onError(err)
			return
		}
	}

	// Main event loop - monitor for future writes
	for {
		select {
		case event, ok := <-fw.watcher.Events():
			if !ok {
				// Channel closed - watcher was shut down
				return
			}

			// Only process Write events (file content changed)
			if event.Has(fsnotify.Write) {
				if err := fw.processBatch(); err != nil {
					fw.onError(err)
					return
				}
			}
			// Ignore all other event types (Create, Remove, Rename, Chmod)

		case err, ok := <-fw.watcher.Errors():
			if !ok {
				// Channel closed - watcher was shut down
				return
			}

			// Error from fsnotify - notify parent and exit
			fw.onError(NewInternalError("file watcher error", err))
			return
		}
	}
}

// processBatch reads and processes new complete rows from the database file.
// Calculates row boundaries to handle partial rows correctly.
//
// Algorithm:
//  1. Read current file size
//  2. Calculate gap: currentSize - lastProcessedSize
//  3. Determine complete rows: gap / rowSize (integer division)
//  4. Read and parse each complete row
//  5. Call onRowAdded for each row
//  6. Update lastProcessedSize
//  7. Leave partial rows unprocessed until next batch
//
// Returns:
//   - error: ReadError, CorruptDatabaseError, or error from onRowAdded callback
//
// Thread Safety: Called sequentially from watchLoop goroutine
func (fw *FileWatcher) processBatch() error {
	currentSize := fw.dbFile.Size()
	lastProcessed := fw.lastProcessedSize.Load()

	if currentSize <= lastProcessed {
		// No new data
		return nil
	}

	gap := currentSize - lastProcessed

	// Calculate number of complete rows
	// Integer division discards partial row bytes
	completeRows := gap / int64(fw.rowSize)

	if completeRows == 0 {
		// Only partial row data available - wait for more
		return nil
	}

	// Process each complete row
	for i := int64(0); i < completeRows; i++ {
		rowIndex := (lastProcessed-int64(HEADER_SIZE))/int64(fw.rowSize) + i
		offset := lastProcessed + i*int64(fw.rowSize)

		// Read row data
		rowBytes, err := fw.dbFile.Read(offset, fw.rowSize)
		if err != nil {
			return NewReadError(fmt.Sprintf("failed to read row at offset %d", offset), err)
		}

		// Parse row
		var rowUnion RowUnion
		if err := rowUnion.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError(fmt.Sprintf("failed to parse row at index %d", rowIndex), err)
		}

		// Notify parent Finder of new row
		if err := fw.onRowAdded(rowIndex, &rowUnion); err != nil {
			return err
		}
	}

	// Update lastProcessedSize to reflect processed rows
	newLastProcessed := lastProcessed + completeRows*int64(fw.rowSize)
	fw.lastProcessedSize.Store(newLastProcessed)

	return nil
}

// NewInternalError creates a new InternalError (wrapper for internal errors).
// This is used for file watcher errors that don't fit other error categories.
func NewInternalError(message string, err error) error {
	return NewReadError(message, err)
}
