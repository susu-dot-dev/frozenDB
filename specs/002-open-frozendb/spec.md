# Feature Specification: Open frozenDB Files

**Feature Branch**: `002-open-frozendb`  
**Created**: 2026-01-09  
**Status**: Draft  

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Open Database in Read Mode (Priority: P1)

A developer wants to open an existing frozenDB database file in read mode to query data without modifying it. They need to ensure the file is properly validated and can be accessed concurrently with other readers and writers.

**Why this priority**: This is the most common use case for frozenDB - reading existing data safely without modification risk while supporting concurrent access patterns.

**Independent Test**: Can be fully tested by opening a valid database file in read mode and performing read operations, delivering immediate data access without write capabilities with concurrent access coordination.

**Acceptance Scenarios**:

1. **Given** a valid frozenDB file exists, **When** developer calls NewFrozenDB with "read" mode, **Then** database opens successfully and allows read operations
2. **Given** a valid frozenDB file is open by one reader, **When** another reader opens the same file, **Then** both can read concurrently without errors
3. **Given** multiple readers are accessing a database, **When** they perform read operations, **Then** all succeed without interference
4. **Given** an invalid frozenDB file, **When** developer calls NewFrozenDB in read mode, **Then** appropriate error is returned

---

### User Story 2 - Open Database in Write Mode (Priority: P1)

A developer needs to open a frozenDB database file in write mode to add new data. They require exclusive access to prevent data corruption and need proper resource management with concurrent access coordination.

**Why this priority**: Write operations are essential for database functionality and require strict locking to maintain data integrity in multi-process environments.

**Independent Test**: Can be fully tested by opening a database file in write mode and verifying exclusive lock acquisition, providing data modification capabilities with proper concurrent access control.

**Acceptance Scenarios**:

1. **Given** a valid frozenDB file exists and no other writers, **When** developer calls NewFrozenDB with "write" mode, **Then** database opens successfully with exclusive lock
2. **Given** a database is open by a writer, **When** another developer tries to open in write mode, **Then** WriteError is returned immediately
3. **Given** a database is open by readers, **When** developer opens in write mode, **Then** exclusive lock is acquired after validation
4. **Given** multiple readers are accessing a database, **When** a writer attempts to open, **Then** writer coordinates properly with existing readers
5. **Given** a writer has exclusive access, **When** readers attempt to open, **Then** readers can access without conflicts

---

### User Story 3 - Resource Management and Cleanup (Priority: P2)

A developer needs to properly manage database resources by closing connections and releasing file locks when operations are complete, even if errors occur during opening.

**Why this priority**: Proper resource management prevents file descriptor leaks and lock contamination, crucial for production systems.

**Independent Test**: Can be fully tested by opening databases in both modes and verifying proper cleanup on close and error conditions.

**Acceptance Scenarios**:

1. **Given** an open database connection, **When** Close() is called, **Then** file descriptor is closed and locks are released
2. **Given** multiple Close() calls on same instance, **When** subsequent calls are made, **Then** operations are idempotent without errors
3. **Given** an error occurs during database opening, **When** opening fails, **Then** all acquired resources are properly released

---

### Edge Cases

- When the database file is corrupted during header validation: Return CorruptDatabaseError
- When file permission denied errors occur during opening: Return PathError (OS-level permission enforcement)
- When system runs out of file descriptors: Return PathError or system error from file operations
- When database files don't exist: Return PathError for missing file
- When file system becomes read-only during operation: Return error on next write operation (not on open)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide NewFrozenDB(path string, mode string) (*FrozenDB, error) function for opening database files
- **FR-002**: System MUST support MODE_READ = "read" and MODE_WRITE = "write" constants for file access modes
- **FR-003**: System MUST validate mode parameter and use spec 001 file path semantics
- **FR-004**: System MUST open file descriptor, then validate frozenDB v1 header per v1_file_format.md
- **FR-005**: System MUST acquire exclusive lock only after valid header AND mode is MODE_WRITE
- **FR-006**: System MUST maintain open file descriptor and lock until Close() is called
- **FR-007**: System MUST provide idempotent Close() method that flushes, closes fd, and releases locks
- **FR-008**: System MUST allow multiple readers and at most one writer to open a new instance concurrently
- **FR-009**: System MUST return WriteError immediately when trying to open database in write mode while opened by another writer
- **FR-010**: System MUST ensure operations on different database files do not interfere with each other
- **FR-011**: System MUST use fixed memory regardless of database file size
- **FR-012**: System MUST close file descriptors and release any acquired locks for ALL error conditions
- **FR-013**: System MUST return CorruptDatabaseError for header validation failures
- **FR-014**: System MUST return WriteError for lock acquisition failures (file in use)
- **FR-015**: System MUST reuse InvalidInputError for invalid path/mode parameters
- **FR-016**: System MUST reuse PathError for filesystem access issues

### Key Entities *(include if feature involves data)*

- **FrozenDB Instance**: Represents an open database connection with file descriptor, lock status, and access mode
- **File Lock**: System-level file lock controlling concurrent access between readers and writers
- **Database Header**: FrozenDB v1 format header containing metadata and validation information
- **Access Mode**: Read or write mode determining available operations and locking behavior

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_002_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Clarifications

### Session 2026-01-09

- Q: What is the maximum expected database file size the system should handle? → A: Not size-limited for this spec (only reads first 64 bytes)
- Q: What specific security model should be used for file access permissions? → A: OS native permissions
- Q: What is the expected concurrency model for readers accessing the same file? → A: Unlimited readers, OS-limited
- Q: What level of logging/observability should be implemented for the open operation? → A: No logging
- Q: What should happen when a write mode open succeeds but the file system becomes read-only during operation? → A: Return error on next write operation

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Database opening completes in under 100ms for typical database files
- **SC-002**: Write lock acquisition fails immediately when another writer holds the lock
- **SC-003**: Resource cleanup completes in under 10ms regardless of database size

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-004**: Zero data loss scenarios in corruption detection tests
- **SC-005**: All concurrent read/write operations maintain data consistency
- **SC-006**: Memory usage remains constant regardless of database size
- **SC-007**: Transaction atomicity preserved in all crash simulation tests
