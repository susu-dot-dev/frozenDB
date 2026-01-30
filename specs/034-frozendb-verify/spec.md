# Feature Specification: frozendb verify

**Feature Branch**: `034-frozendb-verify`  
**Created**: 2026-01-29  
**Status**: Draft  
**Input**: User description: "034 Implement frozendb verify. Verify will: ensure the header is correct. Ensure all the checksums (every 10k rows) matches. Ensures that every row after the last checksum row has the parity bit set. Ensure every row matches the v1 file format (by unmarshaling it and validating it). Ensuring that the last partial data row, if it exists, is valid. Do not check to see if row is valid compared to adjacent rows (e.g that a transaction is improperly nested)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Determine if File is Valid (Priority: P1)

A developer or system administrator needs to know whether a frozenDB file is valid or corrupted. They call the verify operation and receive a clear answer: either the file is valid, or there is corruption with details about what's wrong and where.

**Why this priority**: This is the fundamental question users need answered - "Is my database file OK?" Everything else builds on this.

**Independent Test**: Can be fully tested by creating a valid database file and verifying it succeeds, then creating corrupted files (bad header, bad checksum, bad parity, bad row format) and verifying each reports the appropriate error.

**Acceptance Scenarios**:

1. **Given** a valid frozenDB file, **When** verify is executed, **Then** the operation returns success
2. **Given** a frozenDB file with corrupted header, **When** verify is executed, **Then** the operation returns an error identifying the header problem
3. **Given** a frozenDB file with a checksum mismatch, **When** verify is executed, **Then** the operation returns an error identifying which checksum block failed
4. **Given** a frozenDB file with incorrect parity bytes on a row after the last checksum, **When** verify is executed, **Then** the operation returns an error identifying the row with bad parity
5. **Given** a frozenDB file with an invalid row format, **When** verify is executed, **Then** the operation returns an error identifying the malformed row

---

### User Story 2 - Validate Checksum Blocks (Priority: P2)

A user wants to ensure that all checksum blocks in the file are valid. The verify operation validates the initial checksum (covering the header) and every subsequent checksum block (covering the previous 10,000 rows).

**Why this priority**: Checksum validation provides strong assurance that large blocks of data haven't been corrupted. This is a specific, important subset of file validation.

**Independent Test**: Can be tested by creating files with multiple checksum blocks, verifying success, then corrupting each checksum block individually and verifying failure is detected for each.

**Acceptance Scenarios**:

1. **Given** a file with header and initial checksum, **When** verify is executed, **Then** the initial checksum covering the header is validated
2. **Given** a file with 25,000 rows (2 checksum blocks after initial), **When** verify is executed, **Then** all checksum blocks are validated
3. **Given** a file with corrupted second checksum block, **When** verify is executed, **Then** the error identifies the second checksum block as invalid

---

### User Story 3 - Validate Rows After Last Checksum (Priority: P3)

A user wants to ensure that rows after the last checksum block have valid parity bytes. Since these rows aren't covered by a checksum yet, parity provides per-row integrity checking.

**Why this priority**: This handles the "tail" of the file that hasn't accumulated enough rows for a checksum block yet. Less critical than P1/P2 but still important.

**Independent Test**: Can be tested by creating a file with rows beyond the last checksum, verifying parity validation occurs, then corrupting parity on one of those rows and verifying failure.

**Acceptance Scenarios**:

1. **Given** a file with 12,000 rows (10,000 in checksum block, 2,000 after), **When** verify is executed, **Then** parity is validated for the 2,000 rows after the last checksum
2. **Given** a file where row 11,500 has corrupted parity, **When** verify is executed, **Then** the error identifies row 11,500 as having invalid parity
3. **Given** a file with a valid partial data row as the last row, **When** verify is executed, **Then** the partial row is validated according to its state (State 1, 2, or 3)

---

### Edge Cases

