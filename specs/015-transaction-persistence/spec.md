# Feature Specification: Transaction File Persistence

**Feature Branch**: `015-transaction-persistence` **Created**: 2026-01-21
**Status**: Draft **Input**: User description: "Add file persistence to the
Transaction struct, via the FileManager. When a user calls Begin(), the
Transaction struct writes the PartialDataRow to disk. When the user calls
AddRow, any previous PartialDataRow is written to disk, then a new
PartialDataRow is written. When Commit is called, either the NullRow, or the
final data row is written to disk. For this spec, the Transaction can assume
that the total database size is less than 10,000 rows, and thus checksum rows
aren't needed. Checksum ability will be added in a later spec. The Transaction
code can assume the file is valid before opening, consisting of a header, the
checksum row, and some number of full, finalized rows before initializing.
Remember that the FileManager is append-only so the transaction must take care
to only write new bytes. The transaction should wait for the write to complete
as part of the user operation (such as AddRow) and throw an error upon failure"

## User Scenarios & Testing

### User Story 1 - Begin Transaction with Partial Data Row (Priority: P1)

A developer using frozenDB wants to start a transaction. When they call the
Begin() method, the Transaction struct must immediately write a PartialDataRow
to disk to indicate the start of an in-progress transaction.

**Why this priority**: This is the foundation of transaction persistence.
Without properly writing the PartialDataRow on Begin(), there is no record that
a transaction has started, which is essential for recovery and integrity.

**Independent Test**: Can be tested by calling Begin() and verifying that a
PartialDataRow is appended to the database file. Delivers value by ensuring
transaction state is captured for recovery.

**Acceptance Scenarios**:

1. **Given** a valid database file with header and finalized rows, **When** a
   user calls Begin(), **Then** a PartialDataRow is appended to the file
2. **Given** the database file is at the end of existing data, **When** Begin()
   is called, **Then** the PartialDataRow is written at the correct position
   following the last finalized row
3. **Given** a write operation fails, **When** Begin() is called, **Then** an
   error is thrown and no PartialDataRow is written

---

### User Story 2 - Add Row Updates Partial Data Rows (Priority: P1)

A developer wants to add rows to an in-progress transaction. When they call
AddRow(), the Transaction must first write the previous PartialDataRow (if one
exists) to disk, then write a new PartialDataRow representing the added row.

**Why this priority**: This is the core operation for building transactions.
Each AddRow() must persist the transaction state incrementally, allowing
recovery if the system crashes before commit.

**Independent Test**: Can be tested by calling AddRow() multiple times and
verifying that each call results in a new PartialDataRow on disk. Delivers value
by enabling transaction building with persistence.

**Acceptance Scenarios**:

1. **Given** an active transaction with a PartialDataRow on disk, **When**
   AddRow() is called, **Then** the previous PartialDataRow is written as
   finalized and a new PartialDataRow is appended
2. **Given** an active transaction, **When** AddRow() is called and the write
   fails, **Then** an error is thrown and the transaction state remains
   unchanged
3. **Given** an active transaction, **When** AddRow() is called, **Then** the
   write completes synchronously before returning

---

### User Story 3 - Commit Transaction with Final Row (Priority: P1)

A developer wants to finalize a transaction. When they call Commit(), the
Transaction must write either a NullRow (if the transaction has no data rows) or
the final data row (if rows were added) to disk to complete the transaction.

**Why this priority**: This is the completion of the transaction lifecycle.
Without proper commit handling, transactions cannot be finalized and the
database remains in an in-progress state.

**Independent Test**: Can be tested by calling Commit() and verifying the
correct row type (NullRow or data row) is appended. Delivers value by enabling
transaction completion and database state finalization.

**Acceptance Scenarios**:

1. **Given** an active transaction with added rows, **When** Commit() is called,
   **Then** the final data row is written to disk
2. **Given** an active transaction with no added rows, **When** Commit() is
   called, **Then** the current PartialDataRow (from Begin()) is written as a
   NullRow to disk, resulting in exactly one row
3. **Given** an active transaction, **When** Commit() is called and the write
   fails, **Then** an error is thrown and the transaction remains incomplete

---

### User Story 4 - Concurrent Transaction Operations (Priority: P1)

A developer wants to use a single Transaction instance from multiple goroutines
safely. When they call Begin(), AddRow(), Commit(), or other Transaction methods
concurrently from different goroutines, the Transaction struct must handle
synchronization correctly without data races or corruption.

