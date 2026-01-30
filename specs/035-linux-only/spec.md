# Feature Specification: Linux-Only Platform Restriction

**Feature Branch**: `035-linux-only`  
**Created**: 2026-01-30  
**Status**: Draft  
**Input**: User description: "035 linux-only. Restrict the codebase to being linux-only for now. We are using syscalls and don't want to ensure mac compatibility right now. Since this is a small, essentially bug-fix type of story this should be an extremely small spec, with just one user story, and the functional requirement that we only build releases for linux in the github workflow. We'll also need a FR to change references to be linux-only, and a FR to allow breaking changes to S_033 to remove the mac requirements"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Linux-Only Platform Support (Priority: P1)

The frozenDB project is transitioning to Linux-only support due to its use of platform-specific syscalls. The build system, documentation, and release workflows must be updated to reflect that macOS (and other platforms) are no longer supported. This simplifies the codebase and allows developers to focus exclusively on Linux compatibility without maintaining cross-platform code.

**Why this priority**: This is a critical infrastructure change that affects the entire development and release process. It must be completed to establish the correct platform expectations and prevent users from attempting to use frozenDB on unsupported platforms.

**Independent Test**: Can be fully tested by: (1) verifying GitHub Actions only builds Linux binaries, (2) confirming documentation references only Linux, (3) validating that spec tests for S_033 macOS requirements are updated or removed. Delivers immediate value by clarifying platform support and preventing wasted effort on cross-platform compatibility.

**Acceptance Scenarios**:

1. **Given** a GitHub release is created, **When** the release workflow triggers, **Then** only Linux binaries (linux/amd64, linux/arm64) are built and attached to the release, with no macOS binaries
2. **Given** the updated release workflow, **When** a developer reviews the configuration, **Then** the build matrix only includes Linux platforms with no darwin/macOS entries
3. **Given** updated documentation and specs, **When** a developer reads platform requirements, **Then** all references consistently indicate Linux-only support

---

### Edge Cases

- What happens to existing macOS-related spec tests from S_033?
- How should the project communicate this platform restriction to users who previously expected cross-platform support?
- What happens if someone attempts to build frozenDB on macOS manually?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: GitHub Actions release workflow MUST build binaries only for Linux (linux/amd64 and linux/arm64)
- **FR-002**: GitHub Actions release workflow MUST NOT build binaries for macOS (darwin) or any other non-Linux platforms
- **FR-003**: Spec S_033 functional requirements FR-011 (macOS darwin/amd64 and darwin/arm64 builds) MUST be removed or marked as obsolete
- **FR-004**: Documentation and spec files referencing cross-platform support MUST be updated to indicate Linux-only support
- **FR-005**: Spec tests for S_033 FR-011 (macOS build requirements) MUST be removed or updated to reflect Linux-only builds

### Key Entities

- **Release Workflow**: The GitHub Actions workflow file (`.github/workflows/release.yml`) that defines which platforms are built during releases
- **Platform Matrix**: The build matrix configuration specifying operating systems and architectures for automated builds
- **Spec Requirements**: Functional requirements in S_033 that previously mandated macOS support

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in appropriate test files based on the component being tested
- Follow naming convention `Test_S_035_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

**Exception for GitHub Actions Workflow Requirements (FR-001, FR-002)**:
- These requirements relate to GitHub Actions workflows which will be manually tested
- Spec test functions MUST be created but should call `t.Skip()` immediately
- Skip comment MUST state: "GitHub Actions workflows are manually tested"
- This documents the requirement while acknowledging the testing approach

**Exception for Documentation Update Requirements (FR-003, FR-004, FR-005)**:
- These requirements relate to documentation and spec file updates which will be manually verified
- Spec test functions MUST be created but should call `t.Skip()` immediately
- Skip comment MUST state: "Documentation and spec updates are manually verified"

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: GitHub Actions release workflow completes in under 5 minutes (reduced from 10 minutes due to fewer platform builds)
- **SC-002**: 100% of release builds succeed for Linux platforms
- **SC-003**: Zero macOS-related build failures occur in release workflows
- **SC-004**: All documentation consistently references Linux-only support with no conflicting cross-platform statements

### Data Integrity & Correctness Metrics

- **SC-005**: Release workflow configuration contains exactly 2 platform entries (linux/amd64 and linux/arm64)
- **SC-006**: Spec S_033 contains no active functional requirements for macOS builds
- **SC-007**: All spec tests related to macOS builds are either removed or properly skipped with documentation

## Assumptions

- **A-001**: Users attempting to build frozenDB on macOS will encounter build errors or undefined behavior, which is acceptable given the Linux-only stance
- **A-002**: Future platform support (if needed) would be added through a new spec rather than reverting these changes
- **A-003**: Existing macOS binaries from previous releases can remain available but will not be updated
- **A-004**: The codebase already uses Linux-specific syscalls that prevent cross-platform compatibility

## Dependencies

- **D-001**: GitHub Actions workflow must be editable and deployable
- **D-002**: S_033 spec file and associated test files must be accessible for updates

## Out of Scope

- Adding build-time checks to prevent compilation on non-Linux platforms
- Creating migration guides for existing macOS users
- Implementing platform detection in the CLI to warn macOS users
- Removing all historical references to cross-platform support in git history
- Adding Linux distribution-specific build targets (Ubuntu, RHEL, etc.)
