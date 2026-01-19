# Feature Specification: Transaction Begin and Commit

**Feature Branch**: `011-transaction-begin-commit`  
**Created**: 2026-01-18  
**Status**: Draft  
**Input**: User description: "Implement Begin() and Commit() to a Transaction. When the Transaction is empty (no rows), calling Begin initializes a PartialDataRow to the PartialDataRowWithStartControl state. When Commit() is called with the PartialDataRow in said state, the PartialDataRow gets a null payload to match the NullRow, and finally Commit() is called on the PartialDataRow, returning a new NullRow. The transaction after this state rows [] with a single NullRow. On the other hand, if Begin() is called at any time besides the initial state, an InvalidActionError is returned. Similarly, if Commit() is called except right after Begin(), return an InvalidActionError"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Empty Transaction Begin and Commit (Priority: P1)

A user needs to create an empty transaction that results in a single NullRow. This is the foundational workflow for handling empty transactions in the database system.

**Why this priority**: This is the core functionality that enables proper state management and empty transaction handling, which is essential for database integrity and proper transaction lifecycle management.

**Independent Test**: Can be fully tested by creating an empty Transaction, calling Begin(), then Commit(), and verifying the resulting Transaction contains exactly one NullRow with proper validation.

**Acceptance Scenarios**:

1. **Given** an empty Transaction with no rows, **When** Begin() is called, **Then** a PartialDataRow is created in PartialDataRowWithStartControl state
2. **Given** a PartialDataRow in PartialDataRowWithStartControl state, **When** Commit() is called, **Then** the Transaction contains exactly one NullRow and no InvalidActionError is returned

---

### User Story 2 - Invalid Action Prevention (Priority: P1)

A user needs protection against invalid state transitions that would corrupt the transaction state. This ensures database integrity by preventing invalid sequences of operations.

**Why this priority**: Preventing invalid actions is critical for maintaining database integrity and avoiding corruption. This is a fundamental safety requirement.

**Independent Test**: Can be fully tested by attempting invalid operations (Begin() when not empty, Commit() at wrong time) and verifying InvalidActionError is returned.

**Acceptance Scenarios**:

1. **Given** a Transaction that already has rows, **When** Begin() is called, **Then** an InvalidActionError is returned
2. **Given** a Transaction that is not in PartialDataRowWithStartControl state after Begin(), **When** Commit() is called, **Then** an InvalidActionError is returned

---



### Edge Cases

- What happens when Transaction is nil or not initialized?
- How does system handle multiple Begin() calls on same empty Transaction?
- What happens if Begin() succeeds but Commit() fails mid-process?
- How does system handle Transaction that is already committed before operations?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Transaction MUST have a Begin() method that initializes a PartialDataRow to PartialDataRowWithStartControl state when the Transaction contains no rows
- **FR-002**: Transaction MUST have a Commit() method that converts a PartialDataRowWithStartControl into a NullRow with null payload
- **FR-003**: Transaction.Begin() MUST return InvalidActionError when called on a Transaction that is not empty (has existing rows)
- **FR-004**: Transaction.Commit() MUST return InvalidActionError when called when the PartialDataRow is not in PartialDataRowWithStartControl state
- **FR-005**: After successful Begin() -> Commit() sequence, Transaction MUST contain exactly one row which is a valid NullRow
- **FR-006**: Transaction MUST validate that the resulting NullRow follows all NullRow specification requirements

### Key Entities *(include if feature involves data)*

- **Transaction**: Container for database rows with state management capabilities, supports Begin() and Commit() operations
- **PartialDataRow**: Incomplete data row with three progressive states (WithStartControl, WithPayload, WithSavepoint)
- **NullRow**: Complete row representing null operation with uuid.Nil key and NR end control
- **InvalidActionError**: Error type returned for invalid state transitions and prohibited operations

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
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

- **SC-001**: Users can complete empty transaction workflow (Begin() -> Commit()) in under 10 milliseconds
- **SC-002**: 100% of invalid operation attempts return appropriate InvalidActionError without state corruption
- **SC-003**: All generated NullRows pass validation with zero tolerance for specification violations
- **SC-004**: Transaction state transitions are atomic with zero intermediate corruption states

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data loss scenarios in corruption detection tests
- **SC-006**: All concurrent read/write operations maintain data consistency
- **SC-007**: Memory usage remains constant regardless of database size
- **SC-008**: Transaction atomicity preserved in all crash simulation tests
