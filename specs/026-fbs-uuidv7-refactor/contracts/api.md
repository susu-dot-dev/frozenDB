# API Specification: FBS UUIDv7 Refactor

## Function Specification

### FuzzyBinarySearch

**Signature**:
```go
func FuzzyBinarySearch(target uuid.UUID, skewMs int64, numKeys int64, get func(int64) (uuid.UUID, error)) (int64, error)
```

**Parameters**:
- `target` (uuid.UUID): The UUIDv7 key to search for. Must be a valid UUIDv7.
- `skewMs` (int64): Time skew window in milliseconds. Must be between 0 and 86400000 inclusive.
- `numKeys` (int64): Number of keys in the array. Must be >= 0.
- `get` (func): Callback function that returns UUIDv7 keys by index.

**Return Values**:
- `int64`: Index of the found key, or -1 if not found.
- `error`: Error if validation fails or get function returns an error.

**Behavior**:
1. Validates that `target` is a valid UUIDv7 key
2. Extracts timestamp from `target` using `ExtractUUIDv7Timestamp()`
3. Performs binary search using extracted timestamps for comparison
4. Applies skew window logic to determine indeterminate range
5. Performs linear scan using full UUID equality for exact matching
6. Returns index of exact UUID match or `KeyNotFoundError`

**Error Conditions**:
- `InvalidInputError`: When `target` is not a valid UUIDv7
- `InvalidInputError`: When `skewMs` is outside valid range
- `InvalidInputError`: When `numKeys` is negative
- `KeyNotFoundError`: When target UUID is not found in array
- Error from `get` function: Propagated as-is

**Performance Characteristics**:
- Time Complexity: O(log n) + k where k = UUIDv7 entries in skew window
- Space Complexity: O(1) - no additional memory allocation
- Thread Safety: Safe for concurrent reads

**Usage Example**:
```go
// Array of UUIDv7 keys
keys := []uuid.UUID{
    uuid.Must(uuid.NewV7()),
    uuid.Must(uuid.NewV7()),
    uuid.Must(uuid.NewV7()),
}

// Search for a specific UUID
target := keys[1]
index, err := FuzzyBinarySearch(
    target,
    1000,  // 1 second skew
    int64(len(keys)),
    func(i int64) (uuid.UUID, error) {
        return keys[i], nil
    },
)

if err != nil {
    // Handle error
}

if index >= 0 {
    // Found at index
}
```


### Callback Function Requirements

The `get` callback function must:
- Return valid UUIDv7 keys
- Handle index bounds correctly (0 <= index < numKeys)
- Preserve the original array ordering
- Return errors appropriately

**Valid Callback Example**:
```go
func getFromArray(keys []uuid.UUID) func(int64) (uuid.UUID, error) {
    return func(i int64) (uuid.UUID, error) {
        if i < 0 || i >= int64(len(keys)) {
            return uuid.Nil, NewInvalidInputError("index out of bounds", nil)
        }
        return keys[i], nil
    }
}
```

## Error Handling

### Validation Errors

All validation errors use `InvalidInputError` with descriptive messages:

```go
// Invalid UUIDv7
NewInvalidInputError("target must be a valid UUIDv7", validationErr)

// Invalid skewMs
NewInvalidInputError("skewMs must be between 0 and 86400000 inclusive", nil)

// Invalid numKeys
NewInvalidInputError("numKeys must be >= 0", nil)
```

### Search Errors

```go
// Key not found
NewKeyNotFoundError("UUID not found in array", nil)
```

## Testing Requirements

### Unit Tests

Unit tests should cover:
- Valid UUIDv7 searches with various array sizes
- Edge cases (empty array, single element)
- Skew window behavior (0, positive, maximum)
- Multiple UUIDs with identical timestamps
- Error conditions (invalid inputs, get function errors)
- Performance benchmarks

### Spec Tests

Spec tests must validate all functional requirements:
- `Test_S_026_FR_001_UUIDv7TargetValidation`
- `Test_S_026_FR_002_TimestampExtractionForComparison`
- `Test_S_026_FR_003_FullUUIDEqualityLinearScan`
- `Test_S_026_FR_004_MultipleUUIDsWithIdenticalTimestamps`
- `Test_S_026_FR_005_PerformanceComplexityPreserved`

## Migration Guide

### From Current Implementation

To migrate from the current int64-based implementation:

1. **Update Function Call**:
   ```go
   // Old
   index, err := FuzzyBinarySearch(targetTimestamp, skewMs, numKeys, getInt64Key)
   
   // New
   index, err := FuzzyBinarySearch(targetUUID, skewMs, numKeys, getUUIDKey)
   ```

2. **Update Callback Function**:
   ```go
   // Old
   func getInt64Key(i int64) (int64, error) {
       return timestamps[i], nil
   }
   
   // New
   func getUUIDKey(i int64) (uuid.UUID, error) {
       return uuids[i], nil
   }
   ```

### Backward Compatibility

The refactored function is not backward compatible with the int64-based version. Both versions can coexist in the codebase if needed, with the UUIDv7 version being the preferred implementation for UUID-based searches.
