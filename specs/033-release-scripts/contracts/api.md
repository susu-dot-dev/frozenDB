# API Contracts: Release Scripts & Version Management

**Feature**: 033-release-scripts  
**Date**: 2026-01-29  
**Purpose**: Define interfaces for version bump script, CLI version command, and build workflows

## Overview

This document specifies the contracts (interfaces) for the release management infrastructure. Since this feature involves scripts, CLI commands, and CI/CD workflows rather than traditional APIs, the "contracts" are command-line interfaces, script parameters, and GitHub Actions workflow specifications.

## 1. Version Bump Script Contract

### Script: `scripts/bump-version.sh`

**Purpose**: Update version in repository files, create release branch, commit, and push

**Invocation**:
```bash
./scripts/bump-version.sh <version>
```

**Parameters**:
- `<version>` (required): Semantic version string with 'v' prefix (e.g., "v0.1.0", "v1.2.3-rc1")

**Exit Codes**:
- `0`: Success - version bumped, branch created, committed, and pushed
- `1`: Error - validation failed, git operation failed, or file operation failed

**Side Effects**:
1. Creates/overwrites `cmd/frozendb/version.go` with new version constant
2. Creates new git branch `release/{version}`
3. Commits changes with message "Bump version to {version}"
4. Pushes branch to remote repository

**Standard Output**:
```
Checking version format...
Version format valid: v0.1.0
Checking for uncommitted changes...
Working directory is clean
Creating release branch: release/v0.1.0
Generating cmd/frozendb/version.go...
Committing changes...
Pushing to remote...
Successfully created release branch: release/v0.1.0
Next steps:
  1. Create a pull request from release/v0.1.0 to main
  2. Review and merge the PR
  3. Create a GitHub release from main with tag v0.1.0
```

**Standard Error (on failure)**:
```
Error: Invalid version format 'v1.2'
Version must follow pattern: vMAJOR.MINOR.PATCH[-PRERELEASE]
Examples: v0.1.0, v1.2.3, v2.0.0-rc1
```

**Validation Behaviors**:
- Validates version format before any file modifications
- Checks for existing branch (local and remote) before creating
- Warns about uncommitted changes, waits for user confirmation
- Fails fast on any error, leaves repository in safe state

**Idempotency**: Running script multiple times with same version will fail (branch already exists) unless user cleans up first. Running with different version is safe.

## 2. CLI Version Command Contract

### Command: `frozendb version`

**Purpose**: Display the version of the installed frozendb CLI

**Invocation**:
```bash
frozendb version
```

**Alternative**: `frozendb --version`

**Parameters**: None

**Exit Code**: `0` (always succeeds)

**Standard Output**:
```
frozendb v0.1.0
```

**Format**: `frozendb <version>`
- Program name: "frozendb"
- Single space separator
- Version: Semantic version with 'v' prefix

**Special Cases**:
- If built without version.go (development build): `frozendb (development)`
- If version.go exists but Version constant is empty: `frozendb (unknown)`

**Integration with Existing Commands**:
```go
// Pseudo-code integration in cmd/frozendb/main.go
func main() {
    if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
        fmt.Printf("frozendb %s\n", getVersion())
        os.Exit(0)
    }
    // ... existing command routing for create, begin, commit, etc.
}

func getVersion() string {
    if Version == "" {
        return "(development)"
    }
    return Version
}
```

**Performance**: <100ms execution time (no I/O, just prints constant)

**Thread Safety**: N/A (single-threaded CLI command)

**Compatibility**: Works with all existing frozendb commands, doesn't interfere with any existing functionality

## 3. GitHub Actions Release Workflow Contract

### Workflow: `.github/workflows/release.yml`

**Purpose**: Automatically build multi-platform binaries when a release is published

**Trigger**:
```yaml
on:
  release:
    types: [published]
```

**Event**: Triggered when a GitHub release is published (manual action by maintainer)

**Inputs**: None (workflow reads release tag from GitHub context)

**Environment**:
- Runner: `ubuntu-latest` (can cross-compile for all platforms)
- Go version: `1.25.5`
- GitHub token: Automatically provided by GitHub Actions

**Build Matrix**:
```yaml
strategy:
  matrix:
    include:
      - os: linux
        arch: amd64
      - os: linux
        arch: arm64
```