- What happens when the file is empty (only header and initial checksum)?
- What happens when the file contains exactly 10,000 rows (boundary condition for checksums)?
- What happens when the last checksum row is corrupted?
- What happens when multiple types of corruption exist in a single file (reports first corruption found)?
- What happens when the file has a valid PartialDataRow in State 1, 2, or 3?
- What happens when the file has an invalid PartialDataRow?
- What happens when the row_size is at the minimum (128 bytes) or maximum (65536 bytes)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST validate that the header is exactly 64 bytes and ends with a newline at byte 63
- **FR-002**: System MUST validate that the header signature is exactly "fDB"
- **FR-003**: System MUST validate that the header version is exactly 1
- **FR-004**: System MUST validate that the header row_size is between 128 and 65536 inclusive
- **FR-005**: System MUST validate that the header skew_ms is between 0 and 86400000 inclusive
- **FR-006**: System MUST validate that the header JSON keys appear in the correct order: sig, ver, row_size, skew_ms
- **FR-007**: System MUST validate that bytes between the JSON end and byte 62 are null bytes (0x00)
- **FR-008**: System MUST validate that the initial checksum row at offset 64 exists and is valid
- **FR-009**: System MUST calculate and verify the CRC32 checksum for the header matches the initial checksum row
- **FR-010**: System MUST validate every checksum row that appears after every 10,000 complete data rows or null rows
- **FR-011**: System MUST calculate and verify the CRC32 checksum matches for each checksum block
- **FR-012**: System MUST validate the parity bytes for every row after the last checksum row
- **FR-013**: System MUST calculate the LRC parity for rows after the last checksum and verify they match the stored parity bytes
- **FR-014**: System MUST validate that every data row has a valid ROW_START byte (0x1F) at position 0
- **FR-015**: System MUST validate that every data row has a valid ROW_END byte (0x0A) at the last position
- **FR-016**: System MUST validate that every data row has a valid start_control character (uppercase alphanumeric)
- **FR-017**: System MUST validate that every data row has a valid two-character end_control sequence
- **FR-018**: System MUST validate that UUIDs in data rows are valid Base64-encoded 24-byte values
- **FR-019**: System MUST validate that UUIDs in data rows represent valid UUIDv7 values when decoded
- **FR-020**: System MUST validate that the JSON payload in data rows is valid UTF-8 encoded JSON
- **FR-021**: System MUST validate that data rows have correct padding (NULL_BYTE characters) in the expected positions
- **FR-022**: System MUST validate that checksum rows have the correct format with start_control 'C' and end_control 'CS'
- **FR-023**: System MUST validate that null rows have the correct format with start_control 'T' and end_control 'NR'
- **FR-024**: System MUST validate that null row UUIDs follow the specification (timestamp equals max_timestamp at insertion time, other fields zero)
- **FR-025**: System MUST detect if a PartialDataRow exists in the file
- **FR-026**: System MUST validate that any PartialDataRow only exists as the last row in the file
- **FR-027**: System MUST validate that PartialDataRow is in one of the three valid states (State 1, 2, or 3)
- **FR-028**: System MUST validate that PartialDataRow State 1 contains only ROW_START and start_control
- **FR-029**: System MUST validate that PartialDataRow State 2 contains ROW_START, start_control, uuid_base64, json_payload, and padding
- **FR-030**: System MUST validate that PartialDataRow State 3 contains all State 2 elements plus the 'S' character for savepoint intent
- **FR-031**: System MUST validate that PartialDataRow does not have any bytes beyond the state boundary
- **FR-032**: System MUST validate that all present fields in a PartialDataRow follow the same validation rules as complete DataRows
- **FR-033**: System MUST report the specific type of corruption detected (header, checksum, parity, row format, partial row)
- **FR-034**: System MUST report the location (byte offset or row number) where corruption is detected
- **FR-035**: System MUST return success when the entire file is valid according to all validation rules
- **FR-036**: System MUST return an error when any validation check fails
- **FR-037**: System MUST validate that DataRow UUIDs do not have all zeros in the non-timestamp part (bytes 7, 9-15)
- **FR-038**: System MUST stop verification and report error on the first corruption detected (fail-fast behavior)
- **FR-039**: System MUST NOT validate transaction nesting or transaction state relationships between rows
- **FR-040**: System MUST NOT validate UUID timestamp ordering between rows

### Key Entities *(include if feature involves data)*

- **Verification Result**: Represents the outcome of a verify operation - either success or failure with error details
- **Corruption Error**: Contains details about detected corruption including type (header, checksum, parity, row format, partial row), location (byte offset or row number), and description of what is invalid

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/verify_spec_test.go`
- Follow naming convention `Test_S_034_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Verify operation correctly detects 100% of single-point corruptions in test files (header, checksum, parity, row format)
- **SC-002**: Verify operation reports the specific type and location of corruption for all error scenarios
- **SC-003**: Verify operation handles all three PartialDataRow states correctly (validation succeeds for valid states, fails for invalid states)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-004**: Zero false positives (valid files incorrectly reported as corrupted) in test suite
- **SC-005**: Zero false negatives (corrupted files incorrectly reported as valid) in test suite containing known corruptions
- **SC-006**: All checksum validation follows IEEE CRC32 specification exactly
- **SC-007**: All parity validation follows LRC specification exactly
- **SC-008**: Verify operation does not modify the database file in any way

## Assumptions

- Verify operation will be exposed as a public API function that can be called programmatically
- Verify operation will accept a file path as input
- The verify operation will read the entire file sequentially from start to end
