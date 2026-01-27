# BinarySearchFinder API Specification

## Finder Strategy Integration

### FinderStrategyBinarySearch
```go
const FinderStrategyBinarySearch FinderStrategy = "binary_search"
```

**Description**: Strategy constant for selecting BinarySearchFinder in finder factory methods.

**Usage**:
```go
db, err := frozendb.OpenDatabase(file, frozendb.FinderStrategyBinarySearch)
```

## BinarySearchFinder Constructor

### NewBinarySearchFinder
```go
func NewBinarySearchFinder(dbFile DBFile, rowSize int32) (*BinarySearchFinder, error)
```

**Description**: Creates a new BinarySearchFinder instance with the specified database file and row size.

**Parameters**:
- `dbFile`: DBFile interface for reading database rows
- `rowSize`: Size of each row in bytes (must be between 128 and 65536)

**Returns**:
- `*BinarySearchFinder`: Initialized finder instance
- `error`: InvalidInputError if parameters are invalid

**Performance**: O(n) initialization to scan for max timestamp

## Finder Interface Implementation

### GetIndex
```go
func (bsf *BinarySearchFinder) GetIndex(key uuid.UUID) (int64, error)
```

**Description**: Returns the index of the first row containing the specified UUID key using binary search.

**Algorithm**:
1. Validate input UUIDv7 key
2. Reject NullRow UUIDs early by checking if non-timestamp part (bytes 7, 9-15) are all zeros
3. Calculate number of logical data rows (DataRows and NullRows, excluding checksum rows)
4. Use FuzzyBinarySearch with logical index mapping
5. Map found logical index back to physical row index
6. Return physical row index or KeyNotFoundError

**Parameters**:
- `key`: UUIDv7 key to search for

**Returns**:
- `int64`: Physical row index containing the key
- `error`: KeyNotFoundError if not found, InvalidInputError for invalid key or NullRow UUID

**Performance**: O(log n) time where n is number of DataRows
**Thread Safety**: Safe for concurrent read access

### GetTransactionStart
```go
func (bsf *BinarySearchFinder) GetTransactionStart(index int64) (int64, error)
```

**Description**: Returns the index of the first row in the transaction containing the specified index.

**Implementation**: Identical to SimpleFinder implementation

**Parameters**:
- `index`: Physical row index within a transaction

**Returns**:
- `int64`: Index of transaction start row
- `error`: InvalidInputError for invalid index, CorruptDatabaseError if no start found

**Performance**: O(k) where k is distance to start (max ~101)

### GetTransactionEnd
```go
func (bsf *BinarySearchFinder) GetTransactionEnd(index int64) (int64, error)
```

**Description**: Returns the index of the last row in the transaction containing the specified index.

**Implementation**: Identical to SimpleFinder implementation

**Parameters**:
- `index`: Physical row index within a transaction

**Returns**:
- `int64`: Index of transaction end row
- `error`: InvalidInputError for invalid index, TransactionActiveError if no end found

**Performance**: O(k) where k is distance to end (max ~101)

### OnRowAdded
```go
func (bsf *BinarySearchFinder) OnRowAdded(index int64, row *RowUnion) error
```

**Description**: Updates the finder's internal state when a new row is added to the database.

**Implementation**: Identical to SimpleFinder implementation

**Parameters**:
- `index`: Expected row index for the new row
- `row`: RowUnion containing the new row data

**Returns**:
- `error`: InvalidInputError for parameter validation failures

**Performance**: O(1) constant time

### MaxTimestamp
```go
func (bsf *BinarySearchFinder) MaxTimestamp() int64
```

**Description**: Returns the maximum timestamp among all complete data and null rows.

**Implementation**: Identical to SimpleFinder implementation

**Returns**:
- `int64`: Maximum timestamp value

**Performance**: O(1) time

## Internal Helper Functions

### logicalToPhysicalIndex
```go
func logicalToPhysicalIndex(logicalIndex int64) int64
```

**Description**: Converts a logical index (used by FuzzyBinarySearch) to a physical row index in the database file.

