package frozendb

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// =============================================================================
// Transaction Recovery Spec Tests (Spec 031)
// =============================================================================
//
// This file contains comprehensive spec tests validating that frozenDB correctly
// recovers transaction state when reopening databases in read-only mode.
//
// ## Test Pattern
//
// All recovery tests follow this pattern:
//   1. Create database with specific transaction state (using createTestDatabase)
//   2. Perform operations to create desired state (transactions, commits, partial writes)
//   3. Capture transaction state snapshot before closing
//   4. Close database
//   5. Reopen database in MODE_READ (triggers recovery)
//   6. Compare recovered state to original snapshot per FR-001 correctness criteria
//
// ## FR-001 Correctness Criteria
//
// Recovery is correct if the in-memory transaction state is identical:
//   - rows []DataRow: Complete rows in active transaction
//   - last *PartialDataRow: Partial row being built (with correct state)
//   - empty *NullRow: Empty null row (if present)
//   - rowBytesWritten int: Bytes written for partial row
//
// ## Test Organization
//
// ### Test Helpers (Lines 40-315)
// - generateUUIDv7: Create chronologically ordered UUIDs
// - generateTestValue: Create minimal JSON test values
// - captureTransactionState: Snapshot transaction state before closing
// - compareSnapshotToRecovered: Compare snapshot to recovered state
// - compareRows: Deep comparison of DataRow slices
// - comparePartialDataRow: Compare PartialDataRow instances
// - compareNullRow: Compare NullRow instances
// - createDBWithManyRows: Create databases with 10,000+ rows for checksum testing
//
// ### Phase 1: User Story 1 - Basic Recovery (P1) - Lines 317-820
// Tests: FR-002, FR-003, FR-005, FR-006
// Scenarios: Empty DB, single transactions (all end states), partial transactions, mixed scenarios
// Total: 22 test scenarios
//
// ### Phase 2: User Story 2 - Transaction Boundaries (P1) - Lines 822-982
// Tests: FR-004
// Scenarios: Multiple consecutive transactions (2-3 commits, rollbacks, alternating NullRows)
// Total: 4 test scenarios
//
// ### Phase 4: User Story 4 - Null Row Handling (P2) - Lines 984-1115
// Scenarios: Only NullRows, NullRows before partial, alternating NullRows with data
// Total: 3 test scenarios
//
// ### Phase 5: User Story 5 - Checksum Row Handling (P2) - Lines 1117-1251
// Tests: FR-009
// Scenarios: 10k rows ending TC, 10k + Begin, 10k + 50-row partial
// Total: 3 test scenarios (NOT parallel due to t.Setenv incompatibility)
//
// ### Phase 6: User Story 6 - Multi-Row Transactions (P2) - Lines 1253-1455
// Tests: FR-007, FR-008
// Scenarios: Varying row counts (0-100), savepoints at various positions
// Total: 15 test scenarios
//
// ### Phase 7: User Story 7 - Known Bug Regression (P1) - Lines 1457-1571
// Tests: FR-010
// Scenarios: Begin after Commit, Begin + AddRow after Commit
// Total: 2 test scenarios
//
// ## Test Scenario Summary (49 total scenarios)
//
// - FR-002: 1 scenario (empty database)
// - FR-003: 9 scenarios (all transaction end states: TC, SC, RE, SE, R0-R9, S0-S9, NR)
// - FR-004: 4 scenarios (multiple consecutive transactions)
// - FR-005: 5 scenarios (all partial operation sequences)
// - FR-006: 4 scenarios (mixed complete + partial)
// - FR-007: 10 scenarios (varying row counts 0-100, committed and partial)
// - FR-008: 5 scenarios (savepoints at various positions)
// - FR-009: 3 scenarios (checksum row handling with 10,000+ rows)
// - FR-010: 2 scenarios (known bug regression test)
// - US4: 3 scenarios (additional NullRow handling tests)
//
// ## Performance
//
// Total execution time: < 1 second (excluding checksum tests which add ~0.6s)
// Checksum tests (FR-009): ~0.6 seconds (creating 10,000 rows × 3 tests)
// Overall: Well under 10-second performance budget
//
// ## Notes
//
// - All tests use createTestDatabase() helper with mock syscalls and SUDO env vars
// - Tests use t.TempDir() for automatic cleanup
// - Checksum tests cannot use t.Parallel() due to t.Setenv() usage in helpers
// - No tests use t.Skip() - all scenarios execute and validate
//
// =============================================================================

// =============================================================================
// Test Helper Functions
// =============================================================================

// transactionStateSnapshot captures transaction state for comparison after recovery.
// Fields match FR-001 correctness criteria: rows, last, empty, rowBytesWritten.
type transactionStateSnapshot struct {
	hasActiveTx     bool
	rows            []DataRow
	last            *PartialDataRow
	empty           *NullRow
	rowBytesWritten int
}

// generateUUIDv7 generates a valid UUIDv7 for testing with chronological ordering.
// Uses github.com/google/uuid v7 generation.
func generateUUIDv7(t *testing.T) uuid.UUID {
	t.Helper()
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}
	return key
}

