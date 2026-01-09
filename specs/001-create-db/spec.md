# Feature Specification: Database File Creation

**Feature Branch**: `001-create-db`  
**Created**: 2025-01-08  
**Status**: Draft  
**Input**: User description: "frozenDB Functional Specification 001: Database File Creation"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Database File Creation without Append-Only (Priority: P1)

As a system administrator, I want to create a new frozenDB database file with the correct header structure so that I have a basic functional key-value store foundation (without immutability protection).

**Why this priority**: This provides the core file creation and header writing functionality that can work without special privileges, enabling basic database operations.

**Independent Test**: Can be fully tested by creating a database file (mocking the append-only step) and verifying it contains a valid frozenDB v1 header.

**Acceptance Scenarios**:

1. **Given** valid path ending in .fdb, valid rowSize (128-65536), and valid skewMs (0-86400000), **When** calling Create function with chattr step skipped, **Then** a new .fdb file is created with proper header
2. **Given** successful creation, **When** inspecting the file, **Then** it has 0644 permissions and contains a valid frozenDB v1 header
3. **Given** invalid row size (outside 128-65536 range), **When** calling Create function, **Then** function returns InvalidInputError
4. **Given** target path already exists, **When** calling Create function, **Then** function returns PathError

---

### User Story 2 - Append-Only Protection Setup (Priority: P2)

As a system administrator with sudo privileges, I want to set the append-only attribute on the database file so that the database maintains immutability guarantees and cannot be accidentally modified.

**Why this priority**: This is the core security feature of frozenDB - without append-only protection, the database loses its immutable properties.

**Independent Test**: Can be fully tested by taking a pre-existing valid database file and successfully applying the append-only attribute.

**Acceptance Scenarios**:

1. **Given** a valid database file exists and running under sudo, **When** setting append-only attribute, **Then** the file gains chattr +a protection
2. **Given** valid database file but not running under sudo, **When** attempting to set append-only attribute, **Then** function returns WriteError
3. **Given** append-only attribute setting fails after header write, **When** function fails, **Then** the original database file is cleaned up
4. **Given** running under sudo with SUDO_USER present, **When** setting attributes, **Then** file ownership is preserved for the original user

---

### User Story 3 - Complete Atomic Creation Process (Priority: P3)

As a system administrator, I want the entire database creation process (header write + append-only protection) to be atomic so that I either get a fully functional immutable database or nothing at all.

**Why this priority**: Prevents partial states where files exist without proper protection, ensuring system integrity.

**Independent Test**: Can be fully tested by running the complete creation process and verifying either success (fully protected file) or failure (no file remains).

**Acceptance Scenarios**:

1. **Given** valid parameters and sudo context, **When** calling Create function, **Then** atomic creation completes with append-only protection applied
2. **Given** any step fails during creation, **When** function exits with error, **Then** no partial files remain on filesystem
3. **Given** system crash during creation, **When** system restarts, **Then** either fully protected file exists or no file exists
4. **Given** append-only step fails, **When** cleanup occurs, **Then** header-written file is removed to prevent corruption

---

### Edge Cases

- What happens when system crashes during file creation?
- How does system handle interrupted attribute setting operations?
- What happens when file ownership changes fail after successful creation?
- How does system handle invalid characters in file paths?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST create database files using Create(config CreateConfig) function with direct struct initialization *(Constitutional: Immutability First)*
- **FR-002**: System MUST validate sudo context via SUDO_USER environment variable presence *(Constitutional: Data Integrity)*
- **FR-003**: System MUST reject direct root execution without sudo context
- **FR-004**: System MUST reject unprivileged user execution without sudo
- **FR-005**: System MUST validate SUDO_UID and SUDO_GID are present and valid integers when SUDO_USER exists
- **FR-006**: System MUST create files with O_CREAT|O_EXCL flags for atomic operation *(Constitutional: Concurrent Read-Write Safety)*
- **FR-007**: System MUST set file permissions to 0644 (owner rw, group/others r)
- **FR-008**: System MUST write frozenDB v1 format header according to specification *(Constitutional: Data Integrity)*
- **FR-009**: System MUST flush data to disk using fdatasync() before setting attributes *(Constitutional: Data Integrity)*
- **FR-010**: System MUST set append-only attribute using ioctl() with FS_APPEND_FL *(Constitutional: Immutability First)*
- **FR-011**: System MUST use direct syscalls (FS_IOC_SETFLAGS) for append-only attribute, not subprocess calls
- **FR-012**: System MUST set append-only attribute after header flush but before file closure
- **FR-013**: System MUST set file ownership to original user when running under sudo
- **FR-014**: System MUST use syscall.Chown() to set original user ownership after file creation
- **FR-015**: System MUST validate rowSize parameter is between 128-65536 inclusive
- **FR-016**: System MUST validate skewMs parameter is between 0-86400000 inclusive
- **FR-017**: System MUST validate path parameter ends with .fdb extension
- **FR-018**: System MUST ensure parent directory exists and is writable
- **FR-019**: System MUST clean up partially created files and have no other side-effects upon any failure
- **FR-020**: System MUST validate path parameter is a non-empty string following Go's os.OpenFile path conventions
- **FR-021**: System MUST handle absolute paths as absolute filesystem paths
- **FR-022**: System MUST handle relative paths relative to the process's current working directory
- **FR-023**: System MUST NOT perform shell expansion (including ~ for home directory)
- **FR-024**: System MUST validate the path is valid for the target Linux filesystem
- **FR-025**: System MUST allow creation of hidden files (path starting with .)
- **FR-026**: System MUST handle paths up to filesystem maximum length
- **FR-027**: System MUST validate path contains only filesystem-allowed characters
- **FR-028**: System MUST be thread-safe for concurrent calls with different paths *(Constitutional: Concurrent Read-Write Safety)*
- **FR-029**: System MUST ensure file creation is atomic to other processes *(Constitutional: Concurrent Read-Write Safety)*
- **FR-030**: System MUST use fixed memory regardless of parameters *(Constitutional: Performance With Fixed Memory)*
- **FR-031**: System MUST minimize disk operations (single create, write, flush, attribute set, ownership change, close) *(Constitutional: Correctness Over Performance)*
- **FR-032**: System MUST detect validation failures before any filesystem operations *(Constitutional: Correctness Over Performance)*

