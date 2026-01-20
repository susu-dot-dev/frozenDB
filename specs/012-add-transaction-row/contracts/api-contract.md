# API Contracts: AddRow Transaction Implementation

**Feature**: 012-add-transaction-row  
**Date**: 2026-01-19  
**Repository**: github.com/susu-dot-dev/frozenDB

## 1. Public API Contract

### 1.1 Transaction Interface

```go
// Transaction represents a frozenDB transaction with AddRow capability
type Transaction interface {
    // Begin starts a new transaction
    Begin() error
    
    // AddRow adds a key-value pair to the current transaction
    AddRow(key uuid.UUID, value string) error
    
    // Commit finalizes the transaction and makes all rows persistent
    Commit() error
    
    // GetMaxTimestamp returns the current maximum timestamp seen in this transaction
    GetMaxTimestamp() int64
    
    // GetRows returns a copy of completed rows in the transaction
    GetRows() []DataRow
}
```

### 1.2 AddRow Method Contract

```go
// AddRow adds a new key-value pair to the transaction
//
// Preconditions:
//   - Transaction must be active (last non-nil, empty nil)
//   - Key must be valid UUIDv7
//   - Value must be non-empty JSON string
//   - Transaction must have < 100 rows
//   - UUID timestamp must satisfy: new_timestamp + skew_ms > max_timestamp
//
// Postconditions:
//   - Previous PartialDataRow is finalized and moved to rows[]
//   - New PartialDataRow is created with provided key-value
//   - max_timestamp is updated if new_timestamp > previous max_timestamp
//   - Transaction row count is incremented
//
// Errors:
//   - InvalidActionError: Transaction not active or already committed/rolled back
//   - InvalidInputError: Invalid UUIDv7, empty value, or >100 rows
//   - KeyOrderingError: Timestamp ordering violation
//
// Thread Safety:
//   - Method is thread-safe with internal mutex locking
//
// Performance:
//   - O(1) amortized time complexity
//   - Fixed memory footprint regardless of database size
func (tx *Transaction) AddRow(key uuid.UUID, value string) error
```

## 2. Error Contract

### 2.1 KeyOrderingError

See data-model.md for the KeyOrderingError struct definition. This error type is returned when UUID timestamp ordering constraints are violated.

### 2.2 Error Hierarchy

```go
// FrozenDBError is the base error type for all frozenDB errors
type FrozenDBError interface {
    error
    
    // GetCode returns the error code for programmatic handling
    GetCode() string
    
    // GetMessage returns the human-readable error message
    GetMessage() string
    
    // GetUnderlying returns the underlying error if any
    GetUnderlying() error
}

// Specific error types implement FrozenDBError interface:
// - InvalidActionError     (FR-001, FR-011 violations)
// - InvalidInputError     (FR-006, FR-007, FR-010 violations)
// - KeyOrderingError      (FR-013, FR-015, FR-016 violations)
```

## 3. State Contract

### 3.1 Transaction State Inference

Transaction state is derived from existing field combinations (see data-model.md for detailed logic).

**State Meanings:**
- Inactive: No transaction activity (no rows, no partial row, no empty row)
- Active: Transaction in progress (partial row exists, no empty row)
- Committed: Transaction completed successfully (empty row exists, no partial row)

### 3.2 PartialDataRowState Enum

```go
type PartialDataRowState int

const (
    PartialDataRowStateStartControl    PartialDataRowState = iota // State 1: Start control only
    PartialDataRowStateKeyValue                                // State 2: Complete key-value data
    PartialDataRowStateSavepointIntent                         // State 3: Savepoint intent indicated
)
```

## 4. Data Contract

### 4.1 DataRowPayload

See existing DataRowPayload definition for key-value data structure requirements (UUIDv7 key, non-empty value).

### 4.2 Transaction Structure

See data-model.md for the complete Transaction struct definition, including:
- Core transaction fields (existing)
- maxTimestamp field (new addition for UUID ordering)
- Thread safety via sync.RWMutex

## 5. Performance Contract

### 5.1 Time Complexity

