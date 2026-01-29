# API Contracts: Recovery Test Helpers

**Feature**: 031-recover-transaction-tests  
**Date**: 2026-01-29

## Overview

This document defines the test helper API for creating databases, performing operations, and comparing transaction state in recovery tests. All helpers are internal to the test file and not part of the public API.

## Test Helper Functions

### Database Creation Helpers

#### createTestDatabase

```go
func createTestDatabase(t *testing.T, path string)
```

Creates a new empty frozenDB database for testing (already exists in frozendb_spec_test.go).

**Parameters**:
- `t *testing.T`: Test context
- `path string`: Absolute path for database file

**Behavior**:
- Creates parent directory if needed
- Uses Create() with mock syscalls (setupMockSyscalls)
- Sets mock SUDO environment variables (SUDO_USER, SUDO_UID, SUDO_GID)
- Creates database with rowSize=1024, skewMs=5000
- Includes header + initial checksum row
- Restores real syscalls after completion

**Example**:
```go
testPath := filepath.Join(t.TempDir(), "test.fdb")
createTestDatabase(t, testPath)
db, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
if err != nil {
    t.Fatalf("Open failed: %v", err)
}
// ... perform operations
db.Close()
```

**Note**: This helper already exists in frozendb_spec_test.go and is used by all existing spec tests. Recovery tests will use the same pattern.

---

#### createDBWithManyRows

```go
func createDBWithManyRows(t *testing.T, targetRows int) string
```

Creates a database with many complete rows (for checksum row testing).

**Parameters**:
- `t *testing.T`: Test context
- `targetRows int`: Target number of complete rows to create (typically 10,000)

**Returns**:
- `string`: Absolute path to created database file

**Behavior**:
- Creates database using createTestDatabase()
- Opens in write mode
- Creates multiple committed transactions with up to 100 rows each
- Continues until targetRows is reached
- Closes database
- Returns path for reopening

**Implementation**:
```go
func createDBWithManyRows(t *testing.T, targetRows int) string {
    testPath := filepath.Join(t.TempDir(), "test.fdb")
    createTestDatabase(t, testPath)
    
    db, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
    if err != nil {
        t.Fatalf("Failed to open: %v", err)
    }
    
    rowsCreated := 0
    for rowsCreated < targetRows {
        tx, err := db.BeginTx()
        if err != nil {
            t.Fatalf("BeginTx failed: %v", err)
        }
        
        rowsInThisTx := min(100, targetRows - rowsCreated)
        for i := 0; i < rowsInThisTx; i++ {
            key := generateUUIDv7(t)
            tx.AddRow(key, json.RawMessage(`{}`))
        }
        
        tx.Commit()
        rowsCreated += rowsInThisTx
    }
    
    db.Close()
    return testPath
}
```

**Performance**: Creates ~2MB of data for 10,000 rows, completes in ~1-2 seconds.

**Example**:
```go
// Create database with 10,000 rows (triggers checksum row insertion)
dbPath := createDBWithManyRows(t, 10000)

// Now add a partial transaction after the checksum
db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
if err != nil {
    t.Fatalf("Open failed: %v", err)
}
tx, err := db.BeginTx()
if err != nil {
    t.Fatalf("BeginTx failed: %v", err)
}
// ... file now ends with: ... ChecksumRow PartialRow
db.Close()
```

**Note**: Uses minimal JSON (`{}`) for performance. Each transaction has 100 rows (maximum) to minimize transaction overhead. Should be marked with `t.Parallel()` to run concurrently with other checksum tests.

---

### State Comparison Helpers

#### compareTransactionState

```go
func compareTransactionState(t *testing.T, original, recovered *FrozenDB)
```

Compares transaction state between two FrozenDB instances per FR-001 correctness definition.

**Parameters**:
- `t *testing.T`: Test context for reporting failures
- `original *FrozenDB`: Original database instance (before close)
- `recovered *FrozenDB`: Recovered database instance (after reopen in read-only mode)

**Behavior**:
- Compares `activeTx` presence (both nil, or both non-nil)
- If both have activeTx, compares:
  - Row count and contents (compareRows)
  - PartialDataRow state (comparePartialDataRow)
  - NullRow state (compareNullRow)
  - rowBytesWritten value
- Reports detailed errors on mismatch

**Failure Modes**:
- Calls `t.Errorf()` for each mismatch found
- Does not stop test execution (allows seeing all differences)

