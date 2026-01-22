# Feature Specification: Transaction Checksum Row Insertion

**Feature Branch**: `016-transaction-checksum`
**Created**: 2026-01-22
**Status**: Draft
**Input**: User description: "Implement support for Transaction to write Checksum rows once the transaction insert's the 10,000 * Nth row"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Automatic Checksum Row Insertion (Priority: P1)

A database system automatically inserts checksum rows at regular intervals to ensure data integrity. The system tracks the count of complete data and null rows across all transactions since the last checksum. When this count reaches 10,000 (or any multiple thereof: 10,000, 20,000, 30,000...), the system writes a checksum row immediately after the specific row that reaches this threshold. Since transactions are limited to 100 rows each, reaching 10,000 rows typically requires multiple transactions, and the checksum row may be inserted at any point within a transaction.

**Why this priority**: This is the core functionality that enables periodic integrity checking. Without automatic checksum row insertion, the database cannot detect corruption in large blocks of data.

**Independent Test**: Can be fully tested by inserting exactly 10,000 data rows across multiple transactions and verifying that a checksum row appears after the 10,000th row but before the 10,001st row.

**Acceptance Scenarios**:

1. **Given** a database with 9,999 complete rows since the last checksum, **When** a transaction inserts the 10,000th row (which could be its first, middle, or last row), **Then** a checksum row must be written immediately after that row
2. **Given** a database with 9,995 complete rows since the last checksum, **When** a transaction with 10 rows is committed (rows 9,996-10,005), **Then** a checksum row must be written after the 5th row of that transaction (the 10,000th row overall), and the transaction must continue with the remaining 5 rows
3. **Given** a database with 9,999 complete rows since the last checksum and a 10-row transaction that will contain the 10,000th row, **When** the transaction writes rows 1-5 before the boundary, **Then** after row 5 (the 10,000th overall), a checksum row is written, and rows 6-10 of the transaction continue normally after it
4. **Given** a database with 10,000 complete rows since the last checksum, **When** the next transaction begins inserting the 10,001st row, **Then** a checksum row must already exist in the file before that row is written

---

### User Story 2 - Checksum Row Transparency to Transactions (Priority: P2)

When the database automatically inserts checksum rows during transaction writes, these checksum rows must be completely transparent to the transaction. The insertion of a checksum row must not affect committed rows, transaction boundaries, savepoints, or any other transaction state accounting. Transactions should behave identically whether checksum rows are present or not.

**Why this priority**: Checksum rows are an internal integrity mechanism and should not interfere with the logical transaction model. Transparency ensures that application code and database users can rely on consistent transaction semantics regardless of checksum placement.

**Independent Test**: Can be tested by creating transactions that cross checksum boundaries and verifying that all committed rows are correctly retrieved and that transaction boundaries, savepoints, and rollback behavior remain correct.

**Acceptance Scenarios**:

1. **Given** a transaction with 5 rows that spans across a checksum boundary, **When** the transaction is committed, **Then** all 5 rows must be visible in queries and the checksum row must not appear in results
2. **Given** a transaction with 3 rows followed by a checksum row, then another transaction with 4 rows, **When** reading all committed data, **Then** exactly 7 data rows must be returned with no gaps or checksum rows
3. **Given** a transaction with a savepoint created after 2 rows, **When** a checksum row is inserted, **Then** the savepoint count and transaction state must remain unchanged
4. **Given** a transaction that is rolled back to savepoint 1 after a checksum row was inserted, **Then** only rows up to and including savepoint 1 must be committed, with no interference from the checksum row

---

### Edge Cases

- How does the system handle multiple checksums in quick succession? The counter resets after each checksum, and the next checksum is written at 10,000 complete rows after that.
- What if the last row before file end is at row count 10,000? No final checksum is required after the last row if the file ends there (per spec section 6.3.3).
- How does the system handle zero rows since last checksum? The initial checksum after header covers row 0 (count = 0), and counting starts fresh from there.
- What happens when a checksum row is inserted between a row with start_control 'R' and the next row? The transaction must continue unaffected, with the next row still using start_control 'R' (not 'T') as the transaction is still open.
- What happens when a transaction's last row is the 10,000th row? The checksum is written after the transaction ends (after the commit row), and the transaction closes normally.
- What happens when the 10,000th row is a null row? The checksum row is written after it, counting null rows toward the 10,000 interval as specified.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST write checksum rows at row positions 10,000, 20,000, 30,000... where each position is calculated as 10,000 complete DataRows and NullRows after the previous checksum row (or header for the first checksum), excluding PartialDataRows from the count
- **FR-002**: System MUST write the checksum row immediately after the row that reaches the checksum threshold (10,000th, 20,000th, 30,000th, etc.), OR before writing the next row after that threshold
- **FR-003**: System MUST NOT count PartialDataRows toward checksum interval calculation
- **FR-004**: System MUST follow all v1_file_format.md requirements for checksum rows, including but not limited to: IEEE CRC32 calculation, start_control = 'C', end_control = 'CS', parity validation before calculation, and Base64 encoding of the 4-byte CRC32 value
- **FR-005**: System MUST ensure checksum rows are transparent to transactions—checksum row insertion MUST NOT affect committed rows, transaction boundaries, savepoints, or transaction state accounting
- **FR-006**: When a checksum row is inserted between rows of an open transaction, the next row MUST maintain the correct start_control ('R' for continuation, not 'T' for new transaction)
- **FR-007**: Checksum rows MUST NOT appear in query results or be counted as committed data

### Key Entities

- **DataRow**: Complete, fixed-width row containing a key-value pair with start_control 'T' or 'R'
- **NullRow**: Complete, fixed-width row with start_control 'T', uuid.Nil, and end_control 'NR'
- **PartialDataRow**: Incomplete row that exists only as the last row in the file, excluded from checksum counting
- **ChecksumRow**: Fixed-width integrity-checking row with start_control 'C' and end_control 'CS' written at 10,000-row intervals

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_016_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Checksum rows appear at exactly row positions 10,000, 20,000, 30,000... (counting only complete DataRows and NullRows)
- **SC-002**: System correctly excludes PartialDataRows from checksum interval counting in all scenarios
- **SC-003**: Checksum rows are written within 1 second of reaching the 10,000-row threshold
- **SC-004**: Memory usage for tracking row count remains constant (O(1) space) regardless of database size
- **SC-005**: Transactions spanning checksum boundaries return exactly the same committed rows as if no checksum rows existed (100% transparency)

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-006**: Zero checksum calculation errors across 100,000 row insertion test
- **SC-007**: All parity bytes are validated successfully before each checksum calculation
- **SC-008**: Checksum rows are correctly written at all multiples of 10,000 rows in large-scale tests (e.g., 1,000,000 rows)
- **SC-009**: CRC32 values in checksum rows match independent verification calculation in all test cases
- **SC-010**: Zero transaction state corruption or boundary errors when checksum rows are inserted during active transactions
