# Implementation Plan: Transaction File Persistence

**Branch**: `015-transaction-persistence` | **Date**: 2026-01-21 | **Spec**:
spec.md **Input**: Feature specification from
`/specs/015-transaction-persistence/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See
`.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Add file persistence to the Transaction struct via the FileManager channel-based
interface. Transaction operations (Begin, AddRow, Commit) will serialize rows to
bytes and send them through a write channel, waiting for synchronous completion.
The caller interacts directly with the FileManager to set up the write channel,
making testing easier. No checksum rows are needed for this spec (<10,000 rows
assumption).

## Technical Context

**Language/Version**: Go 1.25.5 **Primary Dependencies**:
github.com/google/uuid, Go standard library (os, sync, math) **Storage**: Single
append-only file (frozenDB v1 format per docs/v1_file_format.md) **Testing**: go
test with spec tests (Test_S_015_FR_XXX_* pattern) **Target Platform**: Linux
**Project Type**: single (library/package) **Performance Goals**: O(1) seeking
with fixed-width rows, binary search optimization, synchronous writes
**Constraints**: Append-only immutability, fixed memory usage, < 10,000 rows
(checksums not needed) **Scale/Scope**: Transaction file persistence for <
10,000 rows

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

### Pre-Phase 0 Evaluation

- [x] **Immutability First**: Design preserves append-only immutability with no
      delete/modify operations - Transaction writes new bytes only via
      FileManager's append-only interface
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption
      detection are included - Row serialization includes ROW_START (0x1F) and
      ROW_END (0x0A) sentinels per v1_file_format.md
- [x] **Correctness Over Performance**: Any performance optimizations maintain
      data correctness - Synchronous writes ensure data is persisted before
      returning to caller
- [x] **Chronological Ordering**: Design supports time-based key ordering with
      proper handling of time variations - Existing Transaction timestamp
      validation preserved
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and
      writes without data corruption - FileManager uses RWMutex for thread-safe
      append-only writes, Transaction.mu protects internal state for concurrent
      goroutine access (FR-010)
- [x] **Single-File Architecture**: Database uses single file enabling simple
      backup/recovery - Uses existing FileManager single-file interface
- [x] **Spec Test Compliance**: All functional requirements have corresponding
      spec tests in [filename]_spec_test.go files - All FR-001 through FR-009
      will have spec tests

### Post-Phase 1 Re-evaluation

- [x] **Immutability First**: Confirmed - Write pattern uses MarshalText() which
      only appends new bytes via FileManager
- [x] **Data Integrity**: Confirmed - All row types (DataRow, PartialDataRow,
      NullRow) include sentinel and parity bytes
- [x] **Correctness Over Performance**: Confirmed - Synchronous channel pattern
      ensures write completes before operation returns
- [x] **Chronological Ordering**: Confirmed - AddRow() preserves existing
      timestamp validation logic
- [x] **Concurrent Read-Write Safety**: Confirmed - Transaction.mu protects
      internal state (Write lock for changes, Read lock for reads), FileManager
      handles concurrent writes via RWMutex, FR-010 thread-safety requirement
      addressed
- [x] **Single-File Architecture**: Confirmed - FileManager interface is
      append-only single-file, Transaction does not modify this
- [x] **Spec Test Compliance**: Confirmed - All FR-001 through FR-009 have
      corresponding spec tests documented in contracts/go_api.md

**Status**: All gates passed. Proceeding to Phase 2.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
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
├── errors.go              # Structured error types
├── header.go              # Header structure and validation
├── checksum.go            # Checksum row structure
├── data_row.go            # DataRow structure
├── partial_data_row.go    # PartialDataRow structure
├── null_row.go            # NullRow structure
├── row.go                 # Base row interface
 ├── transaction.go         # Transaction struct (MODIFIED - add write channel and rowBytesWritten field)
├── file_manager.go        # FileManager for append-only file operations
├── frozendb.go            # Main database API
├── open.go                # Database opening
├── create.go              # Database creation
├── *test.go               # Unit tests for each module
└── *_spec_test.go         # Spec tests for each module
```

**Structure Decision**: Single package `frozendb` with all core components in
the `frozendb/` directory. This is a Go library package, not a multi-project
application. The existing structure follows Go conventions with implementation
and test files co-located in the same package directory.

## Phase 0: Research - COMPLETED

**Output**: `research.md`

**Decisions Made**:

1. Row Serialization Strategy: Use existing MarshalText() methods on DataRow,
   PartialDataRow, NullRow
2. Synchronous Write Pattern: Use response channel pattern (create responseChan,
   send Data, wait for response)
3. Transaction Integration: Add writeChan field to Transaction, caller sets up
   FileManager
4. Error Handling: Tombstone transaction on write failure, return TombstonedError for subsequent calls
5. PartialDataRow Incremental Write Strategy: Transaction tracks rowBytesWritten
   and slices MarshalText() output
6. Thread-Safety for Concurrent Access: Use existing Transaction.mu
   (sync.RWMutex) to protect all state modifications (FR-010)
7. Checksum Row Handling: Do not write checksum rows (assumes < 10,000 rows)

**Key Findings**:

- All row types already implement MarshalText() returning complete row bytes
- FileManager Data struct includes Response channel for synchronous completion
- User input specifies channel-based interaction (caller manages FileManager)
- Write failure must tombstone transaction and return error (FR-006)
- PartialDataRow state progression is cumulative; must slice MarshalText()
  output to preserve append-only semantics

## Phase 1: Design & Contracts - COMPLETED

**Outputs**:

- `data-model.md`: Entity definitions, state transitions, persistence model,
  error handling
- `contracts/go_api.md`: Transaction API contract with Begin(), AddRow(),
  Commit() documentation
- `quickstart.md`: 3 examples showing basic usage pattern
- `AGENTS.md`: Updated agent context with Go 1.25.5, github.com/google/uuid,
  append-only file

**Design Decisions**:

- Transaction struct adds `writeChan chan<- Data` field
- Transaction struct adds `rowBytesWritten int` field to track PartialDataRow write
  progress (internal field, NOT initialized by caller)
- Write helper function: create responseChan, send Data, wait for error
- PartialDataRow writes use slicing: `newBytes := allBytes[tx.rowBytesWritten:]`
- All operations synchronous (wait for write completion before returning)
- All state modifications protected by Transaction.mu (Write lock for changes,
  Read lock for reads) - FR-010
- Error handling: tombstone transaction on write failure, return TombstonedError for subsequent calls
- Caller responsible for FileManager setup (SetWriter())
- No checksum rows written per FR-009

**Constitution Compliance**: All 7 principles verified and passing

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation                  | Why Needed         | Simpler Alternative Rejected Because |
| -------------------------- | ------------------ | ------------------------------------ |
| [e.g., 4th project]        | [current need]     | [why 3 projects insufficient]        |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient]  |

**Status**: No violations - all constitutional principles satisfied
