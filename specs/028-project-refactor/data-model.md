# Data Model: Project Structure Refactor

**Purpose**: Type organization and package boundaries after the refactor  
**Created**: 2026-01-27  
**Updated**: 2026-01-27  
**Feature**: Project Structure Refactor

## Package Organization Overview

This refactor reorganizes 49 Go files into a clear public/internal structure while respecting Go's type visibility constraints.

## Key Constraint

**Critical Finding**: Only essential types (Header, DataRow, StartControl, EndControl) MUST remain public because they appear in public API signatures:
- `Transaction.Header *Header` (exported struct field)
- `Transaction.GetRows() []DataRow` (return type)
- `DataRow.StartControl StartControl` (field in DataRow)
- `DataRow.EndControl EndControl` (field in DataRow)

**Solution**: Keep only these types in public package. All other types (NullRow, ChecksumRow, PartialDataRow, RowUnion, Finder interface) are internal. StartControl and EndControl types along with all public constants go in `constants.go`, while sentinel byte constants (ROW_START, ROW_END) remain internal.

## Public API Package (pkg/frozendb/)

**Purpose**: Stable public API that external developers can rely on

**Files**: 8 total (down from 49)

**Exported Types**:
- Core types: `FrozenDB`, `Transaction`, `CreateConfig`
- Row types: `DataRow`, `Header`
- Control types: `StartControl`, `EndControl` (required as fields in DataRow)
- Strategy type: `FinderStrategy` (only constants, not interface)
- Error types: All FrozenDBError subtypes

**Exported Constants**:
- Access modes: `MODE_READ`, `MODE_WRITE`
- Finder strategies: `FinderStrategySimple`, `FinderStrategyInMemory`, `FinderStrategyBinarySearch`
- StartControl constants: `START_TRANSACTION`, `ROW_CONTINUE`, `CHECKSUM_ROW`
- EndControl constants: `TRANSACTION_COMMIT`, `ROW_END_CONTROL`, `SAVEPOINT_COMMIT`, `SAVEPOINT_CONTINUE`, `FULL_ROLLBACK`, `NULL_ROW_CONTROL`, `CHECKSUM_ROW_CONTROL`

**Note**: Finder interface is internal. Only FinderStrategy constants are public. Users select finder implementations via strategy constants, not by directly implementing Finder interface.

## Internal Packages

### internal/fields/ - Row Field Structures

**Purpose**: Row field structures, serialization, validation

**Unexported Types**:
- `baseRow[T]` - generic row base type (internal implementation)
- `DataRowPayload`, `NullRowPayload` - internal payload structures
- `NullRow`, `ChecksumRow`, `Checksum` - internal row types (not in public API)
- `PartialDataRow`, `PartialRowState` - internal transaction state
- `RowUnion` - internal polymorphic row parsing (used by Finder interface)
- `RowPayload`, `Validator` - internal interfaces

**Unexported Constants**:
- Sentinel byte constants: `ROW_START`, `ROW_END`, `NULL_BYTE`

**Why These Stay Internal**: Users never directly create or manipulate these types. They work with DataRow which internally uses these implementations. NullRow, ChecksumRow, PartialDataRow, and RowUnion are internal implementation details not exposed in the public API.

### internal/finder/ - Query Implementations

**Purpose**: Row and transaction lookup implementations

**Unexported Types**:
- `Finder` interface - internal abstraction for database lookups
- `SimpleFinder` - O(n) linear search, fixed memory
- `InMemoryFinder` - O(1) lookup with ~40 bytes/row memory
- `BinarySearchFinder` - O(log n) search with time-ordered keys
- `FuzzyBinarySearch` - helper for time-skewed searches

**Why These Stay Internal**: Users select finder strategy via FinderStrategy constants (`FinderStrategySimple`, `FinderStrategyInMemory`, `FinderStrategyBinarySearch`). They never directly interact with Finder interface or implementations - these are internal to `FrozenDB.finder` field.

### internal/fileio/ - File I/O Management

**Purpose**: File operations, locking, validation

**Unexported Types**:
- `DBFile` interface - internal file operations abstraction
- `FileManager` - concrete file manager with flock support
- `Data` - internal write channel communication
- File validation functions

**Why These Stay Internal**: Users never directly interact with file operations. `NewFrozenDB()` handles all file management internally.

## Type Relationships

### Public Type Dependencies

```
FrozenDB
  └── Transaction
       ├── Header (public field)
       └── GetRows() []DataRow
            └── DataRow
                 ├── StartControl (public field)
                 └── EndControl (public field)
```

### Internal Type Dependencies

```
DataRow (public)
  └── DataRowPayload (internal)
       └── baseRow[T] (internal)

NullRow (internal)
  └── NullRowPayload (internal)
       └── baseRow[T] (internal)

Finder (internal)
  └── OnRowAdded(row *RowUnion)
       └── RowUnion (internal)
            ├── DataRow (public)
            ├── NullRow (internal)
            └── ChecksumRow (internal)
```

## Package Boundary Rules

### Public API Rules
1. Only exported types and functions in `pkg/frozendb`
2. Public API can import internal packages
3. All public methods must have corresponding tests
4. Error types must be exported for `errors.As()` checking

### Internal Package Rules
1. Internal packages can import other internal packages
2. Internal packages cannot import `pkg/frozendb` (circular dependency)
3. All internal implementation details are unexported
4. Internal utilities have full test coverage

## Visibility Summary

| Type | Visibility | Location | Reason |
|------|-----------|----------|--------|
| `FrozenDB` | Public | `pkg/frozendb/` | Main database handle |
| `Transaction` | Public | `pkg/frozendb/` | Transaction handle |
| `DataRow` | Public | `pkg/frozendb/` | Returned by `GetRows()` |
| `Header` | Public | `pkg/frozendb/` | Field in `Transaction` |
| `StartControl` | Public | `pkg/frozendb/constants.go` | Field in `DataRow` |
| `EndControl` | Public | `pkg/frozendb/constants.go` | Field in `DataRow` |
| `FinderStrategy` | Public (constants only) | `pkg/frozendb/constants.go` | Parameter to `NewFrozenDB()` |
| `NullRow` | Internal | `internal/fields/` | Not in public API |
| `ChecksumRow` | Internal | `internal/fields/` | Not in public API |
| `PartialDataRow` | Internal | `internal/fields/` | Not in public API |
| `RowUnion` | Internal | `internal/fields/` | Used by internal Finder interface |
| `Finder` interface | Internal | `internal/finder/` | Only strategy constants are public |
| `SimpleFinder` | Internal | `internal/finder/` | Implementation detail |
| `InMemoryFinder` | Internal | `internal/finder/` | Implementation detail |
| `BinarySearchFinder` | Internal | `internal/finder/` | Implementation detail |
| `DBFile` interface | Internal | `internal/fileio/` | File operations abstraction |
| `FileManager` | Internal | `internal/fileio/` | File operations implementation |
