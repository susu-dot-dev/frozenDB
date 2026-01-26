# Feature Specification: MaxTimestamp Finder Protocol Enhancement

**Feature Branch**: `023-maxtimestamp-finder`  
**Created**: 2026-01-26  
**Status**: Draft  
**Input**: User description: "Add MaxTimestamp() as a required function of the Finder protocol. Require that each Finder be able to implement it in O(1) time. MaxTimestamp shall return 0 if there are no data rows in the database (e.g. only checksum rows and or Null Rows). However, once a data row has been inserted, it should be updated. Do not specify a specific algorithm in the spec or the finder protocol about how to initialize or maintain this value as that is implementation specific."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - MaxTimestamp Query for Time-Based Operations (Priority: P1)

As a developer using frozenDB, I need to quickly determine the maximum timestamp of all data and null rows in the database so that I can efficiently implement time-based queries and data synchronization operations.

**Why this priority**: This is fundamental for time-ordered data operations and enables efficient synchronization between different database instances.

**Independent Test**: Can be fully tested by creating a Finder implementation with MaxTimestamp() method and verifying it returns correct values for empty databases, databases with only checksum/null rows, and databases with data rows.

**Acceptance Scenarios**:

1. **Given** a database with no complete rows (only checksum/PartialDataRow entries), **When** MaxTimestamp() is called, **Then** it returns 0
2. **Given** a database with data rows or null rows, **When** MaxTimestamp() is called, **Then** it returns the timestamp of the most recent complete data or null row
3. **Given** a database with only PartialDataRow entries (uncommitted transactions), **When** MaxTimestamp() is called, **Then** it returns 0


### Edge Cases

- What happens when the database contains only PartialDataRow entries (uncommitted transactions)?
- How does the system handle concurrent transactions where PartialDataRow and complete DataRow entries are mixed?
- What is the behavior when multiple transactions commit simultaneously with different timestamps?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Finder protocol MUST require implementation of MaxTimestamp() method
- **FR-002**: MaxTimestamp() method MUST return the maximum timestamp among all complete data and null rows in O(1) time
- **FR-003**: MaxTimestamp() MUST return 0 when the database contains no complete data or null rows (only checksum/PartialDataRow entries)
- **FR-004**: MaxTimestamp() MUST update only when complete DataRow or NullRow entries are added during Transaction.Commit() or Transaction.Rollback() operations

### Key Entities *(include if feature involves data)*

- **Finder Protocol Interface**: Defines the contract for database search operations, now including MaxTimestamp()
- **MaxTimestamp**: Represents the highest timestamp value among all data and null rows in the database
- **Transaction**: Database transaction structure with maxTimestamp field for temporal context
- **Data Row**: Complete DataRow entries that contribute to max timestamp calculations
- **Null Row**: Complete NullRow entries that contribute to max timestamp calculations
- **PartialDataRow**: Incomplete transaction entries that do not affect MaxTimestamp() until committed/rolled back
- **PartialDataRow**: Incomplete transaction entries that do not affect MaxTimestamp() until committed/rolled back

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

- **SC-001**: MaxTimestamp() queries execute in constant time regardless of database size
- **SC-002**: All Finder implementations successfully compile with required MaxTimestamp() method
- **SC-003**: Transaction creation succeeds 100% of the time when provided valid maxTimestamp values
- **SC-004**: finder_protocol.md documentation is updated and accessible to all implementers

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: MaxTimestamp() returns correct values in 100% of test scenarios with various database states
- **SC-006**: Concurrent operations maintain timestamp consistency without race conditions
- **SC-007**: Memory usage for timestamp tracking remains constant regardless of database size
- **SC-008**: Transaction atomicity preserved when maxTimestamp parameter is properly initialized