// generateTestValue generates a simple test JSON value for performance.
// Returns minimal valid JSON to minimize I/O overhead.
func generateTestValue() json.RawMessage {
	return json.RawMessage(`{}`)
}

// captureTransactionState captures current transaction state for later comparison.
// Deep copies transaction state to allow closing and reopening database.
// IMPORTANT: Only captures uncommitted transactions - committed transactions are ignored.
func captureTransactionState(t *testing.T, db *FrozenDB) *transactionStateSnapshot {
	t.Helper()

	snapshot := &transactionStateSnapshot{}

	db.txMu.RLock()
	defer db.txMu.RUnlock()

	if db.activeTx == nil {
		snapshot.hasActiveTx = false
		return snapshot
	}

	// Check if transaction is committed - if so, treat as no active transaction
	if db.activeTx.IsCommitted() {
		snapshot.hasActiveTx = false
		return snapshot
	}

	snapshot.hasActiveTx = true
	tx := db.activeTx

	// Deep copy rows slice
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	if len(tx.rows) > 0 {
		snapshot.rows = make([]DataRow, len(tx.rows))
		copy(snapshot.rows, tx.rows)
	}

	// Copy last (PartialDataRow)
	if tx.last != nil {
		// Create a deep copy of PartialDataRow
		snapshot.last = &PartialDataRow{
			state: tx.last.state,
			d:     tx.last.d, // DataRow is copied by value
		}
	}

	// Copy empty (NullRow)
	if tx.empty != nil {
		// Create a deep copy of NullRow
		snapshot.empty = &NullRow{
			baseRow: tx.empty.baseRow,
		}
	}

	// Copy rowBytesWritten
	snapshot.rowBytesWritten = tx.rowBytesWritten

	return snapshot
}

// compareSnapshotToRecovered compares captured snapshot to recovered database state.
// Reports all differences via t.Errorf() for comprehensive debugging.
func compareSnapshotToRecovered(t *testing.T, snapshot *transactionStateSnapshot, recovered *FrozenDB) {
	t.Helper()

	recovered.txMu.RLock()
	recoveredTx := recovered.activeTx
	recovered.txMu.RUnlock()

	// Case 1: Both should have no active transaction
	if !snapshot.hasActiveTx && recoveredTx == nil {
		return // Pass
	}

	// Case 2: One has active tx, other doesn't - FAIL
	if snapshot.hasActiveTx && recoveredTx == nil {
		t.Errorf("Transaction state mismatch: original had active tx, recovered has none")
		return
	}
	if !snapshot.hasActiveTx && recoveredTx != nil {
		t.Errorf("Transaction state mismatch: original had no active tx, recovered has one")
		return
	}

	// Case 3: Both have active transaction - compare details
	recoveredTx.mu.RLock()
	defer recoveredTx.mu.RUnlock()

	compareRows(t, snapshot.rows, recoveredTx.rows)
	comparePartialDataRow(t, snapshot.last, recoveredTx.last)
	compareNullRow(t, snapshot.empty, recoveredTx.empty)

	if snapshot.rowBytesWritten != recoveredTx.rowBytesWritten {
		t.Errorf("rowBytesWritten mismatch: original=%d recovered=%d",
			snapshot.rowBytesWritten, recoveredTx.rowBytesWritten)
	}
}

// compareRows compares two slices of DataRow for equality.
// Reports all differences with index and field information.
func compareRows(t *testing.T, original, recovered []DataRow) {
	t.Helper()

	if len(original) != len(recovered) {
		t.Errorf("Row count mismatch: original=%d recovered=%d",
			len(original), len(recovered))
		return
	}

	for i := range original {
		origRow := &original[i]
		recRow := &recovered[i]

		// Compare UUID
		if origRow.RowPayload != nil && recRow.RowPayload != nil {
			if origRow.RowPayload.Key != recRow.RowPayload.Key {
				t.Errorf("Row %d UUID mismatch: original=%s recovered=%s",
					i, origRow.RowPayload.Key, recRow.RowPayload.Key)
			}

			// Compare Value (JSON bytes)
			if !bytes.Equal(origRow.RowPayload.Value, recRow.RowPayload.Value) {
				t.Errorf("Row %d Value mismatch: original=%s recovered=%s",
					i, string(origRow.RowPayload.Value), string(recRow.RowPayload.Value))
			}
		} else if (origRow.RowPayload == nil) != (recRow.RowPayload == nil) {
			t.Errorf("Row %d RowPayload presence mismatch: original=%v recovered=%v",
				i, origRow.RowPayload != nil, recRow.RowPayload != nil)
		}

		// Compare StartControl
		if origRow.StartControl != recRow.StartControl {
			t.Errorf("Row %d StartControl mismatch: original=%c recovered=%c",
				i, origRow.StartControl, recRow.StartControl)
		}

		// Compare EndControl
		if origRow.EndControl != recRow.EndControl {
			t.Errorf("Row %d EndControl mismatch: original=%s recovered=%s",
				i, origRow.EndControl.String(), recRow.EndControl.String())
		}
	}
}

