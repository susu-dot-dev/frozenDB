# Feature Specification: Transaction Savepoint and Rollback

**Feature Branch**: `013-transaction-savepoint-rollback`  
**Created**: 2025-01-19  
**Status**: Draft  
**Input**: User description: "Implement Transaction.Savepoint() and Transaction.Rollback(). As a user I can call Transaction.Rollback(0) after a transaction has begun, in order to close the transaction and invalidate all rows in the transaction. If the transaction is empty, then a NullRow is used. As a user, I can call Savepoint() once after any row has been created. Multiple Savepoint() calls will fail with InvalidActionError, along with calling Savepoint() before any rows have been added. Also, creating more than 9 user savepoints in a transaction is disallowed. Then, when a user calls Rollback(n) where n=1-9, the transaction is rolled back to that savepoint, following the rules in the v1_file_format"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create Savepoint During Transaction (Priority: P1)

As a user, I want to create a savepoint after adding data to a transaction so that I can rollback to that specific point later if needed.

**Why this priority**: Savepoint creation is a fundamental requirement for transaction rollback functionality and enables partial transaction recovery.

**Independent Test**: Can be fully tested by creating a transaction, adding data rows, calling Savepoint(), and verifying the savepoint is properly tracked within the transaction structure.

**Acceptance Scenarios**:

1. **Given** a transaction with one or more data rows, **When** Savepoint() is called, **Then** the current row is marked as a savepoint and the savepoint counter is incremented
2. **Given** a transaction with no data rows, **When** Savepoint() is called, **Then** InvalidActionError is returned
3. **Given** a transaction with existing savepoints, **When** Savepoint() is called again and total savepoints would be 10 or more, **Then** InvalidActionError is returned

---

### User Story 2 - Full Transaction Rollback (Priority: P1)

As a user, I want to rollback an entire transaction using Rollback(0) so that all data in the transaction is invalidated and the transaction is properly closed.

**Why this priority**: Full rollback is essential for error recovery and maintaining data integrity when an entire transaction needs to be discarded.

**Independent Test**: Can be fully tested by creating a transaction with data rows, calling Rollback(0), and verifying all rows are invalidated and the transaction is closed with a NullRow if empty.

**Acceptance Scenarios**:

1. **Given** a transaction with data rows, **When** Rollback(0) is called, **Then** all rows in the transaction are invalidated and the transaction is closed
2. **Given** an empty transaction (no data rows), **When** Rollback(0) is called, **Then** a NullRow is created and the transaction is closed
3. **Given** a transaction that is not active, **When** Rollback(0) is called, **Then** InvalidActionError is returned

---

### User Story 3 - Partial Rollback to Savepoint (Priority: P1)

As a user, I want to rollback to a specific savepoint using Rollback(n) where n=1-9 so that all rows created before the savepoint (including the savepoint row itself) are committed and all rows created after the savepoint are invalidated.

**Why this priority**: Partial rollback enables fine-grained transaction control and recovery from specific points within a transaction.

**Independent Test**: Can be fully tested by creating a transaction with multiple savepoints, calling Rollback(n) for a specific savepoint, and verifying that all rows up to and including the savepoint row are committed while subsequent rows are invalidated.

**Acceptance Scenarios**:

1. **Given** a transaction with multiple savepoints, **When** Rollback(n) is called with valid savepoint number n, **Then** all rows from start through savepoint n (inclusive) are committed and all rows after savepoint n through rollback row (inclusive) are invalidated
2. **Given** a transaction with k savepoints where k < 9, **When** Rollback(n) is called with n > k, **Then** InvalidActionError is returned
3. **Given** a transaction that is not active, **When** Rollback(n) is called with any valid n, **Then** InvalidActionError is returned
4. **Given** a transaction with savepoints, **When** Rollback(1) is called, **Then** the savepoint 1 row and all preceding rows are committed, regardless of their position in the transaction

---

---

### Edge Cases

- What happens when Rollback(n) is called with n greater than the number of savepoints?
- How does system handle Savepoint() calls on transactions that are not active?
- What happens when Rollback(0) is called on a transaction that has no rows?
- How does system handle rollback when the savepoint row itself has savepoint flag?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Transaction MUST support creating savepoints after any row has been added using Savepoint() method
- **FR-002**: Transaction MUST support full rollback using Rollback(0) method to invalidate all rows
- **FR-003**: Transaction MUST support partial rollback using Rollback(n) where n=1-9 to rollback to specific savepoint
- **FR-004**: System MUST enforce maximum of 9 user savepoints per transaction
- **FR-005**: System MUST require at least one data row before Savepoint() can be called
- **FR-006**: Savepoint() MUST return InvalidActionError when called on empty transaction
- **FR-007**: Savepoint() MUST return InvalidActionError when more than 9 savepoints would be created
- **FR-008**: Rollback(n) MUST return InvalidActionError when n exceeds number of existing savepoints
- **FR-009**: Rollback() MUST return InvalidActionError when called on inactive transaction
- **FR-010**: Rollback(0) MUST create NullRow for empty transactions
- **FR-011**: Rollback to savepoint n MUST commit all rows from start through savepoint n (inclusive)
- **FR-012**: Rollback to savepoint n MUST invalidate all rows after savepoint n through rollback row (inclusive)
- **FR-013**: Partial rollback MUST create DataRow with rollback end control (Rn or Sn)
- **FR-014**: Full rollback MUST create DataRow with R0 or S0 end control

### Key Entities *(include if feature involves data)*

- **Transaction**: Manages savepoints and rollback operations with proper state tracking
- **Savepoint**: Marker within transaction representing a rollback point, numbered 1-9
- **Rollback Command**: Transaction termination that invalidates specified rows based on savepoint target
- **EndControl**: Two-character sequence encoding rollback behavior (R0-R9, S0-S9)

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/transaction_spec_test.go` 
- Follow naming convention `Test_S_013_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Clarifications

### Session 2025-01-19

- Q: Performance targets specify very fast timing requirements. Are these strict requirements or aspirational goals that can be adjusted based on implementation constraints? → A: Remove specific timing targets and focus on correctness over performance

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All savepoint and rollback operations maintain 100% compliance with v1_file_format specification
- **SC-002**: Zero data corruption scenarios in rollback stress tests with 10,000 concurrent operations
- **SC-003**: Savepoint and rollback operations complete successfully across all supported transaction sizes
- **SC-004**: Memory usage remains constant regardless of number of savepoints in transaction

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data loss scenarios in savepoint/rollback corruption detection tests
- **SC-006**: All concurrent savepoint/rollback operations maintain transaction consistency
- **SC-007**: Transaction atomicity preserved in all rollback crash simulation tests