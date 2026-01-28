package frozendb

import (
	internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

// FinderStrategy selects the finder implementation when creating a FrozenDB.
//
// Memory-performance trade-offs:
//   - FinderStrategySimple: O(row_size) fixed memory regardless of DB size; GetIndex O(n),
//     GetTransactionStart/End O(k) where k ≤ 101. Use when DB is large or memory is bounded.
//   - FinderStrategyInMemory: ~40 bytes per row (uuid map + tx boundary maps); GetIndex,
//     GetTransactionStart, GetTransactionEnd all O(1). Use when DB fits in memory and
//     read-heavy workloads need low latency.
//   - FinderStrategyBinarySearch: Optimized for time-ordered UUID lookups with binary search.
//     GetIndex O(log n) with time-based optimizations for chronologically ordered keys.
type FinderStrategy = internal.FinderStrategy

const (
	// FinderStrategySimple uses fixed memory (one row buffer) regardless of database size.
	// GetIndex is O(n), GetTransactionStart/End are O(k) where k ≤ 101.
	// Best for large databases or memory-constrained environments.
	FinderStrategySimple = internal.FinderStrategySimple

	// FinderStrategyInMemory uses ~40 bytes per row to maintain in-memory indices.
	// All operations (GetIndex, GetTransactionStart, GetTransactionEnd) are O(1).
	// Best for databases that fit in memory with read-heavy workloads.
	FinderStrategyInMemory = internal.FinderStrategyInMemory

	// FinderStrategyBinarySearch uses binary search for time-ordered UUID lookups.
	// GetIndex is O(log n) with time-based optimizations.
	// Best for chronologically ordered keys (UUIDv7) with frequent lookups.
	FinderStrategyBinarySearch = internal.FinderStrategyBinarySearch
)
