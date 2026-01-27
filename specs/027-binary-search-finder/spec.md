# Feature Specification: BinarySearchFinder

**Feature Branch**: `027-binary-search-finder`  
**Created**: 2025-01-26  
**Status**: Draft  
**Input**: User description: "Define a new Finder implementation called BinarySearchFinder. The base premise of the BinarySearchFinder is that it uses the generally ascending timestamp of the UUIDv7 keys to perform a binary search on the database, allowing reads to return in sub-linear time with constant memory overhead. This finder implementation is useful when the database size large or unbounded. If the database size is small and is guaranteed to fit in memory, an InMemoryFinder would be a better choice The spec number is 027. Model the single user story after the 021 in-memory-finder spec. The user wants to combine the fixed-memory benefits of the SimpleFinder with an improved algorithm that takes advantage of ordered keys for faster lookup speed"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Large Database Efficient Lookup (Priority: P1)

As a user working with large or unbounded frozen databases that cannot fit indices in memory, I want to use a BinarySearchFinder so that key lookups complete in sub-linear time O(log n) instead of linear time O(n), while maintaining constant memory usage like SimpleFinder.

**Why this priority**: This is the core value proposition of BinarySearchFinder - providing logarithmic performance for large databases while keeping memory usage constant, addressing the primary limitation of SimpleFinder for large datasets.

**Independent Test**: Can be fully tested by creating a database with known data rows having UUIDv7 keys, initializing a BinarySearchFinder, and verifying that GetIndex operations return correct results with O(log n) performance characteristics while memory usage remains constant regardless of database size.

**Acceptance Scenarios**:

1. **Given** a frozen database with many committed data rows with UUIDv7 keys, **When** I open it with BinarySearchFinder, **Then** GetIndex operations complete in logarithmic time and use constant memory
2. **Given** a database with UUIDv7 keys inserted in generally ascending timestamp order, **When** I search for any key, **Then** binary search correctly identifies the target row or determines it doesn't exist
3. **Given** a database file opened in read-only mode, **When** I perform multiple GetIndex operations, **Then** each operation maintains O(log n) time complexity with fixed memory overhead

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a BinarySearchFinder implementation of the Finder interface
- **FR-002**: BinarySearchFinder MUST use O(log n) disk read operations when finding a key
- **FR-003**: BinarySearchFinder MUST maintain fixed memory usage regardless of database size (similar to SimpleFinder)
- **FR-004**: BinarySearchFinder MUST correctly handle UUIDv7 keys that are generally but not strictly monotonically increasing
- **FR-005**: System MUST allow users to choose BinarySearchFinder using FinderStrategyBinarySearch constant when creating FrozenDB instances
- **FR-006**: BinarySearchFinder MUST pass all finder_conformance_test tests to ensure functional correctness
- **FR-007**: BinarySearchFinder MUST maintain thread-safe access for concurrent GetIndex method calls
- **FR-008**: BinarySearchFinder MUST properly handle Checksum Rows by skipping them during binary search operations
- **FR-009**: DataRow Validate() method MUST reject UUIDs where the non-timestamp part (bytes 7, 9-15) are all zeros, as these represent NullRow UUIDs which are invalid for DataRows
- **FR-010**: BinarySearchFinder GetIndex() method MUST reject search keys that are NullRow UUIDs (detected by checking if non-timestamp part bytes 7, 9-15 are all zeros) before performing binary search

### Key Entities *(include if feature involves data)*

- **BinarySearchFinder**: Logarithmic-time finder implementation that uses binary search on UUIDv7 ordered keys
- **UUIDv7TimestampExtractor**: Helper to extract timestamp components from UUIDv7 keys for search optimization
- **SearchBoundary**: Internal tracking of search range boundaries during binary search operations
- **PerformanceProfile**: Documentation describing O(log n) time complexity and fixed memory characteristics
- **TimestampOrderingStrategy**: Logic to handle generally ascending but potentially non-monotonic UUIDv7 sequences with tolerance for up to 1-second timestamp disorder
- **FuzzyBinarySearch**: Binary search algorithm that handles non-strictly monotonic UUIDv7 keys and checksum row skipping
- **LogicalIndexMapping**: Simple mathematical conversion between logical indices (contiguous data/null row indices) and physical indices (accounting for checksum rows). NullRows have valid logical indices and are included in the logical index space. The mapping formula is: `physicalIndex = logicalIndex + floor(logicalIndex / 10000) + 1`
- **UUID Helper Functions**: Helper functions in `frozendb/uuid_helpers.go` (such as `IsNullRowUUID()`, `ValidateUUIDv7()`, etc.) can be used or extended as needed for UUID validation and NullRow detection. The implementation MAY add new helper functions to `uuid_helpers.go` if additional UUID-related utilities are required for this feature.

### Spec Testing Requirements

Each functional requirement (FR-XXX) MUST have corresponding spec tests that:
- Validate the requirement exactly as specified
- Are placed in `module/[filename]_spec_test.go` where `filename` matches the implementation file being tested
- Follow naming convention `Test_S_027_FR_XXX_Description()`
- MUST NOT be modified after implementation without explicit user permission
- Are distinct from unit tests and focus on functional validation

See `docs/spec_testing.md` for complete spec testing guidelines.

## Success Criteria *(mandatory)*

### Data Integrity & Correctness Metrics *(required for frozenDB)*

- **SC-001**: Zero data integrity differences between BinarySearchFinder and SimpleFinder results across all test scenarios
- **SC-002**: All concurrent read operations maintain search consistency in BinarySearchFinder
- **SC-003**: Binary search correctly handles 100% of UUIDv7 keys even with timestamp ordering edge cases (up to 1-second disorder tolerance)
- **SC-004**: Search algorithm maintains correctness when UUIDv7 timestamps have minor disorder within 1-second tolerance bounds
- **SC-005**: BinarySearchFinder gracefully handles edge cases: empty databases, single-row databases, and databases with only checksum rows
- **SC-006**: DataRow validation correctly rejects all UUIDs with zeroed non-timestamp parts (NullRow UUID pattern)
- **SC-007**: GetIndex() correctly rejects NullRow UUID search keys before performing binary search operations

## Assumptions

- Memory constraints prevent use of InMemoryFinder which requires O(n) memory for indices
- Binary search algorithm will be implemented using file seeking rather than loading data into memory
- The feature builds upon existing Finder interface and conformance testing framework