**Algorithm**: `physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1`

**Important Notes**:
- Logical indices include both DataRows and NullRows (both have valid logical indices)
- NullRows are NOT excluded from logical index space - they have valid logical indices
- The mapping is a simple mathematical operation that accounts for checksum rows inserted every 10,000 data/null rows
- No special handling is needed to exclude NullRows - they are part of the searchable logical space

**Parameters**:
- `logicalIndex`: Index in the logical contiguous array (includes DataRows and NullRows)

**Returns**:
- `int64`: Physical row index accounting for checksum rows

### getLogicalKey
```go
func getLogicalKey(logicalIndex int64) (uuid.UUID, error)
```

**Description**: Adapter function for FuzzyBinarySearch that returns the UUID key at a logical index.

**Algorithm**:
1. Convert logical index to physical index
2. Read row at physical index
3. Return UUID if DataRow or NullRow, skip ChecksumRows

**Parameters**:
- `logicalIndex`: Index in the logical contiguous array (includes DataRows and NullRows)

**Returns**:
- `uuid.UUID`: UUIDv7 key at the logical index (from DataRow or NullRow)
- `error`: ReadError for I/O failures, CorruptDatabaseError for parsing failures

## Usage Examples

### Basic Usage
```go
// Create database with BinarySearchFinder
db, err := frozendb.OpenDatabase(file, frozendb.FinderStrategyBinarySearch)
if err != nil {
    return err
}

// Get index for a key
index, err := db.GetIndex(key)
if err != nil {
    return err
}

// Get transaction boundaries
start, err := db.GetTransactionStart(index)
end, err := db.GetTransactionEnd(index)
```

### Performance Comparison
```go
// BinarySearchFinder: O(log n) lookup, O(1) memory
index, err := binarySearchFinder.GetIndex(key)

// SimpleFinder: O(n) lookup, O(1) memory  
index, err := simpleFinder.GetIndex(key)

// InMemoryFinder: O(1) lookup, O(n) memory
index, err := inMemoryFinder.GetIndex(key)
```

## DataRow Validation Requirements

### DataRow.Validate() UUID Validation

The `DataRow.Validate()` method MUST reject UUIDs where the non-timestamp part (bytes 7, 9-15) are all zeros. This pattern indicates a NullRow UUID, which is invalid for DataRows.

**Validation Rule**:
- A DataRow UUID must have at least one non-zero byte in positions 7, 9-15
- UUIDs with all zeros in bytes 7, 9-15 (the random component) must be rejected with `InvalidInputError`
- This ensures DataRows cannot be created with NullRow UUID patterns

**Implementation Note**: This validation should be added to `DataRowPayload.Validate()` method in `frozendb/data_row.go`.

## Error Handling

### KeyNotFoundError
Returned when the target UUID key is not found in the database after complete search.

### InvalidInputError
Returned for invalid parameters such as:
- Nil or invalid UUIDv7 keys
- NullRow UUIDs (detected by all-zero non-timestamp part bytes 7, 9-15) used as search keys
- Invalid rowSize during initialization
- Out-of-bounds indices

### CorruptDatabaseError
Returned when database corruption is detected during row reading or parsing.

### ReadError
Returned for I/O failures during database file access operations.

## Performance Characteristics

| Operation | Time Complexity | Space Complexity | Notes |
|-----------|----------------|------------------|--------|
| GetIndex | O(log n) | O(1) | n = number of DataRows |
| GetTransactionStart | O(k) | O(1) | k ≤ 101, same as SimpleFinder |
| GetTransactionEnd | O(k) | O(1) | k ≤ 101, same as SimpleFinder |
| OnRowAdded | O(1) | O(1) | Constant time update |
| MaxTimestamp | O(1) | O(1) | Cached value access |

## Thread Safety

BinarySearchFinder provides the same thread safety guarantees as SimpleFinder:
- All Get* methods are safe for concurrent read access
- OnRowAdded is called within transaction write lock context
- Internal state protected by mutex for concurrent access
- No additional locking required for read operations