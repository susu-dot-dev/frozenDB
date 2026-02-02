# Feature Specification: Read-Mode File Updates

**Feature Branch**: `039-read-mode-updates`  
**Created**: 2026-02-01  
**Status**: Draft  
**Input**: User description: "039 read-mode-updates When FrozenDB is opened in read mode, it should remain up-to-date when a different process in write mode updates the file. Right now, the size is read once and never updated. Instead, a file watcher should be used, only when frozen db is opened in read-only mode, which should trigger the DBFile to update its size & trigger subscription callbacks when that changes. It's important to make sure there isn't a timing window - DBFile should make sure that all writes are caught, even in the window between when DBFile is created to the time when the watcher is running. Some more constraints: We don't want ANY public API changes as part of this, so the FileWatcher should be an internal implementation of DBFile. Next, we MUST ensure that file size updates and subscriptions are serialized. There should only be one active update activity (where update is updating the file size and calling any callbacks), at a time. The spec should explicitly require the use of github.com/fsnotify/fsnotify to simplify the file notification process"

## Clarifications

### Session 2026-02-01

- Q: When the file watcher fails to start during DBFile creation (FR-007), should the database fall back to static-size read mode or fail completely? → A: Fail DBFile creation entirely with a descriptive error (fail-fast)
- Q: When a subscriber callback returns an error during the update cycle, should the system stop processing remaining callbacks or continue invoking them? → A: Stop processing remaining callbacks immediately (FR-006, FR-009). The callback that failed MUST have already handled its own error state before returning (e.g., Finder tombstones itself in FR-010)
- Q: When a Finder's Refresh() callback encounters an error, how should it handle the error given that it's running in a background goroutine with no user action to propagate to? → A: Finder MUST set its tombstoned flag to true BEFORE returning the error (FR-010). All subsequent Finder operations MUST check tombstoned state first and return TombstonedError (FR-011). This prevents serving stale data when refresh fails asynchronously.
- Q: Does the Finder tombstoning pattern apply only to read mode, or to write mode as well? → A: Applies to BOTH modes. Finder callbacks run in background goroutines in both read mode (file watcher) and write mode (processWrite). The error handling pattern is identical regardless of which mode triggered the update.
- Q: When the file watcher detects a modification but the file size hasn't changed (metadata-only update like timestamp), should an update cycle execute? → A: Run cycle; size check detects no change, no callbacks invoked
- Q: When DBFile.Close() is called while a file watcher update cycle is actively processing callbacks, should Close() block until the cycle completes or interrupt it immediately? → A: Block Close() until active update cycle completes
- Q: When the system detects file size reduction (truncation), which violates append-only semantics (AS-003), what should happen? → A: Ignore, not possible by OS semantics

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Real-Time Multi-Process Database Reads (Priority: P1)

As a database consumer, when I open FrozenDB in read mode while another process is writing to it, I need to query and retrieve newly written rows using Get() operations without reopening the file or polling, so that my application can access fresh data in near real-time, including during the critical initialization window.

**Why this priority**: This is the core value proposition. Without real-time updates, read-mode database instances become stale - Get() operations won't find newly written keys, making the database unusable for applications that need fresh data. This includes handling the initialization period correctly with no race conditions. This is the minimum viable functionality that makes the feature useful.

**Independent Test**: Can be fully tested by opening a database in read mode in one process (including during active writes), writing data with specific keys from another process, and verifying that Get() operations in the read-mode process can successfully retrieve all newly written rows with no gaps.

**Acceptance Scenarios**:

1. **Given** a database opened in read mode in Process A, **When** Process B (in write mode) appends a new row with key K1, **Then** Process A can successfully retrieve K1 using Get(K1)
2. **Given** a database opened in read mode that initially contains keys K1-K10, **When** another process writes keys K11-K20, **Then** Get() operations for K11-K20 return the newly written data
3. **Given** a database opened in read mode, **When** multiple write operations occur in rapid succession from another process, **Then** subsequent Get() operations can retrieve all newly written keys
4. **Given** a database being actively written to with keys K1, K2, K3..., **When** a read-mode database is opened after K5 is written, **Then** Get() operations successfully retrieve K1 through at least K5 with no missing keys
5. **Given** a database being written to during read-mode initialization, **When** keys are written in the window between file opening and watcher activation, **Then** all written keys are retrievable via Get() with no gaps

