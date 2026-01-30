# API Contract: Linux-Only Platform Restriction

**Branch**: `035-linux-only` | **Date**: 2026-01-30

## Overview

This document specifies the "API" for the Linux-only platform restriction feature. Since this is primarily a documentation and configuration change rather than a programmatic API, this contract defines:

1. The structure of updated configuration files
2. The format of spec test functions
3. The conventions for marking requirements obsolete
4. Success/failure validation criteria

## 1. GitHub Actions Workflow Matrix API

### Contract: release.yml Build Matrix

**Location**: `.github/workflows/release.yml`

**Purpose**: Define platform targets for automated release builds.

**Input Format**:
```yaml
strategy:
  matrix:
    include:
      - os: <string>      # Operating system: "linux" only
        arch: <string>     # Architecture: "amd64" or "arm64"
```

**Required Configuration (FR-001, FR-002)**:
```yaml
strategy:
  matrix:
    include:
      - os: linux
        arch: amd64
      - os: linux
        arch: arm64
```

**Forbidden Configuration (FR-002)**:
```yaml
# MUST NOT contain:
- os: darwin
  arch: amd64
- os: darwin
  arch: arm64
```

**Success Criteria**:
- Matrix contains exactly 2 entries (SC-005)
- Both entries have `os: linux` (FR-001)
- No entries have `os: darwin` (FR-002)
- Architectures are "amd64" and "arm64" (FR-001)

**Failure Conditions**:
- Matrix contains darwin/macOS entries
- Matrix missing linux/amd64 or linux/arm64 entries
- Matrix contains platforms other than linux

**Validation**: Manual inspection of release workflow file and verification of release build artifacts containing only linux binaries.

---

## 2. Spec Requirement Obsolescence API

### Contract: Marking Requirements Obsolete

**Location**: `specs/033-release-scripts/spec.md`

**Purpose**: Mark FR-011 (macOS build requirement) as obsolete per FR-003.

**Current State**:
```markdown
- **FR-011**: GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)
```

**Target State (Option A - Strikethrough with Note)**:
```markdown
- **FR-011**: ~~GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)~~ **OBSOLETE**: Superseded by S_035 (Linux-only platform restriction)
```

**Target State (Option B - Complete Removal)**:
```markdown
[Remove FR-011 entirely and renumber subsequent requirements if needed]
```

**Recommended Approach**: Option A (strikethrough with note) to maintain requirement traceability and historical context.

**Success Criteria**:
- FR-011 is no longer an active requirement (SC-006)
- Change is documented with reference to S_035
- Other S_033 requirements remain intact

**Failure Conditions**:
- FR-011 remains active without obsolescence marking
- Renumbering breaks references to other S_033 requirements
- Obsolescence not documented with reason

**Validation**: Manual inspection of S_033 spec.md file.

---

## 3. Spec Test Function API

### Contract: Test Function Structure

**Location**: `cmd/frozendb/cli_spec_test.go`

**Purpose**: Create spec tests for FR-001 through FR-005, all using `t.Skip()` per spec exceptions.

### 3.1 Test_S_035_FR_001_WorkflowBuildsLinuxOnly

**Function Signature**:
```go
func Test_S_035_FR_001_WorkflowBuildsLinuxOnly(t *testing.T)
```

**Purpose**: Verify GitHub Actions workflow builds only Linux binaries (FR-001).

**Implementation** (Required per spec exceptions):
```go
func Test_S_035_FR_001_WorkflowBuildsLinuxOnly(t *testing.T) {
    t.Skip("GitHub Actions workflows are manually tested")
}
```

**Comment Header** (Recommended):
```go
// Test_S_035_FR_001_WorkflowBuildsLinuxOnly verifies that the release workflow
// builds binaries only for Linux platforms (linux/amd64 and linux/arm64)
//
// Functional Requirement FR-001: GitHub Actions release workflow MUST build
// binaries only for Linux (linux/amd64 and linux/arm64)
//
// Success Criteria SC-001: Release workflow completes in under 5 minutes
// Success Criteria SC-005: Workflow configuration contains exactly 2 platform entries
```

