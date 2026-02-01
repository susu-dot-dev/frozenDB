# Feature Specification: RowEmitter Integration

**Feature Branch**: `038-rowemitter-integration`  
**Created**: 2026-02-01  
**Status**: Draft  
**Input**: User description: "038 RowEmitter-integration. This is a technical spec, so the user story should be written from the point of the frozendb developer. The developer wants to integrate RowEmitter to simplify the logic, make it easier in the future for read-only instances to get updates, and uncouple logic with the transaction. The requirements are: 1. The NewFrozenDB logic must instantiate a RowEmitter, then call dbFile.subscribe() and then hook up a callback that calls RowEmitter.onDBFileNotification. That way, the RowEmitter will start getting events when DBFile changes. 2. Change all of the Finder implementations to require a rowEmitter as part of their New*Finder, and then to subscribe to changes. As part of this, remove OnRowAdded from the Finder spec since this is now an internal coupling. 3. Change the transaction code to not call OnRowAdded. 4. make sure the code works as expected. There really shouldn't be too many spec tests, since this is an internal refactoring. You can explicitly mention that the functional requirements should be skipped for spec testing where it makes sense"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Simplified Row Notification Architecture (Priority: P1)

As a **frozenDB maintainer**, I want the row notification system to use a centralized RowEmitter instead of direct Transaction-to-Finder coupling, so that the codebase is easier to maintain and extend with future features like read-only instance streaming.

**Why this priority**: This is the foundation of the refactoring. It decouples Transaction from Finder, making both components more maintainable and testable. This architectural change enables future enhancements like live replication and reactive queries without requiring further Transaction modifications.

**Independent Test**: Can be fully tested by verifying that all existing database operations (read, write, transaction commit/rollback) produce identical behavior before and after the refactoring, and that Finders receive notifications through RowEmitter subscriptions instead of direct Transaction calls.

**Acceptance Scenarios**:

1. **Given** a FrozenDB instance is initialized with any Finder implementation, **When** the initialization completes, **Then** a RowEmitter is created, subscribed to DBFile notifications, and the Finder is subscribed to the RowEmitter
2. **Given** a transaction writes a new row, **When** the row write completes to disk, **Then** the Finder receives notification through RowEmitter subscription (not through direct Transaction call)
3. **Given** a transaction commits with multiple rows, **When** each row is written, **Then** each Finder receives exactly one notification per completed row through RowEmitter
4. **Given** a transaction is rolled back, **When** the rollback NullRow is written, **Then** the Finder receives notification about the NullRow through RowEmitter

---

### User Story 2 - Foundation for Read-Only Instance Updates (Priority: P2)

As a **frozenDB maintainer**, I want row notifications to flow through a well-defined RowEmitter interface, so that future read-only instances can subscribe to the same notification stream without architectural changes.

**Why this priority**: This establishes the architectural foundation for future distributed features. While not immediately used, having RowEmitter as the single source of row completion events makes it straightforward to add network-based subscribers later.

**Independent Test**: Can be tested by verifying that RowEmitter's subscription mechanism is generic enough to support any callback function, and that multiple independent subscribers can coexist without interfering with each other.

**Acceptance Scenarios**:

1. **Given** a RowEmitter instance, **When** multiple subscribers register callbacks, **Then** all subscribers receive notifications for each new row
2. **Given** a RowEmitter with multiple subscribers, **When** one subscriber returns an error, **Then** other subscribers still receive their notifications
3. **Given** the RowEmitter notification interface, **When** evaluating extensibility for remote subscribers, **Then** the interface design supports adding network-based notification transport without changing existing Finder implementations

---

### Edge Cases

- What happens when RowEmitter notification fails but the row is already written to disk?
- How does the system handle RowEmitter subscription during database recovery after a crash?
- What happens if a Finder subscription callback returns an error during notification?
- How does RowEmitter behave when DBFile notifications arrive while processing previous notifications?
- What happens when FrozenDB is closed while RowEmitter has pending notifications?

## Requirements *(mandatory)*

### Functional Requirements

**Note on Spec Testing**: Since this is an internal refactoring that preserves all existing behavior, most functional requirements do not require new spec tests. The primary validation is that existing spec tests continue to pass unchanged. New spec tests are only noted where they validate critical new integration points.

