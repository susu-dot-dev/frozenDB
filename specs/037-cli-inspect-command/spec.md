# Feature Specification: CLI Inspect Command

**Feature Branch**: `037-cli-inspect-command`  
**Created**: 2026-01-30  
**Status**: Draft  
**Input**: User description: "037 Add the inspect command to the CLI. The inspect command should take in the --path, an optional --offset (default to 0) and an optional limit (default to -1), and an optional --print-header (defaults to false) The inspect command should print out the database in a format that is human readable, and in a table format easy to parse with tools like awk. If print_header is set to true, first print a table with the following columns, in order: Row Size, Clock Skew, File Version. Next, print a second table with the following columns: index, type, key, value, savepoint, tx start, tx end, rollback, parity. For a NullRow, the type should be NullRow, key should be the key of the row. Value should be blank, savepoint should be false, Tx start should be true, tx end should be true, and rollback should be false. For a data row, type should be Data, key and value should match the row. tx start should be true if the start_control is T. Tx end should be true if the end control is TC or SC. Savepoint should be true if the end control starts with S. Rollback should be true if the end control is R[0-9] or S[0-9]. For a checksum row, the type should be Checksum, the value should be the checksum string, and every other column should be blank. All row types should fill in the parity column. For the last row, if it is not finished, the type should be 'partial', and then the fields which are set should be the same as a data row. The unset fields should be blank. As for as the offset is concerned, if the offset is negative, it's an error. Otherwise, offset 0 = the first checksum row. Offset 1 = the first row after the initial checksum and so on. The offset includes every row type, including checksum, data, and null rows. The index column for the output matches the same logic as used by offset. If the offset is greater than the number of rows in the database, this is not an error, just the table will not display any rows. For the limit, any negative number means 'print all rows', else it restricts how many rows to display. If a row fails to parse properly, put the row type as 'error', leave all of the values blank. Then, continue processing the remaining rows, but make sure the exit code is 1."

## Clarifications

### Session 2026-01-30

- Q: The spec mentions "table format easy to parse with tools like awk" and "tab-separated table format," but doesn't specify the exact table rendering approach. What output format should be used? → A: Tab-separated values (TSV) with column headers
- Q: When inspecting large database files (potentially millions of rows), the command needs a strategy for memory usage. What approach should be used? → A: Read one row at a time, then print it, then read the next row etc.
- Q: The spec describes blank values for certain fields (e.g., "value should be blank" for NullRow). In TSV format, how should blank values be represented? → A: Empty string (no characters between tabs)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Database File Inspection and Debugging (Priority: P1)

Developers and operators need to inspect the raw contents of a frozenDB database file to debug issues, verify data integrity, or understand the database structure. The inspect command provides a human-readable view of all rows in the database, including data rows, checksum rows, and null rows, formatted in a way that can be easily parsed by tools like awk, grep, and sed.

**Why this priority**: This is the core functionality that enables all other use cases. Without the ability to inspect the database contents, developers cannot debug, verify, or understand the database structure.

**Independent Test**: Can be fully tested by creating a database with various row types (data, checksum, null, partial) and running the inspect command to verify the output format matches the specification.

**Acceptance Scenarios**:

1. **Given** a frozenDB database file with 5 data rows, **When** a user runs `frozendb inspect --path db.fdb`, **Then** the command displays a table with all 5 data rows showing their index, type, key, value, and transaction control flags
2. **Given** a database with mixed row types (data, checksum, null), **When** a user runs the inspect command, **Then** all row types are displayed with correct formatting and type identification
3. **Given** a database with a partial data row at the end, **When** a user runs the inspect command, **Then** the partial row is displayed with type "partial" and only the available fields populated

---

### User Story 2 - Selective Row Display with Offset and Limit (Priority: P2)

Users working with large databases need to inspect specific portions of the file without loading the entire database into view. The offset and limit parameters allow users to view specific row ranges, enabling efficient inspection of large files.

**Why this priority**: This enhances the core inspection functionality for large databases but isn't required for basic inspection tasks.

**Independent Test**: Can be tested independently by creating a database with 100+ rows and verifying that offset and limit parameters correctly control which rows are displayed.

**Acceptance Scenarios**:

1. **Given** a database with 100 rows, **When** a user runs `frozendb inspect --path db.fdb --offset 10 --limit 20`, **Then** the command displays rows 10-29 (20 rows starting from index 10)
2. **Given** a database with 50 rows, **When** a user runs `frozendb inspect --path db.fdb --offset 100`, **Then** the command succeeds but displays no rows (offset beyond database size is not an error)
3. **Given** a database with any number of rows, **When** a user runs `frozendb inspect --path db.fdb --limit -1`, **Then** all rows from offset 0 to the end are displayed

---

### User Story 3 - Database Header Information Display (Priority: P3)

Users need to quickly verify database configuration parameters (row size, clock skew, file version) before inspecting the row data. The --print-header flag enables users to display database metadata in a separate table.

**Why this priority**: This is a convenience feature that provides useful metadata but isn't essential for inspecting row data.

**Independent Test**: Can be tested independently by creating databases with different configurations and verifying the header table displays correct values.

**Acceptance Scenarios**:

1. **Given** a database with row_size=4096, skew_ms=5000, version=1, **When** a user runs `frozendb inspect --path db.fdb --print-header true`, **Then** the command first displays a header table with these values, then the row data table
2. **Given** any database, **When** a user runs `frozendb inspect --path db.fdb` (without --print-header), **Then** the command displays only the row data table without header information
3. **Given** a database, **When** a user runs `frozendb inspect --path db.fdb --print-header false`, **Then** the command displays only the row data table without header information

---

### User Story 4 - Robust Error Handling for Corrupted Data (Priority: P2)

When inspecting databases with corrupted or malformed rows, users need the tool to continue processing remaining rows while clearly marking problematic rows. The command should identify corrupted rows with type "error" and set exit code 1, but continue inspecting the rest of the file.

**Why this priority**: This is critical for debugging corrupted databases, but not required for inspecting valid databases.

**Independent Test**: Can be tested independently by creating databases with intentionally corrupted rows and verifying error handling behavior.

**Acceptance Scenarios**:

1. **Given** a database with a corrupted data row at index 5, **When** a user runs the inspect command, **Then** row 5 is displayed with type "error" and empty string fields, subsequent rows are displayed normally, and the exit code is 1
2. **Given** a database where all rows parse successfully, **When** a user runs the inspect command, **Then** all rows are displayed with correct types and the exit code is 0
3. **Given** a database with multiple corrupted rows (indices 3, 7, 12), **When** a user runs the inspect command, **Then** all three rows show type "error", other rows display normally, and the exit code is 1

---

### Edge Cases

