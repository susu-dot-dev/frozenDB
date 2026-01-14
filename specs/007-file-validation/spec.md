# Feature Specification: File Creation and Validation with Header + Checksum Row

**Feature Branch**: `007-file-validation`  
**Created**: 2025-01-13  
**Status**: Draft  
**Input**: User description: "A fDB file must contain a header, followed by a checksum row in order for it to be a valid file. Modify the creation flow to write both the header and checksum row to disk. Then, modify the loading-from-disk code to validate both the header and checksum are valid. For validation, the code MUST take into consideration that the row_length could be corrupted until the checksum is verified. Even after verified, it could still be changed by a malicious user trying to induce a buffer overflow. Therefore, the parsing algorithm must take the proper precautions to unmarshal the checksum row and not read past the end of the file, or other ways the row_size can be altered"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Secure File Creation (Priority: P1)

As a frozenDB user, I need to create a new database file that includes both a header and an initial checksum row, so that the file is immediately valid and can be safely validated when opened.

**Why this priority**: This is the fundamental requirement for creating valid frozenDB files that meet the v1 format specification. Without proper file creation, no other database operations are possible.

**Independent Test**: Can be fully tested by creating a new file and verifying it contains both header and checksum row in the correct order with proper validation.

**Acceptance Scenarios**:

1. **Given** valid CreateConfig parameters, **When** Create() is called, **Then** the file contains a 64-byte header followed immediately by a checksum row
2. **Given** file creation succeeds, **When** the file is opened for reading, **Then** both header and checksum row pass validation
3. **Given** file creation completes, **When** checking file size, **Then** it equals exactly 64 bytes + row_size (for the checksum row)

---

### User Story 2 - Secure File Validation on Open (Priority: P1)

As a frozenDB user, I need to open an existing database file and have the system automatically validate both the header and the initial checksum row, protecting me from corrupted or maliciously modified files.

**Why this priority**: This is essential for data integrity and security. Users must be protected against reading corrupted or tampered files, and validation happens on every file open operation.

**Independent Test**: Can be fully tested by creating valid files, corrupted files, and maliciously modified files to ensure validation catches all issues.

**Acceptance Scenarios**:

1. **Given** a valid fDB file with header and checksum, **When** opening the file, **Then** validation succeeds and database opens normally
2. **Given** a file with corrupted header, **When** opening the file, **Then** validation fails with appropriate error
3. **Given** a file with corrupted checksum row, **When** opening the file, **Then** validation fails with appropriate error
4. **Given** a file where row_size was tampered, **When** opening the file, **Then** validation fails without buffer overflow
5. **Given** a file truncated before checksum row end, **When** opening the file, **Then** validation fails without reading past file end

---

### User Story 3 - Protection Against Malicious Row Size Manipulation (Priority: P2)

As a frozenDB user, I need protection against maliciously modified row_size values that could cause buffer overflows during validation, ensuring the parsing algorithm is secure against tampering attempts.

**Why this priority**: This addresses the critical security concern mentioned in the requirements where malicious users might attempt to induce buffer overflows by manipulating the row_size field.

**Independent Test**: Can be fully tested by creating files with maliciously large row_size values and ensuring validation fails safely without overflow.

**Acceptance Scenarios**:

1. **Given** a file with valid header but maliciously large row_size, **When** parsing the checksum row, **Then** validation fails without reading past file end
2. **Given** a file where row_size is smaller than actual checksum row, **When** parsing, **Then** validation detects size mismatch safely
3. **Given** a file with row_size that would cause integer overflow, **When** parsing, **Then** validation handles it safely without crash

---

### Edge Cases

- Files smaller than header + one checksum row MUST be rejected as corrupted
- Files with valid header but missing checksum row MUST be rejected as corrupted
- Checksum rows with correct format but mismatching CRC32 MUST be rejected as corrupted
- All partial/incomplete checksum rows MUST be rejected as corrupted

## Requirements *(mandatory)*

### Functional Requirements

- **FR-000**: System MUST NOT introduce any API changes. All existing public functions and interfaces must remain unchanged with identical signatures and behavior. All security and validation enhancements must be internal implementation details only.

