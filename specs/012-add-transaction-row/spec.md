# Feature Specification: AddRow Transaction Implementation

**Feature Branch**: `012-add-transaction-row`  
**Created**: 2026-01-19  
**Status**: Draft  
**Input**: User description: "Implement AddRow() for a transaction. AddRow can only be called once a transaction has had Begin() called, can be called zero or more times, and then Commit() is called. Since the end_control is not known when AddRow() is called, each AddRow should finalize any previous PartialDataRow, move the previous partial row over to rows[], and then construct the current row as the new PartialDataRow. Commit() must finalize the last PartialDataRow. Take care to not break the scenario with Begin(), Commit() which will produce a NullRow instead."

## Clarifications

### Session 2026-01-19

- Q: What should be the behavior when max_timestamp tracking needs to persist across transaction boundaries for UUID ordering? → A: The transaction will receive the current max_timestamp when it is initialized, and it should update its own copy of max_timestamp as more rows are added
- Q: How should AddRow() handle max_timestamp when the database is empty (no existing rows)? → A: Use zero baseline
- Q: What error message should be returned when UUID timestamp violates ordering constraints? → A: It should be its own error type. Let's go with KeyOrderingError

## Breaking Changes

### Spec 011 FR-004 Amendment

**Original Requirement (Spec 011 FR-004)**: "Transaction.Commit() MUST return InvalidActionError when called when the PartialDataRow is not in PartialDataRowWithStartControl state"

**New Behavior (Spec 012 FR-008)**: Commit() now handles two distinct cases:
1. **Empty transactions**: When `len(rows) == 0` AND partial row is in `PartialDataRowWithStartControl` state → creates NullRow (original spec 011 behavior preserved)
2. **Data transactions**: When partial row is in `PartialDataRowWithPayload` or `PartialDataRowWithSavepoint` state → finalizes the last PartialDataRow to a DataRow with appropriate end_control (TC or SC)

**Rationale**: The original spec 011 FR-004 was designed before AddRow() existed. With the introduction of AddRow(), the Commit() method must now handle data transactions where the partial row has payload. The spec 011 test `Test_S_011_FR_004_CommitReturnsInvalidActionError/commit_on_wrong_partial_state_fails` was updated to reflect this new behavior.

**Impact**: Spec 011 FR-004 should be amended to read: "Transaction.Commit() MUST return InvalidActionError when called on an inactive transaction (no partial row) or when the transaction is already committed."

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Add Multiple Data Rows to Transaction (Priority: P1)

User needs to add multiple key-value pairs to a transaction before committing, with each AddRow() call adding one row while maintaining proper transaction state and row sequencing.

**Why this priority**: Core transaction functionality required for basic database operations

**Independent Test**: Can be fully tested by creating a transaction, calling AddRow() multiple times with different UUIDv7 keys and JSON values, then calling Commit() to verify all rows are properly stored and committed.

**Acceptance Scenarios**:

1. **Given** a new transaction with Begin() called, **When** AddRow() is called with valid UUIDv7 key and JSON value, **Then** a new PartialDataRow is created and the previous PartialDataRow is finalized and moved to rows[]
2. **Given** a transaction with existing rows, **When** AddRow() is called multiple times, **Then** each call properly finalizes the previous row and maintains correct row ordering
3. **Given** a transaction with multiple rows added via AddRow(), **When** Commit() is called, **Then** the last PartialDataRow is finalized with proper end_control and all rows are committed

---

### User Story 2 - Empty Transaction with Begin/Commit (Priority: P1)

User needs to create and commit an empty transaction (Begin() followed immediately by Commit()) which should produce a NullRow instead of a regular DataRow.

**Why this priority**: Ensures backward compatibility with existing empty transaction workflow

**Independent Test**: Can be fully tested by calling Begin() followed immediately by Commit() and verifying a NullRow is produced instead of attempting to finalize a PartialDataRow with data.

**Acceptance Scenarios**:

1. **Given** a new transaction with Begin() called, **When** Commit() is called without any AddRow() calls, **Then** a NullRow is created and stored in the empty field
2. **Given** a transaction that produced a NullRow, **When** the transaction state is examined, **Then** empty field is non-nil and rows field remains empty

---

### User Story 3 - Transaction Error Handling (Priority: P2)

User needs appropriate error handling when AddRow() is called outside of valid transaction states or with invalid parameters.

**Why this priority**: Provides proper user feedback and prevents system corruption

**Independent Test**: Can be fully tested by calling AddRow() in various invalid states and with invalid inputs, verifying appropriate errors are returned.

