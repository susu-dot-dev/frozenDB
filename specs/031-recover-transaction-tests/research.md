# Research: recoverTransaction Test Suite Patterns

**Feature**: 031-recover-transaction-tests  
**Date**: 2026-01-29

## 1. Test Database Creation Pattern

### Decision: Use createTestDatabase() Helper with Mock Syscalls

**Approach**: Tests will use the existing `createTestDatabase()` helper pattern from frozendb_spec_test.go, which uses `Create()` with mock syscalls and mock SUDO environment variables to avoid actual sudo requirements.

**Pattern**:
```go
func createTestDatabase(t *testing.T, path string) {
    t.Helper()
    
    // Ensure parent directory exists
    parentDir := filepath.Dir(path)
    if err := os.MkdirAll(parentDir, 0755); err != nil {
        t.Fatalf("Failed to create parent directory: %v", err)
    }
    
    // Create database file with header + checksum row using Create()
    config := CreateConfig{
        path:    path,
        rowSize: 1024,
        skewMs:  5000,
    }
    
    // Set up mock syscalls for Create()
    setupMockSyscalls(false, false)
    defer restoreRealSyscalls()
    
    // Use mock values for SUDO environment
    t.Setenv("SUDO_USER", MOCK_USER)
    t.Setenv("SUDO_UID", MOCK_UID)
    t.Setenv("SUDO_GID", MOCK_GID)
    
    if err := Create(config); err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }
}

// Usage in tests:
func Test_S_031_FR_XXX(t *testing.T) {
    testPath := filepath.Join(t.TempDir(), "test.fdb")
    createTestDatabase(t, testPath)
    
    // Open in write mode for setup
    db, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
    if err != nil {
        t.Fatalf("NewFrozenDB failed: %v", err)
    }
    
    // Perform operations - BeginTx() is called on db
    tx, err := db.BeginTx()
    if err != nil {
        t.Fatalf("BeginTx failed: %v", err)
    }
    
    // Add rows, savepoints, etc.
    tx.AddRow(generateUUIDv7(t), json.RawMessage(`{}`))
    tx.Commit()
    
    // Close write mode
    db.Close()
    
    // Reopen in read-only mode for recovery test
    db2, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
    if err != nil {
        t.Fatalf("Recovery failed: %v", err)
    }
    defer db2.Close()
    
    // Compare state
    compareTransactionState(t, capturedState, db2)
}
```

**Key Points**:
- `setupMockSyscalls(false, false)` mocks the ioctl syscalls for append-only flag
- `t.Setenv()` sets mock SUDO_USER, SUDO_UID, SUDO_GID environment variables
- `restoreRealSyscalls()` restores real syscalls after Create() completes
- Mock constants: `MOCK_USER = "testuser"`, `MOCK_UID = "12345"`, `MOCK_GID = "12345"` (defined in create_test.go)
- **BeginTx() is called on FrozenDB, not Transaction** - it creates the transaction AND calls Begin() internally

**Rationale**: 
- This is the established pattern used by all existing spec tests
- Create() handles header, initial checksum row, and file structure correctly
- Mock syscalls avoid sudo requirement while testing real file I/O
- t.TempDir() provides automatic cleanup

**Alternative Considered**: Write header and checksum manually - rejected because it duplicates logic and risks inconsistency with Create() implementation.

## 2. Transaction State Comparison

### Decision: Use Test Helper Functions with Direct Field Access

**Approach**: Create helper functions that access Transaction internal fields for comparison. Since tests are in the same package (internal/frozendb), they can access unexported fields directly.

**Key Fields to Compare** (per FR-001):
- `rows []DataRow` - Complete rows in transaction
- `last *PartialDataRow` - Partial row being built
- `empty *NullRow` - Empty null row (if present)
- `rowBytesWritten int` - Bytes written for partial row