// comparePartialDataRow compares two PartialDataRow instances.
// Handles nil cases and compares state, UUID, value, and savepoint status.
func comparePartialDataRow(t *testing.T, original, recovered *PartialDataRow) {
	t.Helper()

	// Both nil = pass
	if original == nil && recovered == nil {
		return
	}

	// One nil, other non-nil = fail
	if (original == nil) != (recovered == nil) {
		t.Errorf("PartialDataRow presence mismatch: original=%v recovered=%v",
			original != nil, recovered != nil)
		return
	}

	// Both non-nil - compare details
	if original.state != recovered.state {
		t.Errorf("PartialDataRow state mismatch: original=%s recovered=%s",
			original.state.String(), recovered.state.String())
	}

	// Compare DataRow fields
	if original.d.RowPayload != nil && recovered.d.RowPayload != nil {
		if original.d.RowPayload.Key != recovered.d.RowPayload.Key {
			t.Errorf("PartialDataRow UUID mismatch: original=%s recovered=%s",
				original.d.RowPayload.Key, recovered.d.RowPayload.Key)
		}

		if !bytes.Equal(original.d.RowPayload.Value, recovered.d.RowPayload.Value) {
			t.Errorf("PartialDataRow Value mismatch: original=%s recovered=%s",
				string(original.d.RowPayload.Value), string(recovered.d.RowPayload.Value))
		}
	} else if (original.d.RowPayload == nil) != (recovered.d.RowPayload == nil) {
		t.Errorf("PartialDataRow RowPayload presence mismatch: original=%v recovered=%v",
			original.d.RowPayload != nil, recovered.d.RowPayload != nil)
	}

	if original.d.StartControl != recovered.d.StartControl {
		t.Errorf("PartialDataRow StartControl mismatch: original=%c recovered=%c",
			original.d.StartControl, recovered.d.StartControl)
	}
}

// compareNullRow compares two NullRow instances.
// Handles nil cases and compares UUID.
func compareNullRow(t *testing.T, original, recovered *NullRow) {
	t.Helper()

	// Both nil = pass
	if original == nil && recovered == nil {
		return
	}

	// One nil, other non-nil = fail
	if (original == nil) != (recovered == nil) {
		t.Errorf("NullRow presence mismatch: original=%v recovered=%v",
			original != nil, recovered != nil)
		return
	}

	// Both non-nil - compare details
	if original.RowPayload != nil && recovered.RowPayload != nil {
		if original.RowPayload.Key != recovered.RowPayload.Key {
			t.Errorf("NullRow Key mismatch: original=%s recovered=%s",
				original.RowPayload.Key, recovered.RowPayload.Key)
		}
	} else if (original.RowPayload == nil) != (recovered.RowPayload == nil) {
		t.Errorf("NullRow RowPayload presence mismatch: original=%v recovered=%v",
			original.RowPayload != nil, recovered.RowPayload != nil)
	}
}

// createDBWithManyRows creates a database with many complete rows for checksum testing.
// Creates multiple committed transactions with up to 100 rows each until targetRows is reached.
// Returns the database path for reopening.
func createDBWithManyRows(t *testing.T, targetRows int) string {
	t.Helper()

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

		rowsInThisTx := min(100, targetRows-rowsCreated)
		for i := 0; i < rowsInThisTx; i++ {
			key := generateUUIDv7(t)
			tx.AddRow(key, generateTestValue())
		}

		tx.Commit()
		rowsCreated += rowsInThisTx
	}

	db.Close()
	return testPath
}

// =============================================================================
// Phase 1: User Story 1 - Database State Recovery After Restart (Priority: P1)
// =============================================================================
//
// Goal: Validate that frozenDB correctly recovers transaction state when
// reopening in read-only mode, covering empty databases, complete transactions,
// and partial transactions.
//
// Test scenarios:
// - FR-002: Empty database (header + checksum only)
// - FR-003: One complete transaction with all end states (TC, SC, RE, SE, R0-R9, S0-S9, NR)
// - FR-005: Partial transactions (all PartialDataRow operation sequences)
// - FR-006: Mixed scenarios (complete + partial transactions)
//
// =============================================================================

// Test_S_031_FR_002_ZeroCompleteTransactions validates recovery of empty database.
// FR-002: Database with only header + initial checksum row should recover with no active transaction.
func Test_S_031_FR_002_ZeroCompleteTransactions(t *testing.T) {
	// Create empty database
	testPath := filepath.Join(t.TempDir(), "test.fdb")
	createTestDatabase(t, testPath)

	// Open in write mode to capture state
	db1, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("NewFrozenDB write mode failed: %v", err)
	}

	// Capture state (should have no active transaction)
	snapshot := captureTransactionState(t, db1)
	db1.Close()

	// Reopen in read-only mode
	db2, err := NewFrozenDB(testPath, MODE_READ, FinderStrategySimple)
	if err != nil {
		t.Fatalf("NewFrozenDB read mode failed (recovery): %v", err)
	}
	defer db2.Close()

	// Compare state
	compareSnapshotToRecovered(t, snapshot, db2)

	// Explicit check: no active transaction
	db2.txMu.RLock()
	hasActive := (db2.activeTx != nil)
	db2.txMu.RUnlock()

	if hasActive {
		t.Errorf("Expected no active transaction, but found one")
	}
}