---

### 3.2 Test_S_035_FR_002_WorkflowNoDarwinBuilds

**Function Signature**:
```go
func Test_S_035_FR_002_WorkflowNoDarwinBuilds(t *testing.T)
```

**Purpose**: Verify GitHub Actions workflow does not build macOS/darwin binaries (FR-002).

**Implementation**:
```go
func Test_S_035_FR_002_WorkflowNoDarwinBuilds(t *testing.T) {
    t.Skip("GitHub Actions workflows are manually tested")
}
```

**Comment Header** (Recommended):
```go
// Test_S_035_FR_002_WorkflowNoDarwinBuilds verifies that the release workflow
// does not build macOS (darwin) binaries for any architecture
//
// Functional Requirement FR-002: GitHub Actions release workflow MUST NOT build
// binaries for macOS (darwin) or any other non-Linux platforms
//
// Success Criteria SC-003: Zero macOS-related build failures occur
// Success Criteria SC-005: Workflow configuration contains no darwin entries
```

---

### 3.3 Test_S_035_FR_003_S033FR011Removed

**Function Signature**:
```go
func Test_S_035_FR_003_S033FR011Removed(t *testing.T)
```

**Purpose**: Verify S_033 FR-011 is marked obsolete or removed (FR-003).

**Implementation**:
```go
func Test_S_035_FR_003_S033FR011Removed(t *testing.T) {
    t.Skip("Documentation and spec updates are manually verified")
}
```

**Comment Header** (Recommended):
```go
// Test_S_035_FR_003_S033FR011Removed verifies that Spec S_033 functional
// requirement FR-011 (macOS builds) has been removed or marked as obsolete
//
// Functional Requirement FR-003: Spec S_033 functional requirements FR-011
// (macOS darwin/amd64 and darwin/arm64 builds) MUST be removed or marked as obsolete
//
// Success Criteria SC-006: Spec S_033 contains no active requirements for macOS builds
```

---

### 3.4 Test_S_035_FR_004_DocumentationLinuxOnly

**Function Signature**:
```go
func Test_S_035_FR_004_DocumentationLinuxOnly(t *testing.T)
```

**Purpose**: Verify documentation references indicate Linux-only support (FR-004).

**Implementation**:
```go
func Test_S_035_FR_004_DocumentationLinuxOnly(t *testing.T) {
    t.Skip("Documentation and spec updates are manually verified")
}
```

**Comment Header** (Recommended):
```go
// Test_S_035_FR_004_DocumentationLinuxOnly verifies that documentation and spec
// files consistently reference Linux-only platform support with no cross-platform claims
//
// Functional Requirement FR-004: Documentation and spec files referencing
// cross-platform support MUST be updated to indicate Linux-only support
//
// Success Criteria SC-004: All documentation consistently references Linux-only support
```

---

### 3.5 Test_S_035_FR_005_S033MacOSTestsRemoved

**Function Signature**:
```go
func Test_S_035_FR_005_S033MacOSTestsRemoved(t *testing.T)
```

**Purpose**: Verify S_033 macOS-related spec tests are removed (FR-005).

**Implementation**:
```go
func Test_S_035_FR_005_S033MacOSTestsRemoved(t *testing.T) {
    t.Skip("Documentation and spec updates are manually verified")
}
```

**Comment Header** (Recommended):
```go
// Test_S_035_FR_005_S033MacOSTestsRemoved verifies that spec tests for S_033
// FR-011 (macOS build requirements) have been removed or updated
//
// Functional Requirement FR-005: Spec tests for S_033 FR-011 (macOS build
// requirements) MUST be removed or updated to reflect Linux-only builds
//
// Success Criteria SC-007: All spec tests related to macOS builds are removed
// or properly skipped with documentation
```

---

