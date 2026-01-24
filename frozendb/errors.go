package frozendb

import "fmt"

// FrozenDBError is the base error type for all frozenDB operations.
// All error types must embed this struct to maintain constitutional requirements.
type FrozenDBError struct {
	Code    string // Error code for programmatic handling
	Message string // Human-readable error message
	Err     error  // Underlying error (optional)
}

// Error returns the formatted error message.
func (e *FrozenDBError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for error chaining.
func (e *FrozenDBError) Unwrap() error {
	return e.Err
}

// NewInvalidInputError creates a new InvalidInputError.
func NewInvalidInputError(message string, err error) *InvalidInputError {
	return &InvalidInputError{
		FrozenDBError: FrozenDBError{
			Code:    "invalid_input",
			Message: message,
			Err:     err,
		},
	}
}

// NewPathError creates a new PathError.
func NewPathError(message string, err error) *PathError {
	return &PathError{
		FrozenDBError: FrozenDBError{
			Code:    "path_error",
			Message: message,
			Err:     err,
		},
	}
}

// NewWriteError creates a new WriteError.
func NewWriteError(message string, err error) *WriteError {
	return &WriteError{
		FrozenDBError: FrozenDBError{
			Code:    "write_error",
			Message: message,
			Err:     err,
		},
	}
}

// NewCorruptDatabaseError creates a new CorruptDatabaseError.
func NewCorruptDatabaseError(message string, err error) *CorruptDatabaseError {
	return &CorruptDatabaseError{
		FrozenDBError: FrozenDBError{
			Code:    "corrupt_database",
			Message: message,
			Err:     err,
		},
	}
}

// NewInvalidActionError creates a new InvalidActionError.
func NewInvalidActionError(message string, err error) *InvalidActionError {
	return &InvalidActionError{
		FrozenDBError: FrozenDBError{
			Code:    "invalid_action",
			Message: message,
			Err:     err,
		},
	}
}

// NewKeyOrderingError creates a new KeyOrderingError.
func NewKeyOrderingError(message string, err error) *KeyOrderingError {
	return &KeyOrderingError{
		FrozenDBError: FrozenDBError{
			Code:    "key_ordering",
			Message: message,
			Err:     err,
		},
	}
}

// InvalidInputError is returned for input validation failures.
// Used for: empty path, invalid parameter ranges, wrong file extension.
type InvalidInputError struct {
	FrozenDBError
}

// InvalidActionError is returned for invalid state transitions and actions.
// Used for: calling methods from wrong state, preventing invalid operations.
type InvalidActionError struct {
	FrozenDBError
}

// PathError is returned for filesystem path issues.
// Used for: parent directory missing, path not writable, file already exists.
type PathError struct {
	FrozenDBError
}

// WriteError is returned for file operation failures.
// Used for: sudo context issues, header write failures, attribute setting failures.
type WriteError struct {
	FrozenDBError
}

// CorruptDatabaseError is returned for database corruption detection.
// Used for: header validation failures, malformed file format, invalid field values.
type CorruptDatabaseError struct {
	FrozenDBError
}

// KeyOrderingError is returned when UUID timestamp ordering constraints are violated.
// Used for: AddRow timestamp validation failures when new_timestamp + skew_ms <= max_timestamp.
type KeyOrderingError struct {
	FrozenDBError
}

// NewTombstonedError creates a new TombstonedError.
func NewTombstonedError(message string, err error) *TombstonedError {
	return &TombstonedError{
		FrozenDBError: FrozenDBError{
			Code:    "tombstoned",
			Message: message,
			Err:     err,
		},
	}
}

// NewReadError creates a new ReadError.
func NewReadError(message string, err error) *ReadError {
	return &ReadError{
		FrozenDBError: FrozenDBError{
			Code:    "read_error",
			Message: message,
			Err:     err,
		},
	}
}

// TombstonedError is returned when operations are attempted on a tombstoned FileManager.
type TombstonedError struct {
	FrozenDBError
}

// ReadError is returned for disk read operation failures.
// Used for: file I/O errors, read permission issues, hardware read failures.
type ReadError struct {
	FrozenDBError
}

// NewKeyNotFoundError creates a new KeyNotFoundError.
func NewKeyNotFoundError(message string, err error) *KeyNotFoundError {
	return &KeyNotFoundError{
		FrozenDBError: FrozenDBError{
			Code:    "key_not_found",
			Message: message,
			Err:     err,
		},
	}
}

// KeyNotFoundError is returned when a UUID key cannot be found in the database.
// Used for: GetIndex() operations when the specified key does not exist.
type KeyNotFoundError struct {
	FrozenDBError
}

// NewTransactionActiveError creates a new TransactionActiveError.
func NewTransactionActiveError(message string, err error) *TransactionActiveError {
	return &TransactionActiveError{
		FrozenDBError: FrozenDBError{
			Code:    "transaction_active",
			Message: message,
			Err:     err,
		},
	}
}

// TransactionActiveError is returned when a transaction is still open with no ending row.
// Used for: GetTransactionEnd() when the transaction has not been committed or rolled back.
type TransactionActiveError struct {
	FrozenDBError
}
