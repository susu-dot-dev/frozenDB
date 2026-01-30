# Implementation Plan: frozendb verify

**Branch**: `034-frozendb-verify` | **Date**: 2026-01-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/034-frozendb-verify/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.opencode/command/speckit.plan.md` for the execution workflow and document guidelines.

## Summary

Implement a verify operation for frozenDB that validates file integrity by checking: header correctness, all checksum blocks (every 10k rows), parity bytes on rows after the last checksum, row format compliance with v1 file format specification, and partial data row validity. The verify operation answers the fundamental question "Is my database file valid or corrupted?" and reports specific corruption details when found.

## Technical Context

**Language/Version**: Go 1.25.6  
**Primary Dependencies**: github.com/google/uuid, Go standard library (encoding/base64, encoding/json, hash/crc32)  
**Storage**: Single-file append-only database format (frozenDB v1)  
**Testing**: Go test framework with spec tests in verify_spec_test.go  
**Target Platform**: Linux (amd64, arm64)  
**Project Type**: Single project (database library)  
**Performance Goals**: N/A (correctness-focused, no specific performance targets per spec)  
**Constraints**: Read-only operation, must not modify database file, fail-fast on first corruption  
**Scale/Scope**: Validate files of any size, from empty (header + initial checksum) to unbounded rows  

**GitHub Repository**: github.com/susu-dot-dev/frozenDB

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Verify is read-only and does not modify any data
- [x] **Data Integrity**: Verify validates integrity checks (checksums, parity) built into the format
- [x] **Correctness Over Performance**: Verify prioritizes correctness (100% corruption detection) over speed
- [x] **Chronological Ordering**: Not applicable - verify does not validate timestamp ordering per FR-040
- [x] **Concurrent Read-Write Safety**: Verify is read-only and safe during concurrent operations
- [x] **Single-File Architecture**: Verify validates the single-file database format
- [x] **Spec Test Compliance**: All 40 FRs will have corresponding spec tests in verify_spec_test.go

**Note on FR-039 and FR-040**: Per specification, verify explicitly does NOT validate transaction nesting or UUID timestamp ordering. This is intentional scope limitation to focus on structural and cryptographic integrity validation only.

## Project Structure

### Documentation (this feature)

```text
specs/034-frozendb-verify/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── api.md          # Verify API specification
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/frozendb/
├── verify.go                # Main verify implementation
├── verify_spec_test.go      # Spec tests for all 40 FRs (Test_S_034_FR_XXX_Description)
├── verify_test.go           # Unit tests for internal functions
├── errors.go                # Existing error types (may need new CorruptDatabaseError variants)
├── header.go                # Existing header validation (reuse)
├── checksum.go              # Existing checksum calculation (reuse)
├── row.go                   # Existing baseRow with parity calculation (reuse)
├── data_row.go              # Existing DataRow validation (reuse)
├── null_row.go              # Existing NullRow validation (reuse)
└── partial_data_row.go      # Existing PartialDataRow validation (if exists, else create)
```

**Structure Decision**: frozenDB uses a single Go module structure with internal/frozendb as the core package. All verify functionality will be added to this package alongside existing create, read, and write operations. The verify operation will leverage existing validation functions from header.go, checksum.go, row.go, data_row.go, and null_row.go to avoid duplication.

## Complexity Tracking

No violations - verify operation aligns with all constitutional principles.
