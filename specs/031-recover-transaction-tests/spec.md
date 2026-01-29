# Feature Specification: recoverTransaction Correctness Test Suite

**Feature Branch**: `031-recover-transaction-tests`  
**Created**: 2026-01-29  
**Status**: Draft  
**Input**: User description: "031 recoverTransaction-correctness We need to add an extensive test suite for recoverTransaction to make sure it round-trips properly, as there is already one major blocking bug in the implementation. The test suite should create a database, perform some actions. Then, create a second frozendb instance in read-only mode (so they can operate in parallel) and recover the current state. The in-memory state of db1 and db2 should be equivalent. This test suite should be exhaustive, covering all full and partial states with zero rows, 1 row, two rows, zero transactions, one transaction, two transactions, all null rows and more. The list here is not meant to be prescriptive or exhaustive just to help better define what exhaustive is. There are definitely bugs in the implementation, such as: Begin() AddRow() Commit() Begin() causes the activeTx to incorrectly look at the first closed trasaction, not the partial data row for the new transaction. All bugs found after running the exhaustive tests should be fixed"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Database State Recovery After Restart (Priority: P1)

Database developers need to ensure that when a frozenDB instance reopens a file in read-only mode, it correctly recovers the exact transaction state that was present at the time of the previous write, including any partial (uncommitted) transaction data. This ensures data consistency and allows concurrent read-only access while writes are in progress.

**Why this priority**: Core correctness requirement for data integrity. Without proper recovery, databases could lose transaction state, corrupt in-memory structures, or provide inconsistent views of data across multiple instances.

**Independent Test**: Can be fully tested by creating a database with specific transaction states, closing it, reopening in read-only mode, and comparing the in-memory state of both instances. Delivers the value of validated state recovery mechanism.

**Acceptance Scenarios**:

1. **Given** a database with zero complete transactions, **When** reopened in read-only mode, **Then** the recovered instance has no active transaction and empty state
2. **Given** a database with one complete committed transaction, **When** reopened in read-only mode, **Then** the recovered instance has no active transaction and the committed data is visible
3. **Given** a database with a partial (uncommitted) transaction containing one row, **When** reopened in read-only mode, **Then** the recovered instance has an active transaction with exactly one row in memory
4. **Given** a database with multiple complete transactions and a partial transaction, **When** reopened in read-only mode, **Then** the recovered instance has the correct active transaction state with all rows

---

### User Story 2 - Transaction Boundary Detection (Priority: P1)

Database developers need the recovery mechanism to correctly identify transaction boundaries (where one transaction ends and another begins), especially when the file ends with a partial transaction or when multiple transactions exist with various ending states (commit, rollback, continue).

**Why this priority**: Incorrect boundary detection leads to data corruption or loss. The system must differentiate between closed transactions (TC, SC, R0-R9, S0-S9) and open transactions (RE, SE) to recover the correct state.

**Independent Test**: Can be tested by creating databases with various transaction ending patterns, reopening them, and verifying the transaction count and state boundaries match expectations.

**Acceptance Scenarios**:

1. **Given** a database ending with a complete committed transaction (TC), **When** reopened, **Then** no active transaction exists
2. **Given** a database ending with a transaction marked continue (RE), **When** reopened, **Then** an active transaction exists with all rows from the transaction start
3. **Given** a database ending with a rollback to savepoint (R1), **When** reopened, **Then** no active transaction exists (transaction was closed by rollback)
4. **Given** a database with two complete transactions followed by a partial third transaction, **When** reopened, **Then** only the third transaction is active

---

### User Story 3 - Partial Data Row State Recovery (Priority: P1)

Database developers need the recovery mechanism to correctly handle PartialDataRow states (State 1: start control only, State 2: with key-value data, State 3: with savepoint intent marker 'S'), ensuring the recovered transaction can continue writing from the exact byte position where it stopped.

**Why this priority**: PartialDataRows represent in-progress writes. Incorrect recovery can lead to double-writing bytes, skipping bytes, or corrupting the row structure. This is critical for write resumption.

