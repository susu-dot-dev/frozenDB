# Feature Specification: PartialDataRow Struct Implementation

**Feature Branch**: `009-partial-data-row`  
**Created**: 2026-01-14  
**Status**: Draft  
**Input**: User description: "009 Implement the PartialDataRow struct, with the ability to: Create a new PartialDataRow, Validate(), MarshalText() and UnmarshalText(). Additionally, create a user story to allow a PartialDataRow to transition from State1 to State2, and from State2 to State3. The functions should be AddRow(key, json), and Savepoint() respectively. These functions should not allow you to transition if the current state is not the right one to allow this transition. Make sure to re-validate the row after a state transition"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Manage PartialDataRow State Transitions (Priority: P1)

A user needs to create and manage a PartialDataRow through its lifecycle, starting from an empty state and progressively building it to completion while maintaining state integrity and validation at each step.

**Why this priority**: This is the core functionality that enables safe transaction building and recovery in frozenDB's append-only architecture.

**Independent Test**: Can be fully tested by creating a PartialDataRow, transitioning through each state, validating at each step, and ensuring proper error handling for invalid transitions.

**Acceptance Scenarios**:

1. **Given** a newly created PartialDataRow in State1, **When** AddRow(key, json) is called with valid UUIDv7 key and JSON data, **Then** the PartialDataRow transitions to State2 and validation passes
2. **Given** a PartialDataRow in State2, **When** Savepoint() is called, **Then** the PartialDataRow transitions to State3 with 'S' end_control character and validation passes
3. **Given** a PartialDataRow in State2, **When** AddRow(key, json) is called again, **Then** the operation fails with an error because State2 cannot transition back to add more data
4. **Given** a PartialDataRow in State3, **When** Savepoint() is called again, **Then** the operation fails with an error because State3 cannot add another savepoint
5. **Given** a PartialDataRow in State1, **When** Savepoint() is called, **Then** the operation fails with an error because State1 cannot create a savepoint without key-value data
6. **Given** a PartialDataRow in any state, **When** Validate() is called, **Then** it returns validation results appropriate to the current state

---

### User Story 2 - Serialization and Deserialization of PartialDataRows (Priority: P2)

A user needs to serialize PartialDataRows to text format for storage and deserialize them back while preserving state and ensuring data integrity.

**Why this priority**: Essential for persistent storage and recovery of in-progress transactions in frozenDB files.

**Independent Test**: Can be fully tested by creating PartialDataRows in each state, serializing them with MarshalText(), then deserializing with UnmarshalText() and verifying state preservation.

**Acceptance Scenarios**:

1. **Given** a PartialDataRow in State1, **When** MarshalText() is called, **Then** it returns the correct byte sequence for State1 format
2. **Given** a valid State1 byte sequence, **When** UnmarshalText() is called, **Then** it creates a PartialDataRow in State1
3. **Given** a PartialDataRow in State2, **When** MarshalText() is called, **Then** it returns the correct byte sequence including key and JSON data
4. **Given** a valid State2 byte sequence, **When** UnmarshalText() is called, **Then** it creates a PartialDataRow in State2 with correct key and data
5. **Given** a PartialDataRow in State3, **When** MarshalText() is called, **Then** it returns the correct byte sequence ending with 'S' character
6. **Given** a valid State3 byte sequence, **When** UnmarshalText() is called, **Then** it creates a PartialDataRow in State3 with savepoint intent
7. **Given** invalid byte sequences, **When** UnmarshalText() is called, **Then** it returns appropriate validation errors

---

### User Story 3 - Complete PartialDataRow Transactions (Priority: P1)

A user needs to complete a PartialDataRow by committing, rolling back to a savepoint, or ending the row with appropriate end control characters, converting the PartialDataRow to a complete DataRow with proper validation.

**Why this priority**: This is essential for transaction completion and data integrity, ensuring PartialDataRows can be properly finalized as valid DataRows.

**Independent Test**: Can be fully tested by creating PartialDataRows in stages 2 and 3, calling each completion method, validating the returned DataRow, and ensuring proper error handling for invalid states and parameters.

