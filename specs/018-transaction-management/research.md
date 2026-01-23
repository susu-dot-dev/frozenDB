# Research Findings: Transaction State Management

## Decision: Use existing FrozenDB and Transaction struct patterns

**Rationale**: The existing codebase has well-designed Transaction struct with thread safety, comprehensive error handling, and proper state management. Rather than creating new patterns, we'll extend the existing FrozenDB struct to include active transaction management and add transaction recovery logic during file loading.

**Alternatives considered**: 
- Create separate transaction manager (rejected for unnecessary complexity)
- Use global transaction state (rejected for thread safety concerns)
- Modify Transaction struct significantly (rejected - current design is solid)

## Current Implementation Analysis

### FrozenDB Structure
**File**: `frozendb/frozendb.go`
- Current fields: `file DBFile`, `header *Header`
- **Missing**: No active transaction field, no mutex for transaction state
- Instance methods are documented as NOT thread-safe (except Close())

### Transaction Structure  
**File**: `frozendb/transaction.go`
- Thread-safe with `sync.RWMutex mu`
- State detection: `active` (last != nil), `committed` (empty != nil), `tombstoned`
- Complete lifecycle methods: `Begin()`, `AddRow()`, `Commit()`, `Rollback()`, `Savepoint()`
- Iterator access via `GetCommittedRows()`

### File Loading Logic
**File**: `frozendb/open.go`  
- Current `validateDatabaseFile()` only validates header and checksum
- **Missing**: Transaction state detection, PartialDataRow handling
- `RowUnion` provides row type detection via control bytes

### Error Handling Patterns
**File**: `frozendb/errors.go`
- Structured errors with `FrozenDBError` base
- Specific types: `InvalidInputError`, `InvalidActionError`, `CorruptDatabaseError`, etc.
- Pattern: `New*Error(message, err)` constructors

## Implementation Requirements

### Required FrozenDB Changes
1. Add `activeTx *Transaction` field  
2. Add `txMu sync.RWMutex` for thread safety
3. Add transaction recovery logic to `NewFrozenDB()`
4. Add `GetActiveTx() *Transaction` method
5. Add `BeginTx() (*Transaction, error)` method

### Required New Functionality
1. **Transaction Recovery**: Scan file ending for incomplete transactions
2. **PartialDataRow Loading**: Parse and reconstruct in-progress partial rows  
3. **Transaction Detection**: Examine end control characters (`RE`/`SE` = open, others = closed)
4. **Backward Scanning**: Find transaction start (up to 100 rows back if needed)

### Key Implementation Details
- **Performance**: Target <5ms for GetActiveTx(), constant memory usage
- **State Detection**: Use `RowUnion.UnmarshalText()` for row type detection
- **Thread Safety**: Leverage existing Transaction mutex + new FrozenDB txMu
- **Error Handling**: Use existing `InvalidActionError` for BeginTx() conflicts
- **Recovery Scope**: Handle all transaction endings (TC, SC, R0-R9, S0-S9, NR)

## File Format Integration

The v1 file format already supports all required transaction state detection:
- **End Control Characters**: `RE`/`SE` indicate open transactions
- **PartialDataRows**: Can only exist as last row, represent in-progress writes
- **Transaction Boundaries**: Clear start (`T`) and end (`C`/`0-9`) markers
- **Checksum Rows**: Appear every 10,000 rows, don't affect transaction state

Implementation will leverage existing `RowUnion` parsing and follow established patterns for error handling and thread safety.