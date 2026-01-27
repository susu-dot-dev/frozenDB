# Feature Specification: FBS UUIDv7 Refactor

**Feature Branch**: `026-fbs-uuidv7-refactor`  
**Created**: 2026-01-27  
**Status**: Draft  
**Input**: User description: "026 Refactor FBS (FuzzyBinarySearch) to work with UUIDv7 keys, instead of int64 timestamps. The algorithm should extract the timestamp portion for comparison, but use equality of the entire UUID to find the exact match (during the linear scan portion). With this modification, there may be multiple identical timestamps, but UUID's are unique in the array"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - FBS Algorithm Refactoring for UUIDv7 Keys (Priority: P1)

As a frozenDB developer, I need the FuzzyBinarySearch algorithm to work with UUIDv7 keys instead of raw int64 timestamps so that the algorithm can properly search through UUID-ordered data while maintaining the O(log n) + k performance characteristics.

**Why this priority**: This is a fundamental refactoring that affects the core search algorithm's ability to work with the actual UUIDv7 key format used throughout the database system.

**Independent Test**: Can be fully tested by implementing the refactored FuzzyBinarySearch with UUIDv7 keys and verifying both correctness (100% accurate UUID matching) and performance (O(log n) + k complexity) across comprehensive test scenarios.

**Acceptance Scenarios**:

1. **Given** an array of UUIDv7 keys ordered by timestamp, **When** calling FuzzyBinarySearch with a target UUIDv7, **Then** it returns the exact index of the matching UUID
2. **Given** multiple UUIDv7 keys with identical timestamps in the array, **When** searching for one of them, **Then** it finds the exact UUID match using full UUID equality during linear scan
3. **Given** UUIDv7 keys within the skew window but not matching the target, **When** searching, **Then** it correctly returns KeyNotFoundError
4. **Given** valid search parameters with UUIDv7 keys, **When** calling the refactored FuzzyBinarySearch function, **Then** it executes without runtime errors

---

### Edge Cases

- What happens when multiple UUIDv7 keys have identical timestamps but different random components?
- How does system handle UUIDv7 keys at the boundaries of the skew window?
- What happens when UUIDv7 timestamp extraction encounters invalid UUID formats?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Refactored FuzzyBinarySearch MUST accept UUIDv7 keys instead of int64 timestamps for the target parameter and get callback function, and the UUID's must be validated as a v7 UUId
- **FR-002**: FuzzyBinarySearch MUST extract timestamp portion from UUIDv7 for binary search comparison using ExtractUUIDv7Timestamp function and use the timestamp portion for comparison with the skew, not the full UUID
- **FR-003**: FuzzyBinarySearch MUST use full UUID equality comparison during linear scan portion to find exact matches
- **FR-004**: FuzzyBinarySearch MUST handle multiple UUIDv7 keys with identical timestamps correctly by using UUID equality for final match determination
- **FR-005**: Refactored algorithm MUST preserve O(log n) + k  value lookup complexity where k = count of UUIDv7 entries in [target_timestamp-skewMs, target_timestamp+skewMs]

### Key Entities *(include if feature involves data)*

- **FuzzyBinarySearch**: The main search algorithm that will be refactored to work with UUIDv7 keys
- **UUIDv7 Key**: A 128-bit UUID where the first 48 bits contain the millisecond timestamp used for ordering, with random components ensuring uniqueness
- **Timestamp Extraction**: The process of extracting the 48-bit timestamp from a UUIDv7 for comparison purposes during binary search
- **Linear Scan**: The final phase where full UUID equality is used to find the exact match among UUIDv7 keys with identical timestamps

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

- **SC-001**: FuzzyBinarySearch correctly locates target UUIDv7 keys in 100% of test cases where the target exists within the array
- **SC-002**: FuzzyBinarySearch returns correct KeyNotFoundError in 100% of test cases where target UUIDv7 does not exist
- **SC-003**: Algorithm maintains O(log n) + k performance characteristics with UUIDv7 keys as verified by benchmarks
- **SC-004**: Linear scan portion correctly handles multiple UUIDv7 keys with identical timestamps using UUID equality matching
- **SC-005**: All existing FuzzyBinarySearch test cases continue to pass after refactoring

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-006**: Zero data loss scenarios in UUIDv7 key search correctness tests
- **SC-007**: All UUIDv7 timestamp extractions remain consistent with ExtractUUIDv7Timestamp function
- **SC-008**: Memory usage remains constant regardless of database size (O(1) space complexity preserved)
- **SC-009**: UUIDv7 uniqueness is preserved during search operations - no false positives from timestamp collisions
- **SC-010**: Existing timestamp ordering validation logic continues to work correctly with extracted UUID timestamps

## Breaking changes
This is a breaking change to the 022 spec. Any 022 spec test may be adapted to use a UUIDv7 based key, instead of a timestamp