| Operation | Complexity | Description |
|------------|------------|-------------|
| AddRow     | O(1)       | Amortized constant time |
| GetMaxTimestamp | O(1) | Direct field access |
| GetState   | O(1)       | Direct field access |
| GetRows    | O(n)       | n = number of rows in transaction |

### 5.2 Memory Complexity

| Operation | Memory | Description |
|------------|--------|-------------|
| Transaction creation | O(1) | Fixed size struct |
| AddRow     | O(1) | Fixed additional memory per row |
| Row storage | O(n) | n = number of rows, max 100 |

### 5.3 Thread Safety Guarantees

```go
// Thread Safety Contract:
//
// Read Operations:
//   - GetMaxTimestamp(), GetRows() use read locks
//   - Multiple concurrent reads allowed
//
// Write Operations:
//   - AddRow() uses exclusive write lock
//   - Blocking during write operations
//   - No concurrent writes to same transaction
//
// Consistency:
//   - All operations provide atomic transitions
//   - No partial visibility to callers
//   - Mutex ensures memory ordering guarantees
```

## 6. Integration Contract

### 6.1 FrozenDB Integration

```go
// FrozenDB methods that interact with Transaction
type FrozenDB struct {
    // Existing fields...
    maxTimestamp int64 // Global max timestamp tracking
    mu           sync.RWMutex
}

// BeginTransaction creates a new transaction with current database state
func (db *FrozenDB) BeginTransaction() (*Transaction, error)

// GetMaxTimestamp returns the global maximum timestamp from the database
func (db *FrozenDB) GetMaxTimestamp() int64
```

### 6.2 File Format Integration

All transaction operations must produce rows compliant with v1_file_format.md:

```go
// Row format compliance requirements:
//
// DataRow structure:
//   - ROW_START: 0x1F byte
//   - start_control: 'T' for first row, 'R' for continuation
//   - uuid_base64: 24-byte Base64 encoding of UUIDv7
//   - json_payload: UTF-8 JSON string followed by NULL_BYTE padding
//   - end_control: Two bytes (RE, TC, SC, SE, etc.)
//   - parity_bytes: Two-byte LRC checksum
//   - ROW_END: 0x0A byte
//
// NullRow structure:
//   - uuid: uuid.Nil Base64 encoded
//   - value: No user data (immediate padding)
//   - end_control: "NR"
```

## 7. Testing Contract

### 7.1 Spec Test Requirements

Each functional requirement FR-XXX must have corresponding spec test:

```go
// Naming convention: Test_S_012_FR_XXX_Description
//
// Location: frozendb/transaction_spec_test.go
//
// Requirements:
//   - Test exactly as specified in requirement
//   - No modifications after implementation without user permission
//   - Distinct from unit tests, focus on functional validation
```

### 7.2 Unit Test Coverage

```go
// Unit test requirements:
//
// Method Coverage:
//   - All public methods with success and error paths
//   - All private methods critical for functionality
//
// Edge Cases:
//   - Boundary conditions (100 rows, 9 savepoints)
//   - Empty database scenarios
//   - Timestamp edge cases
//
// Concurrency:
//   - Thread safety under concurrent access
//   - Race condition detection
//   - Deadlock prevention
```

## 8. Validation Contract

### 8.1 Input Validation Matrix

| Input | Validation | Error Type | Reference |
|-------|-------------|------------|-----------|
| Transaction state | Must be active (last non-nil, empty nil) | InvalidActionError | FR-001, FR-011 |
| UUID | Valid UUIDv7 format | InvalidInputError | FR-006 |
| Timestamp | new_ts + skew_ms > max_ts | KeyOrderingError | FR-013, FR-016 |
| Value | Non-empty string | InvalidInputError | FR-007 |
| Row count | < 100 rows | InvalidInputError | FR-010 |

### 8.2 State Validation

```go
// Transaction transitions:
//   Inactive -> Active (Begin() creates partial row)
//   Active -> Committed (Commit() sets empty row, clears partial row)
```

This contract document provides the complete API specification for implementing AddRow functionality while maintaining frozenDB's constitutional principles and ensuring thread safety, data integrity, and performance requirements.