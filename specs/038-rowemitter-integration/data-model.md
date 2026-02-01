# Data Model: RowEmitter Integration

**Feature**: 038-rowemitter-integration  
**Date**: 2026-02-01  
**Status**: Complete

## Overview

This is primarily an internal refactoring with minimal data model changes. This document captures the few actual data model changes introduced by the RowEmitter integration.

## Data Model Changes

### 1. NewFrozenDB Initialization Sequence

**Change**: Add RowEmitter creation step during initialization.

**New Initialization Order**:
1. Open and validate DBFile
2. Read and validate header
3. **Create RowEmitter** (new step)
4. Create Finder with RowEmitter parameter
5. Recover transaction state

**Implementation**:
```go
func NewFrozenDB(...) (*FrozenDB, error) {
    dbFile := ...
    header := ...
    
    // NEW: Create RowEmitter to wire components together
    rowEmitter, err := NewRowEmitter(dbFile, int(rowSize))
    
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

**Rationale**: RowEmitter acts as initialization glue to wire DBFile and Finder together. Not stored after initialization.

### 2. Notification Flow

**Before**:
```
Transaction → finder.OnRowAdded(index, row)
```

**After**:
```
Transaction → DBFile → RowEmitter → Finder callback
```

**Key Change**: Notification flow is indirect through RowEmitter subscription instead of direct method call.

## No Other Data Model Changes

- FrozenDB struct: unchanged (no new fields)
- Finder structs: unchanged (no new fields)
- Transaction struct: unchanged (still has finder field)
- Row formats: unchanged
- File format: unchanged
- Transaction semantics: unchanged

## References

- Feature Spec: `/specs/038-rowemitter-integration/spec.md`
- Research Document: `/specs/038-rowemitter-integration/research.md`
- API Contract: `/specs/038-rowemitter-integration/contracts/api.md`
