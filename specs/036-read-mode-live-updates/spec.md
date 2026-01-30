# Feature Specification: Read-Mode Live Updates for Finders

**Feature Branch**: `036-read-mode-live-updates`  
**Created**: Fri Jan 30 2026  
**Status**: Draft  
**Input**: User description: "FrozenDB finders should receive updates when opened in read-mode, to handle the scenario when another process writes to the file. That way, as a user I can always retrieve new keys added with data safety upon concurrency. This extra detection is not necessary when opened in write mode, because there is only a single writer allowed. In addition, we want to make sure there are no timing conditions between when the Finder is initialized (where it may perform a scan of the database), to the time the watcher is started, in case there are writes between these two timestamps. As part of the same requirement, we don't want writes occuring during initialization to corrupt the finder process."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Live Key Discovery in Read-Only Access (Priority: P1)

A user opens a FrozenDB database in read-mode while another process is actively writing new keys. The user wants to retrieve all keys, including those added after opening the database, without having to close and reopen the connection.

**Why this priority**: This is the core requirement and enables real-time concurrent access patterns essential for multi-process architectures. Without this, users would need to restart connections to see new data, breaking the immutability promise of the database.

**Independent Test**: Can be fully tested by opening a database in read-mode in one process, writing new keys from a second process, and verifying that queries in the first process return the newly added keys. Delivers immediate value for concurrent read/write scenarios.

**Acceptance Scenarios**:

1. **Given** a FrozenDB database is opened in read-mode, **When** another process adds new keys to the database, **Then** subsequent queries in read-mode return the newly added keys
3. **Given** multiple processes are reading the same database, **When** one separate process writes new data, **Then** all read-mode processes see the updated data independently

---

### User Story 2 - Race-Free Initialization (Priority: P2)

A user initializes a Finder in read-mode at the exact moment another process is writing to the database. The initialization process includes scanning the database and setting up file watchers. The user expects no data corruption, no missing updates, and no race conditions between the scan and watcher setup.

**Why this priority**: Prevents critical data integrity issues during the vulnerable initialization window. This is essential for correctness but is a subset of the P1 scenario focused specifically on startup timing.

**Independent Test**: Can be tested by repeatedly initializing Finders while concurrent writes are happening, verifying that no data is missed and no corruption occurs. Delivers value by ensuring safe initialization patterns.

**Acceptance Scenarios**:

1. **Given** a write process is actively adding keys to the database, **When** a Finder is initialized in read-mode during these writes, **Then** the Finder captures all keys without data loss or corruption
2. **Given** a database scan takes 100ms and a write occurs at 50ms into the scan, **When** the watcher is started at 100ms, **Then** the write at 50ms is not missed by either the scan or the watcher

---

### User Story 3 - Write-Mode Finder Behavior (Priority: P3)

A user opens a database in write-mode. Since only one writer can access the database at a time (enforced by OS-level file locks), the user does not need or expect live update detection for their Finder.

**Why this priority**: This is a non-functional optimization requirement - write-mode Finders should skip the overhead of file watching since concurrent writes cannot occur. Lower priority because it's about efficiency, not correctness.

**Independent Test**: Can be tested by verifying that write-mode Finders do not start file watchers and that their performance is not impacted by the live update mechanism. Delivers value through performance optimization.

**Acceptance Scenarios**:

1. **Given** a database is opened in write-mode, **When** the Finder is initialized, **Then** no file watching mechanism is activated

---

### Edge Cases

