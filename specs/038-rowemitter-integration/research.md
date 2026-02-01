# Research: RowEmitter Integration

**Feature**: 038-rowemitter-integration  
**Date**: 2026-02-01  
**Status**: Complete

## Overview

This document captures the research findings for integrating RowEmitter as the centralized notification hub for row completion events, replacing the direct Transaction→Finder coupling with a decoupled RowEmitter-based subscription model.

## Current Architecture Analysis

### Existing Notification Flow

The current notification flow follows this pattern:

1. **Transaction** writes row data to DBFile via `writeChan`
2. **DBFile (FileManager)** writes bytes to disk in `writerLoop()`
3. **DBFile** notifies subscribers (no current subscribers)
4. **Transaction** calls `finder.OnRowAdded(index, row)` directly after write completes
5. **Finder** updates internal state (indexes, transaction boundaries, maxTimestamp)

**Key Finding**: The direct Transaction→Finder coupling means Transaction must know about Finder's existence and call its OnRowAdded method. This creates tight coupling and requires Transaction to handle finder notification failures.

### RowEmitter Implementation (Already Exists)

Analysis of `internal/frozendb/row_emitter.go` reveals:

- **NewRowEmitter(dbFile DBFile, rowSize int)**: Creates RowEmitter with DBFile subscription
- **Subscribe(callback func(index int64, row *RowUnion) error)**: Registers callbacks for row notifications
- **onDBFileNotification()**: Handles DBFile write events, calculates completed rows, reads row data, and notifies subscribers
- **Close()**: Cleans up DBFile subscription

**Key Finding**: RowEmitter already has full infrastructure for:
- Subscribing to DBFile write events
- Calculating which rows are complete based on file size changes
- Reading row data from disk
- Notifying multiple subscribers with (index, row) pairs
- Handling errors in notification chain

### Finder Interface Current API

From `internal/frozendb/finder.go`, the Finder interface includes:

```go
type Finder interface {
    GetIndex(key uuid.UUID) (int64, error)
    GetTransactionStart(index int64) (int64, error)
    GetTransactionEnd(index int64) (int64, error)
    OnRowAdded(index int64, row *RowUnion) error
    MaxTimestamp() int64
}
```

**Key Finding**: OnRowAdded is part of the public Finder interface contract. Removing it is a breaking change to the internal API.

### Finder Constructor Signatures

**SimpleFinder**:
```go
func NewSimpleFinder(dbFile DBFile, rowSize int32) (*SimpleFinder, error)
```

**InMemoryFinder**:
```go
func NewInMemoryFinder(dbFile DBFile, rowSize int32) (*InMemoryFinder, error)
```

**BinarySearchFinder**:
```go
func NewBinarySearchFinder(dbFile DBFile, rowSize int32) (*BinarySearchFinder, error)
```

**Key Finding**: All three finder constructors currently take `(dbFile DBFile, rowSize int32)`. They need to accept `rowEmitter *RowEmitter` instead to enable subscription during initialization.

## Technical Decisions

### Decision 1: RowEmitter Subscription Timing

**Question**: When should RowEmitter subscribe to DBFile and when should Finders subscribe to RowEmitter?

**Analysis**:
- RowEmitter must subscribe to DBFile during `NewRowEmitter()` initialization
- Finders must subscribe to RowEmitter during their constructor (`New*Finder()`)
- This ensures all components are wired before any writes occur

**Decision**: 
- RowEmitter subscribes to DBFile in `NewRowEmitter()` (already implemented)
- Finders subscribe to RowEmitter in their constructors
- NewFrozenDB creates RowEmitter, then passes it to Finder constructors

**Rationale**: Early subscription ensures no events are missed. The initialization sequence (DBFile → RowEmitter → Finder) guarantees the notification pipeline is complete before database operations begin.

**Alternatives Considered**:
- Lazy subscription (subscribe on first write): Rejected because it adds complexity and risks missing events if initialization order changes
- Manual wiring after construction: Rejected because it violates constructor completeness principle (objects should be fully initialized after construction)

