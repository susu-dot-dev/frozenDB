# Feature Specification: Release Scripts & Version Management

**Feature Branch**: `033-release-scripts`  
**Created**: 2026-01-29  
**Status**: Draft  
**Input**: User description: "We want to create the ability to bump the local version, create a branch, git commit the change to a branch, and then push the branch. (The user will manually create a PR and merge that to main later). Next, the user will manually create a git release based off the main branch. When that happens, we want to have a github workflow that builds the frozendb CLI binary for mac and linux. Lastly, the user can verify which version of the CLI they have by running frozendb version, which will return the correct string"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Version Bumping and Release Branch Creation (Priority: P1)

A maintainer needs to prepare a new release of frozenDB. They run a script that increments the version number, creates a dedicated release branch, commits the version change, and pushes it to the remote repository. This allows them to follow a standard release workflow where version changes are reviewed via pull request before being merged to main.

**Why this priority**: This is the foundation of the entire release workflow. Without the ability to properly version and branch, no releases can happen. This is the first step every maintainer will take.

**Independent Test**: Can be fully tested by running the version bump script and verifying that: (1) version.go is created/updated with the new version, (2) a new branch is created with the version changes committed, (3) the branch is pushed to the remote repository. Delivers immediate value by establishing version tracking.

**Acceptance Scenarios**:

1. **Given** a frozenDB repository on the main branch with no existing version.go file, **When** maintainer runs the version bump script with target version "v0.1.0", **Then** version.go is created with "v0.1.0", a new branch "release/v0.1.0" is created, the changes are committed with message "Bump version to v0.1.0", and the branch is pushed to remote
2. **Given** a frozenDB repository with existing version "v0.1.0", **When** maintainer runs the version bump script with target version "v0.2.0", **Then** version.go is updated to "v0.2.0", a new branch "release/v0.2.0" is created, the changes are committed, and the branch is pushed to remote
3. **Given** a repository where the remote push fails, **When** version bump script attempts to push, **Then** the script reports the error clearly and indicates that local changes (branch and commit) remain, allowing the user to manually resolve and push

---

### User Story 2 - CLI Version Command (Priority: P2)

A user who has installed the frozendb CLI wants to check which version they are running. They execute `frozendb version` and receive a clear version string that matches the version from the release.

**Why this priority**: This is essential for debugging and support. Users and maintainers need to know exactly which version of the CLI is installed. This must work before automated builds, as it will be used to verify those builds.

**Independent Test**: Can be fully tested by building the CLI binary, embedding a known version string during build, and running `frozendb version` to verify output matches the embedded version. Delivers value by enabling version verification.

**Acceptance Scenarios**:

1. **Given** frozendb CLI built with version "v0.1.0", **When** user runs `frozendb version`, **Then** the output displays "frozendb v0.1.0" (or similar clear format) and exits with code 0
2. **Given** frozendb CLI built without version information, **When** user runs `frozendb version`, **Then** the output displays "frozendb (development)" or similar indicator showing it's an unversioned build
3. **Given** any version of frozendb CLI, **When** user runs `frozendb --version` (alternative flag format), **Then** the same version information is displayed as with `frozendb version`

---

### User Story 3 - Automated Release Builds (Priority: P3)

A maintainer creates a GitHub release from the main branch (after merging the release PR). GitHub Actions automatically builds frozendb CLI binaries for Linux, attaches them to the release, making them available for download by users.

**Why this priority**: This automates the distribution process, but depends on the version system (P1) being in place and the version command (P2) being implemented. Users can manually build from source until this is ready.

**Independent Test**: Can be fully tested by creating a git tag/release on GitHub and verifying that: (1) the workflow triggers, (2) builds complete successfully for both platforms, (3) binary artifacts are attached to the release. Delivers value by eliminating manual build and distribution steps.

**Acceptance Scenarios**:

1. **Given** a GitHub release is created with tag "v0.1.0" from main branch, **When** the release workflow triggers, **Then** Linux (linux/amd64, linux/arm64) binaries are built, and both binaries are attached to the release as downloadable assets
2. **Given** the build process for any platform fails, **When** the workflow runs, **Then** the workflow fails with clear error messages indicating which platform failed and why
3. **Given** a release is created for a pre-release version (e.g., "v0.1.0-rc1"), **When** the workflow runs, **Then** binaries are built and attached, and the release is marked as a pre-release

---

### Edge Cases

