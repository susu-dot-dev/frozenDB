# API Contract: Fuzzy Binary Search Algorithm

## Function Signature

```go
func FuzzyBinarySearch(target, skewMs, numKeys int64, get func(int64) (int64, error)) (int64, error)
```

## Parameters

| Name | Type | Description | Validation |
|------|------|-------------|------------|
| target | int64 | Unix millisecond timestamp to search for | Must be non-negative |
| skewMs | int64 | Clock skew window in milliseconds | Must be between 0-86400000 inclusive |
| numKeys | int64 | Total number of searchable keys | Must be positive integer |
| get | func | Callback to retrieve timestamp by index | Must not be nil, must be thread-safe |

## Callback Function

```go
type TimestampGetter func(index int64) (int64, error)
```

### Callback Parameters
| Name | Type | Description |
|------|------|-------------|
| index | int64 | Array index to retrieve (0-based) |

### Callback Returns
| Type | Description |
|------|-------------|
| int64 | Unix millisecond timestamp at specified index |
| error | KeyNotFoundError if key doesn't exist, or other errors |

## Return Values

| Type | Description |
|------|-------------|
| int64 | Index of the exact matching timestamp (success case) |
| error | Structured error (failure case) |

## Error Types

| Error Type | Code | When Returned |
|------------|------|--------------|
| InvalidInputError | "invalid_input" | Invalid parameters (negative skew, zero keys, etc.) |
| KeyNotFoundError | "key_not_found" | Target timestamp not found in array |
| ReadError | "read_error" | Callback function failure |
| CorruptDatabaseError | "corrupt_database" | Data integrity issues discovered during search |

## Performance Guarantees

- **Time Complexity**: O(log n) + k where n = numKeys, k = entries within ±skewMs
- **Space Complexity**: O(1) constant memory usage
- **Callback Invocations**: At most O(log n) + k calls to the get() function

## Usage Example

```go
// Example: Find timestamp 1640995200000 with 5ms skew in array of 1000 keys
index, err := FuzzyBinarySearch(
    1640995200000, // target timestamp
    5,             // skewMs
    1000,          // numKeys
    func(i int64) (int64, error) {
        // Implementation-specific timestamp retrieval
        return getTimestampFromDataStructure(i)
    },
)

if err != nil {
    // Handle error appropriately
    return err
}

// Use found index
fmt.Printf("Found at index: %d\n", index)
```

## Integration Notes

- Algorithm is data-structure agnostic - works with any storage system via callback
- Thread-safe for concurrent reads with immutable underlying data
- Compatible with existing frozenDB error handling patterns
- Does not modify any underlying data structures