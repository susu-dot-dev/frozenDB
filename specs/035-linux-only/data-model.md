# Data Model: Linux-Only Platform Restriction

**Branch**: `035-linux-only` | **Date**: 2026-01-30

## Overview

This document describes the data structures and entities affected by the Linux-only platform restriction. Since this is primarily a documentation and configuration change, the "data model" focuses on the structure of configuration files, documentation entities, and test artifacts that must be modified.

## Entity Definitions

### 1. Platform Matrix Entry

**Description**: A configuration entry in GitHub Actions workflow defining a target build platform.

**Attributes**:
- `os` (string): Operating system identifier ("linux" or "darwin")
- `arch` (string): CPU architecture ("amd64" or "arm64")

**Current State (S_033)**:
```yaml
matrix:
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

**Target State (S_035)**:
```yaml
matrix:
  include:
    - os: linux
      arch: amd64
    - os: linux
      arch: arm64
```

**Validation Rules**:
- PM-001: Matrix MUST contain exactly 2 entries (FR-001)
- PM-002: Matrix MUST NOT contain any entries with `os: darwin` (FR-002)
- PM-003: Matrix MUST contain entries for linux/amd64 and linux/arm64 (FR-001)

**State Transition**: 4 platform entries → 2 platform entries (removal of darwin entries)

---

### 2. Functional Requirement Entity

**Description**: A documented requirement in a specification file.

**Attributes**:
- `spec_id` (string): Specification identifier (e.g., "S_033")
- `requirement_id` (string): Requirement identifier (e.g., "FR-011")
- `status` (enum): "active", "obsolete", "superseded"
- `content` (string): Requirement text
- `superseded_by` (string, optional): Reference to superseding spec

**Target Entity (S_033 FR-011)**:
- `spec_id`: "S_033"
- `requirement_id`: "FR-011"
- `status`: "obsolete" or "superseded"
- `superseded_by`: "S_035"
- Original content: "GitHub Actions workflow MUST build frozendb CLI binaries for macOS (darwin/amd64 and darwin/arm64)"

**Validation Rules**:
- FR-001: Obsolete requirements MUST be marked with strikethrough or removed entirely (FR-003)
- FR-002: Superseded requirements SHOULD reference the superseding spec (FR-003)

**State Transition**: active → obsolete/superseded

---

### 3. Spec Test Function

**Description**: A Go test function validating a functional requirement.

**Attributes**:
- `function_name` (string): Following pattern `Test_S_XXX_FR_YYY_Description`
- `spec_id` (string): Specification identifier
- `requirement_id` (string): Functional requirement identifier
- `skip_status` (boolean): Whether test calls `t.Skip()`
- `skip_message` (string, optional): Message explaining why test is skipped
- `location` (string): File path relative to repository root

**Entities to Remove/Update**:

1. **Test_S_033_FR_011_WorkflowBuildsMacOSBinaries**
   - `spec_id`: "S_033"
   - `requirement_id`: "FR-011"
   - `location`: "cmd/frozendb/cli_spec_test.go"
   - **Action**: Remove function entirely (FR-005)

**New Entities to Create**:

1. **Test_S_035_FR_001_WorkflowBuildsLinuxOnly**
   - `spec_id`: "S_035"
   - `requirement_id`: "FR-001"
   - `skip_status`: true
   - `skip_message`: "GitHub Actions workflows are manually tested"
   - `location`: "cmd/frozendb/cli_spec_test.go"

2. **Test_S_035_FR_002_WorkflowNoDarwinBuilds**
   - `spec_id`: "S_035"
   - `requirement_id`: "FR-002"
   - `skip_status`: true
   - `skip_message`: "GitHub Actions workflows are manually tested"
   - `location`: "cmd/frozendb/cli_spec_test.go"

3. **Test_S_035_FR_003_S033FR011Removed**
   - `spec_id`: "S_035"
   - `requirement_id`: "FR-003"
   - `skip_status`: true
   - `skip_message`: "Documentation and spec updates are manually verified"
   - `location`: "cmd/frozendb/cli_spec_test.go"

4. **Test_S_035_FR_004_DocumentationLinuxOnly**
   - `spec_id`: "S_035"
   - `requirement_id`: "FR-004"
   - `skip_status`: true
   - `skip_message`: "Documentation and spec updates are manually verified"
   - `location`: "cmd/frozendb/cli_spec_test.go"

5. **Test_S_035_FR_005_S033MacOSTestsRemoved**
   - `spec_id`: "S_035"
   - `requirement_id`: "FR-005"
   - `skip_status`: true
   - `skip_message`: "Documentation and spec updates are manually verified"
   - `location`: "cmd/frozendb/cli_spec_test.go"

**Validation Rules**:
- ST-001: All spec test function names MUST follow pattern `Test_S_XXX_FR_YYY_Description`
- ST-002: Skipped tests MUST call `t.Skip()` with descriptive message
- ST-003: All functional requirements MUST have corresponding spec tests

---

### 4. Platform Reference Entity

**Description**: A documentation reference to supported platforms in specification files.

**Attributes**:
- `file_path` (string): Path to documentation file
- `line_number` (integer): Line containing platform reference
- `current_text` (string): Current platform reference text
- `target_text` (string): Updated platform reference text
- `priority` (integer): Update priority (1=critical, 4=low)

**Entities to Update** (See research.md for complete list):

**Priority 1 (Critical)**:
- `.github/workflows/release.yml:19-22` - Remove darwin build matrix entries
- `specs/033-release-scripts/spec.md:81` - Mark FR-011 obsolete
- `cmd/frozendb/cli_spec_test.go:858-864` - Remove Test_S_033_FR_011

**Priority 2 (S_033 Supporting Documentation)**:
- `specs/033-release-scripts/contracts/api.md` - 3 locations
- `specs/033-release-scripts/data-model.md` - 8 locations
- `specs/033-release-scripts/research.md` - 9 locations
- `specs/033-release-scripts/plan.md` - 1 location

**Priority 3 (Other Specs)**:
- `specs/034-frozendb-verify/plan.md:18` - Update target platform
- `specs/034-frozendb-verify/contracts/api.md:149` - Update platform support
- `specs/029-cli-implementation/plan.md:18` - Update target platform
- `specs/029-cli-implementation/contracts/api.md:677` - Update platform requirements
- `specs/028-pkg-internal-cli-refactor/plan.md:18` - Update target platform
- `specs/028-pkg-internal-cli-refactor/data-model.md:113` - Remove darwin reference

**Priority 4 (Scripts Documentation)**:
- `scripts/README.md:80` - Update platform requirements

**Validation Rules**:
- PR-001: Priority 1 files MUST be updated for spec completion (FR-001, FR-002, FR-003, FR-005)
- PR-002: Priority 2-4 files SHOULD be updated for consistency (FR-004)
- PR-003: All updated references MUST consistently indicate "Linux (amd64, arm64)" or equivalent

---

## Data Flow

### Update Flow for Platform Matrix

```
Current State (release.yml)
    │
    ├─ darwin/amd64 ──────┐
    ├─ darwin/arm64 ──────┤ REMOVE (FR-001, FR-002)
    ├─ linux/amd64 ───────┤ KEEP
    └─ linux/arm64 ───────┘ KEEP
                           │
                           ▼