### 3.6 Removal: Test_S_033_FR_011_WorkflowBuildsMacOSBinaries

**Current Location**: `cmd/frozendb/cli_spec_test.go` (lines ~858-864)

**Action Required (FR-005)**: Remove this test function entirely.

**Rationale**: This test validates S_033 FR-011 which is now obsolete. Keeping the test would create confusion about platform requirements.

**Success Criteria**:
- Function `Test_S_033_FR_011_WorkflowBuildsMacOSBinaries` does not exist in cli_spec_test.go (SC-007)
- No references to darwin/macOS builds remain in spec test file comments (SC-007)

---

## 4. Documentation Update API

### Contract: Platform Reference Format

**Purpose**: Establish consistent format for platform references across documentation (FR-004).

### 4.1 Target Platform Field

**Current Formats** (found in various specs):
```markdown
**Target Platform**: Linux/macOS/Windows (cross-platform)
**Target Platform**: Linux/macOS/Unix systems  
**Target Platform**: Linux (primary), macOS/Unix-like systems
```

**Standardized Format** (S_035 forward):
```markdown
**Target Platform**: Linux (amd64, arm64)
```

**Locations to Update**:
- `specs/034-frozendb-verify/plan.md:18`
- `specs/029-cli-implementation/plan.md:18`
- `specs/028-pkg-internal-cli-refactor/plan.md:18`

### 4.2 Platform Lists

**Current Format**:
```markdown
- darwin/amd64
- darwin/arm64
- linux/amd64
- linux/arm64
```

**Target Format**:
```markdown
- linux/amd64
- linux/arm64
```

**Locations to Update**:
- `specs/033-release-scripts/contracts/api.md:169-170`
- `specs/033-release-scripts/data-model.md:98-99`

### 4.3 Platform Support Statements

**Current Format**:
```markdown
**Platform Support**: Works on all platforms supporting Go standard library (Linux, macOS, Windows, etc.)
```

**Target Format**:
```markdown
**Platform Support**: Linux (amd64, arm64)
```

**Locations to Update**:
- `specs/034-frozendb-verify/contracts/api.md:149`

### 4.4 Build Matrix Examples

**Current Format**:
```yaml
include:
  - os: darwin
    arch: amd64
  - os: darwin
    arch: arm64
  - os: linux
    arch: amd64
  - os: linux
    arch: arm64
```

**Target Format**:
```yaml
include:
  - os: linux
    arch: amd64
  - os: linux
    arch: arm64
```

**Locations to Update**:
- `specs/033-release-scripts/contracts/api.md:150-152`

---

## 5. Success Validation API

### Contract: Validation Checklist

**Purpose**: Define how to verify all functional requirements are met.

### 5.1 FR-001 Validation: Linux Builds Only

**Method**: Manual inspection + release verification

**Steps**:
1. Inspect `.github/workflows/release.yml`
2. Verify matrix contains linux/amd64 entry ✓
3. Verify matrix contains linux/arm64 entry ✓
4. Verify total matrix entries = 2 ✓
5. Trigger test release and verify only linux binaries are produced ✓

**Success**: All steps pass
**Failure**: Any step fails

### 5.2 FR-002 Validation: No Darwin Builds

**Method**: Manual inspection + release verification

**Steps**:
1. Inspect `.github/workflows/release.yml`
2. Verify matrix contains no darwin/amd64 entry ✓
3. Verify matrix contains no darwin/arm64 entry ✓
4. Verify no other non-linux OS entries ✓
5. Trigger test release and verify no darwin binaries are produced ✓

**Success**: All steps pass
**Failure**: Any step fails

### 5.3 FR-003 Validation: S_033 FR-011 Obsolete

**Method**: Manual file inspection

**Steps**:
1. Open `specs/033-release-scripts/spec.md`
2. Locate FR-011 section
3. Verify FR-011 is marked with strikethrough or removed ✓
4. Verify obsolescence reason references S_035 ✓