### Decision 2: Finder Constructor API Changes

**Question**: How should Finder constructors accept RowEmitter?

**Options Analyzed**:
1. Replace `dbFile DBFile` with `rowEmitter *RowEmitter` entirely
2. Add `rowEmitter *RowEmitter` as additional parameter alongside `dbFile`
3. Keep `dbFile` and derive RowEmitter internally

**Decision**: Replace `dbFile DBFile` with `rowEmitter *RowEmitter` in all constructor signatures.

**New Constructor Signatures**:
```go
func NewSimpleFinder(rowEmitter *RowEmitter, rowSize int32) (*SimpleFinder, error)
func NewInMemoryFinder(rowEmitter *RowEmitter, rowSize int32) (*InMemoryFinder, error)
func NewBinarySearchFinder(rowEmitter *RowEmitter, rowSize int32) (*BinarySearchFinder, error)
```

**Rationale**: 
- Finders still need DBFile for reading rows during GetIndex/GetTransactionStart/GetTransactionEnd operations
- RowEmitter already holds a reference to DBFile (`rowEmitter.dbfile`)
- Finders can access DBFile via `rowEmitter.dbfile` for read operations
- This approach minimizes parameter count while ensuring RowEmitter is the primary interface

**Alternatives Considered**:
- Keep both parameters: Rejected because it creates redundancy (RowEmitter already has DBFile)
- Create RowEmitter internally in Finder: Rejected because it violates single responsibility (Finders shouldn't manage DBFile subscriptions)

**IMPORTANT CORRECTION**: Upon further analysis, the RowEmitter struct has an unexported `dbfile` field. This means Finders cannot access it directly. We need to either:

1. Export the DBFile field in RowEmitter, OR
2. Pass both RowEmitter and DBFile to Finder constructors

**Revised Decision**: Add `rowEmitter *RowEmitter` as additional parameter alongside `dbFile`:

```go
func NewSimpleFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*SimpleFinder, error)
func NewInMemoryFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*InMemoryFinder, error)
func NewBinarySearchFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*BinarySearchFinder, error)
```

**Rationale**: 
- Maintains backward compatibility with read operations that need DBFile
- Clearly separates read operations (via DBFile) from notifications (via RowEmitter)
- Avoids exposing RowEmitter internal DBFile field

### Decision 3: Removing OnRowAdded from Finder Interface

**Question**: How to handle the removal of OnRowAdded from the Finder interface?

**Analysis**:
- OnRowAdded is currently called by Transaction after each row write
- With RowEmitter integration, Finders receive notifications via subscription callbacks
- The subscription callback signature is `func(index int64, row *RowUnion) error`
- This matches OnRowAdded's signature exactly

**Decision**: Remove OnRowAdded from Finder interface and replace with internal subscription callback.

**Implementation Pattern** (for each Finder):
```go
func NewSimpleFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*SimpleFinder, error) {
    sf := &SimpleFinder{
        dbFile:  dbFile,
        rowSize: rowSize,
        size:    dbFile.Size(),
    }
    
    // Subscribe to RowEmitter with onRowAdded as internal callback
    unsubscribe, err := rowEmitter.Subscribe(sf.onRowAdded)
    if err != nil {
        return nil, err
    }
    sf.unsubscribe = unsubscribe // Store for cleanup
    
    return sf, nil
}

// onRowAdded becomes internal method (same implementation as current OnRowAdded)
func (sf *SimpleFinder) onRowAdded(index int64, row *RowUnion) error {
    // Same implementation as current OnRowAdded
}
```

**Rationale**: 
- Removes OnRowAdded from public interface contract
- Makes notification handling internal to Finder implementation
- Enables proper cleanup via unsubscribe function
- Maintains identical notification logic

**Alternatives Considered**:
- Keep OnRowAdded as internal method but don't call it: Rejected because it leaves dead code
- Create wrapper callback: Rejected because it adds unnecessary indirection when implementation is identical

### Decision 4: Transaction Changes

**Question**: What changes are needed in Transaction to remove Finder coupling?

**Analysis** of `internal/frozendb/transaction.go`:
- Transaction currently calls `tx.finder.OnRowAdded(index, row)` after writes
- Need to identify all call sites and remove them
- Transaction should not be aware of Finder or notification mechanism

**Decision**: Remove all `OnRowAdded` calls from Transaction.

**Search Pattern**: Look for `finder.OnRowAdded` or `OnRowAdded` calls in transaction.go

**Rationale**: 
- Transaction only needs to write data to DBFile
- DBFile notifies RowEmitter automatically
- RowEmitter notifies Finders automatically
- No direct Transaction→Finder coupling needed

**Migration Path**:
1. Identify all OnRowAdded call sites in Transaction
2. Remove the calls and any error handling specific to OnRowAdded
3. Verify Transaction no longer references Finder for notifications

### Decision 5: NewFrozenDB Initialization Sequence

**Question**: What is the correct initialization sequence in NewFrozenDB?

**Current Sequence**:
1. Open/validate DBFile
2. Read and validate header
3. Create Finder (passing dbFile, rowSize)
4. Recover transaction state

**Required New Sequence**:
1. Open/validate DBFile
2. Read and validate header
3. Create RowEmitter (passing dbFile, rowSize)
4. Subscribe RowEmitter to DBFile (done in NewRowEmitter)
5. Create Finder (passing dbFile, rowSize, rowEmitter)
6. Finder subscribes to RowEmitter (done in New*Finder)
7. Recover transaction state

**Decision**: Implement the new sequence as specified above.

**Implementation Pattern**:
```go
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error) {
    // 1. Open/validate DBFile
    dbFile, err := NewDBFile(path, mode)
    if err != nil {
        return nil, err
    }
    
    // 2. Read and validate header
    header, err := validateDatabaseFile(dbFile)
    if err != nil {
        return nil, err
    }
    
    rowSize := int32(header.GetRowSize())
    
    // 3. Create RowEmitter (subscribes to DBFile internally)
    rowEmitter, err := NewRowEmitter(dbFile, int(rowSize))
    if err != nil {
        return nil, err
    }
    
    // 4. Create Finder (subscribes to RowEmitter internally)
    var finder Finder
    switch strategy {
    case FinderStrategySimple:
        finder, err = NewSimpleFinder(dbFile, rowSize, rowEmitter)
    case FinderStrategyInMemory:
        finder, err = NewInMemoryFinder(dbFile, rowSize, rowEmitter)
    case FinderStrategyBinarySearch:
        finder, err = NewBinarySearchFinder(dbFile, rowSize, rowEmitter)
    }
    if err != nil {
        return nil, err
    }
    
    // 5. Create FrozenDB and recover transaction state
    db := &FrozenDB{
        file:   dbFile,
        header: header,
        finder: finder,
        // Note: rowEmitter not stored - it's just wiring during initialization
    }
    
    if err := db.recoverTransaction(); err != nil {
        return nil, err
    }
    
    return db, nil
}
```

**Rationale**: This sequence ensures the notification pipeline is fully wired before any operations occur. Each component is initialized in dependency order. The RowEmitter acts as glue during initialization but doesn't need to be retained - it continues running via its DBFile subscription.

### Decision 6: No Cleanup Required

**Question**: Does RowEmitter or its subscriptions need explicit cleanup?

**Analysis**:
- RowEmitter.Subscribe() returns an unsubscribe function
- Finders receive this function but don't need to call it
- No explicit cleanup required when database closes

**Decision**: No cleanup logic needed. Let subscriptions and goroutines naturally terminate.

**Rationale**: 
- When FrozenDB closes DBFile, the writer goroutine stops
- No more notifications will be sent, so subscriptions become inactive naturally
- Go's garbage collector will clean up subscription structures
- No risk of goroutine leaks since writer goroutine is controlled by DBFile lifecycle
- Simpler code without unnecessary cleanup logic

**No Changes Needed**:
- Finders don't need to store unsubscribe function
- FrozenDB doesn't need to store RowEmitter reference
- FrozenDB.Close() doesn't need RowEmitter cleanup
- No Close() method needed on Finder implementations

## Integration Patterns

### Notification Flow After Integration

The new notification flow:

1. **Transaction** writes row data to DBFile via `writeChan`
2. **DBFile (FileManager)** writes bytes to disk in `writerLoop()`
3. **DBFile** notifies all subscribers (including RowEmitter)
4. **RowEmitter.onDBFileNotification()** receives notification
5. **RowEmitter** calculates new complete rows from file size change
6. **RowEmitter** reads each new row from disk
7. **RowEmitter** notifies all subscriber callbacks with (index, row)
8. **Finder.onRowAdded()** receives callback and updates internal state
9. Transaction completes without any Finder awareness

**Key Properties**:
- Transaction has zero knowledge of Finder or RowEmitter
- DBFile has zero knowledge of RowEmitter semantics (just subscription API)
- RowEmitter acts as translation layer (file writes → row completions)
- Finders receive identical notifications as before, just via different path

### Error Handling

**Current Behavior**: Transaction checks OnRowAdded errors and fails transaction if error occurs.

**New Behavior**: 
- RowEmitter.onDBFileNotification() returns first subscriber error
- DBFile subscription mechanism can handle errors from callbacks
- If Finder notification fails, the error propagates back to Transaction via DBFile write result

**Decision**: Maintain identical error handling semantics. First Finder error stops notification chain and propagates to Transaction.

**Rationale**: This preserves current transactional guarantees - if Finder update fails, Transaction knows about it and can handle appropriately.

## Testing Strategy

### Existing Test Coverage

**Key Finding**: Spec states "most functional requirements do not require new spec tests" because this is internal refactoring. Primary validation is existing spec tests passing unchanged.

### New Spec Test Required

**FR-007** requires one integration spec test:
- Test Name: `Test_S_038_FR_007_RowEmitter_Delivers_Notifications_Correctly`
- Location: `internal/frozendb/frozendb_spec_test.go`
- Purpose: Validate that when rows are written, Finders receive notifications in correct order with accurate (index, row) data through RowEmitter subscription

### Validation Approach

1. **Run existing spec test suite** - Must pass 100% without modification
2. **Run new FR-007 integration test** - Validates notification pipeline
3. **Manual code review** - Verify initialization sequence in NewFrozenDB
4. **Memory leak testing** - Confirm proper subscription cleanup

## Dependencies and Risks

### Dependencies

- RowEmitter implementation (already exists and is functional)
- DBFile subscription mechanism (already exists and is functional)
- Subscriber generic implementation (already exists at `internal/frozendb/subscriber.go`)

### Risks

**Risk 1: Initialization Order Bugs**
- **Impact**: If RowEmitter or Finders aren't initialized in correct order, notifications may be missed
- **Mitigation**: Clear documentation of initialization sequence, validation in tests

**Risk 2: Error Propagation Changes**
- **Impact**: If error handling changes subtly, transaction guarantees could be violated
- **Mitigation**: Careful review of error paths, ensure first-error-stops-chain semantics preserved

**Risk 3: Breaking Existing Behavior**
- **Impact**: Subtle timing or ordering differences could break existing functionality
- **Mitigation**: Comprehensive existing spec test suite must pass 100%

## Open Questions

None - all technical decisions resolved through code analysis and architecture review.

## References

- Feature Spec: `/specs/038-rowemitter-integration/spec.md`
- RowEmitter Implementation: `/internal/frozendb/row_emitter.go`
- Finder Interface: `/internal/frozendb/finder.go`
- Finder Implementations: `/internal/frozendb/{simple,inmemory,binary_search}_finder.go`
- FrozenDB Initialization: `/internal/frozendb/frozendb.go`
- Transaction: `/internal/frozendb/transaction.go`
- Subscriber Implementation: `/internal/frozendb/subscriber.go`
