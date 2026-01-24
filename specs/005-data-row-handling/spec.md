# Feature Specification: DataRow Handling

**Feature Branch**: `005-data-row-handling`  
**Created**: 2026-01-11  
**Status**: Draft  
**Input**: User description: "005 - Implement DataRow handling. Building off the foundation for a ChecksumRow, using similar paradigms, implement the functionality to create & validate a DataRow as well as serialize it to a byte[] representation"

## Clarifications

### Session 2026-01-11
- Q: Should DataRow validate JSON string format or accept any string content? → A: Expect JSON strings but don't validate syntax - caller responsible for validation
- Q: Should specific performance timing targets be removed? → A: Remove specific timing targets (1ms, 0.5ms) - defer to planning

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Store Key-Value Data (Priority: P1)

As a frozenDB user, I need to create data rows that store key-value pairs with UUIDv7 keys and JSON string values so that I can persist structured data in the database with proper time ordering.

**Why this priority**: Core functionality for any key-value database - without data rows, the database cannot store user data, making it fundamentally unusable.

**Independent Test**: Can be fully tested by creating a DataRow with a valid UUIDv7 key and JSON string value, serializing it to bytes, and verifying the bytes match the expected v1_file_format.md structure.

**Acceptance Scenarios**:

1. **Given** a valid header and UUIDv7 key, **When** creating a DataRow with JSON string value, **Then** the row should be created successfully with proper structure
2. **Given** a created DataRow, **When** serializing to bytes, **Then** output should match the exact format specification with correct padding and parity
3. **Given** serialized DataRow bytes, **When** unmarshaling back to DataRow, **Then** original UUIDv7 key and JSON string value should be preserved exactly
4. **Given** unmarshaled DataRow with JSON string value, **When** caller needs structured data, **Then** they can deserialize JSON string into their chosen data structure

---

### User Story 2 - Validate Data Row Integrity (Priority: P1)

As a frozenDB user, I need to validate that data rows conform to the file format specification so that I can detect corruption and ensure data integrity.

**Why this priority**: Data integrity is fundamental - without proper validation, corrupted data could silently propagate through the system, violating the database's correctness guarantees.

**Independent Test**: Can be fully tested by attempting to create DataRows with various invalid inputs (wrong UUID version, malformed JSON, invalid controls) and verifying appropriate errors are returned.

**Acceptance Scenarios**:

1. **Given** a UUID that is not UUIDv7 version, **When** creating a DataRow, **Then** validation should fail with appropriate error
2. **Given** a DataRow with invalid start/end control characters, **When** validating, **Then** validation should fail with specific error messages
3. **Given** serialized bytes with corrupted parity, **When** unmarshaling, **Then** validation should detect the integrity failure

---

---

### Edge Cases

- How does system handle maximum row size constraints?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow DataRow creation through manual struct initialization with Header, UUIDv7 key, and JSON string value (no syntax validation at this layer); validation performed through Validate() method
- **FR-002**: System MUST validate that input UUID is UUIDv7 version; Base64 encoding is handled by MarshalText/UnmarshalText implementation
- **FR-003**: System MUST serialize DataRow to exact byte format per v1_file_format.md specification
- **FR-004**: System MUST deserialize DataRow from byte array with complete validation
- **FR-005**: System MUST handle proper start control characters (basic validation for single rows)
- **FR-006**: System MUST handle basic end control character validation for single rows
- **FR-007**: System MUST calculate and validate LRC parity bytes for row integrity
- **FR-008**: System MUST pad JSON string value with NULL_BYTE to fill remaining row space

- **FR-009**: System MUST ensure overall row length matches Header's row_size exactly
- **FR-010**: System MUST validate that start and end control characters are valid single-byte characters for single row context
- **FR-011**: System MUST reject DataRows with nil payload, nil header, or empty/zero UUID
- **FR-012**: System MUST validate all child structs during construction before parent validation

### Key Entities *(include if feature involves data)*

- **DataRow**: Represents a single key-value data row with UUIDv7 key and JSON string value
- **UUIDv7Key**: UUID type input that must be UUIDv7 version; serialization to Base64 handled by MarshalText
- **JSONStringValue**: JSON string value provided by caller with NULL_BYTE padding to fill row space (no syntax validation at this layer)
- **ControlCharacters**: Basic start and end control bytes for single row structure validation

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_005_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create and serialize DataRows with valid UUIDv7 keys and JSON string values efficiently
- **SC-002**: System validates DataRows against all format requirements with 100% accuracy
- **SC-003**: Round-trip serialization (marshal/unmarshal) preserves data with zero data loss
- **SC-004**: Invalid input detection occurs reliably with appropriate error reporting

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption scenarios in all DataRow integrity validation tests
- **SC-006**: All control character sequences maintain proper single-row validation
- **SC-007**: UUIDv7 version validation works correctly, accepting any valid UUIDv7 type input; Base64 encoding handled internally
- **SC-008**: Parity byte calculation and validation catches all single-byte errors in row data
- **SC-008**: JSON string value padding and unpadding preserves data content exactly
- **SC-009**: Memory usage for DataRow operations remains constant regardless of JSON string value size