**Acceptance Scenarios**:

1. **Given** a PartialDataRow in State2, **When** Commit() is called, **Then** it returns a DataRow with end_control "TC" and validation passes
2. **Given** a PartialDataRow in State3, **When** Commit() is called, **Then** it returns a DataRow with end_control "SC" and validation passes
3. **Given** a PartialDataRow in State2, **When** Rollback(0) is called, **When** it returns a DataRow with end_control "R0" and validation passes
4. **Given** a PartialDataRow in State2, **When** Rollback(3) is called, **Then** it returns a DataRow with end_control "R3" and validation passes
5. **Given** a PartialDataRow in State3, **When** Rollback(0) is called, **Then** it returns a DataRow with end_control "S0" and validation passes
6. **Given** a PartialDataRow in State3, **When** Rollback(5) is called, **Then** it returns a DataRow with end_control "S5" and validation passes
7. **Given** a PartialDataRow in State2, **When** EndRow() is called, **Then** it returns a DataRow with end_control "RE" and validation passes
8. **Given** a PartialDataRow in State3, **When** EndRow() is called, **Then** it returns a DataRow with end_control "SE" and validation passes
9. **Given** a PartialDataRow in State1, **When** Commit() is called, **Then** it returns an InvalidActionError because State1 cannot be completed
10. **Given** a PartialDataRow in State2, **When** Rollback(12) is called, **Then** it returns an InvalidActionError because savepoint ID must be 0-9
11. **Given** any PartialDataRow, **When** a completion method returns a DataRow, **Then** Validate() is called on the DataRow before return and any validation error is wrapped appropriately
**Note**: The Rollback() function should only validate that the savepointId is between 0-9 (standard DataRow validation). Transaction-level validation of whether enough savepoints exist is the responsibility of a different layer to be implemented later.

---

### Edge Cases

- What happens when AddRow() is called with invalid UUIDv7 format or invalid JSON?
- How does system handle validation errors during state transitions?
- What happens when UnmarshalText() receives byte sequences longer than expected for the state?
- How are null bytes and unexpected characters handled in JSON payload parsing?
- What happens when Parity bytes are missing or invalid in deserialization?

## Clarifications

### Session 2026-01-14

- Q: What specific error types should be returned for invalid state transitions and validation failures? → A: Create a new InvalidAction error for invalid state transitions. For validation errors, use InvalidInputError when creating a new object, and wrap that in CorruptDatabaseError when UnmarshalText is called. Otherwise, do not create new error types, just have descriptive error messages
- Q: What are the maximum allowed size limits for JSON payloads in PartialDataRow? → A: Per v1_file_format.md requirements (the row_size limits all bytes including the JSON value payload)

