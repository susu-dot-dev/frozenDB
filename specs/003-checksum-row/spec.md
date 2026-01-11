# Feature Specification: Checksum Row Implementation

**Feature Branch**: `003-checksum-row`  
**Created**: 2026-01-10  
**Status**: Draft  
**Input**: User description: "Spec out the 003 spec for the underlying ability to create a checksum row, as per the requirements of the v1_file_format.md. Don't worry about e.g. writing it to the file, just the struct and the ability to serialize it. This will include the ability to generate the checksum, defining the data types for things like the control bits, sentinel bits and more."

## Clarifications

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Create Checksum Row Structure (Priority: P1)

As a frozenDB developer, I need a well-defined structure for checksum rows that can be serialized to the exact byte format specified in v1_file_format.md, so that I can generate valid integrity checkpoints for database blocks.

**Why this priority**: Checksum rows are fundamental to frozenDB's data integrity strategy and must be implemented before any database writing functionality.

**Independent Test**: Can be fully tested by creating a ChecksumRow struct, serializing it to bytes, and validating that the output matches the exact byte layout specification from v1_file_format.md section 6.

**Acceptance Scenarios**:

1. **Given** a new checksum row with CRC32 value, **When** serialized to bytes, **Then** the output follows the exact layout: ROW_START, start_control='C', crc32_base64, padding, end_control='CS', parity_bytes, ROW_END
2. **Given** a serialized checksum row, **When** deserialized, **Then** it returns the original CRC32 value and valid control bytes

---



### User Story 2 - Validate Row Parity (Priority: P2)

As a frozenDB developer, I need to calculate and validate LRC parity bytes for checksum rows, so that individual row integrity can be verified independently of block checksums.

**Why this priority**: Parity bytes provide per-row integrity checking and are required by the file format specification.

**Independent Test**: Can be fully tested by calculating parity for known byte sequences and validating the XOR-based algorithm produces correct results.

**Acceptance Scenarios**:

1. **Given** a complete checksum row byte sequence, **When** calculating parity, **Then** the XOR of all bytes except the last 4 matches the parity bytes
2. **Given** corrupted checksum row bytes, **When** validating parity, **Then** validation fails and corruption is detected

---

### Edge Cases

- **CRC32 Base64 padding**: System handles standard Base64 padding characters "==" for 4-byte CRC32 values as per RFC 4648
- **Minimum row size**: Parity byte calculation works correctly for 128-byte minimum row size, XOR of bytes [0] through [124]
- **Invalid control characters**: CRC32 calculation returns error when input data is empty or invalid; Validate method checks control byte sequences
- **Empty input data**: NewChecksumRow functions return explicit error for empty byte arrays or nil io.Reader
- **Header dependency**: Write method requires valid header with row_size field to calculate proper NULL_BYTE padding
- **Deferred validation**: Creation performs minimal validation; full validation (control bytes, parity, sentinels) handled by separate Validate method

## Requirements *(mandatory)*

<!--
  ACTION REQUIRED: The content in this section represents placeholders.
  Fill them out with the right functional requirements.
-->

### Functional Requirements
- **FR-001**: System MUST implement a ChecksumRow struct with fields matching v1_file_format.md section 6.1 specification
- **FR-002**: System MUST provide serialization method that outputs exact byte layout: ROW_START, start_control='C', crc32_base64 (8 bytes), NULL_BYTE padding, end_control='CS', parity_bytes (2 bytes), ROW_END that includes IEEE CRC32 calculation using polynomial 0xedb88320
- **FR-003**: System MUST encode 4-byte CRC32 values as 8-character Base64 strings with standard padding
- **FR-004**: System MUST calculate LRC parity bytes using XOR algorithm on bytes [0] through [row_size-4]
- **FR-005**: System MUST validate all data rows' parity before calculating block checksums as required by v1_file_format.md section 7.4
- **FR-006**: System MUST handle sentinel bytes correctly: ROW_START (0x1F) and ROW_END (0x0A)
- **FR-007**: System MUST support row sizes from 128 to 65536 bytes as specified in header format
- **FR-008**: System MUST provide deserialization method that can parse checksum rows from byte arrays
- **FR-009**: System MUST validate control byte sequences: start_control='C' and end_control='CS' for checksum rows
- **FR-010**: System MUST validate EVERY bit of the string when deserializing a checksum row, including sentinel bits, parity correctness, padding, and control characters

### Key Entities *(include if feature involves data)*

- **ChecksumRow**: Represents a checksum row with CRC32 value, control bytes, and parity information for data integrity verification
- **CRC32Calculator**: Handles IEEE CRC32 calculation for blocks of data rows according to frozenDB specification, returns error for empty/invalid input
- **ParityValidator**: Calculates and validates LRC parity bytes for individual row integrity checking during separate validation phase

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

<!--
  ACTION REQUIRED: Define measurable success criteria.
  These must be technology-agnostic and measurable.
  For frozenDB, include data integrity and correctness metrics.
-->

### Measurable Outcomes

- **SC-001**: Checksum row serialization produces byte-exact output matching v1_file_format.md specification in 100% of test cases
- **SC-002**: CRC32 calculations match IEEE reference implementation for all test data blocks
- **SC-003**: Parity validation detects 100% of single-bit corruption scenarios in testing
- **SC-004**: CRC32 calculations detect two-bit corruption of the header

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero false negatives in corruption detection across all checksum validation tests
- **SC-006**: All parity byte calculations match XOR specification exactly for all row sizes (128-65536 bytes)
- **SC-007**: Base64 encoding of CRC32 values produces RFC 4648 compliant output with proper padding
- **SC-008**: Checksum row structure maintains immutability - once serialized, content cannot be modified without detection
