package frozendb

import (
	internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

// Transaction represents a database transaction.
// Supports adding rows and committing/rolling back changes.
//
// This type is re-exported from the internal implementation, but excludes
// internal methods that expose internal types (GetEmptyRow, GetRows).
type Transaction = internal.Transaction