**Build Steps**:
1. Checkout repository at release tag
2. Setup Go 1.25.5
3. For each platform/architecture:
   - Set GOOS and GOARCH environment variables
   - Run `go build -o dist/frozendb-{os}-{arch} ./cmd/frozendb`
4. Attach all binaries to the GitHub release

**Output Artifacts**:
- `frozendb-linux-amd64` - Linux x86_64 binary
- `frozendb-linux-arm64` - Linux ARM64 binary

**Success Criteria**:
- All two builds complete without errors
- All binaries successfully attached to release
- Workflow completes within 10 minutes

**Failure Handling**:
- If any build fails, entire workflow fails (fail-fast: false by default)
- Workflow logs display compilation errors
- Release remains published but without complete set of binaries

**Artifact Naming Convention**: `frozendb-{GOOS}-{GOARCH}`

**No Signing/Notarization**: Binaries are not signed (out of scope for this feature)

## 4. Integration with Existing CLI Architecture

### Current CLI Structure (cmd/frozendb/main.go)

**Existing Command Router**:
```go
func main() {
    if len(os.Args) < 2 {
        // Print usage and exit
    }
    
    subcommand := os.Args[1]
    switch subcommand {
    case "create":
        handleCreate()
    case "begin":
        handleBegin()
    case "commit":
        handleCommit()
    // ... other commands
    }
}
```

**Integration Point**: Version command is checked BEFORE the existing router:

```go
func main() {
    // NEW: Check for version command first
    if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
        handleVersion()
    }
    
    // EXISTING: Original command routing
    if len(os.Args) < 2 {
        // Print usage (now includes 'version' command)
    }
    
    subcommand := os.Args[1]
    switch subcommand {
    // ... existing cases
    }
}
```

**Rationale**: Checking version first allows `--version` flag to work as expected in CLI conventions, and prevents confusion with existing command structure.

**No Breaking Changes**: All existing commands work exactly as before. Version command is purely additive.

## 5. Version.go File Contract

### File: `cmd/frozendb/version.go`

**Purpose**: Provide compile-time version constant for CLI

**Package**: `main` (same as cmd/frozendb/main.go)

**Generated By**: `scripts/bump-version.sh`

**File Format**:
```go
// version.go - Generated by scripts/bump-version.sh
// DO NOT EDIT: This file is automatically generated
package main

const Version = "v0.1.0"
```

**Contract Requirements**:
- MUST be valid Go source code that compiles
- MUST declare `package main`
- MUST export constant named `Version` of type `string`
- Value MUST follow semantic versioning format
- MUST include comment warning about auto-generation

**Access Pattern**:
```go
// In main.go or other files in main package
func handleVersion() {
    fmt.Printf("frozendb %s\n", Version)
}
```

**Compilation**: File is compiled into binary like any other Go source file. No special build tags or flags required.

**Version Control**: File IS committed to git (not .gitignored), so version is visible in source history

## 6. Spec Test Contracts

### Test: `scripts_spec_test/bump_version_spec_test.go`

**Purpose**: Validate version bump script functionality

**Package**: `scripts_spec_test`

**Test Functions** (per spec requirements):
- `Test_S_033_FR_001_ScriptUpdatesGoModAndVersionFile`
- `Test_S_033_FR_002_ScriptCreatesReleaseBranch`
- `Test_S_033_FR_003_ScriptCommitsChanges`
- `Test_S_033_FR_004_ScriptPushesBranch`
- `Test_S_033_FR_005_ScriptReportsErrorsClearly`
- `Test_S_033_FR_009_VersionEmbeddedInBinary`
- `Test_S_033_FR_015_ScriptValidatesSemanticVersioning`
- `Test_S_033_FR_016_ScriptChecksUncommittedChanges`

**Test Approach**: Each test creates a temporary git repository, runs the script, and verifies expected outcomes (files created, branches exist, commits made).

**Challenges**: Testing git push requires mock remote or skip. Tests will use local operations where possible.

### Test: `cmd/frozendb/main_spec_test.go`

**Purpose**: Validate CLI version command functionality

**Package**: `main` (test file in same package as cmd/frozendb/main.go)