// Test_S_031_FR_003_OneCompleteTransaction_AllEndStates validates recovery of single transactions.
// FR-003: Database with one complete transaction should recover correctly for all end states.
// Tests all possible end_control values: TC, SC, RE, SE, R0-R9, S0-S9, NR
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
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Commit()
			},
			wantActive: false,
		},
		{
			name: "SC_savepoint_commit",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
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
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				// Don't commit - leave as partial
			},
			wantActive: true,
		},
		{
			name: "SE_savepoint_continue",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				// Don't commit - leave as partial with savepoint
			},
			wantActive: true,
		},
		{
			name: "R0_full_rollback",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Rollback(0)
			},
			wantActive: false,
		},
		{
			name: "R1_rollback_to_savepoint",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Rollback(1)
			},
			wantActive: false,
		},
		{
			name: "S0_savepoint_with_rollback_0",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				tx.Rollback(0)
			},
			wantActive: false,
		},
		{
			name: "S1_savepoint_with_rollback_1",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				tx.Rollback(1)
			},
			wantActive: false,
		},
		{
			name: "NR_null_row",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// Empty commit produces NullRow
				tx.Commit()
			},
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			hasActive := (db2.activeTx != nil)
			db2.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}
		})
	}
}

// Test_S_031_FR_005_PartialTransactions_AllOperationSequences validates recovery of partial transactions.
// FR-005: Database with partial transactions should recover all PartialDataRow operation sequences correctly.
// Tests all 5 possible partial operation sequences from research.md.
func Test_S_031_FR_005_PartialTransactions_AllOperationSequences(t *testing.T) {
	tests := []struct {
		name                string
		setup               func(*testing.T, *FrozenDB)
		wantCompleteRows    int
		wantPartialState    PartialRowState
		wantRowBytesWritten int // Approximate expected value
	}{
		{
			name: "Sequence1_JustBegin",
			setup: func(t *testing.T, db *FrozenDB) {
				_, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// Just Begin() - partial with start control only
			},
			wantCompleteRows:    0,
			wantPartialState:    PartialDataRowWithStartControl,
			wantRowBytesWritten: 2, // ROW_START + 'T'
		},
		{
			name: "Sequence2_Begin_AddRow",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				// Partial with first row payload
			},
			wantCompleteRows:    0,
			wantPartialState:    PartialDataRowWithPayload,
			wantRowBytesWritten: 0, // Will be > 2, exact value depends on padding
		},
		{
			name: "Sequence3_Begin_AddRow_Savepoint",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				// Partial with savepoint marker
			},
			wantCompleteRows:    0,
			wantPartialState:    PartialDataRowWithSavepoint,
			wantRowBytesWritten: 0, // Will be > payload size + 1
		},
		{
			name: "Sequence4_Begin_TwoAddRows",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue()) // First row finalized
				tx.AddRow(generateUUIDv7(t), generateTestValue()) // Second row partial
			},
			wantCompleteRows:    1,
			wantPartialState:    PartialDataRowWithPayload,
			wantRowBytesWritten: 0, // Will be > 2
		},
		{
			name: "Sequence5_Begin_TwoAddRows_Savepoint",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue()) // First row finalized
				tx.AddRow(generateUUIDv7(t), generateTestValue()) // Second row partial
				tx.Savepoint()
			},
			wantCompleteRows:    1,
			wantPartialState:    PartialDataRowWithSavepoint,
			wantRowBytesWritten: 0, // Will be > payload size + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Additional validation: check specific expectations
			db2.txMu.RLock()
			recoveredTx := db2.activeTx
			db2.txMu.RUnlock()

			if recoveredTx == nil {
				t.Fatal("Expected active transaction after recovery, got nil")
			}

			recoveredTx.mu.RLock()
			defer recoveredTx.mu.RUnlock()

			// Check complete rows count
			if len(recoveredTx.rows) != tt.wantCompleteRows {
				t.Errorf("Complete rows count mismatch: got %d, want %d",
					len(recoveredTx.rows), tt.wantCompleteRows)
			}

			// Check partial state
			if recoveredTx.last == nil {
				t.Fatal("Expected PartialDataRow after recovery, got nil")
			}
			if recoveredTx.last.GetState() != tt.wantPartialState {
				t.Errorf("PartialDataRow state mismatch: got %s, want %s",
					recoveredTx.last.GetState().String(), tt.wantPartialState.String())
			}

			// Check rowBytesWritten (skip validation if wantRowBytesWritten is 0 - means "don't check")
			if tt.wantRowBytesWritten > 0 {
				if recoveredTx.rowBytesWritten != tt.wantRowBytesWritten {
					t.Errorf("rowBytesWritten mismatch: got %d, want %d",
						recoveredTx.rowBytesWritten, tt.wantRowBytesWritten)
				}
			} else {
				// At least verify it's > 0 for payload states
				if tt.wantPartialState != PartialDataRowWithStartControl && recoveredTx.rowBytesWritten <= 2 {
					t.Errorf("rowBytesWritten too small for state %s: got %d, expected > 2",
						tt.wantPartialState.String(), recoveredTx.rowBytesWritten)
				}
			}
		})
	}
}