**Independent Test**: Can be tested by creating databases with each of the three PartialDataRow states, reopening them, and verifying the recovered transaction's internal state (rowBytesWritten, partial row contents, state).

**Acceptance Scenarios**:

1. **Given** a database ending with a PartialDataRow in State 1 (only ROW_START + start_control), **When** reopened, **Then** the active transaction has a partial row with 2 bytes written
2. **Given** a database ending with a PartialDataRow in State 2 (with UUID and JSON), **When** reopened, **Then** the active transaction has a partial row with the correct number of bytes written including padding
3. **Given** a database ending with a PartialDataRow in State 3 (with 'S' marker), **When** reopened, **Then** the active transaction has a partial row with savepoint intent marked
4. **Given** a database with zero complete rows and a PartialDataRow, **When** reopened, **Then** the active transaction exists with no complete rows and the partial row

---

### User Story 4 - Null Row Handling in Recovery (Priority: P2)

Database developers need the recovery mechanism to correctly handle NullRows (empty transactions created by Begin() followed immediately by Commit()), ensuring they don't interfere with transaction state detection or boundary identification.

**Why this priority**: NullRows are single-row transactions with special handling. While less common than regular data rows, incorrect handling could cause the recovery algorithm to misidentify transaction boundaries.

**Independent Test**: Can be tested by creating databases with various NullRow patterns (all NullRows, NullRows interspersed with data transactions, NullRows before partial transactions) and verifying correct recovery.

**Acceptance Scenarios**:

1. **Given** a database with only NullRows, **When** reopened, **Then** no active transaction exists
2. **Given** a database with NullRows followed by a partial transaction, **When** reopened, **Then** the active transaction does not include the NullRows
3. **Given** a database alternating between NullRows and complete data transactions, **When** reopened, **Then** no active transaction exists

---

### User Story 5 - Checksum Row Handling in Recovery (Priority: P2)

Database developers need the recovery mechanism to correctly skip checksum rows when identifying the last data row and transaction state, since checksum rows appear every 10,000 complete data rows and are not part of any transaction.

**Why this priority**: Checksum rows can appear as the last row in the file if exactly 10,000 data rows have been written. The recovery algorithm must look past checksum rows to find the actual last data row for state determination.

**Independent Test**: Can be tested by creating databases where the last row is a checksum row (with the previous row being various transaction states) and verifying correct recovery.

**Acceptance Scenarios**:

1. **Given** a database where the last row is a checksum row and the previous row ends with TC, **When** reopened, **Then** no active transaction exists
2. **Given** a database where the last row is a checksum row and the previous row ends with RE, **When** reopened, **Then** an active transaction exists with the correct rows
3. **Given** a database with multiple checksum rows within a large transaction, **When** reopened, **Then** the active transaction includes all data rows, skipping checksum rows

---

### User Story 6 - Multi-Row Transaction Recovery (Priority: P2)

Database developers need the recovery mechanism to correctly reconstruct transactions containing multiple rows (up to the maximum of 100 rows), including transactions with savepoints, ensuring all rows are present in the correct order.

**Why this priority**: Most real-world transactions contain multiple rows. The recovery algorithm must read backwards up to 101 rows (100 data + 1 potential checksum) to find the transaction start marker.

**Independent Test**: Can be tested by creating databases with transactions of varying row counts (2, 10, 50, 100 rows) with different ending states, reopening them, and verifying all rows are recovered.

**Acceptance Scenarios**:

1. **Given** a database with a 2-row partial transaction, **When** reopened, **Then** the active transaction contains exactly 2 rows
2. **Given** a database with a 50-row partial transaction with savepoints, **When** reopened, **Then** the active transaction contains exactly 50 rows with correct savepoint positions
3. **Given** a database with a 100-row partial transaction, **When** reopened, **Then** the active transaction contains exactly 100 rows
4. **Given** a database with a 100-row complete transaction followed by a 1-row partial transaction, **When** reopened, **Then** the active transaction contains only 1 row (not 101)

