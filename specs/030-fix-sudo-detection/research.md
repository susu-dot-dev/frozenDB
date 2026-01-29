# Research: Sudo Detection Logic Bug Fix

**Date**: Thu Jan 29 2026  
**Branch**: 030-fix-sudo-detection  
**Spec**: [spec.md](./spec.md)

## Overview

This document consolidates research findings on the sudo detection logic bug in `internal/frozendb/create.go` and defines the approach for fixing it with comprehensive test coverage.

## Bug Analysis

### Current Behavior (Incorrect)

**Location**: `internal/frozendb/create.go:270-284`

```go
func Create(config CreateConfig) error {
    // Step 1: Validate inputs (lines 265-268)
    if err := config.Validate(); err != nil {
        return err
    }

    // Step 2: Check for direct root execution (lines 270-273) ❌ WRONG ORDER
    if fsInterface.Getuid() == 0 {
        return NewWriteError("direct root execution not allowed", nil)
    }

    // Step 3: Detect sudo context (lines 275-279)
    sudoCtx, err := detectSudoContext()
    if err != nil {
        return err
    }

    // Step 4: Validate sudo context exists (lines 281-284)
    if sudoCtx == nil {
        return NewWriteError("append-only attribute requires sudo privileges", nil)
    }
    ...
}
```

**Problem**: The code checks `Getuid() == 0` before checking for `SUDO_USER` environment variable. When running `sudo frozendb create`, the effective UID is 0 (root), so the function immediately returns "direct root execution not allowed" error without checking if this is a legitimate sudo invocation.

### Root Cause

The check order prioritizes UID validation over sudo context detection, creating a logic bug:
- Valid case: `sudo frozendb create` → UID=0, SUDO_USER=original_user → should PASS but FAILS
- Invalid case: Direct root (logged in as root) → UID=0, SUDO_USER="" → should FAIL and correctly FAILS

The current implementation blocks both cases.

### Desired Behavior (Correct)

**Required Check Order**:
1. Validate inputs
2. **Detect sudo context** (check SUDO_USER, SUDO_UID, SUDO_GID environment variables)
3. **Check if UID==0 AND no SUDO_USER** → reject as direct root execution
4. Validate sudo context exists (required for append-only attribute setting)
5. Proceed with file operations

**Decision Matrix**:

| UID | SUDO_USER Present | SUDO_USER Valid | Behavior |
|-----|-------------------|-----------------|----------|
| 0   | Yes               | Yes             | ✅ ALLOW (sudo execution) |
| 0   | Yes               | No              | ❌ REJECT (invalid sudo context) |
| 0   | No                | N/A             | ❌ REJECT (direct root execution) |
| >0  | Yes               | Yes             | ✅ ALLOW (sudo execution) |
| >0  | No                | N/A             | ❌ REJECT (requires sudo) |

## Existing Infrastructure Analysis

### Mock Testing Infrastructure

**File**: `internal/frozendb/create.go:14-86`

The codebase already has a robust mocking infrastructure for filesystem operations:

```go
type fsOperations struct {
    Getuid func() int
    Lookup func(username string) (*user.User, error)
    Open   func(name string, flag int, perm os.FileMode) (*os.File, error)
    Stat   func(name string) (os.FileInfo, error)
    Mkdir  func(path string, perm os.FileMode) error
    Chown  func(name string, uid, gid int) error
    Ioctl  func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (r1 uintptr, r2 uintptr, err syscall.Errno)
}

// Global variable to allow tests to inject mock filesystem operations
var fsInterface = &defaultFSOps

// Helper functions
func SetFSInterface(ops fsOperations)
func restoreRealFS()
func setupMockFS(overrides fsOperations)
```

**Key Insight**: The `fsInterface.Getuid()` is already mockable, allowing us to simulate root UID (0) in tests without requiring actual sudo execution.

### Environment Variable Testing

**File**: `internal/frozendb/create_spec_test.go:19-43`