**Note**: In practice, tests will use `captureTransactionState()` to snapshot the original state before closing, then use `compareSnapshotToRecovered()` to compare.

---

#### captureTransactionState

```go
func captureTransactionState(t *testing.T, db *FrozenDB) *transactionStateSnapshot
```

Captures current transaction state for later comparison.

**Parameters**:
- `t *testing.T`: Test context
- `db *FrozenDB`: Database to capture state from

**Returns**:
- `*transactionStateSnapshot`: Captured state (can be compared after recovery)

**Behavior**:
- Deep copies transaction state (rows, last, empty, rowBytesWritten)
- Allows closing and reopening database for recovery test
- Used when you can't keep original DB open for comparison

**Example**:
```go
// Capture state before closing
snapshot := captureTransactionState(t, db1)
db1.Close()

// Reopen and compare to snapshot
db2, _ := NewFrozenDB(path, MODE_READ, FinderStrategySimple)
compareSnapshotToRecovered(t, snapshot, db2)
```

---

#### compareSnapshotToRecovered

```go
func compareSnapshotToRecovered(t *testing.T, snapshot *transactionStateSnapshot, recovered *FrozenDB)
```

Compares captured snapshot to recovered database state.

**Parameters**:
- `t *testing.T`: Test context
- `snapshot *transactionStateSnapshot`: Previously captured state
- `recovered *FrozenDB`: Recovered database instance

**Behavior**:
- Compares snapshot to recovered db's activeTx state
- Uses same comparison logic as compareTransactionState
- Reports mismatches via t.Errorf()

---

#### compareRows

```go
func compareRows(t *testing.T, original, recovered []DataRow)
```

Compares two slices of DataRow for equality.

**Parameters**:
- `t *testing.T`: Test context
- `original []DataRow`: Original row slice
- `recovered []DataRow`: Recovered row slice

**Behavior**:
- Compares slice lengths
- For each row, compares:
  - UUID (byte-for-byte)
  - Value (JSON comparison)
  - StartControl byte
  - EndControl string
- Reports index and field for mismatches

---

#### comparePartialDataRow

```go
func comparePartialDataRow(t *testing.T, original, recovered *PartialDataRow)
```

Compares two PartialDataRow instances.

**Parameters**:
- `t *testing.T`: Test context
- `original *PartialDataRow`: Original partial row (may be nil)
- `recovered *PartialDataRow`: Recovered partial row (may be nil)

**Behavior**:
- Handles nil cases (both nil = pass, one nil = fail)
- Compares state (determined by GetState() if available)
- Compares UUID (if present)
- Compares value (if present)
- Compares savepoint flag (if applicable)

---

#### compareNullRow

```go
func compareNullRow(t *testing.T, original, recovered *NullRow)
```

Compares two NullRow instances.

**Parameters**:
- `t *testing.T`: Test context
- `original *NullRow`: Original null row (may be nil)
- `recovered *NullRow`: Recovered null row (may be nil)

**Behavior**:
- Handles nil cases (both nil = pass, one nil = fail)
- Compares UUID
- Compares timestamp component

---

### Utility Helpers

#### generateUUIDv7

```go
func generateUUIDv7(t *testing.T) uuid.UUID
```

Generates a valid UUIDv7 for testing.

**Parameters**:
- `t *testing.T`: Test context (for fatals on generation errors)

**Returns**:
- `uuid.UUID`: Generated UUIDv7

**Behavior**:
- Uses github.com/google/uuid v7 generation: `uuid.NewV7()`
- Ensures chronological ordering (small delay between calls if needed)
- Fails test if generation fails

**Example**:
```go
key := generateUUIDv7(t)
tx.AddRow(key, json.RawMessage(`{"test":"value"}`))
```

---

#### generateTestValue

```go
func generateTestValue() json.RawMessage
```

Generates a simple test JSON value.

**Returns**:
- `json.RawMessage`: Valid JSON test value

**Behavior**:
- Returns `json.RawMessage(`{}`)`  or similar simple JSON
- Always valid, never fails

**Example**:
```go
value := generateTestValue()
tx.AddRow(key, value)
```

---

## Test Pattern

The standard test pattern for recovery tests:

