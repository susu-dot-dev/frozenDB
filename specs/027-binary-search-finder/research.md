# Research: BinarySearchFinder Implementation

## Technical Context Analysis

### Current Architecture
- **SimpleFinder**: O(n) linear scan, constant memory, reference implementation
- **InMemoryFinder**: O(1) lookup, O(n) memory, builds full index in memory
- **FuzzyBinarySearch**: Already exists as a function that can search UUIDv7 arrays with time skew tolerance

### Key Technical Challenge: Checksum Row Mapping

The primary technical challenge is handling checksum rows that occur every 10,000 complete data/null rows. Since FuzzyBinarySearch expects a logically contiguous array of keys, but the physical database has checksum rows interspersed, we need a mapping function.

**Solution Approach**:
- **Logical Index**: Contiguous indices [0, 1, 2, ...] that FuzzyBinarySearch operates on, including both DataRows and NullRows (both have valid logical indices)
- **Physical Index**: Actual row indices in the database file that include checksum rows
- **Mapping Function**: Convert between logical and physical indices accounting for checksum rows. NullRows are NOT excluded from logical index space - they have valid logical indices. The mapping is a simple mathematical operation: `physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1`

### Mapping Algorithm

For a given logical index `L`:
```
physicalIndex = L + floor(L / 10000) + 1
```

For a given physical index `P`:
```
if P is checksum row: skip
logicalIndex = P - floor(P / 10001) - 1
```

### FuzzyBinarySearch Integration

The FuzzyBinarySearch function requires:
- Target UUIDv7 key
- Time skew parameter (from database header)  
- Number of keys in logical array
- A `get` function that returns UUID at a logical index

Our implementation will:
1. Map logical indices to physical row indices
2. Read physical rows, skipping checksum rows
3. Return UUIDv7 keys for DataRows and NullRows (both have valid logical indices)

### Performance Characteristics

**BinarySearchFinder**:
- **Time Complexity**: O(log n) for GetIndex where n = number of DataRows
- **Space Complexity**: O(1) constant memory (same as SimpleFinder)
- **Initialization**: O(n) scan to find max timestamp (same as SimpleFinder)

**Comparison with SimpleFinder**:
- SimpleFinder: O(n) time, O(1) space
- BinarySearchFinder: O(log n) time, O(1) space
- InMemoryFinder: O(1) time, O(n) space

### Thread Safety

Must follow the same thread safety model as SimpleFinder:
- Mutex protection for size and maxTimestamp fields
- Concurrent reads allowed during writes
- GetIndex operations must be thread-safe

### Error Handling

Must maintain consistency with existing error patterns:
- KeyNotFoundError for missing keys
- InvalidInputError for invalid parameters
- CorruptDatabaseError for data integrity issues
- ReadError for I/O failures

### UUIDv7 Validation

Must validate:
- Input keys are valid UUIDv7
- Keys follow timestamp ordering within skew tolerance
- Handle edge cases for timestamp extraction
- DataRow UUIDs must NOT have all-zero non-timestamp parts (bytes 7, 9-15) - this pattern indicates NullRow UUIDs which are invalid for DataRows
- GetIndex() search keys must NOT be NullRow UUIDs (detected by all-zero non-timestamp parts) - these are rejected early before binary search

## Decision: Exact Code Copy vs. DRY

Per user requirement: "The BinarySearchFinder should generally use exactly the same code as the SimpleFinder. The only difference is how GetIndex operates."

**Decision**: Copy SimpleFinder code exactly and modify only GetIndex method. This:
- Ensures behavioral consistency with reference implementation
- Simplifies maintenance and understanding
- Avoids complex abstraction layers
- Makes differences explicit and localized

## Key Technical Unknowns Resolved

1. **Checksum Row Handling**: Use logical-to-physical index mapping
2. **FuzzyBinarySearch Integration**: Wrap with adapter function for logical index access
3. **Thread Safety**: Follow SimpleFinder pattern exactly
4. **Error Handling**: Use existing error types and patterns
5. **UUIDv7 Validation**: Reuse existing validation functions

## Implementation Approach

1. Copy SimpleFinder code to binary_search_finder.go
2. Modify GetIndex to use FuzzyBinarySearch with logical index mapping
3. Add FinderStrategyBinarySearch constant
4. Update finder factory to support new strategy
5. Create comprehensive spec tests
6. Ensure conformance test compatibility