- What happens when a version bump script is run with a version that already exists (same or lower version)?
- How does the system handle version bumps when there are uncommitted changes in the working directory?
- What happens if the release workflow is triggered but the version.go file is missing or corrupted?
- How does the CLI version command behave if it was built with an invalid or malformed version string?
- What happens if GitHub Actions fails to upload artifacts due to network issues or API limits?
- How does the system handle version strings with different formats (semantic versioning with pre-release tags, build metadata)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a script that accepts a target version string and updates a generated version.go file to contain that version
- **FR-002**: System MUST create a new git branch with naming pattern "release/{version}" when version bump script runs
- **FR-003**: System MUST commit the version.go changes with a descriptive commit message including the version number
- **FR-004**: System MUST push the release branch to the remote repository
- **FR-005**: System MUST report errors clearly if any step of the version bump process fails (branch creation, commit, push)
- **FR-006**: frozendb CLI MUST support a "version" subcommand that displays the current version
- **FR-007**: frozendb CLI MUST support "--version" flag as an alternative to the "version" subcommand
- **FR-008**: Version output MUST be in a clear, readable format (e.g., "frozendb v0.1.0")
- **FR-009**: Version information MUST be embedded in the binary through a generated version.go file that is created/updated by the version bump script
- **FR-010**: System MUST provide a GitHub Actions workflow that triggers on release creation
- **FR-011**: ~~GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)~~ **OBSOLETE**: Superseded by S_035 (Linux-only platform restriction)
- **FR-012**: GitHub Actions workflow MUST build frozendb CLI binaries for Linux (linux/amd64 and linux/arm64)
- **FR-013**: All built binaries MUST be attached to the GitHub release as downloadable assets
- **FR-014**: Binary artifacts MUST be named clearly to indicate platform and architecture (e.g., "frozendb-linux-amd64", "frozendb-linux-arm64")
- **FR-015**: Version bump script MUST validate that the provided version string follows semantic versioning format (vX.Y.Z or vX.Y.Z-prerelease)
- **FR-016**: Version bump script MUST check for uncommitted changes and warn the user before proceeding

### Key Entities

- **Version Files**: A generated version.go source file containing the current version string in semantic version format (e.g., "v0.1.0"). The version.go file provides a constant that can be accessed by the CLI at runtime. The project version is also tracked via git tags (e.g., v0.1.0) following Go module conventions.
- **Release Branch**: A git branch created specifically for preparing a release, named "release/{version}", containing the version bump commit
- **CLI Binary**: The compiled frozendb executable for a specific platform/architecture combination, with version information embedded
- **GitHub Release**: A GitHub release object with an associated git tag, triggering automated build processes
- **Build Artifact**: A compiled binary file attached to a GitHub release, named according to platform and architecture

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in appropriate test files based on the component being tested
- Follow naming convention `Test_S_033_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

**Exception for GitHub Actions Workflow Requirements (FR-010 through FR-014)**:
- These requirements relate to GitHub Actions workflows which will be manually tested
- Spec test functions MUST be created but should call `t.Skip()` immediately
- Skip comment MUST state: "GitHub Actions workflows are manually tested"
- This documents the requirement while acknowledging the testing approach

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Maintainers can prepare a release branch in under 2 minutes using the version bump script
- **SC-002**: Users can determine their installed CLI version by running a single command that completes instantly (under 100ms)
- **SC-003**: GitHub Actions builds complete within 10 minutes of release creation for all platforms
- **SC-004**: 100% of GitHub release builds succeed when triggered from valid release tags on main branch
- **SC-005**: Zero manual steps required between creating a GitHub release and having downloadable binaries available

### Data Integrity & Correctness Metrics

- **SC-006**: Version information displayed by CLI exactly matches the version in go.mod and version.go used during build
- **SC-007**: All release branches contain exactly one commit (the version bump commit) compared to their base branch
- **SC-008**: All built binaries execute successfully on their target platforms and correctly report their version
- **SC-009**: Version bump script never creates duplicate release branches or overwrites existing version tags

## Assumptions

- **A-001**: Repository uses semantic versioning (MAJOR.MINOR.PATCH) with optional pre-release tags
- **A-002**: Maintainers have push access to the remote repository
- **A-003**: GitHub Actions has necessary permissions to create and attach release artifacts
- **A-004**: Version will be stored in both go.mod and a generated version.go file at a standard location in the repository
- **A-005**: Release process follows a workflow: version bump → PR → merge to main → create GitHub release → automated builds
- **A-006**: Binary size limits for GitHub release attachments are sufficient (typically 2GB per file)
- **A-007**: The version bump script is idempotent - running it multiple times with the same version is safe
- **A-008**: Cross-compilation for Linux from GitHub Actions runners is supported by Go toolchain

## Dependencies

- **D-001**: Git installed and configured on maintainer's machine for running version bump script
- **D-002**: Remote repository access (GitHub) with appropriate authentication
- **D-003**: GitHub Actions enabled for the repository
- **D-004**: Go toolchain (1.25.5) available in GitHub Actions environment for builds

## Out of Scope

- Automated version number determination (semantic auto-increment based on commit messages)
- Building binaries for Windows platform (Linux only)
- Windows version of bump-version script (Bash script for Unix-like systems only)
- Signing or notarizing binaries
- Publishing binaries to package managers (Homebrew, apt, etc.)
- Automated testing of binaries on target platforms within the workflow
- Rollback mechanism for releases or version bumps
- Changelog generation from commit history