Target State (release.yml)
    │
    ├─ linux/amd64 ───────── Build binary → Attach to release
    └─ linux/arm64 ───────── Build binary → Attach to release
```

### Update Flow for Spec Requirements

```
S_033 Spec (spec.md)
    │
    ├─ FR-001 through FR-010 ──── KEEP (unchanged)
    ├─ FR-011 (macOS builds) ───── MARK OBSOLETE or REMOVE (FR-003)
    └─ FR-012 through FR-016 ───── KEEP (unchanged)
                                   │
                                   ▼
S_033 Spec (updated)
    │
    ├─ FR-001 through FR-010 ──── Active
    ├─ FR-011 ────────────────── Obsolete (superseded by S_035)
    └─ FR-012 through FR-016 ───── Active
```

### Update Flow for Spec Tests

```
cli_spec_test.go (current)
    │
    ├─ Test_S_033_FR_010_... ──── KEEP
    ├─ Test_S_033_FR_011_... ──── REMOVE (FR-005)
    ├─ Test_S_033_FR_012_... ──── KEEP
    │                             │
    │                             ▼
    │                    cli_spec_test.go (updated)
    │                             │
    │                             ├─ Test_S_033_FR_010_... ── Existing test
    │                             ├─ Test_S_033_FR_012_... ── Existing test
    │                             └─ Test_S_035_FR_001_... ── New (skip)
    │                                Test_S_035_FR_002_... ── New (skip)
    │                                Test_S_035_FR_003_... ── New (skip)
    │                                Test_S_035_FR_004_... ── New (skip)
    │                                Test_S_035_FR_005_... ── New (skip)
