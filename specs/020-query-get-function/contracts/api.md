# frozenDB Get Function API Specification

## Overview

The Get function provides query capabilities for frozenDB, allowing users to retrieve JSON values by UUID key with proper transaction validation and unmarshaling.

## Method Signature

```go
func (db *FrozenDB) Get(key uuid.UUID, value any) error
```

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `key` | `uuid.UUID` | UUIDv7 key identifying the database entry. Must not be uuid.Nil. |
| `value` | `any` | Destination data structure for JSON unmarshaling. Must be a non-nil pointer. |

## Return Value

| Type | Description |
|------|-------------|
| `error` | nil on success, FrozenDBError on failure |

## Behavior

### Success Conditions

1. **Key Found**: The UUID key exists in the database
2. **Committed Transaction**: The row is part of a committed transaction or valid partial rollback
3. **Valid JSON**: The stored JSON data can be successfully unmarshaled into the destination
4. **Transaction Visibility**: The row is visible according to transaction state rules

### Error Conditions

| Error | When Returned | Description |
|-------|---------------|-------------|
| `InvalidInputError` | `value` is nil or not a pointer | Destination parameter validation failed |
| `KeyNotFoundError` | Key not found in committed data | Key doesn't exist or is only in rolled back transactions |
| `InvalidDataError` | JSON unmarshal fails | Stored JSON cannot be unmarshaled into destination |
| `ReadError` | File I/O failures | Disk read operations failed |
| `CorruptDatabaseError` | Data corruption detected | Row format is invalid or corrupted |
| `TransactionActiveError` | Transaction still active | Key is in an uncommitted transaction |

## Usage Examples

### Basic Usage

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

// Define destination
var user User

// Retrieve data
err := db.Get(userUUID, &user)
if err != nil {
    // Handle error
}

// Use populated struct
fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
```

### Nested JSON Structures

```go
type Profile struct {
    Bio     string `json:"bio"`
    Active  bool   `json:"active"`
}

type User struct {
    ID      int      `json:"id"`
    Name    string   `json:"name"`
    Profile Profile  `json:"profile"`
}

var user User
err := db.Get(userUUID, &user)
if err != nil {
    // Handle error
}
```

### Nested JSON Structures

```go
type Profile struct {
    Bio     string `json:"bio"`
    Active  bool   `json:"active"`
}

type User struct {
    ID      int      `json:"id"`
    Name    string   `json:"name"`
    Profile Profile  `json:"profile"`
}

var user User
err := db.Get(userUUID, &user)
if err != nil {
    // Handle error
}
```



## Transaction Validation Rules

### Committed Transactions

- Transaction ends with `TC` or `SC` end_control
- All rows from transaction start through commit are valid
- Get returns data for all rows in committed transactions

### Partial Rollbacks

- Transaction ends with `R1-R9` or `S1-S9` end_control
- Rows from transaction start through rollback savepoint N are valid
- Rows after savepoint N through rollback command are invalid
- Get returns data only for rows at or before the rollback savepoint

### Full Rollbacks

- Transaction ends with `R0` or `S0` end_control
- All rows in transaction are invalid
- Get returns KeyNotFoundError for all keys in fully rolled back transactions

### Active Transactions

- Transaction has no ending row
- Get returns TransactionActiveError
- No rows are visible until transaction is committed or rolled back

## Thread Safety

- Multiple Get operations can execute concurrently
- Get operations are safe during concurrent write operations
- Finder implementations provide thread-safe access to internal state

## Performance Characteristics

- **Key Lookup**: O(n) where n = number of rows (linear scan implementations)
- **Transaction Validation**: O(k) where k ≤ 101 (maximum transaction size)
- **JSON Unmarshaling**: Depends on data size and complexity
- **Memory Usage**: O(1) constant (does not scale with database size)

## Compatibility

- Destination parameter follows `json.Unmarshal` patterns
- Supports all JSON-compatible Go types
- Handles nested structs, slices, maps, and primitive types
- Maintains type safety through pointer validation

## Integration Notes

- Uses Finder protocol for row location and transaction boundary detection
- Leverages existing frozenDB file format and error handling
- Integrates with existing transaction management system
- Maintains consistency with other frozenDB operations