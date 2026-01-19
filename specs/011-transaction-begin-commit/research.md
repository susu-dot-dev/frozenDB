# Research: Transaction Begin and Commit Implementation

## Decision: State Machine Pattern with Mutex Protection

**Rationale**: Based on research into Go database transaction patterns, the most appropriate approach for frozenDB's transaction state management is a state machine pattern with mutex protection. This provides:

1. **Thread Safety**: Mutex protection ensures concurrent access safety
2. **Clear State Transitions**: Explicit states prevent invalid operation sequences
3. **Atomic Operations**: State changes are atomic and verifiable
4. **Error Clarity**: Structured error handling with specific error types

## Key Research Findings

### 1. Transaction State Management

The Transaction struct requires three new fields in addition to existing `rows []DataRow`:
- `empty *NullRow` - Represents the final empty row after commit
- `last *PartialDataRow` - Tracks the current partial data row being built
- `state TransactionState` - Manages the current transaction state

State transitions must be atomic and validated:
- `StateInactive` → `StateActive` (via Begin())
- `StateActive` → `StateCommitted` (via Commit())

### 2. Nil Pointer Field Handling

Safe nil pointer handling is critical in Go:
```go
func (t *Transaction) validateStateForBegin() error {
    if t.empty != nil {
        return NewInvalidActionError("Begin", "empty row already exists")
    }
    if t.last != nil {
        return NewInvalidActionError("Begin", "partial row already exists")  
    }
    if len(t.rows) > 0 {
        return NewInvalidActionError("Begin", "rows already exist")
    }
    return nil
}
```

### 3. InvalidActionError Integration

The existing frozenDB error handling pattern already supports `InvalidActionError`. The implementation should:
- Use existing `NewInvalidActionError()` constructor
- Provide clear, specific error messages
- Follow established error wrapping patterns

### 4. Atomic State Transitions

State changes must be atomic to prevent corruption:
```go
func (t *Transaction) Begin() error {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    if err := t.validateStateForBegin(); err != nil {
        return err
    }
    
    t.state = StateActive
    t.last = &PartialDataRow{
        // Initialize with start control only
    }
    return nil
}
```

### 5. Memory Management

- Use pointer fields for optional data (empty, last)
- Maintain fixed memory usage regardless of database size
- Follow frozenDB's immutability principles

## Alternatives Considered

1. **Atomic operations only**: Rejected in favor of mutex for simplicity and clarity
2. **Functional approach**: Rejected due to complexity and existing imperative codebase
3. **Separate state manager**: Rejected as over-engineering for this use case

## Integration with Existing Code

The implementation must integrate with:
- Existing `DataRow`, `PartialDataRow`, `NullRow` types
- Current `InvalidActionError` from error handling system
- Existing validation patterns in frozenDB
- Spec testing requirements per `docs/spec_testing.md`

## Performance Considerations

- Mutex overhead is acceptable for transaction boundaries (Begin/Commit operations)
- Memory usage remains constant (no scaling with database size)
- Validation costs are minimal and necessary for correctness

## Next Steps

The research informs the data model design in Phase 1, specifically:
1. Transaction struct with proper field types and initial states
2. State transition validation methods  
3. Error handling integration
4. API contract definitions for Begin() and Commit() methods