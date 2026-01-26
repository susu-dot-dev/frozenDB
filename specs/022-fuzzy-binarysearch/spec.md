# Feature Specification: Fuzzy Binary Search

**Feature Branch**: `022-fuzzy-binarysearch`  
**Created**: 2026-01-25  
**Status**: Draft  
**Input**: User description: "022 fuzzy-binarysearch. This spec covers the core technical capability to search an array for a given key. The twist is that the array is not strictly in ascending order, so a normal binary search won't work. Instead, each entry may be out of order based on skew_ms. (Full details are in docs/v1_file_format.md). Since this logic is complicated, before making an entirely new Finder instance, we want to abstract away and make sure we have the FuzzyBinarySearch (FBS) implementation correct. Given a target and a skew, FBS should locate the index of that target, or return KeyNotFoundError. This should happen in roughly O(logn) + k time, where k is the number of keys that have been inserted within +- the skew of the target time, since every entry in that range will have to be linearly searched. Since we want to focus on the algorithm, and not the file structure or row structure, this spec should abstract away the details. So, for example, FBS should not read FrozenDB rows. Instead, it should be told the # of keys, along with a callback function get(index int64) which will return a unix timestamp in milliseconds for that index, or KeyNotFoundError if not present."

## Clarifications

### Session 2026-01-25

- Q: What is the expected maximum array size (number of keys) that the FuzzyBinarySearch algorithm should handle efficiently? → A: Not needed to be specified, as binary search scales regardless of the array size
- Q: What is the maximum acceptable skew_ms value that the algorithm must support? → A: Directly reference the v1_file_format.md for requirements around skew
- Q: What are the specific performance targets for the O(logn) + k complexity? → A: No specific timing numbers, just base it on number of callbacks to get the key
- Q: Should the algorithm handle concurrent access to the callback function during search? → A: The algorithm should be thread-safe, however there are some important simplifications: The underlying data will never change. The callback must be thread-safe, and always return the same data. So, the only thread safety would be if there were any global data structures when calling the function twice. Otherwise, e.g. the stack will be all that is needed for thread safety
- Q: What timestamp format should the callback function return? → A: Unix milliseconds (int64)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Robust Fuzzy Binary Search Algorithm (Priority: P1)

As a system developer, I need a reliable fuzzy binary search algorithm that can handle out-of-order entries within a skew window while maintaining O(logn) performance, so that I can accurately locate timestamps in datasets where clock skew prevents standard binary search from working correctly.

**Why this priority**: This core algorithm must be both performant and perfectly reliable since it's not a straightforward binary search - the complexity introduced by clock skew makes correctness critical, and poor performance would make the database impractical for production workloads.

**Independent Test**: Can be fully tested by implementing the FuzzyBinarySearch algorithm with controlled test datasets and verifying both correctness (100% accurate results) and performance (O(logn) + k complexity) across comprehensive scenarios.

**Acceptance Scenarios**:

1. **Given** a dataset where entries may be out of order but only within the allowed skew window, **When** searching for an existing timestamp, **Then** the algorithm returns the exact index of that unique entry
2. **Given** a dataset of timestamps, **When** searching for a timestamp that doesn't exist, **Then** the algorithm returns KeyNotFoundError
3. **Given** an empty dataset, **When** searching for any timestamp, **Then** the algorithm returns KeyNotFoundError
4. **Given** valid search parameters, **When** calling the FuzzyBinarySearch function, **Then** it executes without runtime errors

---

### Performance and Reliability Requirements

The FuzzyBinarySearch algorithm must meet specific performance characteristics and handle edge cases reliably:

**Performance Requirements**:
- Search time complexity: O(logn) + k where k is the number of entries within ±skew of the target
- Space complexity: O(1) - constant memory usage regardless of array size
- Algorithm should scale logarithmically with array size for minimal skew scenarios
- Performance measured by number of callback function calls, not specific timing targets

**Edge Case Handling**:
- Proper error propagation when callback functions return errors
- Correct handling of empty arrays and single-element arrays  
- Validation of input parameters with appropriate error responses
- Graceful handling of timestamps outside the array's range

**Integration Requirements**:
- Must work with the existing frozenDB error handling patterns
- Callback function signature: get(index int64) (int64, error) returning Unix milliseconds
- Must handle arrays of any size supported by the system (binary search scales regardless of size)
- Algorithm must be thread-safe, assuming underlying data never changes and callback is thread-safe
- Skew window limits as defined in v1_file_format.md (0-86400000ms)

---

### Edge Cases

- What happens when the callback function returns errors for some indices but not others? [Answer: Errors should be properly propagated]
- How does system handle arrays where all entries are within the skew window of each other? [Answer: Algorithm degrades to linear search within valid bounds]
- What happens when the target timestamp is outside the range of all entries in the array? [Answer: Returns KeyNotFoundError]

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: FuzzyBinarySearch MUST accept parameters for target timestamp (Unix milliseconds), skew window (as per v1_file_format.md), number of keys, and a function to access timestamps by index, and return the exact index of the unique target timestamp if found, or KeyNotFoundError if not found
- **FR-002**: The algorithm MUST perform at most O(logn) + k Get() function calls where k is the number of entries within ±skew of the target
- **FR-003**: FuzzyBinarySearch MUST handle datasets where entries may be out of order due to clock skew up to the configured skew_ms value
- **FR-004**: The algorithm MUST properly propagate KeyNotFoundError from the timestamp access function

### Key Entities *(include if feature involves data)*

- **FuzzyBinarySearch**: The main search algorithm implementation that handles fuzzy time-based lookups
- **TimestampAccess**: Function for accessing Unix millisecond timestamps by index, enabling data-structure agnostic access
- **SearchResult**: Return type containing either the found index or an error

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_022_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: FuzzyBinarySearch correctly locates target timestamps in 100% of test cases where the target exists within the array
- **SC-002**: Algorithm returns KeyNotFoundError for 100% of test cases where the target does not exist in the array
- **SC-003**: Search performance scales logarithmically with array size, achieving O(logn) + k complexity measured by callback function call count in benchmark tests
- **SC-004**: Algorithm handles all edge cases (empty arrays, single elements, callback errors) without crashes or incorrect results

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-005**: Zero false positive results in correctness tests (algorithm never returns wrong index)
- **SC-006**: Zero false negative results in correctness tests (algorithm never fails to find existing targets)
- **SC-007**: All callback errors are properly propagated without data corruption
- **SC-008**: Memory usage remains constant regardless of array size (O(1) space complexity)
- **SC-009**: Algorithm is thread-safe for concurrent read operations assuming immutable underlying data and thread-safe callback
