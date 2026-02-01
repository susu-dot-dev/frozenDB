# Feature Specification: RowEmitter Layer for Decoupled Row Coordination

**Feature Branch**: `036-row-emitter-layer`  
**Created**: 2026-01-31  
**Status**: Draft  
**Input**: User description: "As a developer, the coordination between writing rows, the finder, and the database is too fragile and tightly coupled. Define a generic RowEmitter layer to properly decouple the pieces of the system. The purpose of the RowEmitter is to check the growth of the file size since the last time it ran. It should calculate the number of full rows completed that it has not yet emitted, and then send OnRowAdded events to any interested caller. To avoid tight coupling, we want to use late-binding in both directions. The DBFile interface should gain a Subscribe(callback func) method (as well as an appropriate unsubscribe). DBFile will call every callback after the write is complete and the DBFile size has been updated. Then, the RowEmitter will receive the callback, query the latest size (from DBFile), and determine how many new row(s) have been completed since it last ran. It will then follow a similar pattern and send any updated rows to callbacks that did a Subscribe() to the RowEmitter. All of this should be synchronous on the same thread. The first error returned is propagated backwards in both subscriber cases. One specific functional requirement to make sure to add is the case where the RowEmitter is initialized with the DBFile in a PartialDataRow state. When the row is completed, the Emitter should recognize that the row, which was started before it initialized, is now complete, and it should emit that row. Another functional requirement should be that when two complete rows have been written since the last subscription, it emits the first row to all subscribers, and then the second row."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Row Completion Notification System (Priority: P1)

As a database component, I need to be notified when complete rows are written to the file so that I can process them without tightly coupling to the write mechanism, including handling partial rows that complete after I initialize and processing multiple rows in correct order.

**Why this priority**: Right now, update logic is tightly woven into the transaction. This only works for write semantics and doesn't generalize to reads. This core notification system must handle all row completion scenarios (single, partial, multiple) to ensure no data is missed.

**Independent Test**: Can be fully tested by: (1) writing complete rows and verifying notifications, (2) initializing with a partial row and verifying notification when it completes, (3) writing multiple rows and verifying separate sequential notifications for each.

**Acceptance Scenarios**:

1. **Given** a RowEmitter is subscribed to a DBFile, **When** a complete row is written to the DBFile, **Then** the RowEmitter receives a notification and emits the completed row to its subscribers
2. **Given** a RowEmitter has a subscriber, **When** a complete row is written to the DBFile, **Then** the subscriber receives exactly one OnRowAdded event with the row information
3. **Given** no rows have been written, **When** the RowEmitter checks for new rows, **Then** no events are emitted to subscribers
4. **Given** a DBFile contains a PartialDataRow, **When** a RowEmitter is initialized and the partial row is completed, **Then** the RowEmitter emits the newly-completed row to its subscribers
5. **Given** two complete rows have been written since the last notification, **When** the RowEmitter processes the file growth, **Then** it emits the first row to all subscribers, then emits the second row to all subscribers (in order)

---

### User Story 2 - Multiple Independent Subscribers (Priority: P2)

As multiple database components (e.g., indexer, query engine, validator), we need to independently receive notifications about completed rows without interfering with each other's processing or subscription lifecycle.

**Why this priority**: This decouples the listeners, and allows for various parts of the system to be updated when rows are written, without being aware of each other. This enables the system to scale to multiple consumers.

**Independent Test**: Can be fully tested by subscribing multiple callbacks to the RowEmitter, writing rows, and verifying that all subscribers receive notifications independently, and that unsubscribing one doesn't affect others.

**Acceptance Scenarios**:

1. **Given** two subscribers are registered with the RowEmitter, **When** a complete row is written, **Then** both subscribers receive the OnRowAdded event
2. **Given** three subscribers where one unsubscribes, **When** a row is completed, **Then** only the two remaining subscribers receive the event
3. **Given** all subscribers have unsubscribed, **When** a row is completed, **Then** the RowEmitter processes the row but does not invoke any callbacks
4. **Given** five complete rows have been written, **When** a new subscriber registers, **Then** the subscriber does not receive historical events (only future events)

---

### User Story 3 - Error Propagation for System Reliability (Priority: P2)

As a database system, I need errors from row processing components to propagate back through the notification chain so that write operations can be rolled back or retried when downstream processing fails.

**Why this priority**: This ensures the system maintains consistency by allowing components to signal failures and halt further processing when something goes wrong, preventing silent data loss or corruption.

**Independent Test**: Can be fully tested by registering a subscriber that returns an error, writing a row, and verifying: (1) the error propagates back to the caller, (2) subsequent subscribers don't receive the event, (3) the error originates from the correct subscriber.

**Acceptance Scenarios**:

1. **Given** one subscriber returns an error, **When** a row event is emitted, **Then** the error propagates back and subsequent subscribers do not receive the event
2. **Given** the first of three subscribers returns an error, **When** a row is written, **Then** the second and third subscribers do not receive the event
3. **Given** a RowEmitter subscriber returns an error, **When** the DBFile triggers notifications, **Then** the error propagates back to the DBFile's original write operation
4. **Given** multiple rows are being emitted, **When** a subscriber returns an error on the first row, **Then** the second row is not emitted to any subscriber

