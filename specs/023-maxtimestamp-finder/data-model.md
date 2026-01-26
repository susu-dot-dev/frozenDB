# Data Model: MaxTimestamp Finder Protocol Enhancement

**Date**: 2026-01-26  
**Purpose**: Data entities and validation rules for MaxTimestamp optimization

## New Entity Definitions

### InMemoryFinder Enhancement

**New field**: `maxTimestamp int64`
- **Purpose**: Tracks the maximum timestamp among all complete data and null rows
- **Initialization**: Set to 0 at start of `buildIndex()`, updated during row processing
- **Scope**: Internal state only, exposed through `MaxTimestamp()` method
- **Validation**: Must be >= 0, only increased (never decreased) during operation

### SimpleFinder Enhancement

**New fields**:
```go
maxTimestamp     int64
mu               sync.RWMutex
```
- **Purpose**: Maintain maximum timestamp with thread safety
- **maxTimestamp**: Same semantics as InMemoryFinder, initialized during creation
- **mu**: Mutex for protecting maxTimestamp during concurrent access and updates
- **Validation**: Same timestamp constraints as InMemoryFinder

## Modified Data Structures

### Transaction Struct (Modified)

**Removed field**: `maxTimestamp int64`
- **Impact**: Eliminates redundant storage and synchronization complexity
- **Migration**: Existing `GetMaxTimestamp()` method will delegate to `tx.finder.MaxTimestamp()`
- **State transition**: Value is now computed on-demand rather than stored

**Behavior changes**:
- `AddRow()`: No longer updates maxTimestamp field
- `recoverTransaction()`: No longer calculates maxTimestamp from recovered rows
- `GetMaxTimestamp()`: Now returns `tx.finder.MaxTimestamp()` instead of stored field

### Finder Interface (Modified)

**New method**: `MaxTimestamp() int64`
- **Purpose**: O(1) retrieval of maximum timestamp among complete data and null rows
- **Return value**: 0 if no complete data or null rows exist, otherwise maximum timestamp
- **Implementation requirement**: Must be O(1) time complexity
- **Thread safety**: Must be safe for concurrent read access

## Row Type Classification for MaxTimestamp

### DataRow
- **Contributes to MaxTimestamp**: Yes
- **Timestamp source**: Extracted from UUIDv7 key using existing timestamp extraction logic
- **Completeness requirement**: Must be complete DataRow (not PartialDataRow)
- **Validation**: UUID must be valid version 7, timestamp must be >= 0

### NullRow  
- **Contributes to MaxTimestamp**: Yes
- **Timestamp source**: Extracted from UUIDv7 key
- **Completeness requirement**: Must be complete NullRow
- **Validation**: Same as DataRow

### ChecksumRow
- **Contributes to MaxTimestamp**: No
- **Reason**: Integrity check rows with no temporal significance
- **Handling**: Ignored during max timestamp calculation

### PartialDataRow
- **Contributes to MaxTimestamp**: No
- **Reason**: Incomplete transaction entries, temporal significance only after commit/rollback
- **Handling**: Ignored during max timestamp calculation
- **State transition**: Becomes DataRow/NullRow after transaction completion, then contributes

## Validation Rules

### MaxTimestamp Value Validation
- **Range constraint**: Must be >= 0
- **Monotonicity**: Never decreases during normal operation
- **Initial value**: 0 for empty databases or databases with only checksum/PartialDataRow entries

### Row Processing Validation
- **Row type verification**: Must correctly identify DataRow vs NullRow vs ChecksumRow vs PartialDataRow
- **UUID validation**: All contributing rows must have valid UUIDv7 keys
- **Timestamp extraction**: Must use consistent timestamp extraction logic across all finder implementations
- **Completeness check**: Must distinguish between complete and incomplete rows

### Finder Implementation Validation
- **Time complexity**: All implementations must guarantee O(1) access time
- **Thread safety**: Must handle concurrent read access without corruption
- **Consistency**: All implementations must return identical results for same database state

## State Transitions

### Database State Changes Affecting MaxTimestamp

**Transaction Commit**:
1. PartialDataRow entries are converted to DataRow/NullRow
2. New complete rows may increase maxTimestamp
3. Finder implementations update their cached maxTimestamp if applicable

**Transaction Rollback**:
1. PartialDataRow entries are discarded (no impact on maxTimestamp)
2. No change to maxTimestamp since PartialDataRows never contributed

**Database Recovery**:
1. During opening, finder processes all rows to rebuild state
2. maxTimestamp is calculated based on complete rows only
3. Recovery of incomplete transactions should not affect maxTimestamp

### Finder Cache State Transitions

**InMemoryFinder cache states**:
- `BUILDING`: During buildIndex() execution
- `READY`: After buildIndex() completion, maxTimestamp is available
- No invalidation required in append-only architecture

**SimpleFinder state**: Always in correct state when initialized. maxTimestamp is calculated during NewSimpleFinder() creation and maintained incrementally via OnRowAdded() callbacks. No state transitions required beyond initialization.

## Error Condition Mappings

### Error Conditions
- **Corrupted timestamp data**: Return existing FileCorruptionError for file corruption scenarios
- **Concurrent access conflicts**: Handled internally with mutex, no external error needed
- All implementations maintain proper synchronization without exposing additional error types

## Data Flow Relationships

### Row Processing Flow
```
Database File → Row Parsing → Row Type Identification → [If DataRow/NullRow] → Timestamp Extraction → Max Update → Finder.maxTimestamp
```

### Transaction Query Flow
```
Transaction.GetMaxTimestamp() → tx.finder.MaxTimestamp() → [Implementation-specific] → Return cached or calculated value
```

### Finder Initialization Flow
```
Database Open → Finder Creation → [SimpleFinder: lazy calc] OR [InMemoryFinder: buildIndex with max tracking] → Ready State
```

## Memory Impact Analysis

### Per-Database Memory Changes
- **Transaction struct**: -8 bytes (removed maxTimestamp field)
- **InMemoryFinder**: +8 bytes (added maxTimestamp field)
- **SimpleFinder**: +17 bytes (maxTimestamp + bool + sync.RWMutex overhead)
- **Net change**: Small increase per database instance

### Scalability Characteristics
- **InMemoryFinder**: Memory increase is constant regardless of database size
- **SimpleFinder**: Memory increase is constant regardless of database size
- **Transaction scaling**: Improved due to reduced struct size and synchronization

### Performance Characteristics
- **Read operations**: O(1) for MaxTimestamp() after initialization
- **Write operations**: No additional overhead for InMemoryFinder, minimal for SimpleFinder cache invalidation
- **Recovery operations**: Same complexity as existing index rebuilding