**Helper Function Pattern**:
```go
// compareTransactionState compares transaction state between original and recovered instances
func compareTransactionState(t *testing.T, original, recovered *FrozenDB) {
    t.Helper()
    
    originalTx := original.activeTx
    recoveredTx := recovered.activeTx
    
    // Case 1: Both should be nil (no active transaction)
    if originalTx == nil && recoveredTx == nil {
        return // Pass
    }
    
    // Case 2: One is nil, other is not - FAIL
    if (originalTx == nil) != (recoveredTx == nil) {
        t.Errorf("Transaction state mismatch: original=%v recovered=%v", 
            originalTx != nil, recoveredTx != nil)
        return
    }
    
    // Case 3: Both have active transaction - compare details
    compareRows(t, originalTx.rows, recoveredTx.rows)
    comparePartialDataRow(t, originalTx.last, recoveredTx.last)
    compareNullRow(t, originalTx.empty, recoveredTx.empty)
    
    if originalTx.rowBytesWritten != recoveredTx.rowBytesWritten {
        t.Errorf("rowBytesWritten mismatch: original=%d recovered=%d",
            originalTx.rowBytesWritten, recoveredTx.rowBytesWritten)
    }
}

func compareRows(t *testing.T, original, recovered []DataRow) {
    t.Helper()
    if len(original) != len(recovered) {
        t.Errorf("Row count mismatch: original=%d recovered=%d", 
            len(original), len(recovered))
        return
    }
    for i := range original {
        // Compare UUID, value, end control, etc.
        if original[i].UUID != recovered[i].UUID {
            t.Errorf("Row %d UUID mismatch", i)
        }
        // ... more comparisons
    }
}
```

**Rationale**:
- Tests are in same package, can access unexported fields
- Helper functions reduce code duplication across tests
- Clear error messages for debugging
- Follows existing test patterns in frozenDB codebase

**Alternative Considered**: Export Transaction fields for testing - rejected to maintain encapsulation.

## 3. PartialDataRow State Creation

### Decision: Exhaustive List of PartialDataRow Operation Sequences

**All possible PartialDataRow states** are created by these operation sequences:

1. `db.BeginTx()` - Creates transaction and calls Begin() - partial with start control only
2. `db.BeginTx() → tx.AddRow()` - Partial with first row payload
3. `db.BeginTx() → tx.AddRow() → tx.Savepoint()` - Partial with savepoint marker
4. `db.BeginTx() → tx.AddRow() → tx.AddRow()` - Partial with second row (first row finalized)
5. `db.BeginTx() → tx.AddRow() → tx.AddRow() → tx.Savepoint()` - Partial with second row and savepoint

**Generalized pattern**: Any sequence of:
- Exactly one `db.BeginTx()` (starts transaction, calls Begin() internally)
- Zero or more `tx.AddRow()` calls (0 = just BeginTx, 1+ = with data rows)
- Optional `tx.Savepoint()` as final operation (adds 'S' marker to partial)
- No terminating operation (`tx.Commit()`/`tx.Rollback()` would close transaction)

**Examples for tests**:

```go
// Sequence 1: Just BeginTx()
tx, _ := db.BeginTx()
// Partial: ROW_START + 'T', rowBytesWritten=2

// Sequence 2: BeginTx + one AddRow
tx, _ := db.BeginTx()
tx.AddRow(key1, value1)
// Partial: ROW_START + 'T' + UUID + JSON + padding, rowBytesWritten=calculated

// Sequence 3: BeginTx + AddRow + Savepoint
tx, _ := db.BeginTx()
tx.AddRow(key1, value1)
tx.Savepoint()
// Partial: Same as #2 but with 'S' at end, rowBytesWritten=calculated+1

// Sequence 4: BeginTx + two AddRows
tx, _ := db.BeginTx()
tx.AddRow(key1, value1)  // This row gets finalized
tx.AddRow(key2, value2)  // This becomes the partial
// Partial for second row: ROW_START + 'R' + UUID + JSON + padding

// Sequence 5: BeginTx + two AddRows + Savepoint
tx, _ := db.BeginTx()
tx.AddRow(key1, value1)  // This row gets finalized
tx.AddRow(key2, value2)  // This becomes the partial
tx.Savepoint()
// Partial: Same as #4 but with 'S' at end
```