```go
func Test_S_031_FR_XXX_Description(t *testing.T) {
    // Step 1: Create database using helper
    testPath := filepath.Join(t.TempDir(), "test.fdb")
    createTestDatabase(t, testPath)
    
    // Step 2: Open in write mode and perform operations
    db1, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
    if err != nil {
        t.Fatalf("NewFrozenDB write mode failed: %v", err)
    }
    
    // Perform operations - BeginTx() is called on db, not tx
    tx, err := db1.BeginTx()
    if err != nil {
        t.Fatalf("BeginTx failed: %v", err)
    }
    
    tx.AddRow(generateUUIDv7(t), generateTestValue())
    // ... more operations (or leave partial)
    
    // Step 3: Capture state before closing
    snapshot := captureTransactionState(t, db1)
    db1.Close()
    
    // Step 4: Reopen in read-only mode
    db2, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
    if err != nil {
        t.Fatalf("NewFrozenDB read mode failed (recovery): %v", err)
    }
    defer db2.Close()
    
    // Step 5: Compare state per FR-001
    compareSnapshotToRecovered(t, snapshot, db2)
}
```

**Key Points**:
- Always use `createTestDatabase(t, path)` to create the database file
- Always use `t.TempDir()` for test isolation
- **BeginTx() is called on FrozenDB (db), not Transaction (tx)**
- BeginTx() creates the transaction AND calls Begin() internally
- Capture state before closing db1 (can't access after Close())
- Reopen in MODE_READ to trigger recovery
- Use helper functions for state comparison

---

## Table-Driven Test Pattern

For testing multiple scenarios within a single FR:

```go
func Test_S_031_FR_003_OneCompleteTransaction_AllEndStates(t *testing.T) {
    tests := []struct {
        name       string
        setup      func(*testing.T, *FrozenDB)
        wantActive bool
    }{
        {
            name: "TC_commit",
            setup: func(t *testing.T, db *FrozenDB) {
                tx, err := db.BeginTx()
                if err != nil {
                    t.Fatalf("BeginTx failed: %v", err)
                }
                tx.AddRow(generateUUIDv7(t), json.RawMessage(`{}`))
                tx.Commit()
            },
            wantActive: false,
        },
        {
            name: "RE_continue",
            setup: func(t *testing.T, db *FrozenDB) {
                tx, err := db.BeginTx()
                if err != nil {
                    t.Fatalf("BeginTx failed: %v", err)
                }
                tx.AddRow(generateUUIDv7(t), json.RawMessage(`{}`))
                // Don't commit - leave partial
            },
            wantActive: true,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create DB
            testPath := filepath.Join(t.TempDir(), "test.fdb")
            createTestDatabase(t, testPath)
            
            // Open and run setup
            db1, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
            if err != nil {
                t.Fatalf("Open failed: %v", err)
            }
            
            tt.setup(t, db1)
            
            // Capture and close
            snapshot := captureTransactionState(t, db1)
            db1.Close()
            
            // Reopen and compare
            db2, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
            if err != nil {
                t.Fatalf("Recovery failed: %v", err)
            }
            defer db2.Close()
            
            compareSnapshotToRecovered(t, snapshot, db2)
            
            // Optionally check wantActive flag
            hasActive := (db2.activeTx != nil)
            if hasActive != tt.wantActive {
                t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
            }
        })
    }
}
```

---

## Implementation Notes

### Thread Safety

All test helpers are single-threaded and do not require mutex protection. Tests execute sequentially within each test function.

### Error Handling

- Database creation errors: Fatal (t.Fatalf)
- Operation errors: Fatal (t.Fatalf) - tests assume operations should succeed
- Comparison errors: Non-fatal (t.Errorf) - allows seeing all differences

### Performance

- Use 1024-byte rowSize (standard from createTestDatabase)
- Use minimal JSON (`{}`) for performance
- generateUUIDv7() may add small delays (~1ms) to ensure chronological ordering

### Test Isolation

- Each test uses t.TempDir() for isolation
- No shared state between tests
- Checksum row tests can run in parallel with t.Parallel()

---

## State Snapshot Structure

```go
type transactionStateSnapshot struct {
    hasActiveTx     bool
    rows            []DataRow
    last            *PartialDataRow
    empty           *NullRow
    rowBytesWritten int
}
```

Used by captureTransactionState() and compareSnapshotToRecovered().

---

## Notes

- All helpers use `t.Helper()` to ensure correct line numbers in failure reports
- Comparison functions report all differences, not just first mismatch
- UUIDv7 generation maintains chronological ordering via timestamps
- PartialDataRow state detection uses existing GetState() method where available
- BeginTx() is ALWAYS called on FrozenDB, never on Transaction