**Test Functions** (per spec requirements):
- `Test_S_033_FR_006_CLIVersionSubcommand`
- `Test_S_033_FR_007_CLIVersionFlag`
- `Test_S_033_FR_008_VersionOutputFormat`

**Test Approach**: Build binary with known version, execute it with `version` subcommand and `--version` flag, verify output format.

### Test: `cmd/frozendb/main_spec_test.go` (GitHub Actions Requirements)

**Purpose**: Document GitHub Actions workflow requirements (but skip actual tests)

**Test Functions** (per spec requirements):
- `Test_S_033_FR_010_GitHubWorkflowTriggersOnRelease`
- `Test_S_033_FR_012_WorkflowBuildsLinuxBinaries`
- `Test_S_033_FR_013_WorkflowAttachesBinariesToRelease`
- `Test_S_033_FR_014_BinaryArtifactsNamedCorrectly`

**Implementation**: Each test immediately calls `t.Skip("GitHub Actions workflows are manually tested")` as specified in the spec.

**Rationale**: Documents requirements while acknowledging manual testing approach for CI/CD workflows.

## 7. Error Handling Contracts

### Script Error Responses

**Format**: All errors written to stderr, exit code 1

**Error Categories**:

1. **Validation Errors** (before any changes):
   ```
   Error: Invalid version format '{version}'
   Version must follow pattern: vMAJOR.MINOR.PATCH[-PRERELEASE]
   ```

2. **State Errors** (repository state issues):
   ```
   Error: Branch 'release/v0.1.0' already exists
   Please check: git branch -a | grep release/v0.1.0
   ```

3. **Operation Errors** (git/file operations):
   ```
   Error: Failed to create branch
   Git error: [git error message]
   ```

4. **Remote Errors** (push failures):
   ```
   Error: Failed to push to remote
   Local changes committed to: release/v0.1.0
   You can retry with: git push origin release/v0.1.0
   ```

**Consistency**: All errors include "Error:" prefix and actionable information

### CLI Error Responses

**Version Command**: Never errors (worst case displays "(development)")

**Other Commands**: Existing error handling unchanged (see cmd/frozendb/errors.go)

## 8. Compatibility and Integration Notes

### Backwards Compatibility

**Existing Functionality**: No changes to existing CLI commands or workflows
- All database operations work exactly as before
- Existing build process (make build-cli) works unchanged
- CI workflow continues to test all code

**Additive Only**: This feature only adds new functionality:
- New `version` command and `--version` flag
- New `scripts/` directory with bump-version script
- New `.github/workflows/release.yml` workflow
- New `cmd/frozendb/version.go` file (ignored by existing code)

### Forward Compatibility

**Version File Updates**: Script can be run multiple times with different versions
- Old version.go is overwritten
- Each release branch has its own version

**Workflow Evolution**: GitHub Actions workflow can be enhanced later:
- Add binary signing (currently out of scope)
- Add automated testing of binaries (currently out of scope)
- Add publishing to package managers (currently out of scope)

### Dependencies

**No New External Dependencies**:
- Script uses standard Unix tools: bash, git, sed
- CLI uses existing Go standard library
- GitHub Actions uses standard actions only

**Go Module**: Existing `go.mod` already has required dependencies (none new)

### Performance Characteristics

**Version Bump Script**: 
- Time: <2 minutes (spec requirement SC-001)
- I/O: Minimal (2 file writes, standard git operations)
- Network: Single push operation

**CLI Version Command**:
- Time: <100ms (spec requirement SC-002)
- I/O: None (constant is in memory)
- CPU: Negligible (single printf)

**GitHub Actions Workflow**:
- Time: <10 minutes for all builds (spec requirement SC-003)
- Parallelism: 4 builds run concurrently
- Resources: Standard GitHub Actions runner limits

### Testing Strategy

**Unit Tests**: Not applicable (infrastructure feature)

**Spec Tests**: Per specification requirements
- Scripts: Test in temporary git repositories
- CLI: Test built binaries
- Workflows: Manual testing (t.Skip in code)

**Integration Tests**: Manual end-to-end validation
1. Run bump-version script
2. Create PR and merge
3. Create GitHub release
4. Verify binaries build and attach
5. Download binary and run `frozendb version`

**Regression Prevention**: Spec tests ensure version functionality remains correct across changes