**Key Insight**: The important distinction for recovery testing is:
- Number of complete rows in transaction (0 to 99)
- Whether partial row exists (yes after BeginTx/AddRow, no after Commit)
- Whether partial has savepoint marker (yes after Savepoint)

**Rationale**: This enumeration makes it clear what test scenarios are needed - we don't need to test "states" but rather specific operation sequences that leave the transaction in a partial state.

## 4. Checksum Row Positioning

### Decision: Test Checksum Rows with Helper to Create Many Rows

**Challenge**: Checksum rows appear every 10,000 complete data/null rows. Creating 10,000 rows in a single transaction violates the 100-row maximum.

**Solution**: Create multiple transactions to accumulate rows:

```go
// Helper to create database with many rows
func createDBWithManyRows(t *testing.T, targetRows int) string {
    testPath := filepath.Join(t.TempDir(), "test.fdb")
    createTestDatabase(t, testPath)
    
    db, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
    if err != nil {
        t.Fatalf("Failed to open: %v", err)
    }
    
    // Create transactions with 100 rows each
    rowsCreated := 0
    for rowsCreated < targetRows {
        tx, err := db.BeginTx()
        if err != nil {
            t.Fatalf("BeginTx failed: %v", err)
        }
        
        rowsInThisTx := min(100, targetRows - rowsCreated)
        for i := 0; i < rowsInThisTx; i++ {
            key := generateUUIDv7(t)
            value := json.RawMessage(`{}`)
            tx.AddRow(key, value)
        }
        
        tx.Commit()
        rowsCreated += rowsInThisTx
    }
    
    db.Close()
    return testPath
}
```

**Test Scenarios for FR-009**:

1. **10,000 rows ending with TC** - Last data row commits, checksum row follows
   ```go
   dbPath := createDBWithManyRows(t, 10000)
   // File ends: ... DataRow(TC) ChecksumRow
   // Recovery should skip checksum, see no active transaction
   ```

2. **10,000 rows + 1 partial row** - Checksum row, then BeginTx()
   ```go
   dbPath := createDBWithManyRows(t, 10000)
   db, _ := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
   tx, _ := db.BeginTx()
   db.Close()
   // File ends: ... ChecksumRow PartialRow(Begin)
   // Recovery should skip checksum, find partial transaction
   ```

3. **10,000 rows + 50 rows partial** - Checksum followed by partial transaction
   ```go
   dbPath := createDBWithManyRows(t, 10000)
   db, _ := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
   tx, _ := db.BeginTx()
   for i := 0; i < 50; i++ {
       tx.AddRow(generateUUIDv7(t), json.RawMessage(`{}`))
   }
   db.Close()
   // File ends: ... ChecksumRow + 50 row partial transaction
   // Recovery should skip checksum, reconstruct 50-row transaction
   ```

**Performance Consideration**: 
- Each row write is ~200 bytes (header + UUID + minimal JSON + padding)
- 10,000 rows = ~2MB of data
- With buffered writes, should complete in ~1-2 seconds
- Within the 10-second total test budget (3 checksum tests = 3-6 seconds)

**Rationale**: 
- We MUST test checksum row handling to find bugs
- Creating multiple committed transactions is the only way to accumulate 10,000 rows
- Recovery algorithm MUST correctly skip checksum rows when scanning backwards
- This is a critical scenario that could hide bugs in the 101-row scanning logic

**Alternative Considered**: Skip checksum testing - **REJECTED** because we're explicitly looking for bugs and this is a critical code path.

## 5. Complete Test Scenario Catalog

### FR-002: Zero Complete Transactions

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| Empty database | Only header + initial checksum | No active transaction |

**Test Count**: 1 test

### FR-003: One Complete Transaction (All End States)

