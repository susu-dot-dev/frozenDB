package frozendb

import "github.com/susu-dot-dev/frozenDB/pkg/types"

// File constants
const (
	FILE_EXTENSION = ".fdb"
)

// Database operation constants
const (
	CHECKSUM_INTERVAL = 10000 // Checksum rows inserted every 10,000 complete rows
)

// Re-export control types for public API
type StartControl = types.StartControl
type EndControl = types.EndControl

// Re-export StartControl constants
const (
	START_TRANSACTION = types.START_TRANSACTION
	ROW_CONTINUE      = types.ROW_CONTINUE
	CHECKSUM_ROW      = types.CHECKSUM_ROW
)

// Re-export EndControl constants
var (
	TRANSACTION_COMMIT   = types.TRANSACTION_COMMIT
	ROW_END_CONTROL      = types.ROW_END_CONTROL
	SAVEPOINT_COMMIT     = types.SAVEPOINT_COMMIT
	SAVEPOINT_CONTINUE   = types.SAVEPOINT_CONTINUE
	FULL_ROLLBACK        = types.FULL_ROLLBACK
	CHECKSUM_ROW_CONTROL = types.CHECKSUM_ROW_CONTROL
	NULL_ROW_CONTROL     = types.NULL_ROW_CONTROL
)

// FinderStrategy selects the finder implementation when creating a FrozenDB.
//
// Memory-performance trade-offs:
//   - FinderStrategySimple: O(row_size) fixed memory regardless of DB size; GetIndex O(n),
//     GetTransactionStart/End O(k) where k ≤ 101. Use when DB is large or memory is bounded.
//   - FinderStrategyInMemory: ~40 bytes per row (uuid map + tx boundary maps); GetIndex,
//     GetTransactionStart, GetTransactionEnd all O(1). Use when DB fits in memory and
//     read-heavy workloads need low latency.
//   - FinderStrategyBinarySearch: O(log n) search for time-ordered UUIDv7 keys. Use when
//     keys are chronologically ordered and memory is constrained.
type FinderStrategy string

const (
	FinderStrategySimple       FinderStrategy = "simple"
	FinderStrategyInMemory     FinderStrategy = "inmemory"
	FinderStrategyBinarySearch FinderStrategy = "binary_search"
)
