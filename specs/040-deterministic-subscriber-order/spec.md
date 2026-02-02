# Feature Specification: Deterministic Subscriber Callback Order

**Feature Branch**: `040-deterministic-subscriber-order`  
**Created**: 2026-02-01  
**Status**: Draft  
**Input**: User description: "040 Subscriber deterministic order. We always want subscribe callbacks to be in-order of the subscribe order"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Callback Execution Order Guarantee (Priority: P1)

When multiple components subscribe to database updates, the system must invoke their callbacks in the exact order they were registered. This ensures predictable behavior for error handling (callbacks stop on first error in registration order) and any dependencies between callbacks.

**Why this priority**: This is critical for correctness. FR-006 from spec 039 explicitly requires callbacks to be invoked in registration order. The current implementation violates this requirement due to Go map iteration randomness, causing flaky tests and unpredictable production behavior.

**Independent Test**: Can be fully tested by registering multiple callbacks, triggering an update cycle, and verifying callbacks execute in registration order (1, 2, 3, 4, 5).

**Acceptance Scenarios**:

1. **Given** five callbacks are registered in sequence (callback 1, 2, 3, 4, 5), **When** a file update triggers the notification cycle, **Then** callbacks are invoked in registration order: 1, 2, 3, 4, 5
2. **Given** three callbacks are registered, **When** the middle callback (2) unsubscribes before an update, **Then** remaining callbacks (1, 3) are still invoked in their original registration order

---

### Edge Cases

- What happens when a callback unsubscribes itself during execution? (Answer: Current execution completes, callback not in future snapshots)
- What happens when all callbacks are unsubscribed? (Answer: Snapshot returns empty slice, no callbacks invoked)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Subscriber MUST maintain callbacks in registration order using an ordered data structure
- **FR-002**: Snapshot() method MUST return callbacks in the exact order they were registered via Subscribe() calls
- **FR-003**: When a callback unsubscribes, the system MUST remove it from the ordered collection without affecting the relative order of remaining callbacks
- **FR-004**: All existing spec tests for Subscriber (Test_S_039_FR_006_*) MUST pass without modification after implementation
- **FR-005**: The implementation MUST be thread-safe with mutex protection for all operations on the ordered collection

### Key Entities

- **Subscriber**: Generic component that manages callback registration and notification in registration order. Contains an ordered collection of callbacks, a mutex for thread-safety, and a counter for generating unique subscription IDs.

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that validate the requirement exactly as specified. Tests are placed in `internal/frozendb/subscriber_test.go` following naming convention `Test_S_040_FR_XXX_Description()`.

**Note**: FR-004 requires existing spec test `Test_S_039_FR_006_CallbacksInRegistrationOrder` to pass consistently (no longer flaky).

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The flaky test `Test_S_039_FR_006_CallbacksInRegistrationOrder` passes 100 consecutive times without failure
- **SC-002**: All existing subscriber unit tests pass without modification after implementation change

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-003**: All existing file_manager_spec_tests pass without modification (behavioral compatibility maintained)
- **SC-004**: Thread-safety tests demonstrate no race conditions under concurrent Subscribe/Unsubscribe/Snapshot operations

## Assumptions *(mandatory)*

- Unsubscribe operations are rare in production (mainly during cleanup/Close operations)
- The existing Subscriber API (Subscribe, Snapshot, unsubscribe closure) remains unchanged - this is an internal implementation fix

## Dependencies

- No external dependencies - this is an internal refactoring of the Subscriber implementation
- Depends on existing test infrastructure in `internal/frozendb/subscriber_test.go` and `internal/frozendb/file_manager_spec_test.go`
- Must maintain compatibility with existing usages in FileManager, RowEmitter, SimpleFinder, InMemoryFinder, and BinarySearchFinder

## Out of Scope

- Changes to the public Subscriber API (Subscribe, Snapshot signatures remain unchanged)
- Callback priority or weight systems - order is purely based on registration time