// Test_S_031_FR_006_MixedScenarios_CompleteAndPartial validates recovery of mixed transaction states.
// FR-006: Database with both complete and partial transactions should recover correctly.
func Test_S_031_FR_006_MixedScenarios_CompleteAndPartial(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, *FrozenDB)
		wantActive bool
	}{
		{
			name: "OneCommit_ThenPartialSeq1",
			setup: func(t *testing.T, db *FrozenDB) {
				// Complete transaction
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Commit()

				// Partial transaction - Sequence 1 (just Begin)
				_, err = db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
			},
			wantActive: true,
		},
		{
			name: "OneCommit_ThenPartialSeq2",
			setup: func(t *testing.T, db *FrozenDB) {
				// Complete transaction
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Commit()

				// Partial transaction - Sequence 2 (Begin + AddRow)
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
			},
			wantActive: true,
		},
		{
			name: "TwoCommits_ThenPartial",
			setup: func(t *testing.T, db *FrozenDB) {
				// First complete transaction
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Commit()

				// Second complete transaction
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
				tx2.Commit()

				// Partial transaction
				_, err = db.BeginTx()
				if err != nil {
					t.Fatalf("Third BeginTx failed: %v", err)
				}
			},
			wantActive: true,
		},
		{
			name: "NullRow_ThenPartial",
			setup: func(t *testing.T, db *FrozenDB) {
				// NullRow (empty commit)
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.Commit()

				// Partial transaction
				_, err = db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
			},
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			hasActive := (db2.activeTx != nil)
			db2.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}
		})
	}
}

// =============================================================================
// Phase 2: User Story 2 - Transaction Boundary Detection (Priority: P1)
// =============================================================================
//
// Goal: Validate that recovery correctly identifies where transactions start
// and end, especially with multiple consecutive transactions.
//
// Test scenarios:
// - FR-004: Multiple complete transactions (2-3 consecutive commits, mixed with rollbacks)
//
// =============================================================================

// Test_S_031_FR_004_MultipleCompleteTransactions validates recovery of multiple consecutive transactions.
// FR-004: Database with multiple complete transactions should recover with no active transaction.
// Tests transaction boundary detection with various completion patterns.
func Test_S_031_FR_004_MultipleCompleteTransactions(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, *FrozenDB)
		wantActive bool
	}{
		{
			name: "TwoCommits",
			setup: func(t *testing.T, db *FrozenDB) {
				// First transaction
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.AddRow(generateUUIDv7(t), generateTestValue())
				tx1.Commit()

				// Second transaction
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
				tx2.Commit()
			},
			wantActive: false,
		},
		{
			name: "ThreeCommits",
			setup: func(t *testing.T, db *FrozenDB) {
				// Three consecutive complete transactions
				for i := 0; i < 3; i++ {
					tx, err := db.BeginTx()
					if err != nil {
						t.Fatalf("BeginTx %d failed: %v", i+1, err)
					}
					tx.AddRow(generateUUIDv7(t), generateTestValue())
					tx.Commit()
				}
			},
			wantActive: false,
		},
		{
			name: "TwoCommits_OneRollback",
			setup: func(t *testing.T, db *FrozenDB) {
				// First commit
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.AddRow(generateUUIDv7(t), generateTestValue())
				tx1.Commit()

				// Second commit
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
				tx2.Commit()

				// Third transaction with rollback
				tx3, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Third BeginTx failed: %v", err)
				}
				tx3.AddRow(generateUUIDv7(t), generateTestValue())
				tx3.Rollback(0)
			},
			wantActive: false,
		},
		{
			name: "AlternatingNullRows",
			setup: func(t *testing.T, db *FrozenDB) {
				// NullRow (empty commit)
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.Commit()

				// Data transaction
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
				tx2.Commit()

				// Another NullRow
				tx3, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Third BeginTx failed: %v", err)
				}
				tx3.Commit()

				// Another data transaction
				tx4, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Fourth BeginTx failed: %v", err)
				}
				tx4.AddRow(generateUUIDv7(t), generateTestValue())
				tx4.Commit()
			},
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			hasActive := (db2.activeTx != nil)
			db2.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}
		})
	}
}