### Key Entities *(include if feature involves data)*

- **Database File**: Immutable append-only key-value store with frozenDB v1 format
- **File Header**: Metadata structure containing configuration and integrity information
- **Sudo Context**: Security context for privileged operations with original user preservation

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Database file creation completes in under 5 seconds for standard configurations
- **SC-002**: 100% of validation failures are detected before any filesystem operations
- **SC-003**: Zero partially created files remain after any failure scenario
- **SC-004**: All created files maintain append-only protection in Linux filesystem tests

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero corruption scenarios detected in header write integrity tests
- **SC-006**: All concurrent file creation attempts maintain atomicity without race conditions
- **SC-007**: Memory usage remains constant regardless of file size parameters
- **SC-008**: Transaction atomicity preserved in all crash simulation tests during creation
- **SC-009**: File ownership correctly set to original user in 100% of sudo context tests
- **SC-010**: Append-only attribute successfully applied in 100% of creation operations

## Spec Testing Requirements *(mandatory)*

### Spec Test Coverage

All functional requirements (FR-001 through FR-032) MUST have corresponding spec tests following the guidelines in `docs/spec_testing.md`. Spec tests validate functional requirements from user/system perspective and are distinct from unit tests.

### Spec Test Requirements

**File Naming**: `create_spec_test.go` (corresponding file under test + `spec_test.go`)

**Test Function Pattern**: `Test_S001_FR_XXX_Description()`
- FR_XXX corresponds to functional requirement being tested
- Description is camelCase description of validation

**Mandatory Coverage**: Every FR-XXX requirement must have at least one corresponding spec test function
- **No exceptions allowed**: Each requirement must have test coverage
- **Test-driven implementation**: Functional requirements are not considered implemented without passing spec tests
- **Immutable tests**: Once implemented, spec tests cannot be modified without explicit user permission

### Key Spec Test Categories

**Input Validation Tests** (FR-015, FR-016, FR-017, FR-020, FR-027):
- Parameter range validation with data-driven tests
- Path format and character validation
- Empty and invalid input handling

**Sudo Context Tests** (FR-002, FR-003, FR-004, FR-005):
- Sudo environment variable detection and validation
- Permission rejection scenarios
- Original user ownership verification

**File Creation Tests** (FR-006, FR-007, FR-018):
- Atomic file creation with O_CREAT|O_EXCL
- Permission and ownership setting
- Parent directory validation

**Header Writing Tests** (FR-008, FR-009):
- frozenDB v1 header format compliance
- Data flush integrity with fdatasync

**Append-Only Protection Tests** (FR-010, FR-011, FR-012):
- ioctl() attribute setting with FS_APPEND_FL
- Direct syscall usage (no subprocess calls)
- Attribute timing sequence

**Path Handling Tests** (FR-021, FR-022, FR-023, FR-024, FR-025, FR-026):
- Absolute vs relative path handling
- Hidden file creation support
- Filesystem path validation

**Atomic Operation Tests** (FR-019, FR-028, FR-029, FR-030, FR-031, FR-032):
- Thread safety for concurrent operations
- Race condition prevention
- Memory usage constraints
- Cleanup on failure scenarios

### Compliance Verification

**Definition of "Implemented"**: A functional requirement is only considered implemented when:
1. Implementation code exists and compiles
2. All corresponding spec tests pass
3. No existing spec tests are broken
4. Success criteria are met

**Review Checklist**:
- [ ] All FR-XXX requirements have corresponding tests
- [ ] All spec tests pass (or have documented t.Skip() with valid reasons)
- [ ] No previous spec tests are modified
- [ ] Test coverage matches requirement scope exactly
- [ ] Every FR-XXX has at least one test function (no missing coverage)

**Test Execution**:
```bash
# Run spec tests for frozendb package
go test ./frozendb/spec_tests/...

# Run all spec tests with coverage
go test -cover ./.../spec_tests/...
```