---

### User Story 2 - Consistent Query Results During Updates (Priority: P1)

As a database consumer, when my read-mode database receives file updates from another process, I need Get() operations to return consistent results that reflect a coherent database state, so that I don't see partial updates or race conditions in my query results.

**Why this priority**: Without serialized internal updates, Get() operations could return inconsistent results - seeing partial file size updates or racing with Finder updates. This would make the database unreliable for any practical use. Serialization is essential for correct query behavior.

**Independent Test**: Can be fully tested by performing Get() operations during rapid file updates and verifying that each Get() result is consistent with a valid database state (never returning partial or corrupted data).

**Acceptance Scenarios**:

1. **Given** a read-mode database with frequent updates from another process, **When** Get() operations are performed during updates, **Then** each Get() returns results consistent with a complete file state (not partial updates)
2. **Given** a read-mode database receiving rapid file modifications, **When** Get() is called for newly written keys, **Then** results are deterministic and consistent (the same key always returns the same value until the next update)
3. **Given** multiple rapid file modifications occurring, **When** Get() operations query different keys, **Then** all results reflect a coherent database state with no torn reads or inconsistent views

---

### User Story 3 - Error Propagation from Background Updates (Priority: P1)

As a database consumer, when a Finder refresh error occurs during background file updates (read mode or write mode), I need my next Get() operation to return an error indicating the Finder is unusable, so that I don't receive stale or incorrect data after a refresh failure.

**Why this priority**: Background update callbacks (file watcher or processWrite) run in goroutines with no user action to propagate errors to. If a Finder's refresh fails but continues returning cached results, users receive stale data with no indication of the problem. Tombstoning ensures asynchronous errors are surfaced on the next user action, maintaining data consistency.

**Independent Test**: Can be fully tested by simulating a Finder refresh error during a background update cycle (inject error in Refresh()), then verifying that subsequent Get() operations return TombstonedError instead of stale data.

**Acceptance Scenarios**:

1. **Given** a read-mode database with an active Finder, **When** the Finder's Refresh() callback encounters an error during a file update cycle, **Then** the Finder tombstones itself before returning the error
2. **Given** a tombstoned Finder from a prior refresh error, **When** a user calls Get() for any key, **Then** Get() returns TombstonedError without attempting to access potentially stale data
3. **Given** a tombstoned Finder, **When** a user calls any public Finder method (Get, All, etc.), **Then** all methods return TombstonedError consistently
4. **Given** a write-mode database with an active Finder, **When** Finder.Refresh() fails during a processWrite callback, **Then** the same tombstoning behavior occurs as in read mode (mode-independent error handling)
5. **Given** multiple subscribers where one fails, **When** a callback returns an error, **Then** FileManager stops invoking remaining callbacks and the failed callback has already tombstoned itself

---

### User Story 4 - Serialized State Updates (Priority: P2)

As a frozenDB maintainer, when implementing read-mode file watching, I need all internal state updates (file size and Finder synchronization) to be serialized just like write-mode operations are, so that the codebase remains simple and doesn't need to handle concurrent state modifications.

**Why this priority**: FrozenDB's architecture keeps things simple by serializing writes through a single writer goroutine. Extending this serialization model to read-mode updates maintains code simplicity and avoids introducing complex concurrent state management. While not directly user-facing, this design constraint is critical for maintainability and correctness.

**Independent Test**: Can be fully tested by triggering rapid file modifications while monitoring internal state updates, verifying that all updates (size changes and callback invocations) happen serially with no overlapping execution.

**Acceptance Scenarios**:

1. **Given** a read-mode database receiving file update notifications, **When** multiple notifications arrive in rapid succession, **Then** each update cycle (size update + callbacks) completes fully before the next one begins
2. **Given** file watching active in read mode, **When** an update cycle is processing, **Then** subsequent file notifications are queued and processed serially
3. **Given** the existing write-mode serialization model (single writer goroutine), **When** implementing read-mode updates, **Then** the same serialization pattern is applied to maintain architectural consistency

