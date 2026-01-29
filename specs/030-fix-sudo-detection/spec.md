# Bug Fix: Sudo Detection Logic

**Feature Branch**: `030-fix-sudo-detection`  
**Created**: Thu Jan 29 2026  
**Status**: Draft  
**Input**: User description: "Right now, if you launch sudo frozendb create, it fails with this error: Error: write_error: direct root execution not allowed, because the sudo detection logic is incorrect. As a user I can properly create a database when run under sudo, but not when run as root"

## User Scenarios & Testing *(mandatory)*

### User Story - Run Command with Sudo While Preventing Direct Root Execution (Priority: P1)

As a system administrator, I can run `sudo frozendb create` to create databases with proper permissions, while the system still correctly prevents direct root execution (logged in as root without sudo).

**Why this priority**: Core bug fix blocking database creation functionality.

**Independent Test**: Can be fully tested by running `sudo frozendb create <path>` as a non-root user with sudo privileges, and separately testing that direct root execution (without sudo) is still blocked.

**Acceptance Scenarios**:

1. **Given** a user with sudo privileges (not logged in as root), **When** they run `sudo frozendb create /path/to/db.frozen`, **Then** the database is created successfully
2. **Given** a user logged in directly as root (no SUDO_USER environment variable), **When** they run `frozendb create /path/to/db.frozen`, **Then** the system rejects the operation with error "direct root execution not allowed"

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow database creation when running with sudo (SUDO_USER environment variable present)
- **FR-002**: System MUST reject database creation when running as direct root (UID=0 with no SUDO_USER environment variable)

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/create_spec_test.go` to match the implementation in `internal/frozendb/create.go`
- Follow naming convention `Test_S_030_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `sudo frozendb create` succeeds without "direct root execution not allowed" errors
- **SC-002**: Direct root execution (without sudo) is blocked 100% of the time