```

## Error Conditions

### EC-001: Incomplete Matrix Removal
**Condition**: Darwin entries remain in GitHub Actions workflow after update
**Detection**: Manual workflow inspection or release build includes darwin binaries
**Handling**: Build should be manually stopped; workflow must be corrected

### EC-002: FR-011 Still Active
**Condition**: S_033 FR-011 remains marked as active requirement
**Detection**: Manual spec file review
**Handling**: Mark as obsolete or remove per FR-003

### EC-003: Spec Test Not Removed
**Condition**: Test_S_033_FR_011_WorkflowBuildsMacOSBinaries still exists in test file
**Detection**: Running spec tests may show passing test for obsolete requirement
**Handling**: Remove test function per FR-005

### EC-004: Inconsistent Documentation
**Condition**: Some documentation files reference macOS while others indicate Linux-only
**Detection**: Manual documentation review or developer confusion
**Handling**: Update all Priority 2-4 files per FR-004

## Relationships

### Spec to Workflow Relationship
```
S_033 Spec
    ├─ Defines: FR-011 (macOS builds)
    │   └─ Implemented by: .github/workflows/release.yml (darwin matrix entries)
    │       └─ Validated by: Test_S_033_FR_011_WorkflowBuildsMacOSBinaries
    │
    └─ Superseded by: S_035 Spec
        ├─ Defines: FR-001, FR-002 (Linux-only builds)
        │   └─ Implemented by: .github/workflows/release.yml (linux-only matrix)
        │       └─ Validated by: Test_S_035_FR_001, Test_S_035_FR_002 (skipped)
        │
        └─ Defines: FR-003, FR-004, FR-005 (Documentation updates)
            └─ Validated by: Test_S_035_FR_003, FR_004, FR_005 (skipped)
```

### Documentation Consistency Relationship
```
Primary Sources (Priority 1)
    ├─ .github/workflows/release.yml
    ├─ specs/033-release-scripts/spec.md
    └─ cmd/frozendb/cli_spec_test.go
         │
         └─ Must be consistent with ─────────┐
                                             │
Secondary Sources (Priority 2-4)           │
    ├─ specs/033-release-scripts/*         │
    ├─ specs/034-frozendb-verify/*         │
    ├─ specs/029-cli-implementation/*       │
    ├─ specs/028-pkg-internal-cli-refactor/* │
    └─ scripts/README.md ───────────────────┘
```

## Summary

This specification affects 4 primary entity types:

1. **Platform Matrix Entries** (1 file, 2 entries removed)
2. **Functional Requirements** (1 requirement marked obsolete)
3. **Spec Test Functions** (1 removed, 5 added as skipped)
4. **Platform References** (14 documentation files updated)

All changes maintain data integrity by ensuring consistency across documentation, tests, and configuration files.
