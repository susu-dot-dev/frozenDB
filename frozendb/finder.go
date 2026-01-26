package frozendb

import "github.com/google/uuid"

// FinderStrategy selects the finder implementation when creating a FrozenDB.
//
// Memory-performance trade-offs:
//   - FinderStrategySimple: O(row_size) fixed memory regardless of DB size; GetIndex O(n),
//     GetTransactionStart/End O(k) where k ≤ 101. Use when DB is large or memory is bounded.
//   - FinderStrategyInMemory: ~40 bytes per row (uuid map + tx boundary maps); GetIndex,
//     GetTransactionStart, GetTransactionEnd all O(1). Use when DB fits in memory and
//     read-heavy workloads need low latency.
type FinderStrategy string

const (
	FinderStrategySimple   FinderStrategy = "simple"
	FinderStrategyInMemory FinderStrategy = "inmemory"
)

// Finder defines methods for locating rows and transaction boundaries in frozenDB files.
// This interface enables different finder implementations with varying performance characteristics
// while maintaining identical functional behavior.
//
// All finder implementations must:
// - Return identical results for all valid inputs
// - Handle all row types defined in the v1 file format specification
// - Maintain internal consistency between read operations and row addition notifications
// - Provide thread-safe access for concurrent Get* method calls
//
// Index Scheme:
//   - Index 0: First checksum row after header (bytes [64 .. 64+row_size-1])
//   - Index 1: First data/null row (bytes [64+row_size .. 64+2*row_size-1])
//   - Index N: Nth row after header (bytes [64+N*row_size .. 64+(N+1)*row_size-1])
type Finder interface {
	// GetIndex returns the index of the first row containing the specified UUID key.
	// Only complete DataRows are searched; ChecksumRows, NullRows, and PartialDataRows are skipped.
	//
	// Parameters:
	//   - key: The UUIDv7 key to search for (must not be uuid.Nil)
	//
	// Returns:
	//   - index: Zero-based index of the matching DataRow
	//   - error: KeyNotFoundError if not found, InvalidInputError for invalid UUID,
	//            CorruptDatabaseError for data corruption, ReadError for I/O failures
	//
	// Thread-safe: May be called concurrently with other Get* methods
	GetIndex(key uuid.UUID) (int64, error)

	// GetTransactionEnd returns the index of the last row in the transaction containing
	// the specified index. The transaction end is identified by rows with transaction-ending
	// end_control values: TC, SC, R0-R9, S0-S9, or NR.
	//
	// Parameters:
	//   - index: Index of a row within the transaction (must be a DataRow or NullRow)
	//
	// Returns:
	//   - endIndex: Index of the row with transaction-ending end_control
	//   - error: InvalidInputError for invalid indices or checksum rows,
	//            TransactionActiveError if transaction has no ending row,
	//            CorruptDatabaseError for invalid control bytes or malformed transactions,
	//            ReadError for I/O failures
	//
	// If the input index itself ends the transaction, returns the same index.
	// Thread-safe: May be called concurrently with other Get* methods
	GetTransactionEnd(index int64) (int64, error)

	// GetTransactionStart returns the index of the first row in the transaction containing
	// the specified index. The transaction start is identified by rows with start_control='T'.
	//
	// Parameters:
	//   - index: Index of a row within the transaction (must be a DataRow or NullRow)
	//
	// Returns:
	//   - startIndex: Index of the row with start_control='T' in the transaction chain
	//   - error: InvalidInputError for invalid indices or checksum rows,
	//            CorruptDatabaseError for invalid control bytes or no transaction start found,
	//            ReadError for I/O failures
	//
	// If the input index itself starts the transaction, returns the same index.
	// Thread-safe: May be called concurrently with other Get* methods
	GetTransactionStart(index int64) (int64, error)

	// OnRowAdded is called when a new row is successfully written to the database.
	// Updates finder internal state to include the new row for subsequent operations.
	// Called within transaction write lock context; implementation must not attempt
	// to acquire additional transaction locks.
	//
	// Parameters:
	//   - index: Index of the newly added row (must follow sequential ordering)
	//   - row: Complete row data of the newly added row
	//
	// Returns:
	//   - error: InvalidInputError if index validation fails or gaps in sequential ordering,
	//            CorruptDatabaseError if row data cannot be parsed
	//
	// Preconditions:
	//   - Row data is successfully written and persistent on disk
	//   - Called within transaction write lock context
	//   - Index follows zero-based scheme and sequential ordering
	//
	// Postconditions:
	//   - GetIndex() can locate the new row by its UUID key
	//   - Transaction boundary methods handle the new index correctly
	//   - Confirmed file size updated to include new row
	//
	// Not thread-safe with itself: Calls are guaranteed sequential (no self-racing)
	// Blocks until completion before returning to caller
	OnRowAdded(index int64, row *RowUnion) error

	// MaxTimestamp returns the maximum timestamp among all complete data and null rows
	// in O(1) time. Returns 0 if no complete data or null rows exist.
	//
	// Returns:
	//   - int64: Maximum timestamp value, or 0 if no complete data/null rows exist
	//
	// Time Complexity: O(1) - must execute in constant time
	// Thread-safe: Safe for concurrent read access
	MaxTimestamp() int64
}