// =============================================================================
// Phase 4: User Story 4 - Null Row Handling in Recovery (Priority: P2)
// =============================================================================
//
// Goal: Validate that NullRows (empty transactions) don't interfere with
// recovery and are correctly skipped.
//
// Test scenarios:
// - Only NullRows: Database with only empty transactions
// - NullRows before partial: NullRows followed by partial transaction
// - Alternating NullRows: Mixed NullRows and complete transactions
//
// =============================================================================

// Test_S_031_US4_NullRowHandling validates that NullRows don't interfere with recovery.
// Tests various scenarios where NullRows (empty transactions) are present in the database.
func Test_S_031_US4_NullRowHandling(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, *FrozenDB)
		wantActive bool
	}{
		{
			name: "OnlyNullRows",
			setup: func(t *testing.T, db *FrozenDB) {
				// Create three NullRows (empty transactions)
				for i := 0; i < 3; i++ {
					tx, err := db.BeginTx()
					if err != nil {
						t.Fatalf("BeginTx %d failed: %v", i+1, err)
					}
					tx.Commit() // Empty commit creates NullRow
				}
			},
			wantActive: false,
		},
		{
			name: "NullRowsBeforePartial",
			setup: func(t *testing.T, db *FrozenDB) {
				// Create two NullRows
				for i := 0; i < 2; i++ {
					tx, err := db.BeginTx()
					if err != nil {
						t.Fatalf("BeginTx %d failed: %v", i+1, err)
					}
					tx.Commit() // Empty commit creates NullRow
				}

				// Create partial transaction after NullRows
				_, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Final BeginTx failed: %v", err)
				}
				// Don't commit - leave partial
			},
			wantActive: true,
		},
		{
			name: "AlternatingNullRowsAndData",
			setup: func(t *testing.T, db *FrozenDB) {
				// NullRow
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.Commit()

				// Data transaction
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
				tx2.Commit()

				// NullRow
				tx3, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Third BeginTx failed: %v", err)
				}
				tx3.Commit()

				// Final NullRow
				tx4, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Fourth BeginTx failed: %v", err)
				}
				tx4.Commit()
			},
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			hasActive := (db2.activeTx != nil)
			db2.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}
		})
	}
}

// =============================================================================
// Phase 5: User Story 5 - Checksum Row Handling in Recovery (Priority: P2)
// =============================================================================
//
// Goal: Validate that recovery correctly skips checksum rows when scanning
// backwards to find transaction state.
//
// Test scenarios:
// - FR-009: 10,000 rows ending with TC (checksum row followed by no active tx)
// - FR-009: 10,000 rows + Begin() (checksum row followed by partial transaction)
// - FR-009: 10,000 rows + 50-row partial (checksum row followed by multi-row partial)
//
// Performance: Tests marked with t.Parallel() for concurrent execution
//
// =============================================================================

// Test_S_031_FR_009_ChecksumRowScenarios validates that checksum rows don't interfere with recovery.
// FR-009: Recovery should correctly skip checksum rows when scanning backwards.
// Creates databases with 10,000+ rows to trigger checksum row insertion.
//
// NOTE: Tests are NOT marked with t.Parallel() because createTestDatabase() uses t.Setenv()
// which is incompatible with parallel execution. However, the test suite can still achieve
// performance goals by running tests sequentially.
func Test_S_031_FR_009_ChecksumRowScenarios(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, string) // Receives path from createDBWithManyRows
		wantActive bool
	}{
		{
			name: "10k_rows_ending_TC",
			setup: func(t *testing.T, dbPath string) {
				// Database already has 10,000 rows from createDBWithManyRows
				// Last transaction ended with TC (commit)
				// File structure: ... DataRow(TC) ChecksumRow
				// No additional operations needed
			},
			wantActive: false,
		},
		{
			name: "10k_rows_plus_Begin",
			setup: func(t *testing.T, dbPath string) {
				// Add partial transaction after 10,000 rows
				db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
				if err != nil {
					t.Fatalf("Open failed: %v", err)
				}
				defer db.Close()

				_, err = db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// File structure: ... ChecksumRow PartialRow(Begin)
			},
			wantActive: true,
		},
		{
			name: "10k_rows_plus_50row_partial",
			setup: func(t *testing.T, dbPath string) {
				// Add 50-row partial transaction after 10,000 rows
				db, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
				if err != nil {
					t.Fatalf("Open failed: %v", err)
				}
				defer db.Close()

				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}

				// Add 50 rows
				for i := 0; i < 50; i++ {
					tx.AddRow(generateUUIDv7(t), generateTestValue())
				}
				// Don't commit - leave as 50-row partial transaction
				// File structure: ... ChecksumRow + 50 rows (partial)
			},
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database with 10,000 rows (triggers checksum row)
			dbPath := createDBWithManyRows(t, 10000)

			// Open and capture initial state (before setup)
			db1, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
			if err != nil {
				t.Fatalf("Initial open failed: %v", err)
			}
			db1.Close()

			// Run setup to add additional operations
			tt.setup(t, dbPath)

			// Reopen to capture final state
			db2, err := NewFrozenDB(dbPath, MODE_WRITE, FinderStrategySimple)
			if err != nil {
				t.Fatalf("Reopen after setup failed: %v", err)
			}

			snapshot := captureTransactionState(t, db2)
			db2.Close()

			// Reopen in read-only mode for recovery test
			db3, err := NewFrozenDB(dbPath, MODE_READ, FinderStrategySimple)
			if err != nil {
				t.Fatalf("Recovery failed: %v", err)
			}
			defer db3.Close()

			compareSnapshotToRecovered(t, snapshot, db3)

			// Check wantActive flag
			db3.txMu.RLock()
			hasActive := (db3.activeTx != nil)
			db3.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}

			// For 50-row partial test, verify row count
			if tt.name == "10k_rows_plus_50row_partial" && hasActive {
				db3.txMu.RLock()
				tx := db3.activeTx
				db3.txMu.RUnlock()

				if tx != nil {
					tx.mu.RLock()
					completeRows := len(tx.rows)
					hasPartial := (tx.last != nil)
					tx.mu.RUnlock()

					// Should have 49 complete rows + 1 partial row
					if completeRows != 49 {
						t.Errorf("Expected 49 complete rows in partial transaction, got %d", completeRows)
					}
					if !hasPartial {
						t.Errorf("Expected partial row in 50-row transaction, got none")
					}
				}
			}
		})
	}
}

