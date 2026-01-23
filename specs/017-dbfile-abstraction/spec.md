# Feature Specification: DBFile Read/Write Modes and File Locking

**Feature Branch**: `017-dbfile-abstraction`  
**Created**: 2026-01-22  
**Status**: Draft

## Clarifications

### Session 2026-01-22

- Q: How should DBFile interface be enhanced to support read and write modes with appropriate locking? → A: Add new constructor `NewDBFile(path, mode)` that returns configured interface, keeping existing methods unchanged
- Q: What are the specific performance expectations for concurrent reader scenarios? → A: No specific performance target - just ensure no deadlocks or blocking between readers
- Q: What are the exact expectations for file lock cleanup when applications crash or are terminated unexpectedly? → A: OS automatically releases locks when process terminates - no additional cleanup mechanism required  
**Input**: User description: "Add the ability for DBFile to be opened in read or read/write mode. Also add the ability to set the appropriate Flocks when opening the file. Then, route the relevant functions in open.go to use DBFile instead, as the only underlying struct for file operations. Refactoring the create codepath is explicitly out of scope as the usecase is too different"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Open DBFile in Read Mode (Priority: P1)

Applications need to open existing database files in read-only mode for querying data without modifying the database. This enables multiple concurrent readers while maintaining data integrity.

**Why this priority**: Read access is fundamental for all frozenDB applications that need to query existing data without modification

**Independent Test**: Can be fully tested by opening a database file in read mode and verifying that read operations succeed while write operations are properly blocked

**Acceptance Scenarios**:

1. **Given** an existing valid frozenDB file, **When** opening it in read mode, **Then** the DBFile opens successfully and allows read operations
2. **Given** a DBFile opened in read mode, **When** attempting write operations, **Then** all write operations return appropriate errors

---

### User Story 2 - Open DBFile in Write Mode with Exclusive Lock (Priority: P1)

Applications need to open database files in read/write mode with exclusive locking to ensure data integrity during modifications and prevent concurrent writers.

**Why this priority**: Write access with proper locking is essential for data integrity and preventing corruption in multi-process environments

**Independent Test**: Can be fully tested by opening a database in write mode, verifying exclusive lock acquisition, and confirming that concurrent write attempts are blocked

**Acceptance Scenarios**:

1. **Given** an existing valid frozenDB file, **When** opening it in write mode, **Then** the DBFile opens successfully with an exclusive lock
2. **Given** a DBFile opened in write mode, **When** another process attempts to open the same file in write mode, **Then** the second attempt fails with an appropriate lock error
3. **Given** a DBFile opened in write mode, **When** another process attempts to open the same file in read mode, **Then** the read mode opening succeeds (readers should not be blocked by writers)

---

### User Story 3 - Refactor open.go Functions to Use DBFile (Priority: P2)

Internal code refactoring to consolidate file operations through the DBFile interface, eliminating direct os.File usage in open.go functions.

**Why this priority**: Code consolidation improves maintainability and enables better testing through interface abstraction

**Independent Test**: Can be fully tested by verifying that all open.go functions successfully use DBFile for file operations while maintaining existing behavior

**Acceptance Scenarios**:

1. **Given** the refactored code, **When** calling functions that previously used os.File directly, **Then** all operations work identically but now use DBFile interface
2. **Given** existing test suite, **When** running all tests, **Then** all tests pass without modification

---

### Edge Cases

- What happens when attempting to open a non-existent file in read mode?
- How does system handle file permission denied scenarios for both read and write modes?
- What happens when the database file is corrupted during open operations?
- How are file locks properly released when the application crashes or is terminated? (Clarified: OS automatically releases locks on process termination)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A new constructor function `NewDBFile(path string, mode string)` MUST be added to create DBFile instances with appropriate mode configuration for read-only access with no file locking
- **FR-002**: The `NewDBFile(path string, mode string)` constructor MUST support read-write mode with exclusive file locking when mode parameter indicates write access
- **FR-003**: File locking MUST use OS-level flocks to ensure cross-process coordination
- **FR-004**: Multiple concurrent readers MUST be allowed to access the same file simultaneously
- **FR-005**: Only one writer MUST be allowed to access a file at any given time
- **FR-006**: open.go functions MUST be refactored to use DBFile interface instead of direct os.File operations
- **FR-007**: The create codepath MUST NOT be refactored as it has different use cases
- **FR-008**: File locks MUST be properly released when DBFile is closed
- **FR-009**: Read mode attempts on non-existent files MUST follow existing error handling patterns in open.go (return PathError for non-existent files)
- **FR-010**: Write mode MUST use only non-blocking lock acquisition to fail fast if another process has the file locked, matching current implementation behavior

### Key Entities *(include if feature involves data)*

- **DBFile Interface**: Existing interface enhanced with new `NewDBFile(path, mode)` constructor for mode-based opening, keeping all existing methods unchanged
- **File Mode Enumeration**: Read and write access modes with corresponding lock behaviors
- **File Lock Manager**: Component responsible for acquiring and releasing OS-level file locks

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

- **SC-001**: Applications can open database files in read mode and perform read operations within 100ms
- **SC-002**: Applications can open database files in write mode and acquire exclusive locks within 200ms
- **SC-003**: Multiple concurrent readers can access the same file without deadlocks or blocking between readers
- **SC-004**: Concurrent write attempts to the same file are properly blocked and return appropriate errors within 50ms

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption scenarios in concurrent read/write testing
- **SC-006**: All file locking operations maintain proper exclusivity guarantees
- **SC-007**: File resources are properly released in all error scenarios; crash scenarios rely on OS automatic lock release
- **SC-008**: Existing API behavior is preserved after refactoring to use DBFile interface