---

### Edge Cases

- **RESOLVED**: When the file watcher detects a modification but the file size hasn't changed (metadata-only update), update cycle runs but detects no size change, so no callbacks are invoked (listening to fsnotify.Write events only reduces these occurrences)
- **RESOLVED**: File size reduction (truncation) is not possible by OS semantics given append-only operations; no special handling required
- **RESOLVED**: If a subscriber callback returns an error during processing, FileManager stops processing remaining callbacks immediately (FR-006, FR-009). The callback MUST have handled its own error state before returning - specifically, Finder MUST tombstone itself on ANY Refresh() error (FR-010, FR-011), ensuring subsequent Get() calls return TombstonedError instead of stale data
- **RESOLVED**: Finder tombstoning applies to BOTH read mode and write mode. Update cycle callbacks run in background goroutines in both modes, so Finder must handle refresh errors identically regardless of mode
- How does the system behave when the watched file is deleted or renamed while being watched?
- **RESOLVED**: When the file watcher fails to start (e.g., system resource limits), DBFile creation fails entirely with a descriptive error (fail-fast, no fallback)
- How does the system handle rapid successive file modifications faster than callbacks can process?
- **RESOLVED**: When DBFile.Close() is called while a file watcher is active, Close() blocks until any active update cycle completes before stopping the watcher and cleaning up
- How does the system handle file system events that arrive after Close() has been called but before the watcher fully shuts down?

## Requirements *(mandatory)*

### Functional Requirements

#### Core File Watching (Read Mode Only)

- **FR-001**: File watcher be created ONLY when opened in read-only mode, and the watcher MUST listen ONLY for `fsnotify.Write` events and update internal file size when modifications are detected

#### Race-Free Initialization

- **FR-002**: Initial file size MUST be captured during DBFile creation before the watcher starts
- **FR-003**: DBFile MUST ensure zero timing gaps between file opening and watcher activation where writes could be missed

#### Serialized Update Cycle

- **FR-004**: All update cycles (file size update + subscriber callbacks) MUST execute serially with only one active cycle at a time
- **FR-005**: When an update cycle executes, file size update MUST complete before any subscriber callbacks are invoked; if file size has not changed since the last update, no callbacks are invoked
- **FR-006**: Subscriber callbacks MUST be invoked in registration order synchronously within the update cycle; first callback error stops processing remaining callbacks (matching existing RowEmitter behavior in write mode)

#### Subscriber Error Handling

- **FR-009**: When a subscriber callback returns an error during an update cycle (read mode or write mode), the callback MUST have handled the error appropriately before returning; FileManager stops invoking remaining callbacks but does NOT attempt to handle or propagate the error (no user action to return error to)
- **FR-010**: Finder implementations MUST include a tombstoned state (atomic boolean); when Finder.Refresh() encounters ANY error, it MUST set tombstoned=true BEFORE returning the error
- **FR-011**: After a Finder is tombstoned, ALL public Finder methods (Get, All, etc.) MUST check the tombstoned state FIRST and return TombstonedError without attempting to access potentially stale data; tombstoned state is permanent for the Finder's lifetime

#### Lifecycle Management

- **FR-007**: If file watcher fails to start during DBFile creation, DBFile creation MUST fail entirely with a descriptive error (no fallback to static-size mode)
- **FR-008**: When DBFile.Close() is called on a read-mode instance, Close() MUST block until any active update cycle completes, then stop and clean up the file watcher with no goroutine leaks


### Key Entities *(include if feature involves data)*

- **File Watcher**: Internal component of FileManager that monitors file system events using fsnotify, active only in read mode. Detects file modifications and triggers size updates and subscriber notifications. Lifecycle is managed by FileManager. Note: Finder (the query engine) subscribes to these notifications to stay synchronized with file changes, enabling Get() operations to find newly written rows.