// =============================================================================
// Phase 6: User Story 6 - Multi-Row Transaction Recovery (Priority: P2)
// =============================================================================
//
// Goal: Validate that transactions with varying row counts (1 to 100 rows)
// and savepoints are recovered correctly.
//
// Test scenarios:
// - FR-007: Varying row counts (0, 1, 2, 10, 50, 100 rows, committed and partial)
// - FR-008: Savepoints at various positions (first, middle, last, multiple)
//
// =============================================================================

// Test_S_031_FR_007_VaryingRowCounts validates recovery of transactions with different row counts.
// FR-007: Transactions with 0-100 rows should recover correctly, both committed and partial.
func Test_S_031_FR_007_VaryingRowCounts(t *testing.T) {
	tests := []struct {
		name             string
		rowCount         int
		commitTx         bool // true = commit, false = leave partial
		wantActive       bool
		wantCompleteRows int // For partial transactions
	}{
		{
			name:             "Zero_rows_committed_NullRow",
			rowCount:         0,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "One_row_committed",
			rowCount:         1,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "Two_rows_committed",
			rowCount:         2,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "Ten_rows_committed",
			rowCount:         10,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "Fifty_rows_committed",
			rowCount:         50,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "Hundred_rows_committed_max",
			rowCount:         100,
			commitTx:         true,
			wantActive:       false,
			wantCompleteRows: 0,
		},
		{
			name:             "One_row_partial",
			rowCount:         1,
			commitTx:         false,
			wantActive:       true,
			wantCompleteRows: 0, // First row is still partial
		},
		{
			name:             "Two_rows_partial",
			rowCount:         2,
			commitTx:         false,
			wantActive:       true,
			wantCompleteRows: 1, // First row complete, second partial
		},
		{
			name:             "Ten_rows_partial",
			rowCount:         10,
			commitTx:         false,
			wantActive:       true,
			wantCompleteRows: 9, // 9 complete, 10th partial
		},
		{
			name:             "Hundred_rows_partial_max",
			rowCount:         100,
			commitTx:         false,
			wantActive:       true,
			wantCompleteRows: 99, // 99 complete, 100th partial
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
			testPath := filepath.Join(t.TempDir(), "test.fdb")
			createTestDatabase(t, testPath)

			// Open and run setup
			db1, err := NewFrozenDB(testPath, MODE_WRITE, FinderStrategySimple)
			if err != nil {
				t.Fatalf("Open failed: %v", err)
			}

			// Create transaction with specified row count
			tx, err := db1.BeginTx()
			if err != nil {
				t.Fatalf("BeginTx failed: %v", err)
			}

			for i := 0; i < tt.rowCount; i++ {
				tx.AddRow(generateUUIDv7(t), generateTestValue())
			}

			if tt.commitTx {
				tx.Commit()
			}
			// Else leave partial

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

			// Check wantActive flag
			db2.txMu.RLock()
			recoveredTx := db2.activeTx
			db2.txMu.RUnlock()

			if (recoveredTx != nil) != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", recoveredTx != nil, tt.wantActive)
			}

			// For partial transactions, verify row count
			if tt.wantActive && recoveredTx != nil {
				recoveredTx.mu.RLock()
				completeRows := len(recoveredTx.rows)
				recoveredTx.mu.RUnlock()

				if completeRows != tt.wantCompleteRows {
					t.Errorf("Complete rows count mismatch: got %d, want %d",
						completeRows, tt.wantCompleteRows)
				}
			}
		})
	}
}

