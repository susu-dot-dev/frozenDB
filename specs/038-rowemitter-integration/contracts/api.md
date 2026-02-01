# API Contract: RowEmitter Integration

**Feature**: 038-rowemitter-integration  
**Date**: 2026-02-01  
**Package**: `internal/frozendb`  
**Status**: Complete

## Overview

This document defines the API changes for RowEmitter integration. All changes are internal to `internal/frozendb` - no public API changes in `pkg/frozendb`.

---

## API Changes

### 1. Finder Interface - Remove OnRowAdded

**Before**:
```go
type Finder interface {
    GetIndex(key uuid.UUID) (int64, error)
    GetTransactionStart(index int64) (int64, error)
    GetTransactionEnd(index int64) (int64, error)
    OnRowAdded(index int64, row *RowUnion) error  // ← REMOVED
    MaxTimestamp() int64
}
```

**After**:
```go
type Finder interface {
    GetIndex(key uuid.UUID) (int64, error)
    GetTransactionStart(index int64) (int64, error)
    GetTransactionEnd(index int64) (int64, error)
    MaxTimestamp() int64
}
```

**Impact**: Internal breaking change. All Finder implementations must remove OnRowAdded from their method set.

---

### 2. NewSimpleFinder - Add RowEmitter Parameter

**Before**:
```go
func NewSimpleFinder(dbFile DBFile, rowSize int32) (*SimpleFinder, error)
```

**After**:
```go
func NewSimpleFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*SimpleFinder, error)
```

**Implementation**:
- Subscribe to RowEmitter in constructor: `_, err := rowEmitter.Subscribe(sf.onRowAdded)`
- Implement `onRowAdded` method to update file size
- Discard unsubscribe function (no cleanup needed)

---

### 3. NewInMemoryFinder - Add RowEmitter Parameter

**Before**:
```go
func NewInMemoryFinder(dbFile DBFile, rowSize int32) (*InMemoryFinder, error)
```

**After**:
```go
func NewInMemoryFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*InMemoryFinder, error)
```

**Implementation**:
- Subscribe to RowEmitter in constructor: `_, err := rowEmitter.Subscribe(imf.onRowAdded)`
- Rename `OnRowAdded` to `onRowAdded` (internal method)
- Discard unsubscribe function (no cleanup needed)

---

### 4. NewBinarySearchFinder - Add RowEmitter Parameter

**Before**:
```go
func NewBinarySearchFinder(dbFile DBFile, rowSize int32) (*BinarySearchFinder, error)
```

**After**:
```go
func NewBinarySearchFinder(dbFile DBFile, rowSize int32, rowEmitter *RowEmitter) (*BinarySearchFinder, error)
```

**Implementation**:
- Subscribe to RowEmitter in constructor: `_, err := rowEmitter.Subscribe(bsf.onRowAdded)`
- Rename `OnRowAdded` to `onRowAdded` (internal method)
- Discard unsubscribe function (no cleanup needed)

---

### 5. NewFrozenDB - Add RowEmitter Creation

**Signature**: Unchanged

**Implementation Change**:
```go
func NewFrozenDB(path string, mode string, strategy FinderStrategy) (*FrozenDB, error) {
    dbFile, err := NewDBFile(path, mode)
    // ... validate header ...
    
    // NEW: Create RowEmitter
    rowEmitter, err := NewRowEmitter(dbFile, int(rowSize))
    if err != nil {
        return nil, err
    }
    
    // MODIFIED: Pass rowEmitter to all Finder implementations
    var finder Finder
    switch strategy {
    case FinderStrategySimple:
        finder, err = NewSimpleFinder(dbFile, rowSize, rowEmitter)
    case FinderStrategyInMemory:
        finder, err = NewInMemoryFinder(dbFile, rowSize, rowEmitter)
    case FinderStrategyBinarySearch:
        finder, err = NewBinarySearchFinder(dbFile, rowSize, rowEmitter)
    }
    
    // Note: rowEmitter not stored in FrozenDB struct
    db := &FrozenDB{
        file:   dbFile,
        header: header,
        finder: finder,
    }
    
    return db, nil
}
```

---

### 6. Transaction - Remove OnRowAdded Calls

**No signature changes**

**Implementation Change**:
- Remove all `tx.finder.OnRowAdded(index, row)` calls
- Remove OnRowAdded error handling
- Notification happens automatically via RowEmitter

**Note**: Transaction still has `finder Finder` field for operations like `MaxTimestamp()`.

---

## No Public API Changes

All public APIs in `pkg/frozendb` remain unchanged:
- `frozendb.NewFrozenDB(path, mode, strategy)` - External signature identical
- All Transaction methods - Unchanged
- All FinderStrategy constants - Unchanged

---

## Testing

**New Spec Test Required**:
- `Test_S_038_FR_007_RowEmitter_Delivers_Notifications_Correctly` in `frozendb_spec_test.go`
- Validates notification pipeline works correctly

**Existing Tests**:
- All existing spec tests must pass without modification (100% pass rate)

---

## References

- Feature Spec: `/specs/038-rowemitter-integration/spec.md`
- Research Document: `/specs/038-rowemitter-integration/research.md`
- Data Model: `/specs/038-rowemitter-integration/data-model.md`