- **Update Cycle**: A serialized sequence consisting of: (1) detecting file modification event, (2) updating internal file size, (3) invoking all subscriber callbacks in order (including Finder's callback to refresh its view). Only one update cycle can be active at any time. Runs in background goroutine with no user action to propagate errors to.

- **File Modification Event**: A notification from the file system (via fsnotify) indicating the watched file has been modified. Includes event type (Write, Remove, Rename) and file path.

- **Tombstoned Finder**: A Finder that encountered an error during refresh and marked itself as permanently failed. Tombstoned Finders return TombstonedError on all operations to prevent serving stale data. This pattern is essential because refresh errors occur in background goroutines (file watcher or write mode processWrite) where there is no user action to return errors to.

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `internal/frozendb/file_manager_spec_test.go` for FR-001 through FR-009
- Are placed in `internal/frozendb/finder_spec_test.go` for FR-010 and FR-011 (Finder tombstoning)
- Follow naming convention `Test_S_039_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Read-mode Get() operations can retrieve newly written keys within 1 second of write completion by another process (Story 1)
- **SC-002**: Zero timing gaps exist where writes from another process are invisible to Get() during initialization - 100% key retrieval rate in concurrent tests (Story 1)
- **SC-003**: Get() operations return consistent results during updates with no torn reads or partial state visibility (Story 2)
- **SC-004**: All internal state updates (file size and callbacks) execute serially with no concurrent overlapping execution, maintaining the same simplicity model as write-mode operations (Story 4)
- **SC-005**: Write-mode database operations show zero performance impact from the read-mode file watching feature - identical performance metrics before and after (Story 4)
- **SC-011**: When Finder.Refresh() encounters an error during background update (read or write mode), subsequent Get() operations return TombstonedError, never stale data (Story 3)
- **SC-012**: Finder tombstoning behavior is identical in read mode and write mode - same error handling pattern regardless of what triggered the update (Story 3)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-006**: Zero missed file modifications in race condition stress tests (10,000+ rapid writes during initialization)
- **SC-007**: All file size updates are atomic and serialized (no concurrent updates visible)
- **SC-008**: Zero goroutine leaks after repeated open/close cycles (verified through goroutine profiling)
- **SC-009**: Update cycles execute serially in 100% of test cases (no overlapping execution detected)
- **SC-010**: File watcher cleanup completes within 100ms of Close() call under normal conditions
- **SC-013**: When Finder.Refresh() fails, Finder is tombstoned BEFORE error is returned to caller (verified by checking tombstoned flag immediately after Refresh() returns error)
- **SC-014**: After Finder is tombstoned, 100% of public method calls return TombstonedError (Get, All, etc.)

## Assumptions *(optional)*

- **AS-001**: This feature only needs to work on Linux (fsnotify Linux compatibility is sufficient)
- **AS-002**: File system event notifications are delivered reliably by the Linux kernel (inotify mechanisms are reliable)
- **AS-003**: The append-only nature of frozenDB means file modifications will only increase file size, not decrease it
- **AS-004**: Subscriber callbacks are expected to execute quickly (sub-second); long-running callbacks will delay subsequent file modification processing
- **AS-005**: File system event coalescing by fsnotify or the Linux kernel is acceptable (multiple rapid writes may generate fewer events than actual writes)
- **AS-006**: Read-mode instances are not expected to modify the database file, only observe it
- **AS-007**: The file path provided to DBFile remains valid and unchanged for the lifetime of the DBFile instance

## Dependencies *(optional)*

- **DEP-001**: `github.com/fsnotify/fsnotify` library must be added to project dependencies
- **DEP-002**: Existing DBFile interface and FileManager implementation (as defined in `internal/frozendb/file_manager.go`)
- **DEP-003**: Existing subscription mechanism in FileManager (Subscriber pattern)
- **DEP-004**: Existing atomic size tracking (`currentSize atomic.Uint64`)

## Technical Constraints *(optional)*

- **TC-001**: File watcher must operate entirely within the FileManager struct with no public API exposure. There must be zero public API changes from this spec
- **TC-002**: File watcher must not interfere with existing write-mode file locking mechanisms (syscall.Flock)
- **TC-003**: File watcher must not introduce new external dependencies beyond `github.com/fsnotify/fsnotify`
- **TC-004**: Implementation must maintain thread-safety guarantees of existing FileManager implementation
- **TC-005**: File watcher must properly handle SIGTERM and other shutdown signals through standard Go cleanup patterns
