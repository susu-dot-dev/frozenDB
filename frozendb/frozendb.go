package frozendb

// Access mode constants for opening frozenDB database files
const (
	// MODE_READ opens the database in read-only mode with no lock
	// Multiple readers can access the same file concurrently
	MODE_READ = "read"

	// MODE_WRITE opens the database in read-write mode with exclusive lock
	// Only one writer can access the file at a time
	MODE_WRITE = "write"
)

// FrozenDB represents an open connection to a frozenDB database file
// Instance methods are NOT thread-safe - use one instance per goroutine
// Close() is thread-safe and can be called concurrently from multiple goroutines
type FrozenDB struct {
	// Core file resources
	file DBFile // DBFile interface for file operations

	// Database metadata from header
	header *Header // Parsed header information
}

// NewFrozenDB opens an existing frozenDB database file with specified access mode
// Parameters:
//   - path: Filesystem path to existing frozenDB database file (.fdb extension required)
//   - mode: Access mode - MODE_READ for read-only, MODE_WRITE for read-write
//
// Returns:
//   - *FrozenDB: Database instance ready for operations
//   - error: InvalidInputError, PathError, CorruptDatabaseError, or WriteError
//
// Thread Safety: Safe for concurrent calls on different files
func NewFrozenDB(path string, mode string) (*FrozenDB, error) {
	// Validate input parameters
	// Open database file using DBFile interface
	dbFile, err := NewDBFile(path, mode)
	if err != nil {
		return nil, err
	}

	// Setup cleanup on error
	var cleanupErr error
	defer func() {
		if cleanupErr != nil {
			_ = dbFile.Close()
		}
	}()

	header, err := validateDatabaseFile(dbFile)
	if err != nil {
		cleanupErr = err
		return nil, err
	}

	// Create FrozenDB instance
	db := &FrozenDB{
		file:   dbFile,
		header: header,
	}

	// Validate the FrozenDB instance (ensures internal consistency)
	if err := db.Validate(); err != nil {
		cleanupErr = err
		return nil, err
	}

	return db, nil
}

// Validate validates the FrozenDB struct for internal consistency
// This method is idempotent and can be called multiple times with the same result
func (db *FrozenDB) Validate() error {
	// Validate file handle is not nil
	if db.file == nil {
		return NewInvalidInputError("FrozenDB file handle cannot be nil", nil)
	}

	// Validate header is not nil
	if db.header == nil {
		return NewInvalidInputError("FrozenDB header cannot be nil", nil)
	}

	// Validate header is valid (assumes header was validated during construction)
	// We call Validate() to ensure header remains valid
	if err := db.header.Validate(); err != nil {
		return NewCorruptDatabaseError("FrozenDB header validation failed", err)
	}

	// Validate mode is valid (get from DBFile)
	mode := db.file.GetMode()
	if mode != MODE_READ && mode != MODE_WRITE {
		return NewInvalidInputError("FrozenDB mode must be 'read' or 'write'", nil)
	}

	return nil
}

// Close releases all resources associated with the database connection
// This method is thread-safe and idempotent - multiple concurrent calls are safe
// Returns nil if already closed or cleanup successful
func (db *FrozenDB) Close() error {
	if db.file == nil {
		return nil
	}
	if err := db.file.Close(); err != nil {
		return NewWriteError("failed to close file descriptor", err)
	}
	return nil
}
