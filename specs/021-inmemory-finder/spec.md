# Feature Specification: InMemoryFinder

**Feature Branch**: `021-inmemory-finder`  
**Created**: 2025-01-24  
**Status**: Draft  
**Input**: User description: "Define a new Finder implementation called InMemoryFinder. The base premise of the InMemoryFinder is that it maintains a map of uuid -> indices (row index, matching transaction start and end indices) such that all of the finder operations are O(1). This finder implementation is useful when the database size is small enough so that the index can be fully in-memory. As a user, I can choose which finder strategy I want when creating a FrozenDB instance. When I want to choose performance over fixed memory bounding, I can choose an InMemoryFinder"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - High-Performance Database Queries (Priority: P1)

As a user working with small to medium-sized frozen databases, I want to use an InMemoryFinder so that all key lookups and transaction boundary operations complete in constant time O(1) instead of linear time O(n), providing significantly better performance for read-heavy workloads.

**Why this priority**: This is the core value proposition of the InMemoryFinder - providing O(1) performance for all finder operations when database size allows for full in-memory indexing.

**Independent Test**: Can be fully tested by creating a database with known data rows, initializing an InMemoryFinder, and verifying that GetIndex, GetTransactionStart, and GetTransactionEnd operations return correct results with O(1) performance characteristics.

**Acceptance Scenarios**:

1. **Given** a frozen database with 1000 committed data rows, **When** I open it with InMemoryFinder, **Then** all GetIndex operations complete in constant time regardless of database size
2. **Given** a database with transactions containing multiple rows, **When** I query transaction boundaries for any row, **Then** GetTransactionStart and GetTransactionEnd return correct indices in O(1) time
3. **Given** a database file opened in write mode, **When** new rows are added via transactions, **Then** the InMemoryFinder index is updated and subsequent queries reflect the new data

---

### User Story 2 - Finder Strategy Selection (Priority: P1)

As a user creating a FrozenDB instance, I want to choose between SimpleFinder and InMemoryFinder so I can optimize for either fixed memory usage (SimpleFinder) or maximum performance (InMemoryFinder) based on my specific use case requirements.

**Why this priority**: Without the ability to choose finder strategies, users cannot benefit from the performance improvements offered by InMemoryFinder. This enables the core user choice described in the feature description.

**Independent Test**: Can be fully tested by extending the FrozenDB creation interface to accept a finder type parameter and verifying that the correct finder implementation is instantiated and used.

**Acceptance Scenarios**:

1. **Given** I call NewFrozenDB(filename, mode, FinderStrategySimple), **When** I perform finder operations, **Then** SimpleFinder is used with O(n) performance characteristics
2. **Given** I call NewFrozenDB(filename, mode, FinderStrategyInMemory), **When** I perform finder operations, **Then** InMemoryFinder is used with O(1) performance characteristics
3. **Given** I call NewFrozenDB with invalid FinderStrategy value, **When** the constructor is called, **Then** an appropriate error is returned
4. **Given** existing code using NewFrozenDB(filename, mode), **When** updated to 021 API, **Then** add strategy as third parameter: NewFrozenDB(filename, mode, FinderStrategySimple) or FinderStrategyInMemory

---



## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an InMemoryFinder implementation of the Finder interface
- **FR-002**: InMemoryFinder MUST maintain a map of UUID key to row index for O(1) GetIndex operations
- **FR-003**: InMemoryFinder MUST maintain transaction boundary indices such that given a row index, the transaction start and end indices can be found immediately in O(1) time without any disk reads or seeks
- **FR-004**: InMemoryFinder MUST update its internal index when new rows are committed to make them immediately available to finder operations
- **FR-005**: System MUST allow users to choose finder strategy using type-safe FinderStrategy constants when creating FrozenDB instances
- **FR-006**: InMemoryFinder MUST pass all finder_conformance_test tests to ensure functional correctness
- **FR-007**: InMemoryFinder MUST maintain thread-safe access for concurrent Get* method calls
- **FR-008**: System MUST provide clear documentation about memory-performance trade-offs between finder types
- **FR-009**: The NewFrozenDB function signature MUST accept three parameters: filename, mode, and finder strategy

### Breaking Change Acknowledgment

**APPROVED BREAKING CHANGE**: This feature modifies NewFrozenDB to require three parameters: filename, mode, strategy. Existing code using NewFrozenDB(path, mode) must add the strategy parameter (e.g. FinderStrategySimple or FinderStrategyInMemory).

### Key Entities *(include if feature involves data)*

- **InMemoryFinder**: High-performance finder implementation that maintains complete UUID->index mapping in memory
- **FinderStrategy**: Enumeration or parameter type for selecting between "simple" and "inmemory" finder implementations
- **UUIDIndexMap**: Internal mapping from UUIDv7 keys to row indices for O(1) key lookups
- **TransactionBoundaryMap**: Internal mapping from row indices to their transaction start/end boundaries
- **MemoryUsageProfile**: Documentation and metrics describing memory consumption characteristics

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_021_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: GetIndex operations complete in under 1ms for databases with up to 100,000 rows using InMemoryFinder
- **SC-002**: GetTransactionStart and GetTransactionEnd operations complete in under 1ms regardless of database size using InMemoryFinder  
- **SC-003**: InMemoryFinder construction time scales linearly (O(n)) with database size during initialization
- **SC-004**: Users can successfully choose finder strategy with 100% accuracy during FrozenDB creation
- **SC-005**: Memory usage of InMemoryFinder grows predictably based on documented formulas (bytes per row)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-006**: Zero data integrity differences between InMemoryFinder and SimpleFinder results across all test scenarios
- **SC-007**: All concurrent read/write operations maintain index consistency in InMemoryFinder
- **SC-008**: Transaction atomicity preserved when using InMemoryFinder for all transaction patterns
- **SC-009**: Index corruption detection mechanisms prevent invalid state in InMemoryFinder
- **SC-010**: OnRowAdded updates maintain index consistency across all transaction rollback scenarios
