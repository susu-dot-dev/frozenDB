# Phase 1 Data Model: FBS UUIDv7 Refactor

## Data Entities

This document defines the new and modified data entities for the FBS UUIDv7 refactoring.

## Modified Entities

### FuzzyBinarySearch Function

**Type**: Algorithm function modification

**Current Signature**:
```go
func FuzzyBinarySearch(target, skewMs, numKeys int64, get func(int64) (int64, error)) (int64, error)
```

**New Signature**:
```go
func FuzzyBinarySearch(target uuid.UUID, skewMs int64, numKeys int64, get func(int64) (uuid.UUID, error)) (int64, error)
```

**Field Changes**:
- `target`: Changed from `int64` to `uuid.UUID` - the UUIDv7 key being searched for
- `get`: Changed from returning `int64` to returning `uuid.UUID` - callback function that returns UUIDv7 keys by index

**Validation Rules**:
- `target` MUST be a valid UUIDv7 (validated using `ValidateUUIDv7()`)
- `skewMs` MUST be between 0 and 86400000 inclusive (24 hours)
- `numKeys` MUST be >= 0
- `get` function must return valid UUIDv7 keys

**State Changes**:
- Algorithm extracts timestamp from `target` using `ExtractUUIDv7Timestamp()`
- Binary search comparison uses extracted timestamps
- Linear scan comparison uses full UUID equality (`uuid.UUID == uuid.UUID`)

## New Data Flow Relationships

### Algorithm Flow Changes

**Input Processing**:
1. Validate `target` is UUIDv7 using `ValidateUUIDv7()`
2. Extract timestamp from `target` using `ExtractUUIDv7Timestamp()`
3. Use extracted timestamp for binary search comparisons

**Binary Search Phase**:
1. Extract timestamp from each key returned by `get` function
2. Compare extracted timestamps with target timestamp
3. Partition into three zones: `< target-timestamp`, `== target-timestamp`, `> target-timestamp`
4. Apply skew window logic to determine indeterminate range

**Linear Scan Phase**:
1. For keys within skew window, compare full UUID values
2. Return index of exact UUID match
3. Return `KeyNotFoundError` if no exact UUID match found

### Error Condition Mappings

**Validation Errors**:
- Invalid UUIDv7 target → `InvalidInputError` with descriptive message
- Invalid skewMs → `InvalidInputError` with range specification
- Invalid numKeys → `InvalidInputError` with minimum value specification
- Invalid UUIDs from get function → `InvalidInputError` from validation

**Search Errors**:
- Target not found → `KeyNotFoundError` (existing error type)
- Get function errors → wrapped and returned as-is

## Integration Points

### Finder Interface Compatibility

**Current Finder Interface**:
```go
type Finder interface {
    GetIndex(key uuid.UUID) (int64, error)
}
```

**Refactored FuzzyBinarySearch Usage**:
- Direct compatibility with `Finder` interface
- `target` parameter matches `key` parameter type
- Return signature matches interface requirements

### Callback Function Interface

**Required Callback Signature**:
```go
func(index int64) (uuid.UUID, error)
```

**Implementation Requirements**:
- Must return valid UUIDv7 keys
- Must preserve original array ordering
- Must handle index bounds correctly

## Performance Characteristics

### Time Complexity
- Preserved O(log n) + k where k = UUIDv7 entries in [target_timestamp-skewMs, target_timestamp+skewMs]
- UUID validation: O(1) per UUID
- Timestamp extraction: O(1) per UUID
- Binary search: O(log n)
- Linear scan: O(k)

### Space Complexity
- Preserved O(1) - no additional memory allocation
- UUID validation works in-place
- Timestamp extraction uses bitwise operations
- No data structures created beyond minimal local variables

## Correctness Constraints

### UUIDv7 Uniqueness Handling
- Multiple UUIDv7 keys with identical timestamps supported
- Linear scan phase ensures exact UUID matching
- Timestamp collisions do not affect search correctness

### Chronological Ordering Preservation
- Timestamp extraction preserves original ordering
- Binary search respects UUIDv7 time-based ordering
- Skew window handling maintains time-based search semantics

### Error Handling Consistency
- All validation errors use `InvalidInputError`
- Search failures use existing `KeyNotFoundError`
- Error messages follow established patterns