- What happens when a file watcher detects a partial write (row is incomplete)? The Finder processes only complete rows (those with ROW_END sentinel); partial rows at the end of the file are naturally excluded by row boundary calculation.
- How does the system handle rapid successive writes (e.g., 1000 writes per second)? The watcher processes rows as fast as the watchLoop goroutine can read and send OnRowAdded notifications. Natural batching occurs through fsnotify's event coalescing - multiple write events are delivered as single notifications without explicit batching logic.
- How does the Finder handle writes that fail integrity checks (e.g., missing ROW_START/ROW_END sentinels)? The Finder should return an error as appropriate
- What happens if the watcher itself fails to start during initialization? The Finder should fail initialization with a clear error rather than silently operating without live updates.
- How does the system handle extremely large batches of writes (e.g., 100,000 keys added at once)? The watchLoop goroutine processes all rows sequentially, sending OnRowAdded notifications for each. Queries are not blocked by this processing since the watchLoop runs asynchronously; Finders with their own state (InMemoryFinder, BinarySearchFinder) update incrementally via OnRowAdded callbacks without blocking query paths.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST enable file watching for Finders opened in read-mode to detect when another process appends new data to the database file
- **FR-002**: System MUST NOT enable file watching for Finders opened in write-mode since exclusive write access prevents concurrent writes
- **FR-003**: Finders in read-mode MUST detect and incorporate new keys added to the database by other processes without requiring the user to close and reopen the database
- **FR-004**: System MUST ensure that Finder initialization (including any initial database scan) and file watcher startup are synchronized such that no writes are missed during the initialization window
- **FR-005**: System MUST ensure that partially-written rows (rows without ROW_END sentinel) are not exposed to Finder queries until the row is complete
- **FR-006**: System MUST handle file watcher failures during Finder initialization by failing the initialization with a clear error message
- **FR-007**: System MUST handle rapid successive writes efficiently without blocking Finder query operations for extended periods

### Key Entities

- **Finder**: The query interface for retrieving keys from a FrozenDB database, which can be opened in either read-mode (allowing concurrent access) or write-mode (exclusive access with write permissions)
- **File Watcher**: A mechanism that monitors the database file for changes (append operations) and notifies the Finder when new data is available
- **Database File**: The append-only file containing all database data, including keys, values, and control structures (transaction markers, checksums, sentinels)
- **Row Sentinel**: Special byte markers (ROW_START 0x1F and ROW_END 0x0A) that delimit complete rows and enable integrity checking

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

- **SC-001**: Users can query newly added keys from a read-mode Finder within 2 seconds of those keys being written by another process
- **SC-002**: Finder initialization completes without data loss even when concurrent writes occur during the initialization window (100% capture rate in stress tests)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-003**: Zero data loss scenarios in concurrent read/write tests with initialization timing variations
- **SC-004**: Finders correctly handle all edge cases (partial writes, file deletion, corruption) without crashes or undefined behavior

## Assumptions

- The file system provides reliable file change notifications through inotify on Linux
- The append-only file format ensures that existing data is never modified, only new data is appended
- OS-level file locks successfully prevent multiple write-mode processes from accessing the same database simultaneously
- File watching incurs minimal overhead on modern operating systems
- Finders perform queries that are compatible with incremental updates (e.g., they don't cache the entire database in memory)
- The database file remains on a local file system (not a network file system where file watching may be unreliable)

## Out of Scope

- Real-time push notifications to client applications (this feature only updates internal Finder state)
- Network-based file system support with reliable change detection
- Rollback or undo mechanisms for detected changes
- Conflict resolution for distributed writes (only local single-writer model is supported)
- Performance optimization for extremely large single writes (>1GB)
- Live updates for databases opened over network protocols

## Dependencies

- Existing FrozenDB file format specification (docs/v1_file_format.md) for row structure and sentinel definitions
- OS-level file locking mechanism already in use for write-mode exclusivity
- **New dependency**: `github.com/fsnotify/fsnotify` for file system event notifications on Linux (inotify backend)
  - **Rationale**: Provides idiomatic Go channel-based API for file watching, eliminating the need for complex syscall.Select() bridge infrastructure with Unix pipes and coordination channels
  - **Trade-off**: This is an exception to the "no external dependencies" guideline, justified by the significant reduction in implementation complexity (avoids bridge goroutines, pipe FDs, and stopChan coordination)
  - **Alternative considered**: Raw syscall.InotifyInit/InotifyAddWatch requires complex bridging between Go channels and file descriptors, essentially reimplementing what fsnotify provides
  - **Library quality**: Production-ready with 10.5k+ stars, used by 314k+ projects, well-maintained
  - **Platform support**: Linux only for this feature (though fsnotify itself is cross-platform)
