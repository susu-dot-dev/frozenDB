# Feature Specification: Header Refactor

**Feature Branch**: `008-header-refactor`  
**Created**: 2025-01-14  
**Status**: Draft  
**Input**: User description: "008 1. Eliminate Dual Header Creation Pattern - Single Header creation should provide both struct access and marshaled bytes, eliminating the current pattern of creating both headerBytes and Header struct separately. 2. Consistent Struct Pattern Alignment - Header should follow same constructor/validation/marshaling patterns as DataRow and ChecksumRow (direct initialization + Validate() + MarshalText/UnmarshalText). 3. Clear Code Organization - Move Header and related functionality from create.go to dedicated header.go file. 4. Maintain Backward Compatibility - All existing Header APIs and behaviors must remain unchanged. Key Requirements (High-Level) File Organization: - FR-001: Move Header struct and all Header methods to dedicated header.go Pattern Alignment: - FR-003: Add MarshalText() method to Header struct (eliminate generateHeader function) - FR-004: Use direct struct initialization + Validate() (no NewHeader constructor needed) - FR-005: Single Header creation followed by MarshalText() when bytes needed Eliminate Dual Creation: - FR-006: Remove generateHeader() function completely - FR-007: Update create.go to use Header.MarshalText() instead of separate header generation Compatibility: - FR-009: Maintain all existing Header getter methods unchanged - FR-010: Maintain existing Header.UnmarshalText() behavior exactly - FR-011: Maintain exact 64-byte header format compatibility"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Header Creation Simplification (Priority: P1)

As a developer using frozenDB, I want to create database headers using a single consistent pattern so that my code is simpler and follows the same patterns as other database components.

**Why this priority**: This eliminates the current dual creation pattern that forces developers to manage both headerBytes and Header struct separately, reducing complexity and potential errors.

**Independent Test**: Can be tested by creating a Header struct, calling Validate(), then MarshalText() to get bytes - this single flow replaces the current generateHeader() + separate Header creation pattern.

**Acceptance Scenarios**:

1. **Given** valid rowSize and skewMs parameters, **When** I create a Header struct directly and call MarshalText(), **Then** I get the same 64-byte header format as before
2. **Given** an existing Header struct, **When** I call Validate() then MarshalText(), **Then** the output matches generateHeader() output exactly

---

### User Story 2 - Code Organization Improvement (Priority: P2)

As a developer maintaining frozenDB, I want all Header-related functionality in a dedicated file so that code is better organized and easier to navigate.

**Why this priority**: Moving Header functionality from create.go to header.go improves code organization and separates concerns, making the codebase more maintainable.

**Independent Test**: Can be verified by confirming all Header struct definitions, methods, and related functionality are moved to header.go without breaking existing functionality.

**Acceptance Scenarios**:

1. **Given** the current codebase, **When** I move Header functionality to header.go, **Then** all existing tests continue to pass
2. **Given** the new file organization, **When** I import frozendb, **Then** all Header APIs remain accessible unchanged

---

### Edge Cases

- What happens when Header is created with invalid field values? (Should be caught by Validate())
- How does the system handle MarshalText() calls on unvalidated Header structs? (Should validate during marshaling)
- What happens when Header.UnmarshalText() receives malformed bytes? (Should return appropriate error)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST move Header struct and all Header methods to dedicated header.go file
- **FR-002**: Header MUST implement MarshalText() method that eliminates need for generateHeader() function
- **FR-003**: Header MUST support direct struct initialization + Validate() pattern (no NewHeader constructor)
- **FR-004**: System MUST use single Header creation followed by MarshalText() when bytes needed
- **FR-005**: Header.MarshalText() MUST return identical byte format to current generateHeader() output
- **FR-006**: System MUST maintain all existing Header getter methods unchanged
- **FR-007**: System MUST maintain exact 64-byte header format compatibility

### Key Entities *(include if feature involves data)*

- **Header**: frozenDB v1 text-based header format with exactly 64 bytes containing JSON content + null padding + newline
- **HeaderJSON**: Internal JSON representation for serialization/deserialization

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_XXX_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Header creation pattern reduced from 2 steps to 1 step (single struct creation vs dual creation)
- **SC-002**: 100% of existing Header APIs remain functionally unchanged (backward compatibility)

### Data Integrity & Correctness Metrics *(required for frozenDB)*
- **SC-005**: Zero header format compatibility regressions in all database creation tests
- **SC-006**: All Header marshaling/unmarshaling operations maintain exact byte-level compatibility  
- **SC-007**: Header validation maintains same error detection and reporting capabilities
