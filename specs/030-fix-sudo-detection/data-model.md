# Data Model: Sudo Detection Logic Bug Fix

**Date**: Thu Jan 29 2026  
**Branch**: 030-fix-sudo-detection  
**Spec**: [spec.md](./spec.md)  
**Research**: [research.md](./research.md)

## Overview

This document describes the data structures, validation rules, and state transitions for the sudo detection logic bug fix. Since this is a bug fix rather than a new feature, no new data structures are introduced. This document focuses on the error conditions and validation state changes.

## Existing Data Structures

### SudoContext (Unchanged)

**Location**: `internal/frozendb/create.go:127-147`

```go
type SudoContext struct {
    user string // Original username from SUDO_USER
    uid  int    // Original user ID from SUDO_UID
    gid  int    // Original group ID from SUDO_GID
}
```

**Fields**:
- `user`: Original username from SUDO_USER environment variable
- `uid`: Original user ID from SUDO_UID environment variable (must be > 0)
- `gid`: Original group ID from SUDO_GID environment variable (must be > 0)

**Validation Rules** (Existing):
- `user` must not be empty
- `uid` must be > 0
- `gid` must be > 0
- `uid` must match the UID of the user returned by `user.Lookup(user)`

**Detection Logic** (Existing):
- Returns `nil` if SUDO_USER environment variable is not set (not running under sudo)
- Returns error if SUDO_UID or SUDO_GID are missing or invalid format
- Returns error if user doesn't exist or UID doesn't match

## State Transitions

### Current State Flow (Buggy)

```
Create() called
    ↓
Validate inputs (CreateConfig.Validate())
    ↓
Check UID == 0? ← BUG: Checked too early
    ↓ YES (root)
    ↓
❌ Return "direct root execution not allowed"
    
[Never reaches sudo context detection when UID=0]
```

### Fixed State Flow

```
Create() called
    ↓
Validate inputs (CreateConfig.Validate())
    ↓
Detect sudo context (detectSudoContext())
    ↓
    ├─→ Error? → ❌ Return error (invalid sudo context)
    ↓
Check (UID == 0 AND sudoCtx == nil)?
    ↓ YES (direct root)
    ↓
    ❌ Return "direct root execution not allowed"
    ↓ NO (sudo execution OR non-root)
    ↓
Check sudoCtx == nil?
    ↓ YES (non-root without sudo)
    ↓
    ❌ Return "append-only attribute requires sudo privileges"
    ↓ NO (valid sudo context)
    ↓
Proceed with file creation
```

## Error Conditions

### New Error Logic

The bug fix introduces a **combined condition** for direct root execution detection:

**Condition**: `UID == 0 AND sudoCtx == nil`

This replaces the old condition: `UID == 0`

### Error Mapping Table

| Condition | SUDO_USER | sudoCtx | Error Returned |
|-----------|-----------|---------|----------------|
| UID == 0 | Present, Valid | Valid object | ✅ No error (proceed) |
| UID == 0 | Present, Invalid | Error | ❌ "invalid SUDO_UID format" or similar (from detectSudoContext) |
| UID == 0 | Missing | nil | ❌ "direct root execution not allowed" (FR-002) |
| UID > 0 | Present, Valid | Valid object | ✅ No error (proceed) |
| UID > 0 | Missing | nil | ❌ "append-only attribute requires sudo privileges" |

### Error Types

All errors use existing error types from `errors.go`:

1. **WriteError** - Used for operational errors
   - Message: "direct root execution not allowed"
   - Cause: nil
   - Triggered by: UID == 0 AND sudoCtx == nil

2. **WriteError** - Used for sudo context validation errors
   - Message: "missing SUDO_UID or SUDO_GID environment variables"
   - Message: "invalid SUDO_UID format"
   - Message: "invalid SUDO_GID format"
   - Message: "original user not found"
   - Message: "SUDO_UID does not match SUDO_USER"
   - Triggered by: detectSudoContext() validation failures

3. **WriteError** - Used for privilege requirement errors
   - Message: "append-only attribute requires sudo privileges"
   - Cause: nil
   - Triggered by: sudoCtx == nil (no sudo context detected)

## Validation Rules

### FR-001: Allow Sudo Execution

**Preconditions**:
- SUDO_USER environment variable is set and not empty
- SUDO_UID environment variable is set and valid integer > 0
- SUDO_GID environment variable is set and valid integer > 0
- SUDO_USER exists as a valid system user
- SUDO_UID matches the user's actual UID

**Postconditions**:
- `detectSudoContext()` returns valid SudoContext object
- UID check passes (even if UID == 0)
- File creation proceeds

