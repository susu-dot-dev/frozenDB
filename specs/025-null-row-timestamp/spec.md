# Feature Specification: NullRow Timestamp Modification

**Feature Branch**: `025-null-row-timestamp`  
**Created**: 2026-01-26  
**Status**: Draft  
**Input**: User description: "025 null-row-timestamp. This spec modifies the NullRow's UUID value. Instead of it being all zeros, the timestamp part is changed to match the maxTimestamp in the database. This means that null rows will properly adhere to the timestamp requirements of UUIDs"

## Clarifications

### Session 2026-01-26

- Q: How should NullRow UUID timestamp generation behave when system clock moves backward? → A: Always use maxTimestamp, ignoring system clock
- Q: What are the acceptable performance characteristics for maxTimestamp lookup during NullRow creation? → A: Already defined in spec 023 (O(1) requirement)
- Q: Should system automatically upgrade legacy NullRows during normal operations, or maintain them as-is? → A: Breaking change - all databases will use new-format NullRows

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

### User Story 1 - NullRow UUID Timestamp Compliance (Priority: P1)

As a frozenDB system, I need NullRows to use UUIDv7 values with proper timestamps instead of uuid.Nil so that all rows in the database adhere to UUIDv7 timestamp requirements and maintain temporal ordering consistency.

**Why this priority**: This is a critical data integrity requirement that affects the fundamental UUID ordering guarantees of the database system. Without proper timestamp adherence, NullRows violate the UUIDv7 specification and could cause issues with timestamp-based operations.

**Independent Test**: Can be fully tested by creating NullRows and verifying their UUID timestamps match the current maxTimestamp, delivering proper UUIDv7 compliance.

**Acceptance Scenarios**:

1. **Given** an empty database, **When** a NullRow is created, **Then** the NullRow UUID timestamp should be 0
2. **Given** a database with maxTimestamp of 1000, **When** a NullRow is created, **Then** the NullRow UUID timestamp should be 1000
3. **Given** a database with maxTimestamp of 5000, **When** multiple NullRows are created, **Then** all NullRows should have timestamp 5000

---

### User Story 2 - Database MaxTimestamp Tracking Integration (Priority: P1)

As a frozenDB system, I need the maxTimestamp tracking to properly account for NullRows with real timestamps so that timestamp ordering validation works correctly across all row types.

**Why this priority**: This ensures the timestamp ordering algorithm works correctly when NullRows participate in the maxTimestamp calculation, maintaining data consistency and validation accuracy.

**Independent Test**: Can be fully tested by inserting data rows, creating NullRows, and verifying maxTimestamp calculations include the NullRow timestamps appropriately.

**Acceptance Scenarios**:

1. **Given** a database with DataRows up to timestamp 2000, **When** a NullRow is created, **Then** maxTimestamp should remain 2000
2. **Given** a database with maxTimestamp 3000, **When** a NullRow is created, **Then** the NullRow should use timestamp 3000 and maxTimestamp should remain 3000
3. **Given** a database with maxTimestamp 1000, **When** a NullRow is created followed by a DataRow with timestamp 1500, **Then** maxTimestamp should become 1500

**Important Note**: The NullRow's UUID timestamp is fixed at insertion time and does not change after the row is written to the database. In scenario 3, the NullRow retains timestamp 1000 even after maxTimestamp increases to 1500. The Finder's `MaxTimestamp()` method tracks the dynamic maximum across all rows, while each NullRow's UUID timestamp is an immutable value from its insertion time.

---

### User Story 3 - New Format Transition (Priority: P2)

As a frozenDB developer, I need to ensure all NullRows use the new timestamp-aware format so that the system maintains consistent UUIDv7 behavior across all operations.

**Why this priority**: This ensures data consistency and eliminates legacy format complexity since the codebase is under development.

**Independent Test**: Can be fully tested by creating NullRows and verifying they all use the new timestamp-aware format.

**Acceptance Scenarios**:

1. **Given** any database operation, **When** a NullRow is created, **Then** it must use the new timestamp-aware format
2. **Given** maxTimestamp tracking, **When** NullRows are processed, **Then** they should contribute correctly to timestamp calculations
3. **Given** database operations, **When** NullRows are read/written, **Then** they should follow consistent UUIDv7 format

---

[Add more user stories as needed, each with an assigned priority]

### Edge Cases

- What happens when maxTimestamp is 0 (empty database) and a NullRow is created?
- System always uses cached maxTimestamp for NullRow timestamps, ignoring system clock movements
- Not applicable - breaking change eliminates legacy NullRows

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements

- **FR-001**: System MUST generate NullRow UUIDs with timestamp component equal to current maxTimestamp, ignoring system clock
- **FR-002**: System MUST maintain maxTimestamp tracking correctly when NullRows use real timestamps
- **FR-003**: System MUST handle empty databases (maxTimestamp = 0) by creating NullRows with timestamp 0
- **FR-004**: System MUST implement breaking change to new-format NullRows (no backward compatibility needed)
- **FR-005**: System MUST validate timestamp ordering algorithm works correctly with new NullRow timestamp behavior
- **FR-006**: System MUST update both SimpleFinder and InMemoryFinder to handle new NullRow timestamp logic
- **FR-007**: System MUST ensure NullRows still adhere to all other UUIDv7 requirements (random components, proper encoding)

### Key Entities *(include if feature involves data)*

- **NullRow**: Modified to use timestamp-aware UUID instead of uuid.Nil
- **MaxTimestamp Tracker**: Updated to properly handle NullRows with real timestamps
- **UUID Generator**: Enhanced to create UUIDs with specific timestamps for NullRows

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_025_FR_XXX_Description()`
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

- **SC-001**: 100% of newly created NullRows use UUIDs with timestamp matching current maxTimestamp
- **SC-002**: All NullRows consistently use new timestamp-aware UUID format
- **SC-003**: Timestamp ordering validation passes in all test scenarios with new NullRow behavior
- **SC-004**: MaxTimestamp calculation accuracy remains at 100% across all row type combinations

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: 100% format consistency across all NullRow operations
- **SC-006**: All UUID timestamp ordering validations pass with new NullRow implementation
- **SC-007**: Memory usage remains constant regardless of NullRow UUID format changes
- **SC-008**: Transaction atomicity preserved in all tests involving NullRows with real timestamps
