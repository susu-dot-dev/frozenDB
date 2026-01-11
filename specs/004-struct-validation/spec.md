# Feature Specification: Struct Validation and Immutability

**Feature Branch**: `004-struct-validation`  
**Created**: 2026-01-11  
**Status**: Draft  
**Input**: User description: "Add the 004 spec for standardizing struct validation and immutability. We want to ensure that the fields of structs are valid, but we also don't want to be checking that every single time that data is used. For the most part, the fields of a struct are not meant to be changed after construction. We can thus simplify our reasoning by converting most fields into lowercase (e.g rowSize instead of RowSize). That way, we know our code inside the package will do the right thing, and we don't have to worry about outside callers changing fields they shouldn't. Fields that make sense for users of the package to access should be exposed as Getter functions. Next, there are three possible ways to initialize a struct: Direct initialization, The NewStruct() function pattern, or UnmarshalText(). Support all three of these methods via the following Validator pattern: All structs which need to have some form of potentially invalid state of their fields should have a Validate() method. That method should assume any sub-structs are already valid, since the sub-struct creation method should have already called Validate() on that method. So for the current struct, Validate should check things like: Are my non-struct fields (those without Validate()) proper? Are my fields with struct pointers non-null? Are my fields with sub-structs valid given my context? For the latter one, an example would be the StartControl type would check to make sure that the start control is any valid code. However, for the checksum code the Start Control must be \"C\", so the ChecksumRow would add this extra layer of validation. With this type of validiation defined and implemented, the construction logic looks like this: 1. If directly creating a struct %{}, you must call the Validate() error method on that struct before using it in any further action. 2. The NewStruct() constructor pattern must call Validate() before returning. 3. The UnmarshalText() function must call Validate() before returning. 4. All of these methods must apply the same logic to any sub-fields they create as part of the process"

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.
  
  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Consistent Validation Pattern Across All Constructor Paths (Priority: P1)

As a developer, I want all structs to use a single Validate() method that is called consistently regardless of how I construct the struct (direct initialization, NewStruct(), or UnmarshalText()), so that I have a predictable and reliable validation pattern throughout the codebase.

**Why this priority**: This establishes the foundational validation pattern that all other validation features depend on. Without consistent validation across constructor paths, developers cannot trust that structs are valid regardless of how they were created.

**Independent Test**: Can be fully tested by creating a struct using each of the three constructor methods and verifying that Validate() is called in all cases, delivering consistent validation behavior across all initialization patterns.

**Acceptance Scenarios**:

1. **Given** a struct with Validate() method, **When** developer directly initializes struct with `struct{...}`, **Then** Validate() must be called before the struct can be used in any operation
2. **Given** a struct with Validate() method, **When** developer calls NewStruct() constructor function, **Then** Validate() is automatically called before the function returns
3. **Given** a struct with Validate() method, **When** developer calls UnmarshalText() to deserialize, **Then** Validate() is automatically called before the function returns
4. **Given** any constructor path, **When** Validate() returns an error, **Then** the constructor path returns that error and the struct is not usable

---

### User Story 2 - Clear Parent-Child Validation Responsibilities (Priority: P1)

As a developer, I want parent structs to validate their child structs in context while child structs validate their own fields independently, so that I understand exactly where validation logic belongs and can reason about validation failures clearly.

**Why this priority**: Clear separation of validation responsibilities prevents duplicate validation logic and ensures that context-specific validation (like ChecksumRow requiring StartControl='C') happens at the right level. This is essential for maintainable validation code.

**Independent Test**: Can be fully tested by creating a parent struct with child struct fields, verifying that child Validate() is called during child construction, and parent Validate() only checks parent-specific context, delivering clear validation boundaries.

**Acceptance Scenarios**:

1. **Given** a parent struct with child struct fields, **When** parent struct is constructed, **Then** child structs are validated via their own Validate() methods during child construction
2. **Given** a parent struct with child struct fields, **When** parent Validate() is called, **Then** parent assumes child structs are already valid and only validates parent-specific context
3. **Given** a parent struct with context-specific child validation (e.g., ChecksumRow requires StartControl='C'), **When** parent Validate() is called, **Then** parent validates that child structs meet parent's contextual requirements
4. **Given** a child struct with invalid fields, **When** child Validate() is called during construction, **Then** child Validate() returns error before parent validation occurs

---

### User Story 3 - Field Immutability Through Unexported Fields (Priority: P2)

As a developer, I want struct fields to be unexported (lowercase) by default with exported getter functions for fields that need external access, so that I can trust that struct fields are not modified after construction and simplify reasoning about struct state.

**Why this priority**: Field immutability prevents accidental modification of validated struct state and reduces the need for repeated validation checks. This improves code safety and performance by ensuring structs remain in their validated state.

**Independent Test**: Can be fully tested by attempting to modify struct fields directly and verifying that unexported fields prevent external modification, while getter functions provide controlled access, delivering immutability guarantees.

**Acceptance Scenarios**:

1. **Given** a struct with unexported fields, **When** developer attempts to modify fields from outside the package, **Then** compilation fails preventing external modification
2. **Given** a struct with fields that need external access, **When** developer accesses those fields, **Then** getter functions (e.g., GetFieldName()) provide read-only access
3. **Given** a validated struct with unexported fields, **When** struct is used throughout the codebase, **Then** fields remain in their validated state without additional validation checks
4. **Given** a struct with both exported and unexported fields, **When** struct is constructed and validated, **Then** only exported getter functions allow external field access

### Edge Cases

