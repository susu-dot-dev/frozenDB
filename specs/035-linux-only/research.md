# Research: Linux-Only Platform Restriction

**Branch**: `035-linux-only` | **Date**: 2026-01-30

## Overview

This research document identifies all files in the frozenDB codebase that reference macOS/darwin platform support and must be updated to reflect the Linux-only platform restriction. The project is transitioning away from cross-platform support due to the use of Linux-specific syscalls.

## Decision: Linux-Only Platform Support

**Rationale**:
- frozenDB uses Linux-specific syscalls that cannot be reliably implemented cross-platform
- Maintaining cross-platform compatibility adds unnecessary complexity without current user demand
- Focusing on Linux-only allows for deeper integration with Linux-specific features
- Reduces CI/CD build time and complexity by removing macOS builds

**Alternatives Considered**:
1. **Maintain cross-platform support with platform-specific implementations**: Rejected because syscall abstractions would add significant complexity and testing burden
2. **Add build-time platform checks**: Rejected as out of scope for this change; users attempting macOS builds will encounter natural compilation errors
3. **Keep macOS in documentation but mark as unsupported**: Rejected because it creates confusion and wastes developer time investigating compatibility

## File Inventory

### Critical Files (Priority 1)

These files directly affect build automation and functional requirements:

#### 1. `.github/workflows/release.yml`
- **Current state**: Build matrix includes darwin/amd64 and darwin/arm64 (lines 19-22)
- **Required change**: Remove darwin entries, keep only linux/amd64 and linux/arm64
- **Impact**: Implements FR-001 and FR-002
- **Pattern to remove**:
  ```yaml
  - os: darwin
    arch: amd64
  - os: darwin
    arch: arm64
  ```

#### 2. `specs/033-release-scripts/spec.md`
- **Current state**: FR-011 mandates macOS darwin/amd64 and darwin/arm64 builds (line 81)
- **Required change**: Mark FR-011 as obsolete/superseded by S_035 or remove entirely
- **Impact**: Implements FR-003
- **Additional references**: Lines 6, 44, 52, 81, 139, 151, 153 mention macOS builds

#### 3. `cmd/frozendb/cli_spec_test.go`
- **Current state**: Contains `Test_S_033_FR_011_WorkflowBuildsMacOSBinaries` (lines 858-864)
- **Required change**: Remove test function or update to skip with documentation
- **Impact**: Implements FR-005
- **Additional reference**: Line 885 comments mention darwin-amd64 naming

### Spec 033 Supporting Documentation (Priority 2)

These files document the design and implementation of S_033 release automation:

#### 4. `specs/033-release-scripts/contracts/api.md`
- **References**: Lines 150-152 (build matrix), lines 169-170 (binary names), line 316 (test reference)
- **Required change**: Remove darwin from build matrix examples, update binary artifact lists
- **Impact**: Supports FR-004

#### 5. `specs/033-release-scripts/data-model.md`
- **References**: Lines 98-99 (naming table), 106 (platform definition), 116 (filename pattern), 223-224, 289, 292 (state machine)
- **Required change**: Remove darwin from all data models and state machine diagrams
- **Impact**: Supports FR-004

#### 6. `specs/033-release-scripts/research.md`
- **References**: Lines 70, 163-165, 185, 187, 196-197, 208, 211
- **Required change**: Update decision rationale and platform references to Linux-only
- **Impact**: Supports FR-004

#### 7. `specs/033-release-scripts/plan.md`
- **References**: Line 18 (target platform lists darwin/amd64, darwin/arm64)
- **Required change**: Update target platform to "Linux (amd64, arm64)"
- **Impact**: Supports FR-004

### Other Spec Files (Priority 3)

Recent specs that reference cross-platform support:

#### 8. `specs/034-frozendb-verify/plan.md`
- **References**: Line 18 - "Linux/macOS/Windows (cross-platform)"
- **Required change**: Update to "Linux (amd64, arm64)"
- **Impact**: Maintains consistency across recent specs

#### 9. `specs/034-frozendb-verify/contracts/api.md`
- **References**: Line 149 - Platform support mentions Linux, macOS, Windows
- **Required change**: Update to indicate Linux-only support
- **Impact**: Maintains consistency across recent specs

#### 10. `specs/029-cli-implementation/plan.md`
- **References**: Line 18 - "Linux/macOS/Unix systems"
- **Required change**: Update to "Linux systems"
- **Impact**: Maintains consistency

