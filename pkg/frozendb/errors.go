package frozendb

import (
	internal "github.com/susu-dot-dev/frozenDB/internal/frozendb"
)

// FrozenDBError is the base error type for all frozenDB operations.
// All error types embed this struct to maintain consistent error handling.
type FrozenDBError = internal.FrozenDBError

// InvalidInputError is returned for input validation failures.
// Used for: empty path, invalid parameter ranges, wrong file extension.
type InvalidInputError = internal.InvalidInputError

// InvalidActionError is returned for invalid state transitions and actions.
// Used for: calling methods from wrong state, preventing invalid operations.
type InvalidActionError = internal.InvalidActionError

// PathError is returned for filesystem path issues.
// Used for: parent directory missing, path not writable, file already exists.
type PathError = internal.PathError

// WriteError is returned for file operation failures.
// Used for: sudo context issues, header write failures, attribute setting failures.
type WriteError = internal.WriteError

// CorruptDatabaseError is returned for database corruption detection.
// Used for: header validation failures, malformed file format, invalid field values.
type CorruptDatabaseError = internal.CorruptDatabaseError

// KeyOrderingError is returned when UUID timestamp ordering constraints are violated.
// Used for: AddRow timestamp validation failures when new_timestamp + skew_ms <= max_timestamp.
type KeyOrderingError = internal.KeyOrderingError

// TombstonedError is returned when operations are attempted on a tombstoned FileManager.
type TombstonedError = internal.TombstonedError

// ReadError is returned for disk read operation failures.
// Used for: file I/O errors, read permission issues, hardware read failures.
type ReadError = internal.ReadError

// KeyNotFoundError is returned when a UUID key cannot be found in the database.
// Used for: GetIndex() operations when the specified key does not exist.
type KeyNotFoundError = internal.KeyNotFoundError

// TransactionActiveError is returned when a transaction is still open with no ending row.
// Used for: GetTransactionEnd() when the transaction has not been committed or rolled back.
type TransactionActiveError = internal.TransactionActiveError

// InvalidDataError is returned for JSON data that cannot be unmarshaled.
// Used for: JSON syntax errors, type mismatches, malformed data in stored values.
type InvalidDataError = internal.InvalidDataError

// Error constructor functions

// NewInvalidInputError creates a new InvalidInputError.
func NewInvalidInputError(message string, err error) *InvalidInputError {
	return internal.NewInvalidInputError(message, err)
}

// NewInvalidActionError creates a new InvalidActionError.
func NewInvalidActionError(message string, err error) *InvalidActionError {
	return internal.NewInvalidActionError(message, err)
}

// NewPathError creates a new PathError.
func NewPathError(message string, err error) *PathError {
	return internal.NewPathError(message, err)
}

// NewWriteError creates a new WriteError.
func NewWriteError(message string, err error) *WriteError {
	return internal.NewWriteError(message, err)
}

// NewCorruptDatabaseError creates a new CorruptDatabaseError.
func NewCorruptDatabaseError(message string, err error) *CorruptDatabaseError {
	return internal.NewCorruptDatabaseError(message, err)
}

// NewKeyOrderingError creates a new KeyOrderingError.
func NewKeyOrderingError(message string, err error) *KeyOrderingError {
	return internal.NewKeyOrderingError(message, err)
}

// NewTombstonedError creates a new TombstonedError.
func NewTombstonedError(message string, err error) *TombstonedError {
	return internal.NewTombstonedError(message, err)
}

// NewReadError creates a new ReadError.
func NewReadError(message string, err error) *ReadError {
	return internal.NewReadError(message, err)
}

// NewKeyNotFoundError creates a new KeyNotFoundError.
func NewKeyNotFoundError(message string, err error) *KeyNotFoundError {
	return internal.NewKeyNotFoundError(message, err)
}

// NewTransactionActiveError creates a new TransactionActiveError.
func NewTransactionActiveError(message string, err error) *TransactionActiveError {
	return internal.NewTransactionActiveError(message, err)
}

// NewInvalidDataError creates a new InvalidDataError.
func NewInvalidDataError(message string, err error) *InvalidDataError {
	return internal.NewInvalidDataError(message, err)
}