- What happens when a struct has no fields that need validation? (Structs without Validate() method are considered always valid)
- How does system handle nested struct validation when child struct has no Validate() method? (Parent assumes child is valid if no Validate() exists)
- What happens when Validate() is called multiple times on the same struct instance? (Validate() should be idempotent and return same result)
- How does system handle structs with nil pointer fields that should be non-nil? (Validate() must check for nil pointers and return error)
- What happens when UnmarshalText() partially succeeds but Validate() fails? (UnmarshalText() must return validation error, struct remains in invalid state)
- How does system handle circular validation dependencies between parent and child? (Child validates first during construction, parent validates context afterward)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide Validate() error method on all structs that require field validation
- **FR-002**: System MUST call Validate() when struct is directly initialized via struct literal before struct can be used in operations
- **FR-003**: System MUST call Validate() in all NewStruct() constructor functions before returning the struct instance
- **FR-004**: System MUST call Validate() in all UnmarshalText() methods before returning from unmarshaling
- **FR-005**: System MUST make Validate() idempotent (calling multiple times returns same result)
- **FR-006**: System MUST have Validate() assume all child struct fields are already valid (child Validate() called during child construction)
- **FR-007**: System MUST have Validate() check non-struct fields for validity (primitive types, strings, numbers, etc.)
- **FR-008**: System MUST have Validate() check that struct pointer fields are non-nil when required
- **FR-009**: System MUST have Validate() check that child struct fields meet parent's contextual requirements (e.g., ChecksumRow requires StartControl='C')
- **FR-010**: System MUST convert exported struct fields to unexported (lowercase) to prevent external modification after construction
- **FR-011**: System MUST provide getter functions (e.g., GetFieldName()) for struct fields that need external read access
- **FR-012**: System MUST ensure getter functions return read-only access to struct fields
- **FR-013**: System MUST call Validate() on child structs during their construction (in NewStruct() or UnmarshalText()) before parent validation
- **FR-014**: System MUST allow structs without Validate() method (considered always valid, no validation required)

### Key Entities *(include if feature involves data)*

- **Validatable Struct**: A struct type that implements Validate() error method to check field validity and contextual requirements
- **Child Struct**: A struct field within a parent struct that has its own Validate() method and is validated during its own construction
- **Parent Struct**: A struct that contains child struct fields and validates contextual requirements in its Validate() method
- **Getter Function**: An exported function (e.g., GetFieldName()) that provides read-only access to unexported struct fields
- **Constructor Path**: One of three methods for creating struct instances: direct struct literal initialization, NewStruct() function, or UnmarshalText() method

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `frozendb/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_004_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

### Spec Test Coverage

All functional requirements (FR-001 through FR-014) MUST have corresponding spec tests following the guidelines in `docs/spec_testing.md`. Spec tests validate functional requirements from developer perspective and are distinct from unit tests.

### Spec Test Requirements

**File Naming**: Spec tests should be placed in the appropriate `*_spec_test.go` files corresponding to the implementation files containing the structs being validated (e.g., `row_spec_test.go`, `checksum_spec_test.go`, `create_spec_test.go`).

**Test Function Pattern**: `Test_S_004_FR_XXX_Description()`
- FR_XXX corresponds to functional requirement being tested
- Description is camelCase description of validation

**Mandatory Coverage**: Every FR-XXX requirement must have at least one corresponding spec test function
- **No exceptions allowed**: Each requirement must have test coverage
- **Test-driven implementation**: Functional requirements are not considered implemented without passing spec tests
- **Immutable tests**: Once implemented, spec tests cannot be modified without explicit user permission

### Key Spec Test Categories

**Constructor Path Validation Tests** (FR-002, FR-003, FR-004):
- Direct struct initialization with Validate() requirement
- NewStruct() constructor calling Validate()
- UnmarshalText() calling Validate()

**Parent-Child Validation Tests** (FR-006, FR-009, FR-013):
- Child validation during child construction
- Parent assuming child validity
- Context-specific parent validation (e.g., ChecksumRow StartControl requirement)
- Validation order and dependency handling

**Field Immutability Tests** (FR-010, FR-011, FR-012):
- Unexported field enforcement (compilation errors)
- Getter function read-only access
- Field modification prevention after construction

**Validation Logic Tests** (FR-001, FR-005, FR-007, FR-008, FR-014):
- Validate() method implementation and idempotency
- Non-struct field validation
- Nil pointer field validation

### Compliance Verification

**Definition of "Implemented"**: A functional requirement is only considered implemented when:
1. Implementation code exists and compiles
2. All corresponding spec tests pass
3. No existing spec tests are broken
4. Success criteria are met

**Review Checklist**:
- [ ] All FR-XXX requirements have corresponding tests
- [ ] All spec tests pass (or have documented t.Skip() with valid reasons)
- [ ] No previous spec tests are modified
- [ ] Test coverage matches requirement scope exactly
- [ ] Every FR-XXX has at least one test function (no missing coverage)

**Test Execution**:
```bash
# Run spec tests for frozendb package
go test ./frozendb -run "^Test_S_004"
```

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of structs with validation requirements implement Validate() method consistently
- **SC-002**: 100% of constructor paths (direct init, NewStruct(), UnmarshalText()) call Validate() before returning struct instances
- **SC-003**: All struct fields that should be immutable are unexported (lowercase) with getter functions for external access
- **SC-004**: Zero validation bypass scenarios where structs can be used without validation in any constructor path
- **SC-005**: 100% of parent structs correctly assume child structs are valid during parent validation

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-006**: Zero instances where invalid struct state causes data corruption or incorrect database operations
- **SC-007**: All struct validation maintains data integrity requirements from v1_file_format.md specification
- **SC-008**: Memory usage remains constant regardless of struct complexity (validation does not allocate excessive memory)
- **SC-009**: Validation performance does not degrade with nested struct depth (validation remains efficient)