The existing spec tests use `t.Setenv()` to set environment variables:

```go
func setupValidSudoEnv(t *testing.T) func() {
    currentUser, err := user.Current()
    if err != nil {
        t.Skip("Cannot get current user for testing")
        return func() {}
    }

    // Set valid sudo environment
    t.Setenv("SUDO_USER", currentUser.Username)
    t.Setenv("SUDO_UID", currentUser.Uid)
    t.Setenv("SUDO_GID", currentUser.Gid)

    return func() {
        // Cleanup handled by t.Setenv()
    }
}
```

**Key Insight**: We can use `t.Setenv()` to control SUDO_USER environment variables independently from the mocked UID, allowing us to test all combinations of the decision matrix.

## Testing Strategy

### Approach: Test-First Bug Replication

Per the user's requirement: *"The real trick of the planning stage is to make sure we can still properly test this without requiring sudo. Make sure the mocks are set up in such a way that we can replicate the bug (and ensure that we can replicate the failure), before making the fix and ensuring the tests now pass."*

**Phase 1: Write Failing Tests**
1. Create `Test_S_030_FR_001_AllowSudoExecution` that:
   - Sets `SUDO_USER`, `SUDO_UID`, `SUDO_GID` environment variables to valid values
   - Mocks `fsInterface.Getuid()` to return 0 (root UID)
   - Mocks `fsInterface.Lookup()` to return valid user
   - Calls `Create()` expecting success
   - **This test will FAIL** with current bug (showing "direct root execution not allowed")

2. Create `Test_S_030_FR_002_RejectDirectRootExecution` that:
   - Clears `SUDO_USER`, `SUDO_UID`, `SUDO_GID` environment variables
   - Mocks `fsInterface.Getuid()` to return 0 (root UID)
   - Calls `Create()` expecting "direct root execution not allowed" error
   - **This test will PASS** with current code (already working correctly)

**Phase 2: Fix Implementation**
1. Reorder checks in `Create()` function to:
   - Move `detectSudoContext()` call before UID check
   - Change UID check to: `if fsInterface.Getuid() == 0 && sudoCtx == nil`

**Phase 3: Verify Tests Pass**
1. Both FR-001 and FR-002 tests should now pass
2. All existing Spec 001 tests must continue to pass (no modifications to existing tests)

### Mock Configuration for Bug Replication

**FR-001 Test Configuration** (Sudo execution - currently FAILS, should PASS after fix):
```go
setupMockFS(fsOperations{
    Getuid: func() int { return 0 }, // Simulate root UID
    Lookup: func(username string) (*user.User, error) {
        return &user.User{
            Uid:      "1000",
            Gid:      "1000",
            Username: username,
            Name:     "Test User",
            HomeDir:  "/home/testuser",
        }, nil
    },
    // Other operations use real implementations or success mocks
})

t.Setenv("SUDO_USER", "testuser")
t.Setenv("SUDO_UID", "1000")
t.Setenv("SUDO_GID", "1000")
```

**FR-002 Test Configuration** (Direct root - currently PASSES, should continue to PASS):
```go
setupMockFS(fsOperations{
    Getuid: func() int { return 0 }, // Simulate root UID
})

t.Setenv("SUDO_USER", "")
t.Setenv("SUDO_UID", "")
t.Setenv("SUDO_GID", "")
```

## Implementation Approach

### Decision: Minimal Code Change

**Rationale**: 
- Existing `detectSudoContext()` function already validates all sudo environment variables
- Existing error messages are clear and appropriate
- Only the check order needs to be fixed

**Alternatives Considered**:
1. **Combine UID and SUDO_USER checks into single conditional** - Rejected: Would make logic harder to test and understand
2. **Add separate helper function for root detection** - Rejected: Adds unnecessary complexity for a simple reordering
3. **Reorder existing checks** (SELECTED) - Chosen: Minimal change, maintains all existing behavior

### Code Change