- Q: How should PartialDataRow integrate with existing frozenDB transaction management? → A: This functionality is NOT to be integrated into the Transaction struct right now. That will be a future story
- Q: Should MarshalText() include padding bytes for State2 and State3? → A: Yes, use full DataRow padding calculation for States 2 and 3
- Q: What should UnmarshalText() do when encountering validation errors? → A: Always wrap the underlying validation error in CorruptDatabaseError
- Q: How should State3 MarshalText() handle bytes beyond the 'S' character? → A: Include calculated padding bytes up to the expected full row size

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST create a standalone PartialDataRow struct that supports three distinct states as defined in v1_file_format.md (not integrated with Transaction struct)
- **FR-002**: System MUST provide AddRow(key, json) function that transitions from State1 to State2 only when current state is State1
- **FR-003**: System MUST provide Savepoint() function that transitions from State2 to State3 only when current state is State2
- **FR-004**: System MUST prevent state transitions when current state does not permit the transition (State1→State3, State2→State1, State3→any)
- **FR-005**: System MUST re-validate the PartialDataRow after any state transition
- **FR-006**: System MUST provide Validate() function that checks row integrity according to current state requirements, including for State3: start_control validation, UUID validation, JSON payload validation, padding length validation, END_CONTROL first byte verification as 'S', and END_CONTROL second byte verification as null (0x00)
- **FR-007**: System MUST provide MarshalText() function that serializes the current state to the exact byte format specified in v1_file_format.md, including calculated padding bytes for State2 and State3 using the full DataRow calculation (row_size - len(json_payload) - 31)
- **FR-008**: System MUST provide UnmarshalText() function that deserializes byte sequences into appropriate PartialDataRow states
- **FR-009**: System MUST validate UUIDv7 format and Base64 encoding in AddRow() function
- **FR-010**: System MUST validate JSON payload format in AddRow() function
- **FR-011**: System MUST maintain state immutability - once a PartialDataRow transitions to a higher state, it cannot revert
- **FR-012**: System MUST generate InvalidAction error for invalid state transitions, InvalidInputError for validation errors during creation, and CorruptDatabaseError wrapping InvalidInputError for all UnmarshalText() validation failures
- **FR-013**: System MUST provide Commit() function that transitions from State2 to DataRow with end_control "TC" or from State3 to DataRow with end_control "SC"
- **FR-014**: System MUST provide Rollback(savepointId int) function that transitions from State2 to DataRow with end_control "R[0-9]" or from State3 to DataRow with end_control "S[0-9]"
- **FR-015**: System MUST provide EndRow() function that transitions from State2 to DataRow with end_control "RE" or from State3 to DataRow with end_control "SE"
- **FR-016**: System MUST validate savepointId parameter is between 0-9 for Rollback() function, returning InvalidActionError for invalid values
- **FR-017**: System MUST prevent completion functions (Commit, Rollback, EndRow) from being called on State1 PartialDataRows, returning InvalidActionError
- **FR-018**: System MUST return &DataRow{} and error from all completion functions, with the DataRow being valid only if error is nil
- **FR-019**: System MUST call Validate() on the generated DataRow before returning it from completion functions, propagating any validation errors appropriately

### Key Entities *(include if feature involves data)*

- **PartialDataRow**: An incomplete data row that exists in one of three progressive states during transaction building
  - **State1**: ROW_START and START_CONTROL bytes only (transaction begin)
  - **State2**: State1 + Base64 UUIDv7 key + JSON payload + calculated padding bytes (complete key-value data with padding)
  - **State3**: State2 + 'S' character + remaining padding (savepoint intent for transaction)
- **Completion Methods**: Functions that convert PartialDataRows to complete DataRows with appropriate end_control characters
  - **Commit()**: Sets end_control to "TC" (from State2) or "SC" (from State3) for transaction commit
  - **Rollback(savepointId int)**: Sets end_control to "R[0-9]" (from State2) or "S[0-9]" (from State3) for rollback to savepoint
  - **EndRow()**: Sets end_control to "RE" (from State2) or "SE" (from State3) for transaction continuation
- **State**: Current phase of PartialDataRow construction (1, 2, or 3)
- **UUIDv7 Key**: Time-ordered unique identifier in Base64 format (24 bytes)
- **JSON Payload**: User data in JSON format with UTF-8 encoding, limited by row_size constraint using full DataRow calculation (31 bytes + UUID + JSON payload + end_control + parity + ROW_END must fit within row_size)

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_009_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: PartialDataRow state transitions complete successfully 100% of the time when called from valid states
- **SC-002**: Invalid state transitions are rejected with appropriate error messages 100% of the time
- **SC-003**: MarshalText() and UnmarshalText() preserve state and data integrity for all three states
- **SC-004**: Validation checks catch all format violations according to v1_file_format.md specifications
- **SC-005**: PartialDataRow completion functions (Commit, Rollback, EndRow) generate correct end_control characters based on current state 100% of the time
- **SC-006**: Completion functions validate returned DataRows successfully and handle validation errors appropriately
- **SC-007**: Invalid completion attempts (wrong state, invalid savepoint ID) are rejected with InvalidActionError 100% of the time

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data corruption scenarios in state transition and serialization tests
