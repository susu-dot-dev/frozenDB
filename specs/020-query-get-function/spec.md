# Feature Specification: Query Get Function

**Feature Branch**: `020-query-get-function`  
**Created**: 2025-01-24  
**Status**: Draft  
**Input**: User description: "Users can query frozenDB with a UUID, and receive the unmarshaled JSON value for the row, if the key exists and that row was committed. Users receive a KeyNotFound error if the key either does not exist, or exists but was not one of the rows committed during a transaction. A new InvalidData error (wrapping a JSON unmarshal error) will be returned if the value previously stored cannot be unmarshaled into the given destination value. Next, when rows are added and committed or rolled back, these rows will be visible to the Get function"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Retrieve Committed Data (Priority: P1)

Users need to retrieve JSON values from frozenDB using UUID keys, receiving properly unmarshaled data for any keys that are part of committed transactions.

**Why this priority**: This is the core functionality that enables frozenDB to serve as a key-value store, essential for any application using the database.

**Independent Test**: Can be fully tested by writing data to a database file, committing it, then retrieving it - demonstrating the complete read/write cycle.

**Acceptance Scenarios**:

1. **Given** a database with a fully committed transaction containing UUID key "committed-123" with JSON value `{"name":"test"}`, **When** the user calls Get with key "committed-123" and a destination struct, **Then** the struct contains the name "test"

2. **Given** a database with a transaction that was partially rolled back to savepoint 1, where UUID key "partial-commit" appears at or before savepoint 1, **When** the user calls Get with key "partial-commit", **Then** the corresponding JSON value is returned and properly unmarshaled

3. **Given** a database with complex nested JSON `{"user":{"id":123,"profile":{"age":25,"active":true}}}` in a committed transaction, **When** the user calls Get with the corresponding UUID key and a nested destination struct, **Then** all nested fields are correctly populated in the destination structure

4. **Given** a database with stored JSON `{"name":"test"}` for a key, **When** the user calls Get with that key but provides a destination struct expecting an integer field "name", **Then** an InvalidData error wrapping the JSON unmarshal error is returned

5. **Given** a database with stored malformed JSON for a key, **When** the user calls Get with that key and any destination, **Then** an InvalidData error wrapping the JSON unmarshal error is returned

---

### User Story 2 - Handle Uncommitted Data (Priority: P1)

Users need Get operations to only return values from rows that are part of committed transactions, and return KeyNotFound errors for any uncommitted data.

**Why this priority**: This ensures users only see committed, durable data and prevents access to incomplete or rolled-back data.

**Independent Test**: Can be fully tested by creating various transaction states and verifying Get only returns committed data.

**Acceptance Scenarios**:

1. **Given** a database with no rows matching UUID key "missing-456", **When** the user calls Get with key "missing-456", **Then** a KeyNotFound error is returned

2. **Given** a database with a transaction that was fully rolled back (rollback to savepoint 0) containing UUID key "rolled-back-789", **When** the user calls Get with key "rolled-back-789", **Then** a KeyNotFound error is returned

3. **Given** a database with a transaction that was partially rolled back to savepoint 1, where UUID key "partial-rollback" appears after savepoint 1, **When** the user calls Get with key "partial-rollback", **Then** a KeyNotFound error is returned

4. **Given** a database with UUID key "uncommitted-key" in the current active transaction that has not been committed, **When** the user calls Get with key "uncommitted-key", **Then** a KeyNotFound error is returned

---

### User Story 3 - Immediate Transaction Visibility (Priority: P2)

When I commit a transaction, I can immediately Get() any value from that transaction, and when I perform a partial rollback, the data visibility updates immediately to reflect the rollback state.

**Why this priority**: Provides users with a consistent mental model about when data becomes visible and helps them understand the consistency guarantees of the system.

**Independent Test**: Can be fully tested by performing transaction operations and immediately calling Get to verify visibility changes.

**Acceptance Scenarios**:

1. **Given** an empty database, **When** the user adds a key-value pair and commits the transaction, **Then** the next Get call with that key immediately returns the committed value

2. **Given** a database with a key-value pair in an ongoing transaction, **When** the user rolls back the transaction, **Then** the next Get call with that key immediately returns a KeyNotFound error

3. **Given** a database with an existing committed key, **When** the user adds the same key in a new transaction and commits, **Then** the next Get call with that key immediately returns the new value

4. **Given** a database with a transaction containing multiple rows where a savepoint was created after the second row, **When** the user performs a partial rollback to that savepoint, **Then** the next Get calls immediately return the first two rows and return KeyNotFound for the third row

---

### Edge Cases

- What happens when the database file contains corrupted data rows?
- How does Get handle concurrent read/write operations?
- What happens when the destination value is nil?
- How does Get behave with UUID timestamp ordering violations?
- What happens when the database file contains only checksum rows and no data rows?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST unmarshal and populate the user's destination struct with JSON data for UUID keys that exist in fully committed transactions (transactions ending with TC or SC)
- **FR-002**: System MUST unmarshal and populate the user's destination struct with JSON data for UUID keys that appear at or before the savepoint in partially rolled back transactions
- **FR-003**: System MUST return KeyNotFound error when key does not exist anywhere in the database file
- **FR-004**: System MUST return KeyNotFound error when key exists only in fully rolled back transactions (rollback to savepoint 0)
- **FR-005**: System MUST return KeyNotFound error when key exists only after the savepoint in partially rolled back transactions
- **FR-006**: System MUST return KeyNotFound error when key exists only in the current active (uncommitted) transaction
- **FR-007**: System MUST return InvalidData error wrapping JSON unmarshal errors when stored JSON cannot be unmarshaled into the provided destination struct
- **FR-008**: System MUST return InvalidData error when stored JSON is malformed and cannot be parsed
- **FR-009**: System MUST make committed, or partially rolled back transaction data immediately visible in the next Get call after commit/rollback

### Key Entities *(include if feature involves data)*

- **UUID Key**: Unique identifier for database entries using UUIDv7 format with timestamp ordering
- **JSON Value**: Arbitrary JSON data stored as UTF-8 text that can be unmarshaled into user-provided structs
- **Committed Transaction**: Transaction where all rows from start through commit are valid for queries
- **Rolled Back Transaction**: Transaction where rows are invalidated based on rollback scope
- **PartialDataRow**: Incomplete row that is not visible to Get operations
- **KeyNotFound Error**: Structured error indicating no valid committed row exists for the key
- **InvalidData Error**: Structured error wrapping JSON unmarshal failures

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-001**: Get operations never return data from rolled-back transactions
- **SC-002**: Get operations never return data from PartialDataRows
- **SC-003**: All returned JSON values exactly match the originally stored committed data
- **SC-004**: Key uniqueness is maintained - Get returns only one value per UUID key (the latest committed version)
- **SC-005**: Concurrent Get operations during write transactions maintain data consistency, and provide guarantees about visibility by the time Commit() or Rollback() occurs