**Before** (lines 270-284):
```go
// Check for direct root execution (FR-003)
if fsInterface.Getuid() == 0 {
    return NewWriteError("direct root execution not allowed", nil)
}

// Detect sudo context (required for proper operation)
sudoCtx, err := detectSudoContext()
if err != nil {
    return err
}

// Validate that we have proper sudo context for append-only setting
if sudoCtx == nil {
    return NewWriteError("append-only attribute requires sudo privileges", nil)
}
```

**After** (proposed):
```go
// Detect sudo context first (required for proper operation)
sudoCtx, err := detectSudoContext()
if err != nil {
    return err
}

// Check for direct root execution - only reject if no sudo context
if fsInterface.Getuid() == 0 && sudoCtx == nil {
    return NewWriteError("direct root execution not allowed", nil)
}

// Validate that we have proper sudo context for append-only setting
if sudoCtx == nil {
    return NewWriteError("append-only attribute requires sudo privileges", nil)
}
```

**Note**: The last check `if sudoCtx == nil` will still catch non-root users without sudo, ensuring backward compatibility.

## Existing Tests Impact Analysis

### Spec Tests (Spec 001)

**File**: `internal/frozendb/create_spec_test.go`

All existing Spec 001 tests use `setupValidSudoEnv()` helper which sets valid SUDO_USER, SUDO_UID, SUDO_GID environment variables. The mocking strategy in those tests already simulates valid sudo execution.

**Impact**: No existing Spec 001 tests should be affected because:
1. They all set valid sudo environment variables
2. They mock successful operations (including ioctl for append-only)
3. The reordered checks will still allow these tests to pass

**Verification Required**: Run `go test -v ./internal/frozendb -run "^Test_S_001_"` before and after the fix to confirm no regressions.

### Unit Tests

**File**: `internal/frozendb/create_test.go`

**Analysis Required**: Need to check if any unit tests directly test the UID check behavior. If so, they may need to be updated to account for the new check order.

**Approach**: Will examine unit tests during implementation phase to determine if any need updates (unit tests CAN be modified, unlike spec tests).

## Validation Approach

### Pre-Implementation Validation
1. Write FR-001 test → Verify it FAILS with bug
2. Write FR-002 test → Verify it PASSES (already working)
3. Run all Spec 001 tests → Verify they still PASS

### Post-Implementation Validation
1. Implement fix
2. Run FR-001 test → Verify it now PASSES
3. Run FR-002 test → Verify it still PASSES
4. Run all Spec 001 tests → Verify they still PASS
5. Run all unit tests → Verify no regressions

### Edge Cases to Test
- UID=0, valid SUDO_USER → Allow
- UID=0, no SUDO_USER → Reject (direct root)
- UID=0, invalid SUDO_UID format → Reject (invalid sudo context)
- UID=0, SUDO_UID doesn't match SUDO_USER → Reject (invalid sudo context)
- UID>0, no SUDO_USER → Reject (requires sudo)

## Spec Testing Requirements

Per `docs/spec_testing.md`:
- File: `internal/frozendb/create_spec_test.go` (co-located with create.go)
- Naming: `Test_S_030_FR_001_*` and `Test_S_030_FR_002_*`
- Must follow table-driven test pattern where applicable
- Must use `t.Setenv()` for environment variable setup
- Must use `setupMockFS()` for filesystem operation mocking
- Must verify both success and error paths
- Must NOT modify existing Test_S_001_* tests

## Summary

**Key Findings**:
1. Bug is a simple check order issue in `Create()` function
2. Existing mock infrastructure fully supports testing without sudo
3. Fix requires minimal code change (reorder + combine conditions)
4. All existing tests should continue to pass
5. New Spec 030 tests will validate the fix

**Next Steps** (Phase 1 - Design & Contracts):
1. Document error flow in data-model.md
2. Create API contract for Create function behavior in contracts/api.md
3. Re-evaluate Constitution Check (should remain passing)