**Error States**:
- Missing SUDO_UID/SUDO_GID → "missing SUDO_UID or SUDO_GID environment variables"
- Invalid format → "invalid SUDO_UID format" or "invalid SUDO_GID format"
- User not found → "original user not found"
- UID mismatch → "SUDO_UID does not match SUDO_USER"

### FR-002: Reject Direct Root Execution

**Preconditions**:
- Process running with UID == 0 (effective root)
- SUDO_USER environment variable is NOT set (or empty)

**Postconditions**:
- `detectSudoContext()` returns nil (no sudo context)
- UID check fails
- Error returned: "direct root execution not allowed"

**Error States**:
- This IS the error state - direct root execution is blocked

## Data Flow Changes

### Function Call Sequence (Fixed)

```
Create(config CreateConfig)
    ↓
config.Validate() → Validates rowSize, skewMs, path
    ↓ [CHANGE: Moved this call earlier]
detectSudoContext() → Returns (sudoCtx, err)
    ↓
    ├─→ err != nil → Return err
    ↓
[CHANGE: Combined condition]
if Getuid() == 0 && sudoCtx == nil → Return "direct root execution not allowed"
    ↓
if sudoCtx == nil → Return "append-only attribute requires sudo privileges"
    ↓
createFile(path) → File operations
    ↓
setOwnership(path, sudoCtx) → chown to original user
    ↓
setAppendOnlyAttr(fd) → ioctl to set append-only
```

### Key Changes

1. **detectSudoContext() called earlier**: Now executes before UID check
2. **Combined UID check**: `UID == 0 && sudoCtx == nil` instead of just `UID == 0`
3. **No new data structures**: All existing structures remain unchanged
4. **No new error types**: All existing error types are reused

## Impact on Existing Behavior

### Preserved Behaviors

1. **Sudo context validation**: All existing validation in `detectSudoContext()` remains unchanged
2. **Non-root without sudo rejection**: Non-root users without sudo are still rejected (sudoCtx == nil check)
3. **File creation process**: All file operations after validation remain unchanged
4. **Error messages**: All error messages remain unchanged
5. **Mock infrastructure**: All existing mocks continue to work

### Changed Behaviors

1. **Sudo with root UID**: Now PASSES instead of being rejected
   - Before: UID==0 → Immediate rejection
   - After: UID==0 with valid SUDO_USER → Allowed

2. **Direct root rejection timing**: Error message unchanged, but check happens after sudo detection
   - Before: Checked immediately after input validation
   - After: Checked after sudo context detection

## Testing Data Scenarios

### Scenario 1: Valid Sudo Execution (FR-001)

**Input**:
- UID: 0
- SUDO_USER: "testuser"
- SUDO_UID: "1000"
- SUDO_GID: "1000"
- User lookup returns valid user with UID 1000

**Expected Output**:
- detectSudoContext() returns valid SudoContext{user: "testuser", uid: 1000, gid: 1000}
- Combined check passes (UID==0 but sudoCtx != nil)
- File creation proceeds
- ✅ SUCCESS

### Scenario 2: Direct Root Execution (FR-002)

**Input**:
- UID: 0
- SUDO_USER: "" (empty or not set)
- SUDO_UID: "" (empty or not set)
- SUDO_GID: "" (empty or not set)

**Expected Output**:
- detectSudoContext() returns nil (no sudo context)
- Combined check fails (UID==0 AND sudoCtx == nil)
- ❌ Error: "direct root execution not allowed"

### Scenario 3: Invalid Sudo Context (Edge Case)

**Input**:
- UID: 0
- SUDO_USER: "testuser"
- SUDO_UID: "invalid"
- SUDO_GID: "1000"

**Expected Output**:
- detectSudoContext() returns error: "invalid SUDO_UID format"
- Error returned before reaching UID check
- ❌ Error: "invalid SUDO_UID format"

### Scenario 4: Non-Root Without Sudo (Existing Behavior)

**Input**:
- UID: 1000
- SUDO_USER: "" (not set)

**Expected Output**:
- detectSudoContext() returns nil
- Combined check passes (UID != 0, so direct root check doesn't apply)
- sudoCtx == nil check fails
- ❌ Error: "append-only attribute requires sudo privileges"

## Summary

**Data Structure Changes**: None - all existing structures preserved

**Validation Changes**: 
- Combined UID check: `UID == 0 && sudoCtx == nil` (new logic)
- Sudo context detection: Moved earlier in execution flow (reordering)

**Error Flow Changes**:
- New success path: UID==0 with valid SUDO_USER now succeeds
- Preserved error paths: All existing error conditions still trigger correctly

**Testing Implications**:
- Must test both UID==0 cases: with and without SUDO_USER
- Must verify all existing error paths still work
- Must ensure no regressions in existing Spec 001 tests
