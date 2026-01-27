# Feature Specification: NullRow Struct Implementation

**Feature Branch**: `010-null-row-struct`  
**Created**: 2025-01-18  
**Status**: Draft  
**Input**: User description: "Add a NullRow struct. It should have a Validate() method, as well as the ability to Marshal and Unmarshal the null row into a format matching the requirements of the file format"

## Clarifications

### Session 2025-01-18

- Q: What error handling patterns should be used for NullRow validation and unmarshaling? → A: Follow existing struct validation patterns with InvalidInputError for validation failures and CorruptDatabaseError wrapping for unmarshal failures

## User Scenarios & Testing *(mandatory)*

### User Story 1 - NullRow Creation and Validation (Priority: P1)

As a frozenDB developer, I need to create NullRow instances that represent null operations in the database file so that I can handle empty transactions and maintain proper file structure integrity.

**Why this priority**: NullRow is a fundamental building block for transaction handling and file format compliance. Without it, the database cannot properly represent empty transactions or maintain the required transaction boundaries.

**Independent Test**: Can be fully tested by creating NullRow instances and validating they conform to file format specifications without requiring integration with other database components.

**Acceptance Scenarios**:

1. **Given** a new NullRow is created, **When** Validate() is called, **Then** the validation passes with all required fields correctly set
2. **Given** a NullRow with invalid start_control, **When** Validate() is called, **Then** validation fails with appropriate error
3. **Given** a NullRow with invalid UUID, **When** Validate() is called, **Then** validation fails with appropriate error
4. **Given** a NullRow with invalid end_control, **When** Validate() is called, **Then** validation fails with appropriate error

---

### User Story 2 - NullRow Serialization and Deserialization (Priority: P1)

As a frozenDB developer, I need to marshal and unmarshal NullRow instances to/from the binary file format so that I can read and write null operations to the database file.

**Why this priority**: Serialization is essential for persisting NullRows to disk and reading them back. Without proper marshal/unmarshal functionality, the database cannot maintain file integrity or properly process existing files containing NullRows.

**Independent Test**: Can be fully tested by performing round-trip serialization (marshal then unmarshal) and verifying the resulting structure matches the original, without requiring database file operations.

**Acceptance Scenarios**:

1. **Given** a valid NullRow, **When** Marshal() is called, **Then** it produces binary data matching the exact file format specification
2. **Given** binary data representing a valid NullRow, **When** Unmarshal() is called, **Then** it creates a NullRow with correct field values
3. **Given** marshaled NullRow data, **When** immediately unmarshaled, **Then** the resulting NullRow is identical to the original
4. **Given** invalid binary data, **When** Unmarshal() is called, **Then** it returns an appropriate error

---



### Edge Cases

- What happens when marshaling fails due to invalid row size?
- How does system handle NullRow with corrupted parity bytes?
- What occurs when unmarshaling data with incorrect padding?
- How are NullRows handled when they appear in invalid positions within the file?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: NullRow struct MUST have start_control field always set to 'T' (transaction begin)
- **FR-002**: NullRow struct MUST have end_control field always set to 'NR' (null row)
- **FR-003**: NullRow struct MUST use a UUIDv7 with timestamp component equal to the current `max_timestamp` in the database, with all other fields (random components) set to zero. For an empty database, the timestamp is 0.
- **FR-004**: NullRow struct MUST have a Validate() method that verifies all required field values
- **FR-005**: NullRow struct MUST have a Marshal() method that produces binary data matching v1 file format
- **FR-006**: NullRow struct MUST have an Unmarshal() method that can parse binary data into a NullRow instance
- **FR-007**: NullRow struct MUST calculate correct parity bytes for marshaled data
- **FR-008**: NullRow struct MUST handle padding correctly to match fixed row width
- **FR-009**: NullRow validation MUST fail if start_control is not 'T'
- **FR-010**: NullRow validation MUST fail if end_control is not 'NR'
- **FR-011**: NullRow validation MUST fail if UUID timestamp does not equal the current `max_timestamp` or if any non-timestamp fields are not zero
- **FR-012**: Marshal() method MUST return InvalidInputError if row structure is invalid
- **FR-013**: Unmarshal() method MUST return CorruptDatabaseError wrapping validation errors if input data format is invalid


### Key Entities *(include if feature involves data)*

- **NullRow**: A struct representing a null operation row in the frozenDB file format
  - Contains start_control (always 'T'), end_control (always 'NR'), UUID (timestamp equals max_timestamp, other fields zero), parity bytes
  - Provides validation, marshaling, and unmarshaling capabilities
- **ValidationError**: Uses InvalidInputError for validation failures (following existing struct validation patterns)
- **MarshalError**: Uses InvalidInputError for marshaling failures (following existing struct validation patterns)
- **UnmarshalError**: Uses CorruptDatabaseError wrapping validation errors for unmarshal failures (following existing unmarshal patterns)

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/null_row_spec_test.go` where `null_row` matches the implementation file being tested
- Follow naming convention `Test_S_010_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: NullRow creation and validation operations complete in under 1 millisecond
- **SC-002**: Marshal and unmarshal round-trip operations achieve 100% accuracy for valid inputs
- **SC-003**: All error conditions are properly detected and reported with descriptive messages


### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption scenarios in all marshal/unmarshal operations
- **SC-006**: All NullRow parity calculations are mathematically correct per LRC specification
- **SC-007**: NullRow binary format matches v1 file format specification exactly
