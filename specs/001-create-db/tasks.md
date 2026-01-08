# Tasks: Database File Creation

**Input**: Design documents from `/specs/001-create-db/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Spec tests are MANDATORY for all functional requirements (FR-001 through FR-032). Unit tests are optional but recommended for implementation details.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`


- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single Go project**: Top-level frozendb/ package
- **Spec tests**: frozendb/spec_tests/
- **Source files**: frozendb/create.go, frozendb/errors.go

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create Go module with go.mod and basic frozendb package structure
- [ ] T002 Create frozendb/spec_tests/ directory for constitutional spec tests
- [ ] T003 Configure gofmt for Go code standards

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create FrozenDBError base error hierarchy in frozendb/errors.go
- [ ] T005 Create constants for frozenDB v1 format and syscall values in frozendb/create.go
- [ ] T006 Create header generation functions in frozendb/create.go (GenerateHeader, HeaderFormat)
- [ ] T007 Create sudo context detection functions in frozendb/create.go (detectSudoContext, SudoContext struct)
- [ ] T008 Create file cleanup helper in frozendb/create.go (cleanupOnFailure)

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Database File Creation without Append-Only (Priority: P1) 🎯 MVP

**Goal**: Create a new frozenDB database file with correct header structure without immutability protection

**Independent Test**: Can be fully tested by creating a database file (mocking the append-only step) and verifying it contains a valid frozenDB v1 header

### Tests for User Story 1 (MANDATORY - spec tests always required) ⚠️

> **NOTE: Write spec tests FIRST, ensure they FAIL before implementation**
> **Spec tests validate functional requirements FR-XXX and go in spec_tests/ folders**

- [ ] T010 [US1] Spec test for FR-001 (Create function with CreateConfig) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T011 [US1] Spec test for FR-015 (rowSize validation 128-65536) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T012 [US1] Spec test for FR-016 (skewMs validation 0-86400000) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T013 [US1] Spec test for FR-017 (path must end with .fdb) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T014 [US1] Spec test for FR-006 (atomic file creation O_CREAT|O_EXCL) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T015 [US1] Spec test for FR-007 (file permissions 0644) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T018 [US1] Spec test for FR-008 (frozenDB v1 header format) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T019 [US1] Spec test for FR-009 (fdatasync before attribute setting) in frozendb/spec_tests/0001_create_db_test.go

### Implementation for User Story 1

- [ ] T020 [US1] Create CreateConfig struct and Validate method in frozendb/create.go
- [ ] T021 [US1] Implement validateInputs function in frozendb/create.go (parameter ranges, .fdb extension)
- [ ] T022 [US1] Implement validatePath function in frozendb/create.go (parent directory, filesystem checks)
- [ ] T023 [US1] Implement createFile function in frozendb/create.go (atomic creation with proper permissions)
- [ ] T024 [US1] Implement writeHeader function in frozendb/create.go (64-byte header with fdatasync)
- [ ] T025 [US1] Implement public Create function in frozendb/create.go (basic flow without append-only)
- [ ] T026 [US1] Run spec tests for User Story 1 and verify all FR-001, FR-006, FR-007, FR-008, FR-009, FR-015, FR-016, FR-017 requirements have coverage
- [ ] T027 [US1] Verify no t.Skip() calls in User Story 1 spec tests without proper documentation

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Append-Only Protection Setup (Priority: P2)

**Goal**: Set the append-only attribute on the database file for immutability guarantees

**Independent Test**: Can be fully tested by taking a pre-existing valid database file and successfully applying the append-only attribute

### Tests for User Story 2 (MANDATORY - spec tests always required) ⚠️

- [ ] T028 [US2] Spec test for FR-002 (sudo context validation via SUDO_USER) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T029 [US2] Spec test for FR-003 (reject direct root execution) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T030 [US2] Spec test for FR-004 (reject unprivileged user) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T031 [US2] Spec test for FR-005 (validate SUDO_UID/SUDO_GID) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T032 [US2] Spec test for FR-010 (ioctl append-only attribute) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T033 [US2] Spec test for FR-011 (direct syscalls, not subprocess) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T034 [US2] Spec test for FR-012 (append-only after header flush) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T035 [US2] Spec test for FR-013 (file ownership to original user) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T036 [US2] Spec test for FR-014 (syscall.Chown for ownership) in frozendb/spec_tests/0001_create_db_test.go