---

### Edge Cases

- Subscriber callback panics propagate upward and crash the process/goroutine (no panic recovery)
- Reentrancy not permitted: callbacks must not trigger nested events (DBFile callbacks must not write to database; RowEmitter callbacks must not trigger new row completions)
- DBFile size never decreases (append-only architecture guarantees monotonic growth)
- RowEmitter initialized with complete rows emits only future rows per FR-008 (no historical events)
- RowEmitter handles DBFile size changes regardless of row completion state; if size increases without forming a complete row, no events are emitted (only complete rows trigger notifications)
- Subscribers execute synchronously and sequentially; long-running subscriber callbacks will block subsequent subscribers and the write operation (no timeout mechanism)
- Unsubscribe during active notification cycle takes effect immediately; unsubscribed callback will not receive further notifications in current cycle

## Implementation Notes

### Subscription Lifecycle

Close() methods on RowEmitter are provided primarily for unit testing and resource cleanup in test scenarios. In production usage, subscribers are expected to remain active for the lifetime of the database connection with no need for dynamic unsubscription during normal operation. The synchronous, single-threaded design means there are no background goroutines requiring cleanup during startup or shutdown.

## Component Decoupling Requirements

This feature introduces the RowEmitter as a reusable notification layer that decouples row-level event processing from file-level write operations.

### DBFile Notification

DBFile MUST support a mechanism for other components to be notified when data is written. This enables RowEmitter to monitor file changes without DBFile needing to know about rows or application-level concerns.

### RowEmitter Capabilities

RowEmitter MUST act as an intermediary layer that:
- Monitors DBFile for changes via subscription
- Determines when complete rows have been written based on file size growth
- Notifies interested components about row completion via its own subscription mechanism
- Supports multiple independent subscribers without tight coupling

This design allows any component to subscribe to row completion events without requiring modifications to DBFile or other subscribers.

## Clarifications

### Session 2026-01-31

- Q: How does the RowEmitter determine if it's initialized with a DBFile in a PartialDataRow state? → A: RowEmitter queries DBFile during initialization to determine if last row is partial
- Q: What happens when a subscriber's callback panics? → A: Panic propagates upward and crashes the process/goroutine
- Q: How does the RowEmitter handle DBFile size decreasing (e.g., file truncation or corruption)? → A: Not possible, DBFile is append-only
- Q: What happens when the RowEmitter is initialized with a DBFile that already has complete rows? → A: Only emit future rows (already specified in FR-008)
- Q: How does unsubscribing during an active notification cycle behave? → A: Takes effect immediately; won't receive further notifications this cycle

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: RowEmitter MUST support multiple independent components subscribing to row completion notifications
- **FR-002**: RowEmitter MUST emit a notification when a complete row is written to DBFile
- **FR-003**: RowEmitter MUST detect when initialized with a partial row and emit notification when that row completes
- **FR-004**: RowEmitter MUST emit separate notifications for each completed row in chronological order when multiple rows are written since the last notification
- **FR-005**: RowEmitter MUST NOT emit notifications for rows that were already complete before initialization
- **FR-006**: When a subscriber returns an error during notification, the error MUST propagate back to the caller and prevent subsequent subscribers from receiving the notification
- **FR-007**: Unsubscribing from notifications MUST take effect immediately, preventing further notifications to that subscriber

### Key Entities

- **DBFile**: The database file that receives written data. Supports notifying other components when data is written via a Subscribe() method.

- **RowEmitter**: Component that monitors the database file for changes, detects when complete rows have been written based on file size growth, and notifies interested components via its own Subscribe() method. Acts as a decoupling layer between file-level and row-level concerns.

- **Subscriber Components**: Any component that needs to be notified when rows are complete. Examples include indexers, validators, or query engines. Components subscribe to RowEmitter and receive row completion events.

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_036_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Components can subscribe and unsubscribe from row events without modifying DBFile or RowEmitter implementation code
- **SC-002**: Row completion events are delivered to all subscribers in the correct chronological order with zero missed rows
- **SC-003**: System correctly handles RowEmitter initialization in all DBFile states (empty, complete rows, partial row)
- **SC-004**: Error propagation from subscriber callbacks successfully prevents subsequent subscribers from executing and returns error to caller
- **SC-005**: Multiple independent subscribers can receive row events without interfering with each other's processing

### Data Integrity & Correctness Metrics

- **SC-006**: Zero missed row events in test scenarios with 1000+ row writes and multiple subscribers
- **SC-007**: All partial row completion scenarios result in exactly one event emission when the row completes
- **SC-008**: All multi-row write scenarios emit the correct number of events in the correct order (verified by sequence numbers)
- **SC-009**: All error propagation scenarios correctly stop notification chains and return the first error encountered
- **SC-010**: Subscription and unsubscription operations maintain consistent callback lists with zero memory leaks
