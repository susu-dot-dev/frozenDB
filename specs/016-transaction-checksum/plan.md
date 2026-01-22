# Implementation Plan: Transaction Checksum Row Insertion

**Branch**: `016-transaction-checksum` | **Date**: 2026-01-22 | **Spec**:
[spec.md](./spec.md) **Input**: Feature specification from
`/specs/016-transaction-checksum/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See
`.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Implement automatic checksum row insertion at 10,000-row intervals within
transactions. The system must detect when complete rows (DataRows and NullRows,
excluding PartialDataRows) reach multiples of 10,000 and immediately insert a
checksum row. Checksum rows must be transparent to transaction logic, appearing
automatically between rows of ongoing transactions without affecting committed
row counts, transaction boundaries, or savepoint state.

## Technical Context

**Language/Version**: Go 1.25.5 **Primary Dependencies**: github.com/google/uuid
(UUIDv7), Go standard library **Storage**: Single append-only file (per v1 file
format specification) **Testing**: Go testing framework (go test) **Target
Platform**: Linux (via Makefile targets) **Project Type**: single -
Library/database package with append-only file storage **Performance Goals**:

- Insert 10,000 complete rows and write checksum row within 1 second of
  threshold
- Memory usage remains O(1) regardless of database size (fixed overhead for
  tracking row count) **Constraints**:
- Checksum rows must NOT appear in query results or committed row counts
- Row counting must exclude PartialDataRows (only complete DataRows and NullRows
  count)
- Checksum row insertion must not interfere with transaction semantics
  (savepoints, rollback boundaries)
- Must validate parity of all rows before calculating checksum **Scale/Scope**:
  Transaction-level feature integrated into existing Transaction and FileManager
  types

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

- [x] **Immutability First**: Design preserves append-only immutability -
      checksum rows are only appended, never modified in place
- [x] **Data Integrity**: Checksum rows provide CRC32 validation for blocks of
      10,000 rows; parity bytes validated during checksum calculation
- [x] **Correctness Over Performance**: Checksum row insertion prioritizes data
      integrity over minimal write overhead; validates all row parity before
      calculating checksum
- [x] **Chronological Ordering**: Not directly affected - checksum rows use
      start_control='C' and are inserted based on row count, not UUID ordering
- [x] **Concurrent Read-Write Safety**: FileManager's writerLoop provides
      concurrent safety via channels; checksum insertion happens synchronously
      within transaction writes
- [x] **Single-File Architecture**: Uses existing FileManager and single file
      format; checksum rows are appended to same file
- [x] **Spec Test Compliance**: All functional requirements have corresponding
      spec tests in transaction_spec_test.go (designed in Phase 1, to be
      implemented in Phase 2)

## Project Structure

### Documentation (this feature)

```text
specs/016-transaction-checksum/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
frozendb/
├── transaction.go              # Modify: Add NewTransaction function, write checksum row logic
├── file_manager.go            # Modify: Add DBFile interface for mocking
├── row_union.go                # New: RowUnion type with UnmarshalText method
├── checksum.go                # Existing: NewChecksumRow already exists
├── errors.go                  # Existing: All error types defined
├── transaction_spec_test.go    # Modify: Spec tests for FR-001 through FR-007
└── [existing files unchanged]
```

**Structure Decision**: Single package `frozendb` with all implementation in
`frozendb/` directory. Transaction and FileManager are the primary modified
types, with ChecksumRow creation reusing existing `NewChecksumRow()`. RowUnion
type with UnmarshalText method are in separate `row_union.go` file for clarity.
Spec tests co-located in `transaction_spec_test.go` following spec testing guidelines.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                | Why Needed | Simpler Alternative Rejected Because |
| ------------------------ | ---------- | ------------------------------------ |
| (No violations expected) |            |                                      |