- **FR-001**: NewFrozenDB MUST instantiate a RowEmitter, subscribe it to DBFile notifications, and pass it to all Finder constructors during initialization
- **FR-002**: All Finder implementations (SimpleFinder, InMemoryFinder, and BinarySearchFinder) MUST accept a RowEmitter parameter in their constructor and subscribe to it for row notifications
- **FR-003**: The Finder interface MUST no longer include the OnRowAdded method (breaking change in internal API)
- **FR-004**: All Finder implementations MUST update their internal state (SimpleFinder: file size; InMemoryFinder and BinarySearchFinder: indexes, size, maxTimestamp) through RowEmitter subscription callbacks instead of OnRowAdded
- **FR-005**: Transaction MUST NOT call Finder.OnRowAdded() after row writes (remove all notifyFinderRowAdded calls and related logic)
- **FR-006**: All existing database operations (AddRow, Commit, Rollback, checksum insertion) MUST produce identical behavior and on-disk format
- **FR-007**: Finders MUST receive row notifications in the same order as rows are written to disk, with correct row index and RowUnion data
- **FR-008**: Subscription cleanup MUST prevent memory leaks when Finders or FrozenDB instances are closed

**Spec Testing Note**: FR-006 is validated by ensuring all existing spec tests pass without modification. FR-001 through FR-005 and FR-008 represent internal refactoring and should not require dedicated spec tests—behavior validation is sufficient through existing test suites. Integration correctness can be validated through manual code review during implementation.

### Key Entities

This refactoring involves architectural components rather than user-facing data entities:

- **RowEmitter**: Central notification hub that subscribes to DBFile write events and notifies downstream subscribers about completed rows. Acts as a decoupling layer between the write path and consumers of row completion events.

- **DBFile Subscription**: Notification mechanism where DBFile alerts subscribers after successful write operations. RowEmitter acts as the primary subscriber, translating file size changes into individual row completion events.

- **Finder Subscription**: Callback registration mechanism where Finders receive (index, row) pairs from RowEmitter. Replaces the direct Transaction→Finder coupling through OnRowAdded.

- **Notification Flow**: The complete chain: Transaction → writeBytes → DBFile.writerLoop → DBFile subscribers (RowEmitter) → RowEmitter.onDBFileNotification → RowEmitter subscribers (Finders)

### Spec Testing Requirements

**Special Note for This Feature**: This is an internal refactoring that preserves all existing behavior. Therefore, the primary validation mechanism is ensuring all existing spec tests continue to pass without modification.

New spec tests are NOT required for:
- FR-001 through FR-006: Constructor signature changes, internal wiring, and behavioral equivalence validated by existing tests
- FR-008: Memory leak validation can be done through manual testing or existing test infrastructure

Spec tests ARE required for:
- **FR-007 Integration Test** (`Test_S_038_FR_007_RowEmitter_Delivers_Notifications_Correctly`): Validates that when rows are written, Finders receive notifications in correct order with accurate (index, row) data through RowEmitter subscription
  - Place in `frozendb/frozendb_spec_test.go` (integration test across components)

All other validation relies on existing spec test suite passing without modification, confirming behavioral equivalence.

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All existing spec tests pass without modification after refactoring is complete (100% pass rate maintained)
- **SC-002**: Zero direct references to Finder interface in Transaction implementation code
- **SC-003**: All Finder implementations use RowEmitter subscriptions for notifications
- **SC-004**: Code coverage for the notification path (DBFile → RowEmitter → Finder) is at least 90%
- **SC-005**: Manual code review confirms that NewFrozenDB initialization follows the required sequence: DBFile → RowEmitter → Finder(s)

### Data Integrity & Correctness Metrics

- **SC-006**: Zero data loss scenarios - all existing transaction tests confirm identical on-disk format before and after refactoring
- **SC-007**: All concurrent read/write operations maintain data consistency identical to pre-refactoring behavior
- **SC-008**: Transaction atomicity preserved in all crash simulation tests
- **SC-009**: Finder notification order matches row write order in 100% of test scenarios
- **SC-010**: Memory leak tests confirm proper subscription cleanup (no goroutine leaks, no memory growth over repeated open/close cycles)

### Code Quality Metrics

- **SC-011**: Static analysis (go vet, golint) passes with zero new warnings
- **SC-012**: No new public API surface area exposed - all changes are internal to frozendb package
- **SC-013**: Code complexity metrics (cyclomatic complexity) for Transaction remain same or lower than pre-refactoring baseline