### Implementation for User Story 2

- [ ] T037 [US2] Implement setAppendOnlyAttr function in frozendb/create.go (ioctl with FS_IOC_SETFLAGS)
- [ ] T038 [US2] Implement setOwnership function in frozendb/create.go (chown with sudo context)
- [ ] T039 [US2] Enhance Create function to include append-only and ownership steps in frozendb/create.go
- [ ] T040 [US2] Add Linux syscall constants (FS_IOC_GETFLAGS, FS_IOC_SETFLAGS, FS_APPEND_FL) in frozendb/create.go
- [ ] T041 [US2] Integrate sudo context validation into Create function flow in frozendb/create.go
- [ ] T042 [US2] Run spec tests for User Story 2 and verify all FR-002 through FR-014 requirements have coverage
- [ ] T043 [US2] Verify no t.Skip() calls in User Story 2 spec tests without proper documentation

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Complete Atomic Creation Process (Priority: P3)

**Goal**: Ensure the entire database creation process is atomic with proper cleanup on failure

**Independent Test**: Can be fully tested by running the complete creation process and verifying either success (fully protected file) or failure (no file remains)

### Tests for User Story 3 (MANDATORY - spec tests always required) ⚠️

- [ ] T044 [US3] Spec test for FR-018 (parent directory exists and writable) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T045 [US3] Spec test for FR-019 (cleanup partial files on failure) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T046 [US3] Spec test for FR-020 (non-empty string path validation) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T047 [US3] Spec test for FR-021 through FR-027 (path handling scenarios) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T048 [US3] Spec test for FR-028 (thread safety for concurrent calls) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T049 [US3] Spec test for FR-029 (atomic file creation to other processes) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T050 [US3] Spec test for FR-030 (fixed memory usage) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T051 [US3] Spec test for FR-031 (minimized disk operations) in frozendb/spec_tests/0001_create_db_test.go
- [ ] T052 [US3] Spec test for FR-032 (validation before filesystem operations) in frozendb/spec_tests/0001_create_db_test.go

### Implementation for User Story 3

- [ ] T053 [US3] Enhance validatePath function with comprehensive path validation in frozendb/create.go
- [ ] T054 [US3] Implement atomic cleanup logic in cleanupOnFailure function in frozendb/create.go
- [ ] T055 [US3] Enhance Create function with full atomic operation sequence in frozendb/create.go
- [ ] T056 [US3] Add concurrency safety measures and stateless design in frozendb/create.go
- [ ] T057 [US3] Optimize memory usage and disk operations in frozendb/create.go
- [ ] T058 [US3] Run spec tests for User Story 3 and verify all FR-018 through FR-032 requirements have coverage
- [ ] T059 [US3] Verify no t.Skip() calls in User Story 3 spec tests without proper documentation

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T060 Code cleanup and refactoring in frozendb/create.go
- [ ] T061 Performance optimization across all stories (fixed memory verification)
- [ ] T062 Add unit tests for internal functions in frozendb/create_test.go
- [ ] T063 Run spec tests for all modules and ensure every FR-001 through FR-032 has test coverage
- [ ] T064 Verify no spec tests use t.Skip() without comprehensive documentation
- [ ] T065 Validate quickstart.md examples against implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories proceed sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Builds on US1 file creation
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Enhances US1/US2 with atomic guarantees

### Within Each User Story

- Tests (if included) MUST be written and FAIL before implementation
- Input validation before file operations
- Core implementation before integration
- Story complete before moving to next priority

### Task Dependencies

- Setup tasks execute sequentially
- Foundational tasks execute sequentially within Phase 2
- User stories proceed in priority order (P1 → P2 → P3)
- Tests for a user story execute sequentially

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Team Strategy

1. Team completes Setup + Foundational sequentially
2. Once Foundational is done, proceed with stories in priority order:
   - User Story 1 (P1 - file creation)
   - User Story 2 (P2 - append-only)
   - User Story 3 (P3 - atomic operations)
3. Each story completes before moving to the next

---

## Notes

- Tasks execute sequentially with clear dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- All FR-XXX requirements must have corresponding spec tests
- Go project uses top-level frozendb/ package (not pkg/frozendb/)
- Spec tests go in frozendb/spec_tests/ per constitutional requirements
- Use syscalls for append-only attribute, not subprocess calls