**Acceptance Scenarios**:

1. **Given** a transaction without Begin() called, **When** AddRow() is called, **Then** an InvalidActionError is returned
2. **Given** a transaction that is already committed, **When** AddRow() is called, **Then** an InvalidActionError is returned  
3. **Given** a transaction with active PartialDataRow, **When** AddRow() is called with invalid UUIDv7, **Then** an InvalidInputError is returned
4. **Given** a transaction with active PartialDataRow, **When** AddRow() is called with empty JSON value, **Then** an InvalidInputError is returned
5. **Given** a transaction with existing rows, **When** AddRow() is called with UUID timestamp that violates ordering (new_timestamp + skew_ms ≤ max_timestamp), **Then** a KeyOrderingError is returned
6. **Given** a transaction after successful AddRow(), **When** transaction's max_timestamp is examined, **Then** it equals max(previous_max_timestamp, new_timestamp)
7. **Given** a new transaction initialized, **When** max_timestamp is examined, **Then** it contains the current database's max_timestamp or 0 for empty database

---

---

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: Transaction MUST allow AddRow() to be called only after Begin() has been called successfully
- **FR-002**: AddRow() MUST finalize the current PartialDataRow by converting it to a DataRow with ROW_END_CONTROL end_control
- **FR-003**: AddRow() MUST move the finalized previous DataRow to the rows[] slice
- **FR-004**: AddRow() MUST create a new PartialDataRow in PartialDataRowWithStartControl state for the next row
- **FR-005**: AddRow() MUST use ROW_CONTINUE start_control for all rows after the first row in a transaction
- **FR-006**: AddRow() MUST validate UUIDv7 key parameter and return InvalidInputError for invalid UUIDs
- **FR-007**: AddRow() MUST validate JSON value parameter is non-empty and return InvalidInputError for empty values
- **FR-008**: Commit() MUST finalize the last PartialDataRow using appropriate end_control based on transaction state
- **FR-009**: Commit() MUST NOT attempt to finalize PartialDataRow for empty transactions (no AddRow() calls)
- **FR-010**: Transaction MUST maintain maximum 100 rows limit including all finalized rows
- **FR-011**: AddRow() MUST return InvalidActionError when called on committed or inactive transactions
- **FR-012**: Transaction state MUST remain consistent during AddRow() operations with proper mutex locking
- **FR-013**: Transaction MUST receive current max_timestamp when initialized and maintain its own copy during the transaction
- **FR-014**: AddRow() MUST preserve UUID timestamp ordering using the max_timestamp algorithm: new_timestamp + skew_ms > max_timestamp
- **FR-015**: AddRow() MUST update transaction's max_timestamp after successful row insertion to prevent unbounded timestamp decreases
- **FR-016**: AddRow() MUST return KeyOrderingError when UUID timestamp violates ordering constraints
- **FR-017**: For empty databases, max_timestamp MUST start at 0 requiring new_timestamp + skew_ms > 0 for first row

### Key Entities *(include if feature involves data)*

- **Transaction**: Manages transaction state, rows, partial row handling, and its own max_timestamp copy with thread-safe operations
- **PartialDataRow**: In-progress data row that gets finalized and promoted to DataRow on each AddRow() call  
- **DataRow**: Completed data row with proper end_control marking transaction continuation or termination
- **NullRow**: Special row type for empty transactions produced by Begin() + Commit() without AddRow() calls
- **KeyOrderingError**: Error type returned when UUID timestamp violates ordering constraints (new_timestamp + skew_ms ≤ max_timestamp)

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/transaction_spec_test.go` 
- Follow naming convention `Test_S_012_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
  For frozenDB, include data integrity and correctness metrics.
-->

### Measurable Outcomes

- **SC-001**: Users can successfully add up to 100 rows to a transaction using AddRow() method
- **SC-002**: Transaction state transitions work correctly across Begin(), AddRow(), and Commit() calls
- **SC-003**: Error conditions are properly handled with appropriate error types and messages, including KeyOrderingError for UUID ordering violations
- **SC-004**: Empty transaction workflow (Begin+Commit) continues to produce NullRows without data rows

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data loss scenarios in transaction finalization testing
- **SC-006**: All row sequencing maintains proper start_control (T followed by R's) and end_control patterns
- **SC-007**: Thread safety maintained with proper mutex locking during AddRow() operations
- **SC-008**: Transaction atomicity preserved in all AddRow() scenarios including error conditions
