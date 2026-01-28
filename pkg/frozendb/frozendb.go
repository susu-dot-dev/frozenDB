// Package frozendb provides a minimal public API for working with frozenDB databases.
//
// This package re-exports core types and functions from the internal implementation,
// exposing only what's needed for opening, querying, and transacting with existing databases.
// Database creation functionality is intentionally excluded and will be provided via CLI tools.
//
// Import Path: github.com/susu-dot-dev/frozenDB/pkg/frozendb
package frozendb

import (
	internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

// FrozenDB represents an open connection to a frozenDB database file.
// Instance methods are NOT thread-safe - use one instance per goroutine.
// Close() is thread-safe and can be called concurrently from multiple goroutines.
type FrozenDB = internal.FrozenDB

// NewFrozenDB opens an existing frozenDB database file with specified access mode
// and finder strategy.
//
// Parameters:
//   - path: Filesystem path to existing frozenDB database file (.fdb extension required)
//   - mode: Access mode - MODE_READ for read-only, MODE_WRITE for read-write
//   - strategy: FinderStrategySimple, FinderStrategyInMemory, or FinderStrategyBinarySearch
//
// Returns:
//   - *FrozenDB: Database instance ready for operations
//   - error: InvalidInputError (invalid strategy), PathError, CorruptDatabaseError, or WriteError
//
// Thread Safety: Safe for concurrent calls on different files
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error) {
	return internal.NewFrozenDB(path, mode, internal.FinderStrategy(strategy))
}

// Access mode constants for opening frozenDB database files
const (
	// MODE_READ opens the database in read-only mode with no lock.
	// Multiple readers can access the same file concurrently.
	MODE_READ = internal.MODE_READ

	// MODE_WRITE opens the database in read-write mode with exclusive lock.
	// Only one writer can access the file at a time.
	MODE_WRITE = internal.MODE_WRITE
)
