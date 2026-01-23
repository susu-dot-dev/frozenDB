# Feature Specification: Transaction State Management

**Feature Branch**: `018-transaction-management`  
**Created**: 2025-01-23  
**Status**: Draft  
**Input**: User description: "018 When loading a FrozenDB file, the code should check to see if a transaction is in progress, and if so, create an initial Transaction inside FrozenDB to store the initial state. Then, implement FrozenDB.GetActiveTx() *Transaction which returns the current transaction, or nil if the transaction is finalized (committed/rolled back) or if there is no transaction. Next, implement FrozenDb.BeginTx() (*Transaction, err) which returns an error if there is already an active Tx, otherwise it creates a new one and calls Transaction.Begin()"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Transaction Recovery on Database Load (Priority: P1)

When a user opens a frozenDB database file that contains an incomplete transaction (where the last row indicates an ongoing transaction), the system must automatically detect and restore the transaction state so the user can continue working with it.

**Why this priority**: Critical for data integrity and recovery - prevents data loss from interrupted operations and ensures users can complete or roll back transactions after crashes or interruptions.

**Independent Test**: Can be fully tested by creating a database with an in-progress transaction, reopening it, and verifying the transaction state is correctly restored and accessible.

**Acceptance Scenarios**:

1. **Given** a frozenDB file ending with an open transaction (row ends with 'E'), **When** the database is opened, **Then** FrozenDB contains an active Transaction representing the in-progress state
2. **Given** a frozenDB file ending with a completed transaction (row ends with 'C' or rollback), **When** the database is opened, **Then** FrozenDB has no active transaction (GetActiveTx() returns nil)
3. **Given** a frozenDB file ending with a PartialDataRow, **When** the database is opened, **Then** FrozenDB contains an active Transaction representing the partial row state

---

### User Story 2 - Active Transaction Query Interface (Priority: P1)

When a user needs to check if a database currently has an active transaction, they should be able to query this information to determine whether they can start a new transaction or need to handle the existing one.

**Why this priority**: Essential for application workflow management - allows applications to make informed decisions about transaction handling and prevent unintended conflicts.

**Independent Test**: Can be fully tested by opening databases in various states and verifying GetActiveTx() returns the correct transaction reference or nil.

**Acceptance Scenarios**:

1. **Given** a database with an active transaction, **When** GetActiveTx() is called, **Then** it returns the active Transaction instance
2. **Given** a database with no active transaction, **When** GetActiveTx() is called, **Then** it returns nil
3. **Given** a database with a committed transaction, **When** GetActiveTx() is called, **Then** it returns nil (committed transactions are not active)

---

### User Story 3 - Controlled New Transaction Creation (Priority: P2)

When a user wants to start a new transaction, they must use the BeginTx() method which enforces business rules about concurrent transactions and provides proper error handling.

**Why this priority**: Important for maintaining database consistency and preventing user errors from conflicting transaction states.

**Independent Test**: Can be fully tested by attempting to create transactions in various database states and verifying proper success or error conditions.

**Acceptance Scenarios**:

1. **Given** a database with no active transaction, **When** BeginTx() is called, **Then** it creates and returns a new Transaction in active state
2. **Given** a database with an active transaction, **When** BeginTx() is called, **Then** it returns an error indicating a transaction is already active
3. **Given** a database opened with recovered active transaction, **When** BeginTx() is called, **Then** it returns an error until the existing transaction is committed or rolled back

---

### Edge Cases

- System MUST refuse to load frozenDB files with corrupted transaction state (invalid end control sequences) and return error to user
- System MUST refuse to load when PartialDataRow is in invalid state and return error to user
- System MUST refuse to load files with multiple apparent transaction endings (file corruption scenario) and return error to user
- System MUST handle files ending with checksum rows but no data rows as valid (no active transaction)
- System MUST refuse to load when transaction detection logic encounters rows with invalid structure and return error to user

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When loading a frozenDB file, the system MUST scan the last row first to determine if a transaction is currently in progress, then scan backwards up to 100 rows if needed to find transaction start
- **FR-002**: If an in-progress transaction is detected, the system MUST create and initialize a Transaction object representing the current state
- **FR-003**: FrozenDB.GetActiveTx() MUST return the current active Transaction or nil if no transaction is active
- **FR-004**: FrozenDB.GetActiveTx() MUST return nil for committed transactions (they are no longer active)
- **FR-005**: FrozenDB.GetActiveTx() MUST return nil for rolled back transactions (they are no longer active)
- **FR-006**: FrozenDB.BeginTx() MUST create and return a new Transaction when no active transaction exists
- **FR-007**: FrozenDB.BeginTx() MUST return an error when an active transaction already exists (checked via GetActiveTx() within mutex)
- **FR-008**: The system MUST detect transaction state by examining the end control character of the last data row
- **FR-009**: The system MUST handle PartialDataRow states correctly during transaction recovery
- **FR-010**: Transaction detection MUST work for all valid transaction endings (TC, SC, R0-R9, S0-S9, NR, RE, SE)

### Key Entities

- **FrozenDB**: Database connection with new fields for active transaction management
- **Transaction**: Existing transaction structure with enhanced state detection capabilities
- **Active Transaction State**: Determined by file's last row end control character
- **Transaction Detection Logic**: Algorithm to parse file ending and determine transaction status

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/frozendb_spec_test.go` where they test the FrozenDB methods
- Follow naming convention `Test_S_018_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Clarifications

### Session 2025-01-23

- Q: How should the system handle corrupted transaction state during file loading? → A: Strict rejection - refuse to load file with any transaction corruption, return error to user
- Q: What performance/size limits should transaction detection scanning respect during database loading? → A: Starting with last row will tell if there is an in-progress transaction or not. If there is, then a scan backwards for up to 100 rows may be necessary to find correct start of transaction
- Q: Should GetActiveTx() return a reference to the actual Transaction object or a copy/immutable view? → A: Return actual Transaction reference - allows direct interaction with recovered transaction (Transaction is already thread safe)
- Q: What should happen to existing active transaction when BeginTx() is called? → A: Return error without modifying existing transaction
- Q: How should BeginTx() distinguish between "no transaction" vs "transaction in progress" states when checking for conflicts? → A: Check GetActiveTx() for nil/non-nil, within a mutex

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Database loading correctly identifies transaction state in 100% of cases with valid files
- **SC-002**: GetActiveTx() returns accurate results within 5ms for all database states
- **SC-003**: BeginTx() properly prevents concurrent transactions in 100% of test scenarios
- **SC-004**: Transaction recovery works seamlessly without data loss in crash simulation tests

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero data loss scenarios in transaction recovery tests
- **SC-006**: All transaction state detection maintains data consistency across file formats
- **SC-007**: Memory usage for active transaction tracking remains constant regardless of database size
- **SC-008**: Transaction atomicity preserved in all recovery and state management tests