**Why this priority**: Go is a concurrent language and users may call
Transaction methods from multiple goroutines. Thread-safety is essential for
correct operation and prevents data races that could corrupt transaction state.

**Independent Test**: Can be tested by spawning multiple goroutines that call
Transaction methods concurrently with proper synchronization primitives to
verify operations complete safely. Delivers value by ensuring Transaction is
safe for concurrent use.

**Acceptance Scenarios**:

1. **Given** an inactive Transaction, **When** multiple goroutines call Begin()
   concurrently, **Then** only one goroutine succeeds and others receive
   InvalidActionError
2. **Given** an active Transaction, **When** multiple goroutines call AddRow()
   concurrently with different keys, **Then** all rows are added sequentially
   without data races and transaction state remains consistent
3. **Given** an active Transaction, **When** one goroutine calls AddRow() while
   another calls Commit() concurrently, **Then** both operations complete safely
   with proper synchronization

---

### Edge Cases

- What happens when the FileManager write operation times out or encounters a
  disk error?
- How does the system handle if the transaction attempts to write beyond the
  assumed 10,000 row limit?
- What happens if Begin() is called twice without Commit() or rollback between
  calls?
- How does the system handle concurrent transactions on the same database file?
- What happens if a goroutine calls Begin() while another goroutine is in the
  middle of AddRow()?

## Requirements

### Functional Requirements

- **FR-001**: When Begin() is called on a Transaction, the system MUST write a
  PartialDataRow to the database file via the FileManager
- **FR-002**: When AddRow() is called on an active Transaction, the system MUST
  write the previous PartialDataRow (if exists) to disk as a finalized row, then
  write a new PartialDataRow
- **FR-003**: When Commit() is called on a Transaction with added rows, the
  system MUST write the final data row to disk via the FileManager
- **FR-004**: When Commit() is called on a Transaction with no added rows, the
  system MUST write the current PartialDataRow (created by Begin()) as a NullRow
  to disk via the FileManager, resulting in exactly one row
- **FR-005**: All write operations (Begin, AddRow, Commit) MUST complete
  synchronously before the operation returns to the caller
- **FR-006**: If any write operation fails, the system MUST tombstone the
  transaction and return the write error. All subsequent public API calls on the
  tombstoned transaction MUST return TombstonedError
- **FR-007**: The Transaction MUST only append new bytes to the database file
  (no modification of existing data)
- **FR-008**: The Transaction MUST assume the database file is valid on
  initialization (header, checksum row, and finalized rows present)
- **FR-009**: The Transaction MUST NOT write checksum rows (assumes database <
  10,000 rows)
- **FR-010**: Transaction methods (Begin, AddRow, Commit, Rollback, Savepoint)
  MUST be thread-safe when called concurrently from multiple goroutines on the
  same Transaction instance

### Key Entities

- **Transaction**: Represents an in-progress database operation that can add
  rows and must be committed, handles persistence of transaction state via
  FileManager
- **PartialDataRow**: Represents an incomplete transaction row written to disk
  during Begin() and AddRow() operations, indicates transaction is in-progress
- **NullRow**: Represents an empty transaction (no rows added), written to disk
  on Commit() when no data rows exist; the PartialDataRow created by Begin() is
  finalized as a NullRow, resulting in exactly one row
- **FileManager**: Append-only file interface used by Transaction to write rows
  to the database file

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:

- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the
  implementation file being tested
- Follow naming convention `Test_S_015_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All Transaction operations (Begin, AddRow, Commit) persist the
  correct row type to disk in 100% of test scenarios
- **SC-002**: Write operations complete synchronously before returning to the
  caller in all test cases
- **SC-003**: All write failure scenarios correctly throw errors without
  persisting partial data
- **SC-004**: Transactions with varying numbers of rows (0 to 100) correctly
  write NullRow or final data row on Commit()

### Data Integrity & Correctness Metrics

- **SC-005**: Zero data loss scenarios in transaction persistence tests
- **SC-006**: All Transaction operations maintain append-only file semantics (no
  modification of existing bytes)
- **SC-007**: No checksum rows are written in any test scenario (verifying
  <10,000 row assumption)
- **SC-008**: Transaction state remains consistent when write operations fail
- **SC-009**: Concurrent Transaction method calls complete safely without data
  races or corruption in all test scenarios
