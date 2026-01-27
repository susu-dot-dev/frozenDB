# Data Model: BinarySearchFinder

## New Entities

### BinarySearchFinder
The primary finder implementation that provides O(log n) lookup performance using binary search on UUIDv7 ordered keys while maintaining constant memory usage.

**Attributes**:
- `dbFile`: DBFile interface for reading database rows
- `rowSize`: int32 - size of each row in bytes from header
- `size`: int64 - confirmed file size (updated via OnRowAdded)
- `maxTimestamp`: int64 - maximum timestamp among all complete data and null rows
- `mu`: sync.Mutex - protects size and maxTimestamp for concurrent access

**Validation Rules**:
- dbFile cannot be nil
- rowSize must be between 128 and 65536 bytes
- UUIDv7 keys must be validated using ValidateUUIDv7()
- All operations must be thread-safe

### LogicalIndexMapper
Internal helper that maps between logical indices (used by FuzzyBinarySearch) and physical row indices (accounting for checksum rows).

**State Transitions**:
- Logical index L maps to physical index = L + floor(L / 10000) + 1
- Logical indices include both DataRows and NullRows (both have valid logical indices)
- NullRows are NOT excluded from logical index space - they are part of the searchable logical space
- Physical checksum rows are skipped during mapping (they don't have logical indices)
- Mapping is deterministic and reversible
- The mapping is a simple mathematical operation - no special handling needed to exclude NullRows

## Changes to Existing Structures

### Finder Strategy Constants
Addition of `FinderStrategyBinarySearch` constant to the existing finder strategy pattern alongside `FinderStrategySimple` and `FinderStrategyInMemory`.

### Finder Factory Pattern
The existing finder factory in `frozendb/finder.go` will be extended to support `FinderStrategyBinarySearch`, returning a new `BinarySearchFinder` instance.

### DataRow Validation
The `DataRow.Validate()` method (via `DataRowPayload.Validate()`) will be enhanced to reject UUIDs where the non-timestamp part (bytes 7, 9-15) are all zeros. This pattern indicates a NullRow UUID, which is invalid for DataRows. The validation must check that at least one byte in positions 7, 9-15 is non-zero.

## Validation Rules

### UUIDv7 Key Validation
- All keys must be valid UUIDv7 format
- Keys must follow timestamp ordering within skew tolerance
- Invalid keys result in InvalidInputError
- DataRow UUIDs must NOT have all-zero non-timestamp parts (bytes 7, 9-15) - this pattern indicates NullRow UUIDs which are invalid for DataRows
- GetIndex() search keys must NOT be NullRow UUIDs (detected by all-zero non-timestamp parts) - these are rejected early before binary search

### Index Bounds Validation
- Logical indices must be within [0, numLogicalRows)
- Physical indices must be within [0, totalPhysicalRows)
- Out-of-bounds access results in InvalidInputError

### Checksum Row Exclusion
- Checksum rows are never included in logical index space
- Physical checksum rows are transparently skipped
- Corrupted checksum rows result in CorruptDatabaseError

## State Transitions

### Initialization
1. Validate dbFile and rowSize parameters
2. Initialize size from current dbFile.Size()
3. Scan all existing rows to find maxTimestamp
4. Return initialized BinarySearchFinder instance

### GetIndex Operation
1. Validate input UUIDv7 key
2. Reject NullRow UUIDs early by checking if non-timestamp part (bytes 7, 9-15) are all zeros
3. Calculate number of logical data rows (DataRows and NullRows, excluding checksum rows)
4. Use FuzzyBinarySearch with logical index mapping (logical indices include both DataRows and NullRows)
5. Map found logical index back to physical row index
6. Return physical row index or KeyNotFoundError

### OnRowAdded Operation
1. Validate row parameter and index consistency
2. Update size by adding rowSize
3. Update maxTimestamp if row is complete DataRow or NullRow
4. Return success or appropriate error

## Error Condition Mappings

### KeyNotFoundError
- Target UUID not found in database
- Search completes without finding exact match
- Preserved from SimpleFinder behavior

### InvalidInputError
- Nil or invalid UUIDv7 key provided
- NullRow UUID used as search key in GetIndex() (detected by all-zero non-timestamp parts)
- DataRow UUID with all-zero non-timestamp parts (bytes 7, 9-15) in Validate()
- Invalid logical/physical index bounds
- Invalid rowSize parameter during initialization

### CorruptDatabaseError
- Checksum row corruption detected
- Row parsing failures during search
- Data integrity violations

### ReadError
- I/O failures during row reading
- Database file access errors
- Wrapped from underlying dbFile operations

## Data Flow Relationships

### BinarySearchFinder → FuzzyBinarySearch
- Provides logical index mapping function
- Supplies time skew from database header
- Returns UUIDv7 keys for DataRows and NullRows (both have valid logical indices)

### BinarySearchFinder → DBFile
- Reads physical rows at calculated indices
- Handles checksum row skipping transparently
- Maintains same I/O patterns as SimpleFinder

### LogicalIndexMapper → Physical Rows
- Maps logical search indices to physical file positions
- Logical indices include both DataRows and NullRows (both have valid logical indices)
- Skips checksum rows automatically (checksum rows don't have logical indices)
- Ensures search space continuity for binary search algorithm
- Mapping is simple math: physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1