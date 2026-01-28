package frozendb

import "github.com/susu-dot-dev/frozenDB/pkg/types"

// Re-export error types for backward compatibility
type FrozenDBError = types.FrozenDBError
type InvalidInputError = types.InvalidInputError
type InvalidActionError = types.InvalidActionError
type PathError = types.PathError
type WriteError = types.WriteError
type CorruptDatabaseError = types.CorruptDatabaseError
type KeyOrderingError = types.KeyOrderingError
type TombstonedError = types.TombstonedError
type ReadError = types.ReadError
type KeyNotFoundError = types.KeyNotFoundError
type TransactionActiveError = types.TransactionActiveError
type InvalidDataError = types.InvalidDataError

// Re-export error constructors
var NewInvalidInputError = types.NewInvalidInputError
var NewPathError = types.NewPathError
var NewWriteError = types.NewWriteError
var NewCorruptDatabaseError = types.NewCorruptDatabaseError
var NewInvalidActionError = types.NewInvalidActionError
var NewKeyOrderingError = types.NewKeyOrderingError
var NewTombstonedError = types.NewTombstonedError
var NewReadError = types.NewReadError
var NewKeyNotFoundError = types.NewKeyNotFoundError
var NewTransactionActiveError = types.NewTransactionActiveError
var NewInvalidDataError = types.NewInvalidDataError
