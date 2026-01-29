# frozenDB Scripts

This directory contains maintenance and release scripts for frozenDB.

## bump-version.sh

Automates the version bumping process for frozenDB releases.

### Purpose

This script:
1. Validates the provided semantic version format
2. Checks for uncommitted changes in your working directory
3. Generates `cmd/frozendb/version.go` with the version constant
4. Creates a release branch named `release/{version}`
5. Commits the changes
6. Pushes the branch to the remote repository

Note: The project version is tracked via the version.go file (for runtime CLI access) and git tags (for Go module versioning). The go.mod file is not modified by this script, following Go module conventions.

### Usage

#### Direct invocation:

```bash
./scripts/bump-version.sh v0.1.0
```

#### Via Makefile:

```bash
make bump-version VERSION=v0.1.0
```

### Version Format

Versions must follow semantic versioning with a `v` prefix:
- **Format**: `vMAJOR.MINOR.PATCH[-PRERELEASE]`
- **Examples**: 
  - `v0.1.0` - Initial release
  - `v1.2.3` - Standard version
  - `v2.0.0-rc1` - Release candidate
  - `v1.0.0-beta.2` - Beta version

### Workflow

After running the script:

1. **Create Pull Request**: Create a PR from the `release/v0.1.0` branch to `main`
2. **Review Changes**: Review the version update in `version.go`
3. **Merge PR**: Merge the PR to `main`
4. **Create GitHub Release**: 
   - Go to GitHub Releases
   - Create a new release with tag `v0.1.0` (this is the canonical version for Go modules)
   - Publish the release
5. **Automated Build**: GitHub Actions will automatically build binaries for all platforms

### Error Handling

The script provides clear error messages and exits cleanly if:
- Version format is invalid
- Release branch already exists
- Git operations fail
- File generation fails

If the push to remote fails, the script will inform you that local changes are preserved and you can retry the push manually.

### Development Builds

When building without running the version bump script, binaries will display `(development)` as the version:

```bash
$ go build -o frozendb ./cmd/frozendb
$ ./frozendb version
frozendb (development)
```

### Requirements

- Bash shell (Linux, macOS, or WSL on Windows)
- Git repository with at least one commit
- Go 1.25.5 or later
- Write access to remote repository (for push)

### Testing

The script is validated by spec tests in `scripts_spec_test/bump_version_spec_test.go`.

Run the tests with:

```bash
go test -v ./scripts_spec_test
```

### Examples

#### Successful version bump:

```bash
$ ./scripts/bump-version.sh v0.1.0
Checking version format...
Version format valid: v0.1.0
Checking for uncommitted changes...
Working directory is clean
Current branch: main
No remote 'origin' configured (this is OK for local testing)
Creating release branch: release/v0.1.0
Created branch: release/v0.1.0
Generating cmd/frozendb/version.go...
Generated cmd/frozendb/version.go
Staging changes...
Committing changes...
Committed: Bump version to v0.1.0
Pushing to remote...
Pushed branch to remote: release/v0.1.0

Successfully created release branch: release/v0.1.0

Next steps:
  1. Create a pull request from release/v0.1.0 to main
  2. Review and merge the PR
  3. Create a GitHub release from main with tag v0.1.0
  4. Automated builds will attach binaries to the release
```

#### Invalid version format:

```bash
$ ./scripts/bump-version.sh 1.0.0
Checking version format...
Error: Invalid version format '1.0.0'
Version must follow pattern: vMAJOR.MINOR.PATCH[-PRERELEASE]
Examples: v0.1.0, v1.2.3, v2.0.0-rc1
```

#### With uncommitted changes:

```bash
$ ./scripts/bump-version.sh v0.2.0
Checking version format...
Version format valid: v0.2.0
Checking for uncommitted changes...
Warning: You have uncommitted changes in your working directory
Warning: It's recommended to commit or stash them before bumping the version

Uncommitted changes:
 M cmd/frozendb/main.go

Press Enter to continue anyway, or Ctrl+C to cancel
```