#### 11. `specs/029-cli-implementation/contracts/api.md`
- **References**: Line 677 - "Requires Linux/macOS/Unix (file locking via syscall)"
- **Required change**: Update to "Requires Linux (file locking via syscall)"
- **Impact**: Accurately reflects syscall limitations

#### 12. `specs/028-pkg-internal-cli-refactor/plan.md`
- **References**: Line 18 - "Linux (primary), macOS/Unix-like systems"
- **Required change**: Update to "Linux"
- **Impact**: Maintains consistency

#### 13. `specs/028-pkg-internal-cli-refactor/data-model.md`
- **References**: Line 113 - "linux/amd64 (primary), darwin/amd64 (secondary)"
- **Required change**: Update to "linux/amd64, linux/arm64"
- **Impact**: Maintains consistency

### Scripts Documentation (Priority 4)

#### 14. `scripts/README.md`
- **References**: Line 80 - "Bash shell (Linux, macOS, or WSL on Windows)"
- **Required change**: Update to "Bash shell (Linux)" or note that scripts are for Linux development only
- **Impact**: Clarifies development environment requirements

## Files Not Requiring Updates

### Historical Specs (Informational Only)

These older specs reference cross-platform support but are historical records:

- `specs/010-null-row-struct/plan.md` (Line 24)
- `specs/007-file-validation/plan.md` (Line 24)
- `specs/003-checksum-row/contracts/checksum_row_api.md` (Line 388)
- `specs/002-open-frozendb/research.md` (Line 178)

**Decision**: Do not update these files as they document historical design decisions. The platform restriction is a forward-looking change.

### Core Code Files

The actual Go source code does not require changes for this spec. Platform-specific syscalls will naturally fail compilation on non-Linux platforms, which is acceptable per assumption A-001 in the spec.

## Update Patterns

### Pattern 1: GitHub Actions Workflow
```yaml
# REMOVE these entries:
- os: darwin
  arch: amd64
- os: darwin
  arch: arm64

# KEEP these entries:
- os: linux
  arch: amd64
- os: linux
  arch: arm64
```

### Pattern 2: Spec Functional Requirements
```markdown
# BEFORE:
- **FR-011**: GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)

# AFTER (Option A - Mark obsolete):
- **FR-011**: ~~GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)~~ **OBSOLETE**: Superseded by S_035 (Linux-only platform restriction)

# AFTER (Option B - Remove entirely):
[Remove the entire FR-011 line]
```

### Pattern 3: Target Platform Documentation
```markdown
# BEFORE:
**Target Platform**: Linux/macOS/Windows (cross-platform)

# AFTER:
**Target Platform**: Linux (amd64, arm64)
```

### Pattern 4: Platform Lists
```markdown
# BEFORE:
- darwin/amd64
- darwin/arm64
- linux/amd64
- linux/arm64

# AFTER:
- linux/amd64
- linux/arm64
```

## Testing Strategy

All five functional requirements will have corresponding spec tests in `cmd/frozendb/cli_spec_test.go`:

- `Test_S_035_FR_001_WorkflowBuildsLinuxOnly` - Skip with message "GitHub Actions workflows are manually tested"
- `Test_S_035_FR_002_WorkflowNoDarwinBuilds` - Skip with message "GitHub Actions workflows are manually tested"
- `Test_S_035_FR_003_S033FR011Removed` - Skip with message "Documentation and spec updates are manually verified"
- `Test_S_035_FR_004_DocumentationLinuxOnly` - Skip with message "Documentation and spec updates are manually verified"
- `Test_S_035_FR_005_S033MacOSTestsRemoved` - Skip with message "Documentation and spec updates are manually verified"

All tests will use `t.Skip()` immediately as they validate infrastructure and documentation changes (per spec exceptions).

## Summary

**Total files requiring updates**: 14

**Primary impact areas**:
1. GitHub Actions workflow configuration (1 file)
2. S_033 specification and documentation (4 files)
3. CLI spec tests (1 file)
4. Recent spec documentation (6 files)
5. Scripts documentation (1 file)
6. New spec tests for S_035 (1 file - adds 5 test functions)

**No code changes required**: The Go source code naturally enforces Linux-only through syscall usage. Users attempting to build on macOS will receive compilation errors, which is acceptable per spec assumptions.
