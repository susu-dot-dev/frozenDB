# Research: Transaction Savepoint and Rollback Implementation

## Decision: Use Existing Foundation with Public API Extensions

**Rationale**: The frozenDB codebase already has excellent foundation for transaction savepoints and rollback functionality. The core components (end control encoding, state machine, error handling, `PartialDataRow` methods) are implemented correctly following append-only architecture.

**Alternatives considered**: 
- Complete rewrite of transaction system - rejected due to existing solid foundation
- External dependency for transaction management - rejected due to frozenDB's specific append-only requirements

## Key Findings

### 1. Transaction State Management
- **Decision**: Continue using existing state machine pattern (`PartialDataRowWithStartControl`, `PartialDataRowWithPayload`, `PartialDataRowWithSavepoint`)
- **Rationale**: Already properly implements Go best practices for transaction state management with thread-safe `sync.RWMutex`

### 2. Append-Only Rollback Semantics
- **Decision**: Rollbacks add new rows with end control encoding rather than modifying existing rows
- **Rationale**: Maintains frozenDB's immutable architecture as specified in v1_file_format.md section 2.2

### 3. End Control Encoding Patterns
- **Decision**: Use existing `EndControl` implementation in `row.go:64-106`
- **Rationale**: Already supports all required patterns: `R0-R9` (rollback without savepoint), `S0-S9` (rollback with savepoint), `TC`, `SC`, `RE`, `SE`

### 4. Savepoint Numbering and Tracking
- **Decision**: Continue using existing savepoint detection via `EndControl[0] == 'S'` and numbering 1-9 in creation order
- **Rationale**: Already properly implemented and matches file format specification

### 5. Error Handling Patterns
- **Decision**: Use existing structured error patterns (`InvalidActionError`, `InvalidInputError`)
- **Rationale**: Consistent with frozenDB's error handling standards

### 6. In-Memory to Disk Behavior Mirroring
- **Decision**: Leverage existing `PartialDataRow.Savepoint()` and `PartialDataRow.Rollback()` methods
- **Rationale**: Already accurately mirrors file format behavior for append-only operations

## Implementation Requirements

### Missing Components to Implement:
1. **Transaction.Savepoint() method** - Public API that validates transaction state and calls `PartialDataRow.Savepoint()`
2. **Transaction.Rollback(savepointId int) method** - Public API that handles rollback logic and empty transaction cases

### Validation Rules Already Implemented:
- Maximum 9 savepoints per transaction
- Savepoint only allowed after at least one data row
- Proper end control encoding for all rollback scenarios
- Thread-safe transaction state management

## Technical Architecture

### Go Best Practices Applied:
- Thread-safe transaction management with `sync.RWMutex`
- State machine pattern for transaction lifecycle
- Structured error handling with proper wrapping
- Interface-based design for extensibility

### frozenDB Specific Compliance:
- Append-only immutability preserved (no delete/modify operations)
- ROW_START (0x1F) and ROW_END (0x0A) sentinels maintained
- UUIDv7 timestamp ordering upheld
- Fixed memory usage regardless of transaction size

## Conclusion

The implementation requires minimal new code - primarily adding the public `Transaction.Savepoint()` and `Transaction.Rollback()` methods that leverage the existing, well-designed foundation. This approach ensures consistency with the frozenDB architecture while correctly implementing the user-facing savepoint and rollback functionality specified in the requirements.