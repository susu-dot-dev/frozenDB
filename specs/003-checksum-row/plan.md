# Implementation Plan: Checksum Row Implementation

**Branch**: `003-checksum-row` | **Date**: 2026-01-10 | **Spec**: [link](/home/anil/code/frozenDB/specs/003-checksum-row/spec.md)
**Input**: Feature specification from `/specs/003-checksum-row/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This plan implements the core row serialization infrastructure for frozenDB, following the user's exact specifications. The implementation includes baseRow abstraction (unexported) with Header reference (not RowSize field), proper Go enums for StartControl and EndControl, ChecksumPayload with no UUID, and ChecksumRow structures. The implementation will provide exact byte-format compliance with v1_file_format.md, enabling creation of checksum rows with proper CRC32 calculation, Base64 encoding, and dynamic LRC parity validation (not hardcoded).

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Standard library only (encoding/base64, encoding/json, hash/crc32)  
**Storage**: Single file-based frozenDB database (.fdb extension)  
**Testing**: Go testing package with spec tests (Test_S_ prefix) and unit tests  
**Target Platform**: Linux server (primary), cross-platform compatible  
**Project Type**: Single Go package (frozendb)  
**Performance Goals**: Fixed memory usage, O(1) row seeking, efficient CRC32 calculation  
**Constraints**: Append-only immutability, 128-65536 byte fixed-width rows, exact byte format compliance  
**Scale/Scope**: Core infrastructure for database integrity checking, supports unlimited row counts  

**GitHub Repository**: `git@github.com:susu-dot-dev/frozenDB.git` → import path: `github.com/susu-dot-dev/frozenDB/frozendb`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Immutability First**: Design preserves append-only immutability with no delete/modify operations - checksum rows are immutable once written
- [x] **Data Integrity**: Transaction headers with sentinel bytes for corruption detection are included - parity bytes and CRC32 provide integrity checking
- [x] **Correctness Over Performance**: Any performance optimizations maintain data correctness - design prioritizes exact format compliance over performance
- [x] **Chronological Ordering**: Design supports time-based key ordering with proper handling of time variations - checksum rows don't use keys but support existing key ordering infrastructure
- [x] **Concurrent Read-Write Safety**: Design supports concurrent reads and writes without data corruption - immutable rows enable safe concurrent reads
- [x] **Single-File Architecture**: Database uses single file enabling simple backup/recovery - checksum rows are written to single frozenDB file
- [x] **Spec Test Compliance**: All functional requirements have corresponding spec tests in [filename]_spec_test.go files - SPECIFIED in contracts and data-model

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
├── create.go              # Existing: Database creation and header generation
├── create_test.go         # Existing: Tests for database creation
├── create_spec_test.go    # Existing: Spec tests for database creation
├── errors.go              # Existing: Error type definitions
├── frozendb.go            # Existing: Main database operations
├── frozendb_spec_test.go  # Existing: Spec tests for main operations
├── open.go                # Existing: Database opening and header parsing
├── open_test.go           # Existing: Tests for database opening
├── row.go                 # NEW: baseRow (unexported), enums, and row serialization infrastructure
├── row_test.go            # NEW: Unit tests for row operations
├── row_spec_test.go       # NEW: Spec tests for row format compliance
├── checksum.go            # NEW: ChecksumRow and ChecksumPayload implementation
├── checksum_test.go       # NEW: Unit tests for checksum operations
└── checksum_spec_test.go  # NEW: Spec tests for checksum format compliance
```

**Structure Decision**: Single Go package with existing frozendb/ structure. New files will implement row serialization infrastructure following existing patterns from create.go and open.go

## Complexity Tracking

No constitutional violations requiring justification.
