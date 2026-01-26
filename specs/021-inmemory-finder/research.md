# InMemoryFinder Research

This document contains research findings that resolve technical unknowns for implementing the InMemoryFinder feature.

## UUIDv7 Integration and Optimization Opportunities

**Decision**: UUIDv7 keys provide natural chronological ordering that enables optimization but requires careful time skew handling.

**Rationale**: frozenDB already uses UUIDv7 keys with embedded timestamps for natural ordering. InMemoryFinder can leverage this time component for potential binary search optimizations during initialization, though the primary O(1) lookup will use hash maps.

**Alternatives considered**: 
- Pure hash-based lookup (selected) - provides guaranteed O(1) performance
- Hybrid approach with temporal indexing (rejected) - adds complexity without clear benefits for target use cases

**Implementation notes**:
- UUIDv7 validation is already handled in existing DataRow processing
- Time skew for distributed systems is already addressed in frozenDB's UUID handling
- No additional UUID validation needed in InMemoryFinder beyond existing patterns

## Memory Management Strategy

**Decision**: Use Go's built-in map types with pre-allocation based on database size during initialization.

**Rationale**: Go's map implementation provides excellent performance for UUID keys with O(1) average lookup time. Pre-allocation reduces reallocation overhead during database reading.

**Alternatives considered**:
- Custom hash table implementation (rejected) - Go's maps are highly optimized
- Sorted slice with binary search (rejected) - O(log n) vs required O(1) performance
- Two-level indexing (rejected) - unnecessary complexity for target database sizes

**Memory usage formula**:
- UUID map: 24 bytes per entry (UUID overhead + map entry overhead) × row count
- Transaction boundary map: 16 bytes per entry (int64 pairs) × row count  
- Total: ~40 bytes per database row

## Thread Safety Implementation

**Decision**: Use sync.RWMutex for concurrent access, allowing multiple readers but exclusive writes during OnRowAdded.

**Rationale**: FrozenDB's transaction system already provides write locking around OnRowAdded calls, so InMemoryFinder only needs to protect its internal state consistency. RWMutex enables optimal concurrent read performance.

**Alternatives considered**:
- Multiple mutexes with sharding (rejected) - adds complexity without clear benefit for target scales
- Lock-free atomic operations (rejected) - complex to implement correctly for map updates
- Single sync.Mutex (rejected) - would prevent concurrent Get* operations

## Integration Strategy

**Decision**: Allow NewFrozenDB to accept finder strategy selection, but leave the specific implementation of strategy-to-implementation mapping to the implementation.

**Rationale**: Provides the necessary user capability to choose finder strategies while avoiding over-specification of internal implementation details like factory patterns.

**Implementation details**:
- NewFrozenDB(filename, mode, strategy) accepts path, access mode, and finder strategy
- Implementation determines how to map strategy values to concrete finder implementations
- No requirement to use existing factory patterns or registration systems

## OnRowAdded Implementation Requirements

**Decision**: OnRowAdded will update both UUID and transaction boundary maps atomically.

**Rationale**: Maintains consistency between finder state and database state. Sequential index validation is already handled by transaction system, so InMemoryFinder can focus on index updates.

**Implementation approach**:
- Validate row type (DataRow for UUID map, all rows for transaction boundaries)
- Update maps based on row type and control bytes
- Handle PartialDataRow states appropriately

## Database Initialization Strategy

**Decision**: Perform full database scan during InMemoryFinder initialization, building complete in-memory index.

**Rationale**: Provides immediate O(1) performance after initialization. Single O(n) scan cost is acceptable for small to medium databases where InMemoryFinder is intended to be used.

**Alternatives considered**:
- Lazy loading (rejected) - would compromise O(1) performance guarantees
- Incremental loading (rejected) - adds complexity without clear benefits
- Background loading (rejected) - potential race conditions during startup

## Performance Optimization Opportunities

**Decision**: Leverage UUIDv7 time component for initialization optimization but use hash maps for all lookup operations.

**Rationale**: Binary search on sorted keys can speed up initialization when database keys are naturally ordered (common in frozenDB usage), but primary value comes from O(1) hash lookups during operation.

**Optimizations implemented**:
- Batch row processing during initialization to reduce per-row overhead
- Pre-allocated maps based on estimated row count
- Minimal memory allocations in hot paths

## Error Handling Integration

**Decision**: Follow existing frozenDB error handling patterns using structured FrozenDBError types.

**Rationale**: Maintains consistency with existing codebase patterns and error handling expectations.

**Error types to use**:
- InvalidInputError: Parameter validation failures
- CorruptDatabaseError: Data format inconsistencies
- ReadError: I/O operation failures (during initialization)

## Testing Strategy Integration

**Decision**: Register InMemoryFinder with existing conformance test system and add performance benchmarks.

**Rationale**: Leverages existing comprehensive test infrastructure while adding specific performance validation for O(1) behavior.

**Testing approach**:
- Register factory for automated conformance testing
- Add benchmarks for GetIndex, GetTransactionStart, GetTransactionEnd
- Performance tests validating <1ms operation times for target database sizes
- Memory usage validation tests

## Configuration API Design

**Decision**: Replace NewFrozenDB signature to require three parameters: filename, mode, strategy.

**Rationale**: Provides type safety and explicit finder strategy choice while retaining mode selection.

**API design**:
- NewFrozenDB(filename string, mode string, strategy FinderStrategy) - three parameters: filename, mode, strategy
- Support FinderStrategySimple and FinderStrategyInMemory constants
- Return InvalidInputError for invalid strategy values
- Mode is required (MODE_READ or MODE_WRITE); validated by NewDBFile
- This is a breaking change approved in specification

## Memory-Performance Trade-off Documentation

**Decision**: Provide clear documentation comparing SimpleFinder vs InMemoryFinder characteristics.

**Rationale**: Enables informed decision making based on use case requirements.

**Documentation points**:
- SimpleFinder: Fixed memory usage, O(n) performance, suitable for large databases
- InMemoryFinder: Scales with database size (~40 bytes/row), O(1) performance, suitable for small/medium databases
- Performance benchmarks and memory usage examples
- Guidelines for when to choose each strategy