# Feature Specification: FileManager

**Feature Branch**: `014-file-manager`  
**Created**: 2026-01-20  
**Status**: Draft  
**Input**: User description: "Implement a FileManager struct abstraction for thread-safe file operations. The FileManager helps reading and writing raw bytes starting after the initial header & checksum row. For reading, the FileManager should support Read(start int64, size int) (byte[], error). Since the underlying file is append only, and the FileManager is the only object that will be writing, it can allow concurrent read access to any byte less than the last written one. For writing, the abstraction should help enforce a pattern of only one caller (usually a Transaction), being able to write at once. To accomplish this, the FileManager should support SetWriter(<-chan Data) error. FileManager will error out if it is already listening to a writer, otherwise it will accept writes until closed. The Data payload should contain a response channel, to allow callers to wait for the write to be completed. This is spec 014, and do NOT implement any code, as your job is to follow the instructions and generate the spec"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Concurrent Safe File Reading (Priority: P1)

Database operations need to read raw bytes from the file safely while other operations are potentially writing to the same file. The FileManager must allow multiple readers to access different parts of the file concurrently without interfering with each other or with ongoing writes.

**Why this priority**: Thread-safe concurrent reading is fundamental to the frozenDB architecture where reads should not block writes and vice versa.

**Independent Test**: Can be fully tested by creating multiple goroutines that read different byte ranges from the file simultaneously while verifying no data corruption or race conditions occur.

**Acceptance Scenarios**:

1. **Given** a file with existing data, **When** multiple readers call Read with overlapping ranges, **Then** all readers receive consistent byte data without errors
2. **Given** a file with data, **When** a reader attempts to read beyond the current file end, **Then** the operation returns an appropriate error without blocking
3. **Given** ongoing write operations, **When** readers access byte ranges less than the last written byte, **Then** reads complete successfully without interference

---

### User Story 2 - Exclusive Write Access Control (Priority: P1)

Database transactions need to acquire exclusive write access to ensure data consistency. The FileManager must enforce that only one caller can write at any given time, preventing concurrent writes that could corrupt the append-only file format.

**Why this priority**: Exclusive write access is critical for maintaining data integrity in an append-only architecture where multiple writers could cause file corruption.

**Independent Test**: Can be fully tested by having multiple goroutines attempt to get writer access simultaneously and verifying that only one succeeds while others receive appropriate errors.

**Acceptance Scenarios**:

1. **Given** no active writer, **When** a caller requests SetWriter, **Then** the caller successfully obtains writer access
2. **Given** an active writer exists, **When** another caller requests SetWriter, **Then** the operation returns an error indicating writer already exists
3. **Given** an active writer completes and closes, **When** a new caller requests SetWriter, **Then** the new caller successfully obtains writer access

---

### User Story 3 - Asynchronous Write Completion (Priority: P2)

Database transactions need to know when their writes have completed successfully. The FileManager must provide a mechanism for callers to wait for write completion through response channels included in the Data payload.

**Why this priority**: Write completion confirmation is essential for transaction semantics and ensuring data durability before acknowledging success to the calling code.

**Independent Test**: Can be fully tested by submitting write operations through the Data payload and verifying that response channels are properly signaled with success/error status after the write completes.

**Acceptance Scenarios**:

1. **Given** a successful write operation, **When** the Data payload includes a response channel, **Then** the channel receives a success indication after write completion
2. **Given** a failed write operation, **When** the Data payload includes a response channel, **Then** the channel receives an error indication describing the failure
3. **Given** multiple pending writes, **When** writes complete in order, **Then** corresponding response channels are signaled in the same order

---

### Edge Cases & Failure Handling

- **File system out of space**: Return error on response channel and close writer (FR-010 error handling)
- **File corruption detection**: Return error immediately and stop read operation (data integrity preservation)
- **Process crash during write**: No recovery mechanism in FileManager - corruption detection handled by higher layers
- **Large read ranges**: No size limits - caller responsible for memory management

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: FileManager MUST provide Read(start int64, size int) (byte[], error) that returns raw bytes from the specified starting position
- **FR-002**: FileManager MUST allow concurrent read operations for any byte ranges less than the current file end
- **FR-003**: FileManager MUST enforce exclusive write access through SetWriter(<-chan Data) error
- **FR-004**: SetWriter MUST return an error if another writer is already active
- **FR-005**: FileManager MUST accept writes through the Data channel until the channel is closed
- **FR-006**: Data payload MUST contain a response channel for write completion notification
- **FR-007**: FileManager MUST maintain thread-safe access to the underlying file
- **FR-008**: Read operations MUST access only stable, written data (not partial in-flight writes)
- **FR-009**: FileManager MUST track the current file end position for boundary enforcement
- **FR-010**: Write operations MUST append data to the file in the order received through the channel
- **FR-011**: FileManager MUST return errors immediately upon detecting file corruption during reads
- **FR-012**: FileManager MUST signal write errors via response channels and release writer exclusivity
- **FR-013**: FileManager MUST impose no artificial read size limits

### Key Entities *(include if feature involves data)*

- **FileManager**: Thread-safe abstraction for concurrent file operations with exclusive write control
- **Reader**: Concurrent operation that accesses byte ranges within the file
- **Data**: Struct with fields `Bytes []byte` and `Response chan error` for write operations
- **Writer**: Exclusive operation that appends byte data to the file

## Clarifications

### Session 2026-01-20
- Q: What is the exact structure of the Data payload for write operations? → A: Data struct with bytes []byte and response chan error
- Q: How should FileManager handle file system errors during writes? → A: Return error on response channel and close writer
- Q: What is the maximum read size to prevent memory exhaustion? → A: No limit - caller responsibility
- Q: How should FileManager detect and handle file corruption during reads? → A: Return error immediately
- Q: What recovery mechanism should handle process crashes during writes? → A: No recovery needed, out of scope

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_014_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Read operations complete within 10ms for byte ranges up to 1MB
- **SC-002**: 1000 concurrent readers can operate simultaneously without performance degradation
- **SC-003**: Writer acquisition completes within 5ms when no other writer is active
- **SC-004**: All write operations receive completion signals within 100ms of successful disk write

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption incidents detected across all concurrent read/write test scenarios
- **SC-006**: All read operations return consistent data regardless of concurrent write activity
- **SC-007**: Memory usage scales with read size but FileManager adds no constant overhead
- **SC-008**: Transaction atomicity preserved in all simulated crash scenarios during write operations
- **SC-009**: No race conditions detected in thread safety validation tests
- **SC-010**: File append ordering is preserved across all concurrent write scenarios
- **SC-011**: Error signals propagate through response channels within 10ms of detection
- **SC-012**: File corruption detection completes within 5ms for any read operation