package frozendb

import (
	"os"
	"strings"
	"sync"
	"syscall"
)

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
	file *os.File // Open file descriptor
	mode string   // Access mode (read or write)

	// Database metadata from header
	header *Header // Parsed header information

	// State management for thread-safe cleanup
	mu     sync.Mutex // Protects closed flag
	closed bool       // Tracks if Close() has been called
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
	if err := validateOpenInputs(path, mode); err != nil {
		return nil, err
	}

	// Open file descriptor
	file, err := openDatabaseFile(path, mode)
	if err != nil {
		return nil, err
	}

	// Setup cleanup on error
	var cleanupErr error
	defer func() {
		if cleanupErr != nil {
			_ = file.Close()
		}
	}()

	// Read and validate header
	header, err := readAndValidateHeader(file)
	if err != nil {
		cleanupErr = err
		return nil, err
	}

	// Acquire lock if write mode (readers need no locks)
	if mode == MODE_WRITE {
		err = acquireFileLock(file, syscall.LOCK_EX, false)
		if err != nil {
			cleanupErr = err
			return nil, err
		}
	}

	// Create FrozenDB instance
	db := &FrozenDB{
		file:   file,
		mode:   mode,
		header: header,
		closed: false,
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

	// Validate mode is valid
	if db.mode != MODE_READ && db.mode != MODE_WRITE {
		return NewInvalidInputError("FrozenDB mode must be 'read' or 'write'", nil)
	}

	return nil
}

// Close releases all resources associated with the database connection
// This method is thread-safe and idempotent - multiple concurrent calls are safe
// Returns nil if already closed or cleanup successful
func (db *FrozenDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Check if already closed
	if db.closed {
		return nil
	}

	// Mark as closed first to prevent multiple cleanup attempts
	db.closed = true

	// Release file lock if in write mode
	if db.mode == MODE_WRITE && db.file != nil {
		// Ignore lock release errors during cleanup
		_ = releaseFileLock(db.file)
	}

	// Close file descriptor
	if db.file != nil {
		if err := db.file.Close(); err != nil {
			return NewWriteError("failed to close file descriptor", err)
		}
	}

	return nil
}

// validateOpenInputs validates input parameters for NewFrozenDB
func validateOpenInputs(path string, mode string) error {
	// Validate path is not empty
	if path == "" {
		return NewInvalidInputError("path cannot be empty", nil)
	}

	// Validate path has .fdb extension
	if !strings.HasSuffix(path, FILE_EXTENSION) || len(path) <= len(FILE_EXTENSION) {
		return NewInvalidInputError("path must have .fdb extension", nil)
	}

	// Validate mode value
	if mode != MODE_READ && mode != MODE_WRITE {
		return NewInvalidInputError("mode must be 'read' or 'write'", nil)
	}

	return nil
}

// openDatabaseFile opens the database file with appropriate flags for the mode
func openDatabaseFile(path string, mode string) (*os.File, error) {
	// Determine open flags based on mode
	var flags int
	if mode == MODE_READ {
		flags = os.O_RDONLY
	} else { // MODE_WRITE
		flags = os.O_RDWR
	}

	// Open file using fsInterface for testability
	file, err := fsInterface.Open(path, flags, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewPathError("database file does not exist", err)
		}
		if os.IsPermission(err) {
			return nil, NewPathError("permission denied to access database file", err)
		}
		return nil, NewPathError("failed to open database file", err)
	}

	return file, nil
}

// readAndValidateHeader reads first 64 bytes and validates frozenDB v1 header
func readAndValidateHeader(file *os.File) (*Header, error) {
	// Read first 64 bytes
	headerBytes := make([]byte, HEADER_SIZE)
	n, err := file.Read(headerBytes)
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to read header", err)
	}

	if n != HEADER_SIZE {
		return nil, NewCorruptDatabaseError(
			"incomplete header read",
			nil,
		)
	}

	// Parse and validate header using UnmarshalText
	header := &Header{}
	if err := header.UnmarshalText(headerBytes); err != nil {
		return nil, err // Already a CorruptDatabaseError
	}

	return header, nil
}
