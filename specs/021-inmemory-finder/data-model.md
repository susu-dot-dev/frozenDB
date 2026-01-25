# InMemoryFinder Data Model

## Entity Definitions

### InMemoryFinder

**Purpose**: High-performance finder implementation that maintains complete UUID->index mapping in memory for O(1) lookup operations.

**Attributes**:
- `uuidIndex map[uuid.UUID]int64` - Maps UUID keys to their row indices for O(1) GetIndex operations
- `transactionStart map[int64]int64` - Maps row indices to their transaction start boundaries for O(1) GetTransactionStart
- `transactionEnd map[int64]int64` - Maps row indices to their transaction end boundaries for O(1) GetTransactionEnd
- `mu sync.RWMutex` - Protects concurrent access to all maps
- `dbFile DBFile` - Database file interface for initialization reads
- `rowSize int32` - Row size from database header for index calculations
- `size int64` - Confirmed file size for validation

**State Transitions**:
- **Initialization**: Empty maps → Full database scan → Complete indexes
- **Row Addition**: New row → Map updates → Available for queries
- **Concurrent Access**: Multiple readers → RWMutex → Query execution

**Validation Rules**:
- UUID keys must be valid UUIDv7 (handled by existing DataRow validation)
- Row indices must follow sequential ordering (handled by transaction system)
- Transaction boundaries must be consistent with control bytes

### Finder Strategy Selection

**Purpose**: Type-safe selection between available finder implementations.

**Type Definition**:
```go
type FinderStrategy string

const (
    FinderStrategySimple   FinderStrategy = "simple"
    FinderStrategyInMemory FinderStrategy = "inmemory"
)
```

**Supported Values**:
- `FinderStrategySimple` - Use SimpleFinder (fixed memory, O(n) performance)
- `FinderStrategyInMemory` - Use InMemoryFinder (scales with database size, O(1) performance)

**Validation Rules**:
- Strategy must be one of the predefined constants
- Invalid strategies return InvalidInputError with descriptive message
- Implementation determines how to map strategy values to concrete implementations

## Data Relationships

### Finder Interface Compliance

InMemoryFinder implements the Finder interface with identical behavior to SimpleFinder:

1. **GetIndex Operations**:
   - Input: UUIDv7 key
   - Output: Row index or KeyNotFoundError
   - Lookup: O(1) via uuidIndex map
   - Thread-safety: Concurrent reads allowed via RWMutex

2. **Transaction Boundary Operations**:
   - GetTransactionStart: O(1) via transactionStart map
   - GetTransactionEnd: O(1) via transactionEnd map
   - Validation: Same error handling as SimpleFinder

3. **OnRowAdded Integration**:
   - Sequential index validation inherited from transaction system
   - Atomic map updates for all three internal maps
   - Thread-safe execution within transaction write lock

### Memory Usage Scaling

**Memory Components**:
- UUID map: ~24 bytes per database row
- Transaction boundary maps: ~16 bytes per database row
- Total: ~40 bytes per row

**Scaling Behavior**:
- Linear scaling with database size
- Predictable memory consumption
- No additional overhead for concurrent operations

## New Validation Rules

### Finder Strategy Selection

**Rule**: NewFrozenDB(filename, mode, strategy) must validate mode and strategy during FrozenDB creation.

**Implementation**:
- Mode: MODE_READ or MODE_WRITE (validated by NewDBFile)
- Strategy: validate against FinderStrategySimple, FinderStrategyInMemory
- Return InvalidInputError for unknown strategies with descriptive message
- All three parameters required: filename, mode, strategy

### Index Consistency

**Rule**: InMemoryFinder internal maps must remain consistent with database state.

**Implementation**:
- All map updates occur within OnRowAdded transaction write lock context
- Atomic updates maintain consistency between UUID and transaction boundary maps
- Sequential index validation prevents gaps in index mappings

### Memory Usage Documentation

**Rule**: Memory usage characteristics must be clearly documented.

**Implementation**:
- Provide usage formula: ~40 bytes per database row
- Document trade-offs vs. SimpleFinder
- Include performance characteristics and suitable use cases

## Error Condition Mappings

### New Error Scenarios

**Invalid Finder Strategy**:
- Condition: Unknown or invalid finder strategy parameter
- Error Type: InvalidInputError
- Message: "Invalid finder strategy: {strategy}. Supported strategies: simple, inmemory"

**Index Inconsistency**:
- Condition: Internal maps become inconsistent during updates
- Error Type: CorruptDatabaseError
- Message: "Index inconsistency detected in InMemoryFinder"

**Memory Allocation Failure**:
- Condition: Unable to allocate memory for internal maps
- Error Type: ReadError (mapped to allocation failure)
- Message: "Memory allocation failed for InMemoryFinder initialization"

### Existing Error Type Reuse

- **KeyNotFoundError**: When UUID not found in uuidIndex map
- **InvalidInputError**: For invalid parameters in Finder methods
- **CorruptDatabaseError**: For data corruption detected during initialization
- **TransactionActiveError**: When querying transaction boundaries for active transactions

## Data Flow Relationships

### Initialization Flow

1. **Database Open** → File size validation → Map pre-allocation
2. **Database Scan** → Row processing → Map population
3. **Index Build** → Complete maps → Ready for operations

### Query Flow

1. **GetIndex Call** → Read lock → UUID lookup → Result return
2. **Transaction Call** → Read lock → Boundary lookup → Result return
3. **Concurrent Queries** → Multiple read locks → Parallel execution

### Update Flow

1. **Row Write** → Transaction commit → OnRowAdded call
2. **Index Update** → Write lock → Map updates → Lock release
3. **State Update** → Consistency validation → Ready for queries