| Scenario | End State | Description | Expected State |
|----------|-----------|-------------|----------------|
| Single row commit | TC | Begin() → AddRow() → Commit() | No active transaction |
| Single row commit with savepoint | SC | Begin() → AddRow() → Savepoint() → Commit() | No active transaction |
| Single row continue | RE | Begin() → AddRow() (file ends) | Active transaction with 1 row |
| Single row savepoint continue | SE | Begin() → AddRow() → Savepoint() (file ends) | Active transaction with 1 row |
| Full rollback | R0 | Begin() → AddRow() → Rollback(0) | No active transaction |
| Rollback to savepoint 1 | R1 | Begin() → AddRow() → Savepoint() → AddRow() → Rollback(1) | No active transaction |
| Savepoint with rollback to 0 | S0 | Begin() → AddRow() → Savepoint() → Rollback(0) | No active transaction |
| Savepoint with rollback to 1 | S1 | Begin() → AddRow() → Savepoint() → Rollback(1) | No active transaction |
| Null row | NR | Begin() → Commit() (empty) | No active transaction |

**Test Count**: 9 tests (one per end state)

### FR-004: Multiple Complete Transactions

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| Two commits | Begin() → AddRow() → Commit() × 2 | No active transaction |
| Three commits | Begin() → AddRow() → Commit() × 3 | No active transaction |
| Two commits + one rollback | Begin() → AddRow() → Commit() × 2, then Rollback(0) | No active transaction |
| Alternating NullRows and data | NullRow, DataTx, NullRow, DataTx | No active transaction |

**Test Count**: 4 tests

### FR-005: Partial Transactions (All PartialDataRow Operation Sequences)

| Scenario | Operation Sequence | Expected State |
|----------|-------------------|----------------|
| Just Begin | Begin() | Active tx, 0 complete rows, partial (2 bytes) |
| Begin + one AddRow | Begin() → AddRow() | Active tx, 0 complete rows, partial with payload |
| Begin + AddRow + Savepoint | Begin() → AddRow() → Savepoint() | Active tx, 0 complete rows, partial with 'S' marker |
| Begin + two AddRows | Begin() → AddRow() × 2 | Active tx, 1 complete row, partial for 2nd row |
| Begin + two AddRows + Savepoint | Begin() → AddRow() × 2 → Savepoint() | Active tx, 1 complete row, partial with 'S' marker |

**Test Count**: 5 tests (exhaustive list of partial operation sequences)

### FR-006: Mixed Scenarios (Complete + Partial)

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| One commit + partial State 1 | Complete tx, then Begin() | Active transaction from Begin() |
| One commit + partial State 2 | Complete tx, then Begin() → AddRow() | Active transaction with partial |
| Two commits + partial | Complete tx × 2, then Begin() | Active transaction from last Begin() |
| NullRow + partial | NullRow, then Begin() | Active transaction from Begin() |

**Test Count**: 4 tests

### FR-007: Varying Row Counts

| Scenario | Row Count | Description | Expected State |
|----------|-----------|-------------|----------------|
| Zero rows (NullRow) | 0 | Begin() → Commit() | No active transaction |
| One row committed | 1 | Begin() → AddRow() → Commit() | No active transaction |
| Two rows committed | 2 | Begin() → AddRow() × 2 → Commit() | No active transaction |
| Ten rows committed | 10 | Begin() → AddRow() × 10 → Commit() | No active transaction |
| Fifty rows committed | 50 | Begin() → AddRow() × 50 → Commit() | No active transaction |
| 100 rows committed (max) | 100 | Begin() → AddRow() × 100 → Commit() | No active transaction |
| One row partial | 1 | Begin() → AddRow() × 1 (continues) | Active transaction, 0 complete rows |
| Two rows partial | 2 | Begin() → AddRow() × 2 (second continues) | Active transaction, 1 complete row |
| Ten rows partial | 10 | Begin() → AddRow() × 10 (last continues) | Active transaction, 9 complete rows |
| 100 rows partial (max) | 100 | Begin() → AddRow() × 100 (last continues) | Active transaction, 99 complete rows |

**Test Count**: 10 tests

### FR-008: Savepoints at Various Positions

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| Savepoint on first row | Begin() → AddRow() → Savepoint() → Commit() | No active transaction |
| Savepoint on middle row | Begin() → AddRow() × 5 → Savepoint() → AddRow() × 5 → Commit() | No active transaction |
| Savepoint on last row | Begin() → AddRow() × 9 → AddRow() + Savepoint() → Commit() | No active transaction |
| Multiple savepoints | Begin() → AddRow() + Savepoint() × 3 → Commit() | No active transaction |
| Savepoint with partial State 3 | Begin() → AddRow() → Savepoint() (file ends) | Active transaction with State 3 partial |