- What happens when --offset is negative? (Error: invalid offset parameter)
- What happens when --offset is greater than the number of rows? (Not an error, displays zero rows)
- What happens when --limit is 0? (Displays zero rows)
- What happens when the database file doesn't exist? (Error: file not found)
- What happens when the database file is not a valid frozenDB file? (Error: invalid database format)
- What happens when a partial row exists in the middle of the file (not at the end)? (Error: partial row can only be at end of file)
- What happens when --print-header is set to an invalid value (not true/false)? (Error: invalid boolean value)
- What happens when combining --offset beyond file size with --limit? (Displays zero rows, not an error)
- What happens when a checksum row has an invalid checksum? (Display as "error" type, continue processing, exit 1)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept a `--path` flag specifying the frozenDB database file to inspect
- **FR-002**: System MUST accept an optional `--offset` flag (integer, default: 0) specifying the starting row index for display
- **FR-003**: System MUST accept an optional `--limit` flag (integer, default: -1) specifying the maximum number of rows to display, where negative values mean "display all rows"
- **FR-004**: System MUST accept an optional `--print-header` flag (boolean, default: false) specifying whether to display database header information
- **FR-005**: System MUST return an error if `--offset` is negative
- **FR-006**: System MUST display rows in tab-separated values (TSV) format with a header row containing column names, followed by data rows with tab characters separating each column value
- **FR-007**: When `--print-header` is true, system MUST first display a header table with columns: "Row Size", "Clock Skew", "File Version"
- **FR-008**: System MUST display a row data table with columns: "Index", "Type", "Key", "Value", "Savepoint", "Tx start", "Tx end", "Rollback", "Parity"
- **FR-009**: For NullRow types, system MUST display: type="NullRow", key=UUID from row, value=empty string (no characters), savepoint="false", tx_start="true", tx_end="true", rollback="false"
- **FR-010**: For Data row types, system MUST display: type="Data", key=UUID, value=JSON payload, savepoint="true" if end_control starts with 'S' otherwise "false", tx_start="true" if start_control is 'T' otherwise "false", tx_end="true" if end_control is TC or SC otherwise "false", rollback="true" if end_control matches R[0-9] or S[0-9] otherwise "false"
- **FR-011**: For Checksum row types, system MUST display: type="Checksum", value=checksum string, all other columns empty string (no characters) except index and parity
- **FR-012**: For partial data rows (incomplete row at end of file), system MUST display: type="partial", with available fields displayed per Data row rules and unavailable fields as empty string (no characters)
- **FR-013**: All row types MUST display the parity column value
- **FR-014**: The index column MUST use zero-based indexing where index 0 = first checksum row (at offset 64), index 1 = first row after initial checksum, etc.
- **FR-015**: The offset parameter MUST follow the same indexing as the index column (offset 0 = row index 0)
- **FR-016**: If offset is greater than the number of rows in the database, system MUST succeed (exit code 0) and display zero rows
- **FR-017**: If a row fails to parse, system MUST display that row with type="error", all other columns empty string (no characters) except index and parity (if available), and continue processing remaining rows
- **FR-018**: If any row fails to parse during the entire inspection operation, system MUST set exit code to 1 after completing all output
- **FR-019**: If all rows parse successfully, system MUST exit with code 0
- **FR-020**: System MUST display all columns as strings with boolean values represented as "true" or "false"
- **FR-021**: System MUST handle databases with no data rows (only header and initial checksum) by displaying just the checksum row
- **FR-022**: System MUST validate that partial rows can only exist at the end of the file; partial rows in other positions are parsing errors
- **FR-023**: System MUST represent blank/missing field values as empty strings with no characters between tab separators in TSV output (standard TSV convention)
- **FR-024**: System MUST follow the CLI convention of using flexible flag positioning (flags can appear before or after the subcommand)

### Non-Functional Requirements

- **NFR-001**: System MUST use a streaming read-print-read approach where each row is read individually, printed immediately, then the next row is read, ensuring constant memory usage regardless of database size
- **NFR-002**: System MUST seek to the correct file offset position before beginning row-by-row streaming to support efficient offset parameter handling
- **NFR-003**: System MUST support inspection of arbitrarily large database files without memory scaling issues

### Key Entities

- **Database File**: The frozenDB file being inspected, containing header, checksum rows, data rows, null rows, and optionally a partial row
- **Row**: A single row in the database, which can be a checksum row, data row, null row, partial row, or error (failed to parse)
- **Header Metadata**: Database configuration parameters including row_size, skew_ms, and file version
- **Inspection Parameters**: User-specified offset, limit, and print-header flags controlling the inspection output

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `cmd/frozendb/cli_spec_test.go` (matching the main.go implementation file)
- Follow naming convention `Test_S_037_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can inspect any valid frozenDB database file and see all row contents in under 5 seconds for databases with up to 10,000 rows
- **SC-002**: Inspect command output can be successfully parsed by standard Unix tools (awk, grep, sed) for automated analysis
- **SC-003**: Users can identify corrupted rows in a database file through the error row type indicator
- **SC-004**: 100% of partial data rows at the end of files are correctly identified with type "partial"
- **SC-005**: Offset and limit parameters correctly restrict output to requested row ranges in 100% of test cases

### Data Integrity & Correctness Metrics

- **SC-006**: Zero false positives in error row detection (valid rows never marked as error)
- **SC-007**: All row type identifications (Data, NullRow, Checksum, partial) are 100% accurate across all test cases
- **SC-008**: Exit code correctly reflects parsing success (0) or failure (1) in 100% of cases
- **SC-009**: Header metadata values match the actual database file header with 100% accuracy when --print-header is true
- **SC-010**: Index numbering matches the offset logic with 100% consistency (offset 0 = index 0 = first checksum row)
