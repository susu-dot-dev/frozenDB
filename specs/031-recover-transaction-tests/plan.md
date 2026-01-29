# Implementation Plan: recoverTransaction Correctness Test Suite

**Branch**: `031-recover-transaction-tests` | **Date**: 2026-01-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/031-recover-transaction-tests/spec.md`

## Summary

Create an exhaustive test suite for the `recoverTransaction` function that validates correct recovery of in-memory transaction state when reopening a frozenDB file in read-only mode. The test suite will systematically test all transaction states (zero transactions, single transaction, multiple transactions, partial transactions), all PartialDataRow states (1, 2, 3), various row counts (0-100 rows), and special cases (checksum rows, NullRows, savepoints). The suite uses a pattern of creating databases in write mode, reopening in read-only mode, and comparing in-memory state for correctness. This will identify existing bugs (including the known bug where Begin() after Commit() incorrectly references closed transaction state) and validate fixes.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: github.com/google/uuid, Go standard library  
**Storage**: Single append-only .fdb file format  
**Testing**: Go testing package (internal/frozendb/*_spec_test.go pattern)  
**Target Platform**: Linux (file locks, append-only ioctl)  
**Project Type**: Single project (database library)  
**Performance Goals**: Test suite executes in under 10 seconds total  
**Constraints**: Tests must use Create() helpers to avoid sudo permission issues; tests must access internal Transaction state for comparison  
**Scale/Scope**: 50+ test scenarios covering all transaction states and edge cases  

**GitHub Repository**: github.com/anomalyco/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Test suite validates append-only recovery (no modifications)
- [x] **Data Integrity**: Tests verify transaction state integrity after recovery
- [x] **Correctness Over Performance**: Test focus is 100% on correctness, not performance
- [x] **Chronological Ordering**: Tests use UUIDv7 keys with chronological ordering
- [x] **Concurrent Read-Write Safety**: Tests verify read-only mode can recover while write mode holds file
- [x] **Single-File Architecture**: Tests operate on single .fdb files
- [x] **Spec Test Compliance**: All FR-002 through FR-010 will have spec tests in recovery_spec_test.go

**Notes**: This is a test-only feature (no production code changes to file format or transaction semantics). The test suite will identify bugs in the existing `recoverTransaction` implementation which will then be fixed.

## Project Structure

### Documentation (this feature)

```text
specs/031-recover-transaction-tests/
├── plan.md              # This file
├── research.md          # Phase 0: Test patterns, state comparison approach, scenario catalog
├── data-model.md        # Phase 1: Test scenario structure (not needed - tests don't introduce new entities)
├── contracts/           # Phase 1: Test helper API contracts
│   └── api.md          # Test helper functions for DB creation, state comparison
└── spec.md              # Feature specification
```

### Source Code (repository root)

```text
internal/frozendb/
├── recovery_spec_test.go        # NEW: Spec tests for FR-002 through FR-010
├── frozendb.go                  # MODIFY: Bug fixes to recoverTransaction()
├── transaction.go               # MODIFY: Bug fixes to Begin() (if needed)
├── create.go                    # EXISTING: Create() function for test DB creation
├── frozendb_test.go             # EXISTING: May add helper functions here
└── *_test.go                    # EXISTING: Test utilities

docs/
└── v1_file_format.md            # EXISTING: File format reference
```

**Structure Decision**: Single project structure with new recovery_spec_test.go file in internal/frozendb package. This co-locates tests with the recoverTransaction implementation per spec testing guidelines. All test helpers will be defined in recovery_spec_test.go alongside the spec tests.

## Complexity Tracking

No constitutional violations. This is a test-only feature that validates existing functionality.

---

# Phase 0: Research & Decisions ✅ COMPLETE

**Goal**: Resolve how to create test databases, compare transaction state, and enumerate all test scenarios.

**Status**: All research tasks completed. See [research.md](./research.md) for full details.

## Key Decisions

1. ✅ **Test Database Creation Pattern**: Use existing `createTestDatabase()` helper with mock syscalls (setupMockSyscalls) and mock SUDO environment variables to avoid actual sudo requirements
2. ✅ **Transaction State Comparison**: Helper functions with direct field access (tests in same package can access unexported fields)
3. ✅ **PartialDataRow Operations**: 5 exhaustive operation sequences listed
4. ✅ **Checksum Row Testing**: Create 10,000 rows via 100 transactions, test 3 critical scenarios with t.Parallel()
5. ✅ **Complete Test Scenario Catalog**: 43 test scenarios identified across FR-002 through FR-010

## Research Artifacts

- **research.md**: Complete documentation of patterns, decisions, and scenario catalog
- Total test count: 43 scenarios (including 3 checksum row tests)
- Estimated execution time: ~9.5 seconds with t.Parallel() for checksum tests (within 10-second budget)
- **Key Pattern**: createTestDatabase() uses Create() with mock syscalls to avoid sudo

---

# Phase 1: API Contracts ✅ COMPLETE

**Goal**: Define test helper API and test scenario structure.

**Status**: API contracts documented. See [contracts/api.md](./contracts/api.md) for full details.

## API Contracts Summary

Documented in `/contracts/api.md`:

1. ✅ **Database Creation Helpers**
   - `createTestDB(t, rowSize)` - Creates empty database
   - `createDBWithOps(t, rowSize, ops)` - Creates database with operations
   - `txOp` interface and implementations (opBegin, opAddRow, opCommit, opSavepoint, opRollback)

2. ✅ **State Comparison Helpers**
   - `compareTransactionState(t, original, recovered)` - Main comparison per FR-001
   - `compareRows(t, original, recovered)` - Deep DataRow comparison
   - `comparePartialDataRow(t, original, recovered)` - PartialDataRow comparison
   - `compareNullRow(t, original, recovered)` - NullRow comparison

3. ✅ **Utility Helpers**
   - `generateUUIDv7(t)` - Generate test UUIDs with chronological ordering
   - `generateTestValue()` - Generate simple test JSON values
   - `captureTransactionState(t, db)` - Capture state snapshot before closing

4. ✅ **Standard Test Pattern**
   - Create DB → Perform operations → Capture state → Close → Reopen read-only → Compare

## Data Model

**Not applicable**: This feature does not introduce new data entities. All entities (Transaction, PartialDataRow, DataRow, NullRow) exist in the current implementation and are documented in the spec.md file.

---

# Phase 2: Implementation Tasks

**Not created by /speckit.plan - run /speckit.tasks to generate tasks.md**