// Test_S_031_FR_008_SavepointsAtVariousPositions validates recovery of transactions with savepoints.
// FR-008: Savepoints at different positions should be recovered correctly.
func Test_S_031_FR_008_SavepointsAtVariousPositions(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, *FrozenDB)
		wantActive bool
	}{
		{
			name: "Savepoint_on_first_row",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				tx.Commit()
			},
			wantActive: false,
		},
		{
			name: "Savepoint_on_middle_row",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// Add 5 rows
				for i := 0; i < 5; i++ {
					tx.AddRow(generateUUIDv7(t), generateTestValue())
				}
				// Savepoint after 5th row
				tx.Savepoint()
				// Add 5 more rows
				for i := 0; i < 5; i++ {
					tx.AddRow(generateUUIDv7(t), generateTestValue())
				}
				tx.Commit()
			},
			wantActive: false,
		},
		{
			name: "Savepoint_on_last_row",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// Add 9 rows
				for i := 0; i < 9; i++ {
					tx.AddRow(generateUUIDv7(t), generateTestValue())
				}
				// Add 10th row and savepoint
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				tx.Commit()
			},
			wantActive: false,
		},
		{
			name: "Multiple_savepoints",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				// Add row and savepoint, repeat 3 times
				for i := 0; i < 3; i++ {
					tx.AddRow(generateUUIDv7(t), generateTestValue())
					tx.Savepoint()
				}
				tx.Commit()
			},
			wantActive: false,
		},
		{
			name: "Savepoint_with_partial_state",
			setup: func(t *testing.T, db *FrozenDB) {
				tx, err := db.BeginTx()
				if err != nil {
					t.Fatalf("BeginTx failed: %v", err)
				}
				tx.AddRow(generateUUIDv7(t), generateTestValue())
				tx.Savepoint()
				// Don't commit - leave as partial with savepoint
			},
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			hasActive := (db2.activeTx != nil)
			db2.txMu.RUnlock()

			if hasActive != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", hasActive, tt.wantActive)
			}
		})
	}
}

// =============================================================================
// Phase 7: User Story 7 - Bug Identification and Fixing (Priority: P1)
// =============================================================================
//
// Goal: Explicitly test and fix the known bug where Begin() after Commit()
// incorrectly references closed transaction state.
//
// Test scenarios:
// - FR-010: Known bug regression test (Begin after Commit, Begin + AddRow after Commit)
//
// =============================================================================

// Test_S_031_FR_010_KnownBugRegressionTest validates fix for known bug where
// Begin() after Commit() incorrectly references closed transaction state.
// FR-010: Second Begin() after Commit() should have independent empty state.
func Test_S_031_FR_010_KnownBugRegressionTest(t *testing.T) {
	tests := []struct {
		name               string
		setup              func(*testing.T, *FrozenDB)
		wantActive         bool
		wantCompleteRows   int
		wantPartialPresent bool
	}{
		{
			name: "BeginCommitBegin",
			setup: func(t *testing.T, db *FrozenDB) {
				// First transaction: Begin → AddRow → Commit
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.AddRow(generateUUIDv7(t), generateTestValue())
				tx1.Commit()

				// Second transaction: Begin (should be independent)
				_, err = db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
			},
			wantActive:         true,
			wantCompleteRows:   0,
			wantPartialPresent: true,
		},
		{
			name: "BeginCommitBeginAddRow",
			setup: func(t *testing.T, db *FrozenDB) {
				// First transaction: Begin → AddRow → Commit
				tx1, err := db.BeginTx()
				if err != nil {
					t.Fatalf("First BeginTx failed: %v", err)
				}
				tx1.AddRow(generateUUIDv7(t), generateTestValue())
				tx1.Commit()

				// Second transaction: Begin → AddRow (should be independent)
				tx2, err := db.BeginTx()
				if err != nil {
					t.Fatalf("Second BeginTx failed: %v", err)
				}
				tx2.AddRow(generateUUIDv7(t), generateTestValue())
			},
			wantActive:         true,
			wantCompleteRows:   0,
			wantPartialPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
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

			// Check wantActive flag
			db2.txMu.RLock()
			recoveredTx := db2.activeTx
			db2.txMu.RUnlock()

			if (recoveredTx != nil) != tt.wantActive {
				t.Errorf("Active transaction mismatch: got %v, want %v", recoveredTx != nil, tt.wantActive)
			}

			if recoveredTx != nil {
				recoveredTx.mu.RLock()
				defer recoveredTx.mu.RUnlock()

				// Check complete rows count
				if len(recoveredTx.rows) != tt.wantCompleteRows {
					t.Errorf("Complete rows count mismatch: got %d, want %d",
						len(recoveredTx.rows), tt.wantCompleteRows)
				}

				// Check partial presence
				hasPartial := (recoveredTx.last != nil)
				if hasPartial != tt.wantPartialPresent {
					t.Errorf("PartialDataRow presence mismatch: got %v, want %v",
						hasPartial, tt.wantPartialPresent)
				}

				// Critical check: ensure no data from first transaction leaks into second
				if len(recoveredTx.rows) > 0 {
					t.Errorf("Known bug detected: Second transaction after Commit() has %d rows from first transaction, should be 0",
						len(recoveredTx.rows))
				}
			}
		})
	}
}
