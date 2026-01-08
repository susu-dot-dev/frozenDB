# Spec Testing Guidelines

This document defines the comprehensive requirements for spec tests in frozenDB, a Go-based immutable key-value store. Spec tests validate functional requirements from specification documents and are distinct from unit tests.

## Purpose & Scope

Spec tests exist to:
- Validate that functional requirements (FR-XXX) are correctly implemented
- Provide proof that each requirement works as specified
- Detect breaking changes to previously implemented specifications
- Serve as living documentation of system behavior

Spec tests differ from unit tests:
- Unit tests test internal implementation details and edge cases
- Spec tests test functional requirements from user/system perspective
- Both types of tests are required for complete coverage

## File Organization

### Directory Structure
Each Go module (e.g., `internal/db/`, `pkg/storage/`) MUST contain a `spec_tests/` subdirectory:

```
internal/db/
├── db.go
├── db_test.go              # Unit tests
└── spec_tests/
    ├── 001_create_db_test.go
    └── test_helpers.go
```

### File Naming Convention
- Spec test files MUST follow pattern: `[SPEC_NUMBER]_[SPEC_NAME]_test.go`
- File names MUST use underscores instead of spaces
- File names MUST be lowercase
- SPEC_NUMBER is the 4-digit feature number (e.g., 0001, 0010)
- SPEC_NAME is derived from the spec directory name (create_db, add_kv_pair)

## Test Function Naming

### Test Function Convention
Test functions MUST follow pattern: `TestFR-[NUMBER]_[Description]()`
- `FR-[NUMBER]` corresponds to the functional requirement being tested
- `[Description]` is a camelCase description of what is being validated
- Test functions MUST be exported (start with capital T) to run with Go test framework

### Examples
```go
// Testing FR-011: System MUST validate skewMs parameter is between 0-86400000 inclusive
func TestFR_011_NegativeSkewDisallowed(t *testing.T) { /* ... */ }
func TestFR_011_ZeroSkewAllowed(t *testing.T) { /* ... */ }
func TestFR_011_MaxSkewAllowed(t *testing.T) { /* ... */ }
func TestFR_011_ExceedsMaxSkewDisallowed(t *testing.T) { /* ... */ }
```

## Functional Requirement Coverage

### Mandatory Test Coverage Rule
**CRITICAL**: Every single functional requirement (FR-XXX) MUST have at least one corresponding spec test. 

- **No exceptions allowed**: Each FR-XXX must have test coverage
- **Unavoidable skips**: If testing a functional requirement is truly impossible, you MUST use `t.Skip()` with detailed documentation explaining why
- **Skip documentation**: Skip messages MUST include:
  - The specific functional requirement being skipped
  - Detailed explanation of why testing is impossible
  - Any alternative validation approaches considered
  - Context about the limitation (e.g., hardware constraints, external dependencies)

#### Skip Example
```go
func TestFR_999_HardwareFailover(t *testing.T) {
    t.Skip("FR-999: Cannot test automatic hardware failover in unit test environment. " +
          "Requirement involves physical hardware failure detection which cannot be " +
          "simulated without specialized test hardware. Considered alternatives: " +
          "mock hardware interfaces (insufficient), network failure simulation (doesn't " +
          "cover actual hardware scenarios). Validation deferred to integration tests " +
          "with physical hardware.")
}
```

### Data-Driven Tests for Range Validation
For requirements that validate parameter ranges, use data-driven tests:

```go
func TestFR_011_SkewMsValidation(t *testing.T) {
    tests := []struct {
        name    string
        skewMs  int64
        wantErr bool
    }{
        {"negative_skew_disallowed", -1, true},
        {"zero_skew_allowed", 0, false},
        {"positive_skew_allowed", 1000, false},
        {"max_skew_allowed", 86400000, false},
        {"exceeds_max_disallowed", 86400001, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateSkewMs(tt.skewMs)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateSkewMs() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Scope
- Spec tests MUST cover the functional requirement exactly as specified
- Tests MUST NOT go beyond the requirement's scope
- Tests MUST validate both success and failure scenarios where applicable
- Test data MUST use realistic values unless testing edge cases

## Modification Rules

### Strict Protection Rules
**CRITICAL**: The following rules MUST be followed without exception:

1. **Existing Spec Tests MUST NOT be Modified** after a spec is implemented
2. **Previous Spec Test Files MUST NOT be Edited** to accommodate new implementations
3. **Spec Test Requirements MUST NOT be Changed** without user permission
4. **Test Function Signatures MUST NOT be Altered** after implementation

### Allowed Modifications
The ONLY files that may be modified when implementing new specs:

1. **Current Spec's Test File**: `spec_tests/[CURRENT_SPEC]_test.go`
2. **Helper Files**: `spec_tests/test_helpers.go`, `spec_tests/common_test_helpers.go`

### Breaking Changes
If new implementation causes previous spec tests to fail:
1. **MUST STOP** and seek explicit user permission
2. **MUST NOT** modify previous spec tests
3. **MUST** update the functional requirements in the spec if needed
4. **MUST** get user approval for any specification changes

## Implementation Guidelines

### Test Structure
```go
func TestFR_XXX_Description(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Execute
    result, err := db.FunctionUnderTest(params)
    
    // Validate
    if (err != nil) != tt.wantErr {
        t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
    }
    
    if tt.wantResult != result {
        t.Errorf("Function() = %v, want %v", result, tt.wantResult)
    }
}
```

### Helper Functions
Common test setup and validation logic SHOULD be placed in helper files:

```go
// spec_tests/test_helpers.go
func setupTestDB(t *testing.T) *DB {
    // Common database setup for spec tests
}

func cleanupTestDB(t *testing.T, db *DB) {
    // Common database cleanup
}
```

## Compliance Process

### Definition of "Implemented"
A functional requirement is ONLY considered implemented when:
1. The implementation code exists and compiles
2. All corresponding spec tests pass
3. Unit tests also pass (where applicable)
4. No existing spec tests are broken

### Review Checklist
Before considering a spec implemented:
- [ ] All FR-XXX requirements have corresponding tests
- [ ] All spec tests pass (or have documented t.Skip() with valid reasons)
- [ ] No previous spec tests are modified
- [ ] Test coverage matches requirement scope exactly
- [ ] Helper functions are appropriately used
- [ ] Every FR-XXX has at least one test function (no missing coverage)

### Verification Commands
```bash
# Run spec tests for a specific module
go test ./internal/db/spec_tests/...

# Run all spec tests
go test ./.../spec_tests/...

# Run with coverage
go test -cover ./.../spec_tests/...
```

## Examples

### Complete Example: FR-011 Implementation

**Requirement**: System MUST validate skewMs parameter is between 0-86400000 inclusive

**Spec Test File**: `spec_tests/001_create_db_test.go`

```go
package spec_tests

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "yourproject/internal/db"
)

func TestFR_011_SkewMsValidation(t *testing.T) {
    tests := []struct {
        name    string
        skewMs  int64
        wantErr bool
        errType error
    }{
        {
            name:    "negative_skew_disallowed",
            skewMs:  -1,
            wantErr: true,
            errType: &db.InvalidInputError{},
        },
        {
            name:    "zero_skew_allowed",
            skewMs:  0,
            wantErr: false,
        },
        {
            name:    "positive_skew_allowed",
            skewMs:  43200000,
            wantErr: false,
        },
        {
            name:    "max_skew_allowed",
            skewMs:  86400000,
            wantErr: false,
        },
        {
            name:    "exceeds_max_disallowed",
            skewMs:  86400001,
            wantErr: true,
            errType: &db.InvalidInputError{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := db.ValidateSkewMs(tt.skewMs)
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != nil {
                    assert.IsType(t, tt.errType, err)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Enforcement

### Automated Checks
The build system SHOULD include checks to:
- Verify spec test files exist for implemented features
- Ensure spec test naming conventions are followed
- Detect unauthorized modifications to previous spec test files
- Verify every FR-XXX requirement has at least one corresponding test function
- Check that any skipped tests have comprehensive documentation in t.Skip() messages

### Manual Review Process
1. Code reviewers MUST verify no previous spec tests were modified
2. Reviewers MUST confirm new spec tests cover all functional requirements
3. Repository maintainers MUST enforce spec test compliance rules

## Governance

This document is governed by the frozenDB Constitution. Changes to these requirements require constitutional amendment and MUST follow the established governance process.

**Version**: 1.1.0 | **Created**: 2025-01-08 | **Last Updated**: 2025-01-08
