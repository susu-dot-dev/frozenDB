# Feature Specification: Transaction Struct

**Feature Branch**: `006-transaction-struct`  
**Created**: 2025-01-11  
**Status**: Draft  
**Input**: User description: "006 create a Transaction struct: Transaction Struct Feature Specification
Overview
We need to implement a Transaction struct that provides a high-level abstraction for managing collections of DataRow objects representing a single database transaction. This struct will enable developers to work with transactions as logical units, providing methods to iterate committed data, manage savepoints, and determine row commit status.
Technical Requirements
Core Data Structure
The Transaction struct will contain:
- A single slice of DataRow objects (maximum 100 rows)
- The first row must be the start of the transaction (StartControl = 'T')
- The last row is either the end of the transaction or the transaction is still open
Key Capabilities
1. Direct Indexing System
- Index 0 maps to first element of the slice (must be transaction start)
- Direct O(1) access to any row in the slice
- No virtual indexing needed with single slice design
2. Transaction State Management
- IsCommitted() method to determine if transaction is fully completed
- Handles edge case where transaction is still open (last row is continuation)
3. Committed Data Access
- GetCommittedRows() returns an iterator function for rows that are actually committed
- Applies rollback logic according to v1 file format specification
- Handles partial rollbacks to savepoints, full rollbacks, and clean commits
4. Savepoint Management
- GetSavepointIndices() identifies all savepoint locations within transaction
- Savepoints detected via EndControl patterns per v1 specification
- Returns indices for easy reference within the slice
5. Row-Level Commit Status
- IsRowCommitted(index) determines if specific row is committed
- Applies transaction-wide rollback logic to individual row queries
- Supports efficient lookup without iterating entire transaction
6. Validation Framework
- Validate() scans all rows in the slice to ensure transaction integrity
- Verifies first row has StartControl = 'T' (transaction start)
- Checks proper StartControl sequences (T followed by R's for subsequent rows)
- Validates savepoint consistency and rollback target validity
- Ensures only one transaction termination within range (or transaction is still open)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Transaction Data Access (Priority: P1)

Developers need to access committed data from a transaction as a logical unit, regardless of how the underlying DataRow objects were loaded into memory.

**Why this priority**: This is the core functionality that enables developers to work with transactions as high-level abstractions rather than raw row collections.

**Independent Test**: Can be fully tested by creating a Transaction with known DataRow patterns and verifying GetCommittedRows() iterator returns exactly the committed data according to v1 file format rules.

**Acceptance Scenarios**:

1. **Given** a transaction with 3 rows ending in commit (TC), **When** GetCommittedRows() iterator is called, **Then** all 3 rows are returned by the iterator
2. **Given** a transaction with full rollback (R0), **When** GetCommittedRows() iterator is called, **Then** no rows are returned (iterator immediately returns false)
3. **Given** a transaction with savepoint 1 followed by rollback to savepoint 1 (R1), **When** GetCommittedRows() iterator is called, **Then** only rows up to savepoint 1 are returned by the iterator

---

### User Story 2 - Savepoint Management (Priority: P2)

Developers need to identify and work with savepoints within a transaction to understand rollback boundaries and transaction structure.

**Why this priority**: Savepoints are critical for understanding partial rollback scenarios and transaction flow in frozenDB.

**Independent Test**: Can be fully tested by creating transactions with various savepoint patterns and verifying GetSavepointIndices() returns correct indices.

**Acceptance Scenarios**:

1. **Given** a transaction with rows ending in SE, RE, SC, **When** GetSavepointIndices() is called, **Then** indices [0, 2] are returned (indices of savepoint rows)
2. **Given** a transaction with no savepoints, **When** GetSavepointIndices() is called, **Then** empty array is returned
3. **Given** a transaction with savepoints at various positions, **When** GetSavepointIndices() is called, **Then** correct indices are returned in order

---

### User Story 3 - Transaction State Validation (Priority: P3)

Developers need to validate transaction integrity and determine completion status to ensure data consistency.

**Why this priority**: Validation prevents corrupted or incomplete transactions from being processed, maintaining database integrity.

**Independent Test**: Can be fully tested by creating transactions with various invalid patterns and verifying Validate() returns appropriate errors.

**Acceptance Scenarios**:

1. **Given** a transaction with proper T followed by R's, **When** Validate() is called, **Then** no error is returned
2. **Given** a transaction starting with R instead of T, **When** Validate() is called, **Then** validation error is returned
3. **Given** a transaction with rollback to non-existent savepoint, **When** Validate() is called, **Then** validation error is returned

---

### Edge Cases

- How does system handle transaction with invalid end control sequences?
- How does system handle transaction with mixed valid and invalid row sequences?
- How does Validate() handle transactions exceeding 100 row limit?
- How does Validate() handle nil DataRow slices (must return error)?

## Clarifications

### Session 2025-01-11

- Q: What are the performance requirements for the Transaction struct's virtual indexing system? → A: Remove performance constraints for this first implementation
- Q: What error handling behavior should the Validate() method use for invalid transactions? → A: Return structured FrozenDBError with specific error codes
- Q: What are the thread safety requirements for Transaction struct methods? → A: Thread safe, because the underlying array is read-only
- Q: How should Transaction handle empty DataRow slices or nil inputs? → A: Must always be at least one row in the slice. Validate() would fail because the first row is not TransactionStart
- Q: What should be the maximum number of DataRows per Transaction slice for memory management? → A: Max # of rows in a transaction = 100, simplify to single slice input (no virtual indexing needed)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Transaction struct MUST store a single slice of DataRow objects with maximum 100 rows
- **FR-002**: Transaction struct MUST provide direct indexing where index 0 maps to first element of the slice (which must be transaction start)
- **FR-003**: The first row of the slice MUST be the start of the transaction (StartControl = 'T'), verified by Validate()
- **FR-004**: IsCommitted() method MUST return true only when transaction has proper termination (commit or rollback)
- **FR-005**: IsCommitted() method MUST handle edge case where transaction is still open (last row ends with E)
- **FR-006**: GetCommittedRows() method MUST return an iterator function that yields only rows that are committed according to v1 file format rollback logic
- **FR-007**: GetCommittedRows() iterator MUST handle partial rollbacks to savepoints, full rollbacks, and clean commits correctly
- **FR-008**: GetSavepointIndices() method MUST identify all savepoint locations using EndControl patterns with S as first character
- **FR-009**: GetSavepointIndices() method MUST return indices for easy reference within the slice
- **FR-010**: IsRowCommitted(index) method MUST determine if specific row at index is committed
- **FR-011**: IsRowCommitted(index) method MUST apply transaction-wide rollback logic to individual row queries
- **FR-012**: Validate() method MUST scan all rows in the slice to check for inconsistencies
- **FR-013**: Validate() method MUST verify the first row has StartControl = 'T' (transaction start)
- **FR-014**: Validate() method MUST ensure proper StartControl sequences (T followed by R's for subsequent rows)
- **FR-015**: Validate() method MUST validate savepoint consistency and rollback target validity
- **FR-016**: Validate() method MUST ensure only one transaction termination within range (or transaction is still open)
- **FR-017**: Validate() method MUST return CorruptDatabaseError for corruption scenarios or InvalidInputError for logic/instruction errors
- **FR-018**: Transaction struct MUST be thread-safe for concurrent read access (immutable underlying data)

### Key Entities *(include if feature involves data)*

- **Transaction**: High-level abstraction representing a single database transaction with maximum 100 DataRow objects in a single slice. The first row must be the transaction start, and the last row is either the end or the transaction is still open.
- **DataRow**: Existing data row object containing transaction data with start/end control information
- **Index**: Direct index system mapping to DataRow elements in a single slice (0 to len(slice)-1)
- **Savepoint**: Transaction marker identified by EndControl patterns with S as first character

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can create Transaction objects with a single []DataRow slice and access committed data with single method call
- **SC-002**: Direct indexing system provides O(1) access to any row in the slice (max 100 rows)
- **SC-003**: Transaction rollback logic correctly handles all v1 file format scenarios (commit, partial rollback, full rollback)
- **SC-004**: Savepoint detection accurately identifies all savepoint locations within the single slice

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption scenarios in transaction validation tests
- **SC-006**: All transaction state transitions maintain data consistency according to v1 specification
- **SC-007**: Direct indexing maintains correct mapping within single bounded slice (max 100 rows)
- **SC-008**: Transaction rollback logic preserves atomicity in all test scenarios
- **SC-009**: Validate() correctly identifies transactions where first row is not transaction start
- **SC-010**: Validate() correctly handles transactions that are still open (no termination found)