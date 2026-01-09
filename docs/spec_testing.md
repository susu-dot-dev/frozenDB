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
Each Go source file MUST have corresponding test files in the same package directory:

```
frozendb/
├── create.go              # Implementation
├── create_test.go         # Unit tests  
├── create_spec_test.go    # Spec tests
├── errors.go              # Implementation
├── errors_test.go         # Unit tests
└── errors_spec_test.go    # Spec tests
```

### File Naming Convention
- **Unit test files**: `[filename]_test.go` (standard Go convention)
- **Spec test files**: `[filename]_spec_test.go` where `filename` matches the implementation file being tested
- File names MUST use underscores instead of spaces
- File names MUST be lowercase

### Package Declaration
- Unit test files and spec test files MAY use the same package declaration as the source file for testing internal structures, or package_name_test to enforce only testing public interfaces.

## Test Function Naming

### Test Function Convention
Test functions MUST follow pattern: `Test_S_XXX_FR_XXX_Description()`
- `S_XXX` corresponds to the spec number that is being implemented. Always use exactly 3 digits for the spec number
- `FR_XXX` corresponds to the functional requirement being tested. Use as few digits as required, example `FR_1`, `FR_22`, etc.
- `Description` is a camelCase description of what is being validated
- Test functions MUST be exported (start with capital T) to run with Go test framework

### Examples
```go
// Testing Spec 1, FR_011: System MUST validate skewMs parameter is between 0-86400000 inclusive
func Test_S_001_FR_011_NegativeSkewDisallowed(t *testing.T) { /* ... */ }
func Test_S_001_FR_011_ZeroSkewAllowed(t *testing.T) { /* ... */ }
func Test_S_001_FR_011_MaxSkewAllowed(t *testing.T) { /* ... */ }
func Test_S_001_FR_011_ExceedsMaxSkewDisallowed(t *testing.T) { /* ... */ }
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
func Test_S_001_FR_999_HardwareFailover(t *testing.T) {
    t.Skip("FR_999: Cannot test automatic hardware failover in unit test environment. " +
          "Requirement involves physical hardware failure detection which cannot be " +
          "simulated without specialized test hardware. Considered alternatives: " +
          "mock hardware interfaces (insufficient), network failure simulation (doesn't " +
          "cover actual hardware scenarios). Validation deferred to integration tests " +
          "with physical hardware.")
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

1. New spec tests may be freely added to an existing *_spec_test.go file 
2. **Spec tests for a previous spec MUST NOT be Modified** when creating new features, without permission
2. **Changes to a spec test for a previous spec, when allowed MUST correspond with updates to the previous spec** and related documentation, EXCEPT by following the breaking changes instructions. There should always be a current spec that is being worked on. For example, if the current spec is 002, then any spec test starting with `Test_S_001` may NOT be modified


### Breaking Changes
If new implementation causes previous spec tests to fail:
1. **MUST STOP** and seek explicit user guidance
2. If breaking changes are expected, this MUST be documented in the current spec. The documentation should contain what the expected breaking change is, the new behavior, and the allowed ways to modify the code to accomodate this behavior
3. If the change is due to an underspecified, or wrongly specified earlier spec, then the earlier spec must be rewritten to add updates and clarification. Then, the corresponding documentation (whether in the same spec folder, or elsewhere), should also be updated. Finally, you **MUST STOP** and ask if the updated spec is correct
4. Only once the spec has been updated, and approval is given, may existing spec tests be updated

## Implementation Guidelines

## Compliance Process

### Definition of "Implemented"
A functional requirement is ONLY considered implemented when:
1. The implementation code exists and compiles
2. The spec tests actually verify the behavior (and would fail if the desired behavior is not fully met)
3. All corresponding spec tests pass
4. Unit tests also pass (where applicable)
5. No existing spec tests are broken


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
# Run all tests (unit + spec)
go test -v ./...

# Run spec tests only (using naming convention)
go test -v ./... -run "^Test_S_"

# Run spec tests with coverage
go test -v ./... -cover -run "^Test_S_"

```

## Examples

### Complete Example: FR-011 Implementation

**Requirement**: System MUST validate skewMs parameter is between 0-86400000 inclusive

**Spec Test File**: `frozendb/create_spec_test.go`

```go
package frozendb

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func Test_S_001_FR_011_SkewMsValidation(t *testing.T) {
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
            errType: &frozendb.InvalidInputError{},
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
            errType: &frozendb.InvalidInputError{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := frozendb.ValidateSkewMs(tt.skewMs)
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