**Success**: FR-011 clearly marked as obsolete with reference to S_035
**Failure**: FR-011 still appears active or obsolescence not documented

### 5.4 FR-004 Validation: Documentation Updated

**Method**: Manual file inspection across 14 files

**Steps**:
1. Review all files listed in research.md Priority 2-4 sections
2. For each file, verify platform references updated to Linux-only
3. Check for consistency in platform naming conventions
4. Verify no conflicting cross-platform claims remain

**Success**: All 14 files consistently reference Linux-only support
**Failure**: Any file still references macOS/darwin or cross-platform support

### 5.5 FR-005 Validation: macOS Tests Removed

**Method**: Code inspection + test execution

**Steps**:
1. Search `cmd/frozendb/cli_spec_test.go` for "Test_S_033_FR_011"
2. Verify function does not exist ✓
3. Search for "darwin" or "macOS" in spec test comments
4. Run `make test-spec` to confirm removed test doesn't execute
5. Verify 5 new Test_S_035_FR_XXX tests exist and skip properly ✓

**Success**: Test_S_033_FR_011 removed, S_035 tests added and skipping
**Failure**: Old test still exists or new tests missing

---

## 6. Integration Notes

### 6.1 Relationship to S_033

This specification modifies artifacts from S_033 (Release Scripts & Version Management):
- Supersedes FR-011 (macOS builds)
- Maintains FR-001 through FR-010, FR-012 through FR-016 unchanged
- Updates supporting documentation to reflect Linux-only builds

### 6.2 Relationship to Release Process

The release process remains unchanged except for platform targets:
1. Maintainer runs version bump script (unchanged)
2. Creates release branch and PR (unchanged)
3. Merges to main (unchanged)
4. Creates GitHub release (unchanged)
5. Workflow triggers and builds binaries (CHANGED: linux-only)
6. Binaries attached to release (CHANGED: 2 binaries instead of 4)

### 6.3 Compatibility

**Breaking Change**: Yes - this removes macOS binary distribution

**Impact**:
- Users on macOS can no longer download pre-built binaries
- Users on macOS attempting manual builds will encounter syscall compilation errors
- Existing macOS binaries from prior releases remain available but will not be updated

**Migration Path**: None - frozenDB is Linux-only going forward

---

## 7. Performance Characteristics

### Build Time Reduction

**Before** (4 platform builds):
- ~10 minutes for complete release workflow

**After** (2 platform builds):  
- ~5 minutes for complete release workflow (SC-001)

**Improvement**: 50% reduction in CI/CD time and resource usage

---

## 8. Testing Approach

All spec tests for this feature use `t.Skip()` per spec exceptions:

**Rationale**:
- FR-001, FR-002: GitHub Actions workflows are manually tested (cannot be automatically verified in unit tests)
- FR-003, FR-004, FR-005: Documentation and spec updates are manually verified (text content validation is manual)

**Manual Testing Checklist**:
1. Create test release after implementation
2. Verify only linux/amd64 and linux/arm64 binaries are built
3. Verify binaries are correctly named (frozendb-linux-amd64, frozendb-linux-arm64)
4. Verify binaries are attached to release
5. Review all 14 documentation files for consistent Linux-only references
6. Verify S_033 FR-011 is marked obsolete
7. Verify Test_S_033_FR_011 is removed from spec tests

---

## Summary

This contract defines the structure and validation approach for transitioning frozenDB to Linux-only platform support. All changes are to configuration, documentation, and test files - no source code changes are required.

**Key Deliverables**:
1. Updated `.github/workflows/release.yml` with 2-entry Linux-only matrix
2. Updated `specs/033-release-scripts/spec.md` with FR-011 marked obsolete
3. 5 new spec test functions in `cmd/frozendb/cli_spec_test.go` (all skipped)
4. Removal of `Test_S_033_FR_011_WorkflowBuildsMacOSBinaries`
5. Updated platform references across 14 documentation files

**Validation**: All requirements validated through manual inspection and test release verification per sections 5.1-5.5.