---

### User Story 7 - Bug Identification and Fixing (Priority: P1)

Database developers need the test suite to identify existing bugs in the recoverTransaction implementation (such as the known bug where Begin() after a committed transaction incorrectly references the closed transaction's state) and validate that fixes resolve these issues.

**Why this priority**: This is the primary motivation for creating the test suite. Existing bugs must be caught and fixed to ensure correctness. The known bug specifically affects the scenario: Begin() → AddRow() → Commit() → Begin(), which is a common pattern.

**Independent Test**: Can be tested by creating a database, committing a transaction, starting a new transaction, and verifying that the new transaction's state is independent and correct (not referencing the old transaction).

**Acceptance Scenarios**:

1. **Given** a database with the sequence Begin() → AddRow() → Commit() → Begin(), **When** reopened in read-only mode, **Then** the active transaction is the second Begin() with correct empty state
2. **Given** a database with the sequence Begin() → AddRow() → Commit() → Begin() → AddRow(), **When** the new AddRow() is executed, **Then** it creates a new transaction without referencing the old committed transaction
3. **Given** any identified bug from the exhaustive test suite, **When** the bug is fixed, **Then** the corresponding test passes

---

### Edge Cases

- What happens when the file ends exactly at a row boundary with a closed transaction (TC, SC, R0-R9, S0-S9)?
- What happens when the file ends with multiple consecutive checksum rows (edge case scenario)?
- What happens when a transaction is exactly 100 rows and ends as a partial transaction?
- What happens when the database is empty (only header + initial checksum row)?
- What happens when attempting to recover a database file with a size that doesn't align to row boundaries (PartialDataRow present)?
- What happens when recovering a database with alternating transaction types (data transactions, null transactions, partial transactions)?
- What happens when a partial transaction ends with a savepoint intent ('S' marker in State 3)?
- What happens when reopening a database with a partial transaction and then calling Begin() again (should fail)?

## Requirements *(mandatory)*

### Functional Requirements

#### Correctness Definition

- **FR-001**: Recovery correctness is defined as: when a database file is reopened in read-only mode, the recovered in-memory transaction state MUST be identical to the original state at the time of the last write. Specifically:
  - If the original had no active transaction, the recovered instance MUST have no active transaction
  - If the original had an active transaction, the recovered instance MUST have an active transaction with:
    - Identical `rows` slice (same count, same DataRow contents in same order)
    - Identical `last` PartialDataRow (same state, same contents, same rowBytesWritten count)
    - Identical `empty` NullRow (if present)
    - All transaction metadata correctly restored (savepoint counts, transaction boundaries)

**Note**: All subsequent requirements define specific test scenarios that MUST verify recovery correctness per FR-001. The test pattern is: create database (db1) in write mode, perform operations, reopen same file in read-only mode (db2), verify FR-001 correctness criteria.

#### Test Coverage - Transaction States

- **FR-002**: Test suite MUST cover zero complete transactions (empty database after initial checksum)
- **FR-003**: Test suite MUST cover exactly one complete transaction with all end states (TC, SC, R0-R9, S0-S9, NR)
- **FR-004**: Test suite MUST cover multiple complete transactions (2+ consecutive transactions)
- **FR-005**: Test suite MUST cover partial transactions (file ending with open transaction in RE or SE state) with all three PartialDataRow states (State 1, 2, 3)
- **FR-006**: Test suite MUST cover mixed scenarios (complete transactions followed by partial transaction)

#### Test Coverage - Row Counts

- **FR-007**: Test suite MUST cover transactions with varying row counts: zero rows (NullRow only), 1 row, 2 rows, 10 rows, 50 rows, 100 rows
- **FR-008**: Test suite MUST cover transactions with savepoints at various positions (first row, middle row, last row)

#### Test Coverage - Special Cases

- **FR-009**: Test suite MUST cover databases where the last complete row is a checksum row (with previous row in various end states)
- **FR-010**: Test suite MUST include an explicit regression test for the known bug scenario: Begin() → AddRow() → Commit() → Begin() (where the second Begin() creates a partial transaction that must have independent state from the closed first transaction)

### Key Entities *(include if feature involves data)*

- **Database Instance (db1)**: The original frozenDB instance opened in write mode that performs transaction operations and writes data to disk
- **Recovered Database Instance (db2)**: A second frozenDB instance opened in read-only mode that recovers transaction state from the same file
- **Transaction State**: The in-memory representation of a transaction, including completed rows (rows slice), partial row (last), empty row (empty), and metadata (rowBytesWritten)
- **PartialDataRow**: An incomplete row at the end of a file representing in-progress transaction data, existing in one of three states
- **Recovery Algorithm**: The recoverTransaction function that reads the end of a database file and reconstructs the transaction state

### Spec Testing Requirements

Each functional requirement (FR-002 through FR-010) defines a test scenario that MUST have corresponding spec tests that:
- Validate the specific scenario as specified
- Apply the correctness definition from FR-001 to verify recovery
- Are placed in a new file `internal/frozendb/recovery_spec_test.go`
- Follow naming convention `Test_S_031_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

Note: FR-001 defines the correctness criteria. Each test for FR-002 through FR-010 creates a specific database scenario and then applies FR-001's correctness verification.

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All test cases pass, demonstrating that database state recovery produces identical in-memory structures between write and read-only instances
- **SC-002**: The known bug (Begin() after Commit() referencing old transaction) is identified by at least one failing test before the fix
- **SC-003**: After bug fixes are applied, 100% of test cases pass without modification to the test suite
- **SC-004**: Test suite executes in under 10 seconds for all test cases combined (performance baseline for CI/CD)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data loss scenarios - all complete and partial transaction data is correctly recovered in 100% of test cases
- **SC-006**: Concurrent read/write operations maintain consistency - read-only instances can open files while write instances are active (file locking permits this)
- **SC-007**: Memory usage for recovered transactions matches the original transaction's memory footprint (±5% tolerance for internal allocations)
- **SC-008**: Transaction atomicity is preserved - partial transactions are correctly identified as active/uncommitted, complete transactions are correctly identified as closed
- **SC-009**: Transaction boundary detection is 100% accurate across all transaction end states (TC, SC, RE, SE, R0-R9, S0-S9, NR)
- **SC-010**: PartialDataRow state recovery is byte-accurate in all three states, allowing write resumption without data corruption

## Assumptions *(optional)*

- The frozenDB file format specification (v1_file_format.md) is correctly implemented for all transaction operations except recoverTransaction
- Read-only mode correctly prevents writes and allows concurrent access to the file while a write-mode instance is active
- The Transaction struct's internal fields (rows, last, empty, rowBytesWritten) are the authoritative representation of transaction state
- The test suite can access internal Transaction state for comparison (exported fields or test helpers)
- FileManager correctly maintains file locks and allows multiple read-only instances
- The parseTransactionRows function (called by recoverTransaction) correctly parses transaction rows from byte slices

## Dependencies *(optional)*

- Existing Transaction implementation (Begin, AddRow, Commit, Rollback, Savepoint methods)
- FileManager with read-only mode support
- PartialDataRow with all three state implementations
- DataRow, NullRow, and ChecksumRow implementations
- Header and row size calculations
- Finder interface for timestamp tracking (if needed during recovery)

## Out of Scope *(optional)*

- Performance optimizations for recovery of very large transactions (>100 rows is not possible per file format spec)
- Recovery from corrupted database files (corruption detection is separate from state recovery)
- Recovery when file locks are held or file permissions prevent access
- Modifications to the file format specification itself
- Changes to transaction semantics or limits (100 row maximum, savepoint limits, etc.)
- Write-mode recovery (the test focuses on read-only mode recovery, though write-mode recovery uses the same mechanism)

## Open Questions *(optional)*

None - the user description provides clear requirements and the file format specification defines all necessary behavior.