**Test Count**: 5 tests

### FR-009: Checksum Row Scenarios

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| 10,000 rows ending TC | Create 10,000 complete rows, last ends with TC | No active transaction (checksum row present) |
| 10,000 rows + Begin() | 10,000 rows then Begin() partial | Active transaction with 0 complete rows |
| 10,000 rows + 50-row partial | 10,000 rows then 50-row partial transaction | Active transaction with 49 complete rows + 1 partial |

**Test Count**: 3 tests

**Performance Note**: Each test creates 10,000 rows across 100 transactions (100 rows each). Estimated time: ~1-2 seconds per test = 3-6 seconds total for FR-009.

### FR-010: Known Bug Regression Test

| Scenario | Description | Expected State |
|----------|-------------|----------------|
| Begin → Commit → Begin | Begin() → AddRow() → Commit() → Begin() | Active transaction from 2nd Begin(), independent state |
| Begin → Commit → Begin → AddRow | As above, then AddRow() | Active transaction with 1 partial row, no old data |

**Test Count**: 2 tests

### Total Scenario Count

- FR-002: 1 test
- FR-003: 9 tests
- FR-004: 4 tests
- FR-005: 5 tests
- FR-006: 4 tests
- FR-007: 10 tests
- FR-008: 5 tests
- FR-009: 3 tests (checksum row scenarios)
- FR-010: 2 tests

**Total: 43 test scenarios**

### Test Organization Strategy

**Approach**: Use table-driven tests within each FR test function to reduce boilerplate.

```go
func Test_S_031_FR_003_OneCompleteTransaction_AllEndStates(t *testing.T) {
    tests := []struct {
        name       string
        endState   string
        setup      func(*testing.T, *FrozenDB)
        wantActive bool
    }{
        {
            name: "TC_commit",
            endState: "TC",
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
        // ... 8 more test cases
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
        })
    }
}
```

**Rationale**: Table-driven tests provide clear scenario documentation while reducing duplication. Each FR-XXX gets its own test function with sub-tests. The setup function receives only the FrozenDB instance to call BeginTx() and perform operations.

## 6. Test Execution Performance

**Goal**: Test suite must execute in under 10 seconds (SC-004)

**Analysis**:
- Regular tests (40 scenarios): 40 × ~200ms = 8 seconds
- Checksum tests (3 scenarios): 3 × ~1.5s = 4.5 seconds
- **Total estimated: ~12.5 seconds**

**Performance Issue**: Slightly over 10-second budget due to checksum row tests.

**Optimization Strategy**:
1. Run checksum tests in parallel using `t.Parallel()` - can reduce 4.5s to ~1.5s
2. Use minimal JSON values (`{}` instead of larger payloads) - saves I/O time
3. Consider increasing budget to 15 seconds if needed - checksum testing is critical

**Revised Estimate with Parallel**:
- Regular tests: 8 seconds (sequential)
- Checksum tests: 1.5 seconds (parallel across 3 tests)
- **Total: ~9.5 seconds** ✓ Within budget

**Implementation Note**: Mark checksum row tests with `t.Parallel()` to enable concurrent execution.

## Summary of Decisions

1. ✅ **Database Creation**: Use createTestDatabase() with mock syscalls and mock SUDO environment
2. ✅ **State Comparison**: Helper functions with direct field access (same package)
3. ✅ **PartialDataRow Operations**: 5 exhaustive operation sequences listed
4. ✅ **Checksum Rows**: Create 10,000 rows via 100 transactions of 100 rows each, test 3 scenarios
5. ✅ **Scenario Catalog**: 43 test scenarios across FR-002 through FR-010
6. ✅ **Test Organization**: Table-driven tests within each FR test function
7. ✅ **Performance**: Use t.Parallel() for checksum tests to stay within 10-second budget

**Next Step**: Create contracts/api.md documenting the test helper API.
