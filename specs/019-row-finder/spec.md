# Feature Specification: Row Finder Interface and Implementation

**Feature Branch**: `019-row-finder`  
**Created**: 2025-06-18  
**Status**: Draft  
**Input**: User description: "FrozenDB currently has a way to insert rows, but not yet a way to retrieve rows. Fully retrieving rows requires understanding the transaction state to know whether to include or exclude rows. To eventually support these high level needs, we are first going to define, and implement the ability to find the index of a row matching certain criteria. Index 0 is the first checksum row after the header, Index 1 is the first data or null row, etc. We need to be able to find an index with a specific key. Then, given a starting point (aka that index we found), we need to be able to find the beginning and end of the transaction it is in. Define a Finder interface which implements GetIndex(key UUID) (int64, error), GetTransactionEnd(index int64) (int64, error), GetTransactionStart(index int64) (int64 error), along with OnRowAdded(index int64, row *RowUnion). Then, implement a SimpleFinder class which implements this spec in the simplest manner (directly, and linearly scanning the file system). This is needed so we have a reference implementation to compare against for correctness when making more performant finders later"

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

### User Story 1 - Find Row by UUID Key (Priority: P1)

As a frozenDB developer, I need to find the index of a row containing a specific UUID key so that I can locate data within the database file for retrieval operations.

**Why this priority**: This is the foundational capability needed for all row retrieval operations. Without being able to find rows by key, no higher-level query functionality can be implemented.

**Independent Test**: Can be fully tested by creating a database with known rows, then calling GetIndex() with each UUID and verifying the returned indices match the expected positions.

**Acceptance Scenarios**:

1. **Given** a database with multiple data rows, **When** GetIndex() is called with an existing UUID key, **Then** the correct index of that row is returned
2. **Given** a database with data rows, **When** GetIndex() is called with a non-existent UUID key, **Then** an appropriate error is returned
3. **Given** an empty database (header only), **When** GetIndex() is called with any UUID key, **Then** an appropriate error is returned

---

### User Story 2 - Find Transaction Boundaries (Priority: P1)

As a frozenDB developer, I need to find the start and end indices of the transaction containing a specific row so that I can understand transaction context for proper data inclusion/exclusion logic.

**Why this priority**: Transaction boundaries are essential for determining which rows are valid and committed versus which ones were rolled back. This is critical for data correctness.

**Independent Test**: Can be fully tested by creating transactions with various commit/rollback scenarios, then verifying GetTransactionStart() and GetTransactionEnd() return the correct boundaries for rows within those transactions.

**Acceptance Scenarios**:

1. **Given** a committed transaction with multiple rows, **When** GetTransactionStart() and GetTransactionEnd() are called with any row index from that transaction, **Then** the correct start and end indices are returned
2. **Given** a rolled back transaction, **When** GetTransactionStart() and GetTransactionEnd() are called with any row index from that transaction, **Then** the correct boundaries are returned (including rollback row)
3. **Given** a single-row transaction, **When** GetTransactionStart() and GetTransactionEnd() are called with that row index, **Then** both return the same index
4. **Given** an index pointing to a checksum row, **When** GetTransactionStart() or GetTransactionEnd() are called, **Then** an appropriate error is returned

---

### User Story 3 - Handle Row Addition Notifications (Priority: P2)

As a frozenDB developer, I need the finder to be notified when new rows are added so that the finder's internal state can be kept up-to-date for subsequent find operations.

**Why this priority**: This ensures the finder remains accurate as the database grows, which is essential for long-running applications that continuously write data.

**Independent Test**: Can be fully tested by adding rows to a database and verifying that subsequent find operations correctly locate both old and newly added rows.

**Acceptance Scenarios**:

1. **Given** a finder instance, **When** OnRowAdded() is called with a new row index and row data, **Then** the finder's internal state is updated to include the new row
2. **Given** a finder that has received OnRowAdded() notifications, **When** GetIndex() is called for the newly added row, **Then** the correct index is returned
3. **Given** multiple OnRowAdded() calls in sequence, **When** transaction boundary methods are called, **Then** they correctly identify transactions spanning the newly added rows

---

### Edge Cases

- What happens when the database file is corrupted or contains invalid rows?
- How does the finder handle files with PartialDataRows at the end?
- What happens when GetIndex is called with uuid.Nil (which is only valid for NullRows)?

- What happens when transaction boundary detection encounters malformed transaction sequences?

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: System MUST define a Finder interface with methods GetIndex(key UUID) (int64, error), GetTransactionEnd(index int64) (int64, error), GetTransactionStart(index int64) (int64, error), and OnRowAdded(index int64, row *RowUnion)
- **FR-002**: GetIndex() MUST return the index of the first row containing the specified UUID key, or error if not found
- **FR-003**: GetTransactionStart() MUST return the index of the first row in the transaction containing the specified index
- **FR-004**: GetTransactionEnd() MUST return the index of the last row in the transaction containing the specified index  
- **FR-005**: GetTransactionStart() and GetTransactionEnd() MUST return errors when called with invalid indices (negative, out of bounds, or pointing to checksum rows)
- **FR-006**: OnRowAdded() MUST update finder state to include newly added rows for subsequent find operations
- **FR-007**: System MUST implement a SimpleFinder class that provides a direct, linear scan implementation of the Finder interface
- **FR-008**: SimpleFinder MUST scan the file system directly for each operation without using caching or optimization techniques
- **FR-009**: All finder methods MUST properly handle database files containing checksum rows, data rows, null rows, and PartialDataRows
- **FR-010**: Finder methods MUST correctly identify transaction boundaries based on start_control and end_control bytes as defined in the file format specification

### Key Entities *(include if feature involves data)*

- **Finder**: Interface defining methods for locating rows and transaction boundaries in frozenDB files
- **SimpleFinder**: Reference implementation of Finder that uses direct linear scanning of the file system
- **Row Index**: Zero-based integer where index 0 is the first checksum row after the header, index 1 is the first data/null row, etc.
- **Transaction Boundary**: The start and end indices that define the complete span of a transaction including all its data rows and terminal command
- **UUID Key**: The UUIDv7 key used to identify specific data rows within the database

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

- **SC-001**: GetTransactionStart() and GetTransactionEnd() correctly identify transaction boundaries for 100% of valid transaction scenarios
- **SC-002**: SimpleFinder implementation serves as a correct reference implementation that can be used to validate optimized finder implementations
- **SC-003**: Finder interface successfully handles all row types defined in the v1 file format specification

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-004**: All transaction boundary detection methods correctly handle commit, rollback, and savepoint scenarios
- **SC-005**: OnRowAdded() notifications maintain finder state consistency across continuous write operations
