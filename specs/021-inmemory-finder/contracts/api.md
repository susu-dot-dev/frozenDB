# InMemoryFinder API Specification

## Constructor Functions

### NewInMemoryFinder

Creates a new InMemoryFinder instance with complete database indexing.

**Signature**:
```go
func NewInMemoryFinder(dbFile DBFile, rowSize int32) (*InMemoryFinder, error)
```

**Parameters**:
- `dbFile`: DBFile interface for reading database rows
- `rowSize`: Size of each row in bytes from database header

**Returns**:
- `*InMemoryFinder`: Initialized finder with complete in-memory index
- `error`: InvalidInputError if parameters are invalid, ReadError for initialization failures

**Behavior**:
- Validates input parameters following existing patterns
- Performs full database scan to build complete indexes
- Pre-allocates maps based on estimated database size
- Returns ready-to-use finder with O(1) lookup capability

**Performance**:
- Initialization: O(n) where n = number of rows in database
- Memory allocation: ~40 bytes per database row
- Thread-safe: Concurrent Get* operations after initialization

### NewFrozenDB

Updated FrozenDB constructor requiring explicit finder strategy selection.

**Signature**:
```go
func NewFrozenDB(filename string, mode string, strategy FinderStrategy) (*FrozenDB, error)
```

**Parameters**:
- `filename`: Path to frozenDB database file
- `mode`: Access mode - MODE_READ or MODE_WRITE
- `strategy`: Finder strategy - FinderStrategySimple or FinderStrategyInMemory

**Returns**:
- `*FrozenDB`: Initialized database with specified finder
- `error`: InvalidInputError for invalid strategy parameters

**Behavior**:
- Validates finder strategy against supported constants
- Creates appropriate finder implementation
- Replaces previous signature that had no finder selection
- Returns descriptive error for invalid strategy values

**Migration from previous version**:
- Previous (002): `NewFrozenDB(filename string, mode string)`
- New: `NewFrozenDB(filename, mode, FinderStrategySimple)` or `NewFrozenDB(filename, mode, FinderStrategyInMemory)`
- Example: `NewFrozenDB(filename, MODE_READ, FinderStrategySimple)` for read-only with simple finder
- Example: `NewFrozenDB(filename, MODE_WRITE, FinderStrategyInMemory)` for read-write with in-memory finder

## Finder Interface Implementation

### GetIndex

Returns the index of the first row containing the specified UUID key in O(1) time.

**Signature**:
```go
func (imf *InMemoryFinder) GetIndex(key uuid.UUID) (int64, error)
```

**Parameters**:
- `key`: UUIDv7 key to search for (must not be uuid.Nil)

**Returns**:
- `index`: Zero-based index of the matching DataRow
- `error`: KeyNotFoundError if not found, InvalidInputError for invalid UUID

**Performance**:
- Time Complexity: O(1) via hash map lookup
- Thread Safety: Concurrent reads allowed via RWMutex
- Memory Usage: No additional allocations during lookup

### GetTransactionStart

Returns the index of the first row in the transaction containing the specified index in O(1) time.

**Signature**:
```go
func (imf *InMemoryFinder) GetTransactionStart(index int64) (int64, error)
```

**Parameters**:
- `index`: Index of a row within the transaction (must be DataRow or NullRow)

**Returns**:
- `startIndex`: Index of the row with start_control='T' in the transaction
- `error`: InvalidInputError for invalid indices or checksum rows, CorruptDatabaseError for invalid control bytes

**Performance**:
- Time Complexity: O(1) via hash map lookup
- Thread Safety: Concurrent reads allowed via RWMutex
- Behavior: Identical to SimpleFinder implementation

### GetTransactionEnd

Returns the index of the last row in the transaction containing the specified index in O(1) time.

**Signature**:
```go
func (imf *InMemoryFinder) GetTransactionEnd(index int64) (int64, error)
```

**Parameters**:
- `index`: Index of a row within the transaction (must be DataRow or NullRow)

**Returns**:
- `endIndex`: Index of the row with transaction-ending end_control
- `error`: InvalidInputError for invalid indices, TransactionActiveError if transaction has no ending row, CorruptDatabaseError for malformed transactions

**Performance**:
- Time Complexity: O(1) via hash map lookup
- Thread Safety: Concurrent reads allowed via RWMutex
- Behavior: Handles all transaction-ending control bytes (TC, RE, SC, R0-R9, S0-S9, NR)

### OnRowAdded

Updates InMemoryFinder internal state when new rows are successfully written to the database.

**Signature**:
```go
func (imf *InMemoryFinder) OnRowAdded(index int64, row *RowUnion) error
```

**Parameters**:
- `index`: Index of the newly added row (follows sequential ordering)
- `row`: Complete row data of the newly added row

**Returns**:
- `error`: InvalidInputError if index validation fails, CorruptDatabaseError if row data cannot be parsed

**Behavior**:
- Updates UUID index for DataRows with valid UUIDv7 keys
- Updates transaction boundary maps for all row types based on control bytes
- Performs atomic updates to maintain index consistency
- Called within transaction write lock context

**Thread Safety**:
- Not thread-safe with itself: calls are guaranteed sequential
- Blocks until completion before returning to caller
- Uses internal write lock for map updates

## Integration Points

InMemoryFinder should integrate with existing frozenDB systems but the specific implementation details for how finder names map to implementations should be left to the implementation to determine.


## Performance Characteristics

### Memory Usage

**Formula**: ~40 bytes per database row
- UUID map: ~24 bytes per row
- Transaction boundary maps: ~16 bytes per row

**Example**: 10,000 row database ≈ 400KB additional memory usage

### Operation Performance

**GetIndex**: O(1) average time, <1ms for databases up to 100,000 rows
**GetTransactionStart**: O(1) average time, <1ms regardless of database size
**GetTransactionEnd**: O(1) average time, <1ms regardless of database size
**OnRowAdded**: O(1) update time, negligible overhead per row

### Concurrency

**Read Operations**: Unlimited concurrent reads via RWMutex
**Write Operations**: Sequential OnRowAdded calls within transaction write lock
**No Reader-Writer Blocking**: Reads continue during database writes

## Error Handling

### Error Types

All errors use frozenDB structured error types:

- `InvalidInputError`: Parameter validation failures (invalid UUID, index out of range)
- `KeyNotFoundError`: UUID not found in index
- `CorruptDatabaseError`: Data corruption during initialization or updates
- `TransactionActiveError`: Querying boundaries for active transactions
- `ReadError`: I/O failures during initialization

### Error Messages

Descriptive error messages with context for debugging:

```
"Invalid finder strategy: unknown. Supported strategies: simple, inmemory"
"Key not found: 018f1234-5678-9abc-def0-123456789abc"
"Index 123 out of range for database with 100 rows"
```
