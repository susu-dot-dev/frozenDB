# Public API Contract: Project Structure Refactor

**Purpose**: List of public API functions, types, and constants after the refactor  
**Created**: 2026-01-27  
**Feature**: Project Structure Refactor

## Public Functions

### Database Lifecycle
- `Create(config CreateConfig) error`
- `NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error)`
- `(db *FrozenDB) Close() error`
- `(db *FrozenDB) Get(key uuid.UUID, value any) error`
- `(db *FrozenDB) BeginTx() (*Transaction, error)`
- `(db *FrozenDB) GetActiveTx() *Transaction`

### Transaction Operations
- `(tx *Transaction) Begin() error`
- `(tx *Transaction) AddRow(key uuid.UUID, value any) error`
- `(tx *Transaction) Commit() error`
- `(tx *Transaction) Rollback(savepoint int) error`
- `(tx *Transaction) GetRows() []DataRow`

## Public Types

- `FrozenDB` struct
- `Transaction` struct (with exported `Header *Header` field)
- `CreateConfig` struct
- `DataRow` struct (with exported `StartControl` and `EndControl` fields)
- `Header` struct
- `StartControl` type
- `EndControl` type
- `FinderStrategy` type

## Public Constants

### Access Modes
- `MODE_READ = "read"`
- `MODE_WRITE = "write"`

### Finder Strategies
- `FinderStrategySimple FinderStrategy = "simple"`
- `FinderStrategyInMemory FinderStrategy = "inmemory"`
- `FinderStrategyBinarySearch FinderStrategy = "binary_search"`

### StartControl Constants
- `START_TRANSACTION StartControl = 'T'`
- `ROW_CONTINUE StartControl = 'R'`
- `CHECKSUM_ROW StartControl = 'C'`

### EndControl Constants
- `TRANSACTION_COMMIT EndControl = {'T', 'C'}`
- `ROW_END_CONTROL EndControl = {'R', 'E'}`
- `SAVEPOINT_COMMIT EndControl = {'S', 'C'}`
- `SAVEPOINT_CONTINUE EndControl = {'S', 'E'}`
- `FULL_ROLLBACK EndControl = {'R', '0'}`
- `NULL_ROW_CONTROL EndControl = {'N', 'R'}`
- `CHECKSUM_ROW_CONTROL EndControl = {'C', 'S'}`

## Public Error Types

- `FrozenDBError` struct
- `InvalidInputError` struct
- `InvalidActionError` struct
- `PathError` struct
- `WriteError` struct
- `CorruptDatabaseError` struct
- `KeyOrderingError` struct
- `TombstonedError` struct
- `ReadError` struct
- `KeyNotFoundError` struct
- `TransactionActiveError` struct
- `InvalidDataError` struct

### Error Constructors
- `NewInvalidInputError(message string, err error) *InvalidInputError`
- `NewInvalidActionError(message string, err error) *InvalidActionError`
- `NewPathError(message string, err error) *PathError`
- `NewWriteError(message string, err error) *WriteError`
- `NewCorruptDatabaseError(message string, err error) *CorruptDatabaseError`
- `NewKeyOrderingError(message string, err error) *KeyOrderingError`
- `NewTombstonedError(message string, err error) *TombstonedError`
- `NewReadError(message string, err error) *ReadError`
- `NewKeyNotFoundError(message string, err error) *KeyNotFoundError`
- `NewTransactionActiveError(message string, err error) *TransactionActiveError`
- `NewInvalidDataError(message string, err error) *InvalidDataError`

## What is NOT Public API

The following are internal implementation details:

- `Finder` interface
- `SimpleFinder`, `InMemoryFinder`, `BinarySearchFinder` implementations
- `NullRow`, `ChecksumRow`, `PartialDataRow` types
- `RowUnion` type
- `DBFile` interface
- `FileManager` type
- `ValidateUUIDv7()`, `ExtractUUIDv7Timestamp()`, `NewUUIDv7()` functions
- `ROW_START`, `ROW_END`, `NULL_BYTE` constants
- All finder constructors
- File validation helpers
- Transaction recovery functions

## Import Path

```go
import "github.com/susu-dot-dev/frozenDB/pkg/frozendb"
```