- **FR-001**: System MUST calculate the initial checksum row completely before writing any data to disk, then perform a single atomic write of header + checksum row to prevent partial writes if NewChecksumRow fails. The CRC32 calculation must include all bytes since the previous checksum row, which for the first checksum row means bytes [0..63] (the entire header), using IEEE CRC32 algorithm (polynomial 0xedb88320)

- **FR-002**: System MUST validate header and initial checksum row when opening any fDB file, performing all row validations as defined in the v1_file_format.md specification for checksum rows (including parity validation, null byte padding, start_control/end_control validation, and CRC32 verification). Validation MUST fail fast on the first error encountered.

- **FR-003**: System MUST prevent buffer overflow and underflow attacks by validating that file size is sufficient to contain a complete checksum row at the specified row_size before attempting to read or parse the checksum row

- **FR-004**: System MUST validate row_size (128-65536 bytes per v1_file_format.md) against actual file size before trusting it for any parsing operations, ensuring no reads occur beyond actual file boundaries. Any integer overflow attempts in row_size MUST be rejected as file corruption.

- **FR-005**: System MUST verify that the CRC32 checksum in the checksum row correctly covers all bytes since the previous checksum row, which for the first checksum row means bytes [0..63] (the entire header) of the file being validated

- **FR-006**: System MUST ensure that checksum row is positioned at exactly offset row_size from the start of the file (offset 64 + 0 bytes from end of header)

- **FR-007**: System MUST validate that checksum row structure fully complies with v1_file_format.md specification, including but not limited to start_control='C', end_control='CS', proper Base64 encoding of CRC32, correct null byte padding, and valid parity bytes

### Key Entities *(include if feature involves data)*

- **Header**: 64-byte file header containing signature, version, row_size, and skew_ms
- **ChecksumRow**: Fixed-width row containing CRC32 checksum of covered data bytes
- **FileCorruptedError**: Single error type for all validation failures during file parsing

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_007_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

### Allowed Spec Test Modifications for 007 Implementation

**EXPLICITLY ALLOWED**: The following existing spec test files MAY be modified to accommodate 007 feature implementation, provided no API changes occur:

**002-open-frozendb spec tests** (`frozendb/frozendb_spec_test.go`):
- Tests that expect files with only headers may be updated to expect header + checksum row
- File size validation tests may be updated to account for checksum row size
- Header validation tests remain unchanged (header format unchanged)
- Tests for `createTestDatabase()` helper function may be updated to include checksum row creation

**001-create-db spec tests** (`frozendb/create_spec_test.go`):
- File creation size tests may be updated to expect header + checksum row size
- Header-only validation tests may be expanded to include checksum row validation
- Mock file creation helpers may be updated to include checksum row generation

**Rationale**: These modifications are necessary because 007 fundamentally changes the minimum valid file structure from "header only" to "header + checksum row". This affects:
1. File creation flow (must now write both header and checksum row)
2. File validation flow (must now validate both header and checksum row)  
3. Test helpers that create valid test files (must now create complete valid files)

**PROHIBITED**: No API changes, function signature modifications, or public interface alterations are permitted. Only internal implementation changes related to file validation and checksum row handling are allowed.

See `docs/spec_testing.md` for complete spec testing guidelines.

## Clarifications

### Session 2025-01-13

- Q: What is the maximum allowed row_size for security validation? → A: Per the v1_file_format.md spec
- Q: Should validation fail fast or continue checking all issues? → A: Fail fast on first error
- Q: How should integer overflow be handled in row_size? → A: Reject as corrupted file
- Q: What error level should parsing issues use? → A: Single error type
- Q: Should partial checksum rows be considered valid? → A: Reject all partial rows

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of newly created files pass header and checksum validation on first open
- **SC-002**: All buffer overflow protection tests pass under malicious input conditions
- **SC-003**: File creation time remains under 100ms for default configurations including checksum row
- **SC-004**: File opening validation completes within 50ms for typical file sizes

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero false positive validation failures (valid files always pass)
- **SC-006**: Zero false negative validation passes (invalid files always detected)
- **SC-007**: All checksum calculations match CRC32 IEEE standard exactly
- **SC-008**: No memory allocation increase during validation regardless of file size
