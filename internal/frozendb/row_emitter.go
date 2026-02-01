package frozendb

import "sync"

// RowEmitter monitors DBFile for row completion and notifies subscribers.
type RowEmitter struct {
	dbfile            DBFile
	dbfileUnsubscribe func() error
	subscribers       *Subscriber[func(int64, *RowUnion) error]
	mu                sync.Mutex
	lastKnownFileSize int64
	rowSize           int // Cached from header for calculating row boundaries
}

// NewRowEmitter creates a new RowEmitter that monitors the given DBFile.
//
// Parameters:
//   - dbfile: The DBFile instance to monitor (MUST NOT be nil)
//   - rowSize: The row size from database header (used for calculating row boundaries)
//
// Returns:
//   - *RowEmitter: New RowEmitter instance
//   - error: InvalidInputError if dbfile is nil or rowSize is invalid, or errors from DBFile operations
//
// Behavior:
//   - Validates dbfile is not nil and rowSize is valid
//   - Queries DBFile for current size
//   - Initializes lastKnownFileSize to current file size
//   - Subscribes to DBFile for future write notifications
//   - Initializes empty subscriber map
func NewRowEmitter(dbfile DBFile, rowSize int) (*RowEmitter, error) {
	if dbfile == nil {
		return nil, NewInvalidInputError("dbfile cannot be nil", nil)
	}
	if rowSize < MIN_ROW_SIZE || rowSize > MAX_ROW_SIZE {
		return nil, NewInvalidInputError("rowSize must be between 128 and 65536", nil)
	}

	// Query current file size
	currentSize := dbfile.Size()

	// Create RowEmitter with initial state
	emitter := &RowEmitter{
		dbfile:            dbfile,
		subscribers:       NewSubscriber[func(int64, *RowUnion) error](),
		lastKnownFileSize: currentSize,
		rowSize:           rowSize,
	}

	// Subscribe to DBFile for future notifications
	unsubscribe, err := dbfile.Subscribe(func() error {
		return emitter.onDBFileNotification()
	})
	if err != nil {
		return nil, err
	}
	emitter.dbfileUnsubscribe = unsubscribe

	return emitter, nil
}

// Subscribe registers a callback to receive notifications when complete rows are written.
//
// Parameters:
//   - callback: Function to call for each completed row (MUST NOT be nil)
//   - index: Zero-based position of the completed row in the file
//   - row: Pointer to RowUnion containing the completed row data
//   - Returns error if processing fails
//
// Returns:
//   - unsubscribe: Closure to remove the subscription (idempotent)
//   - error: InvalidInputError if callback is nil
//
// Behavior:
//   - Validates callback is not nil
//   - Delegates to internal Subscriber[T].Subscribe()
//   - When DBFile notifies of writes, queries for new complete rows
//   - For each new complete row (in chronological order):
//   - Creates snapshot and calls each subscriber
//   - Stops on first error and propagates back
//
// Thread-safe: May be called concurrently with other RowEmitter methods.
func (re *RowEmitter) Subscribe(callback func(index int64, row *RowUnion) error) (func() error, error) {
	if callback == nil {
		return nil, NewInvalidInputError("callback cannot be nil", nil)
	}
	return re.subscribers.Subscribe(callback), nil
}

// Close cleans up RowEmitter resources and unsubscribes from DBFile.
//
// Behavior:
//   - Calls DBFile unsubscribe function (from initialization)
//   - Marks RowEmitter as closed
//
// Thread-safe: May be called concurrently with other methods.
// Idempotent: Multiple calls are safe (after first call, unsubscribe is already done).
func (re *RowEmitter) Close() error {
	if re.dbfileUnsubscribe != nil {
		if err := re.dbfileUnsubscribe(); err != nil {
			return err
		}
		re.dbfileUnsubscribe = nil // Mark as closed
	}
	return nil
}

// onDBFileNotification is called when DBFile writes occur.
// This handler calculates completed rows from file size change and emits notifications.
//
// Process Flow:
//  1. Query new file size from DBFile
//  2. Compare with lastKnownFileSize to determine growth
//  3. Calculate which rows are now complete from file size
//  4. For each newly completed row in chronological order:
//     - Read row data from file
//     - Get subscriber snapshot
//     - Execute each callback with (index, row)
//     - Stop on first error and propagate backward
//  5. Update lastKnownFileSize to new file size
//
// Row Indexing:
//   - Row 0: Initial checksum row (after header)
//   - Row 1: First data row
//   - Row N: Nth row (including checksum and data rows)
func (re *RowEmitter) onDBFileNotification() error {
	// Lock to read lastKnownFileSize
	re.mu.Lock()
	oldSize := re.lastKnownFileSize
	re.mu.Unlock()

	// Query new file size (no lock held during I/O)
	newSize := re.dbfile.Size()

	// No growth means no new complete rows
	if newSize <= oldSize {
		return nil
	}

	// Calculate file structure:
	// - Header: 64 bytes (HEADER_SIZE)
	// - Row 0: Initial checksum row (rowSize bytes)
	// - Row 1+: Data rows (variable number of complete rows)
	// - Possible partial row at end (size < rowSize)
	headerEnd := int64(HEADER_SIZE)

	// Calculate old and new row counts (including checksum row as row 0)
	oldRowsSize := oldSize - headerEnd
	if oldRowsSize < 0 {
		oldRowsSize = 0 // File smaller than header
	}
	oldRowCount := oldRowsSize / int64(re.rowSize)

	newRowsSize := newSize - headerEnd
	if newRowsSize < 0 {
		newRowsSize = 0
	}
	newRowCount := newRowsSize / int64(re.rowSize)

	// Emit notification for each newly completed row
	for rowIndex := oldRowCount; rowIndex < newRowCount; rowIndex++ {
		// Read row from file (row 0 is at offset HEADER_SIZE)
		rowOffset := headerEnd + (rowIndex * int64(re.rowSize))
		rowBytes, err := re.dbfile.Read(rowOffset, int32(re.rowSize))
		if err != nil {
			return err
		}

		// Parse row
		row := &RowUnion{}
		if err := row.UnmarshalText(rowBytes); err != nil {
			return err
		}

		// Get snapshot of subscribers (lock held only during snapshot creation)
		snapshot := re.subscribers.Snapshot()

		// Execute callbacks (no lock held)
		for _, callback := range snapshot {
			if err := callback(rowIndex, row); err != nil {
				return err // First error stops chain
			}
		}
	}

	// Update lastKnownFileSize (lock held only for state update)
	re.mu.Lock()
	re.lastKnownFileSize = newSize
	re.mu.Unlock()

	return nil
}
