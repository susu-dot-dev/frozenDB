# Data Model: Transaction Begin and Commit

## Transaction Entity

The `Transaction` struct represents a single database transaction with enhanced state management to support the new Begin() and Commit() workflow.

### Core Transaction Structure

**Transaction Struct:**
- `rows []DataRow` - Complete data rows (max 100)
- `empty *NullRow` - Empty null row after successful commit (new)
- `last *PartialDataRow` - Current partial data row being built (new)
- `mu sync.RWMutex` - Mutex for thread safety (new)

### Field Validation Rules

#### `rows []DataRow`
- **Type**: Slice of complete DataRow objects
- **Validation**: Must be empty or contain valid DataRows
- **Constraints**: Maximum 100 rows per frozenDB architecture
- **State Dependency**: Can be non-empty only when transaction is committed with actual data rows (not empty transactions)
- **Important**: NullRows are stored in `empty` field, NOT in `rows[]`. For empty transactions, `rows` remains empty.

#### `empty *NullRow`
- **Type**: Pointer to NullRow (nullable)
- **Validation**: Must be `nil` when transaction is inactive or active
- **Final State**: Points to valid NullRow when transaction is committed
- **Purpose**: Represents the final null row after empty transaction

#### `last *PartialDataRow`
- **Type**: Pointer to PartialDataRow (nullable)
- **Validation**: Must be `nil` when transaction is inactive or committed
- **Active State**: Points to PartialDataRow when transaction is active
- **Purpose**: Tracks in-progress partial data row during transaction

### State Transition Matrix

| From State | To State | Method | Preconditions | Postconditions |
|------------|-----------|--------|----------------|----------------|
| (empty/nil) | (partial) | Begin() | rows empty, empty nil, last nil | last ≠ nil |
| (partial) | (committed) | Commit() | last in PartialDataRowWithStartControl | empty ≠ nil, last = nil, rows unchanged |
| Any | Any | - | - | InvalidActionError |

### Validation Methods

#### Begin() Validation
- `rows` slice must be empty
- `empty` field must be nil
- `last` field must be nil

#### Commit() Validation
- `last` field must be non-nil
- `empty` field must be nil
- `rows` slice must be empty
- `last.GetState()` must equal `PartialDataRowWithStartControl`

### Entity Relationships

#### Transaction → DataRow (Composition)
- **Cardinality**: 0..* (zero or more DataRows)
- **Lifecycle**: DataRows outlive the transaction that created them
- **Immutability**: DataRows are immutable once created

#### Transaction → NullRow (Association)
- **Cardinality**: 0..1 (optional empty row)
- **Lifecycle**: NullRow created only during successful empty transaction commit
- **Purpose**: Represents completed empty transaction

#### Transaction → PartialDataRow (Association)
- **Cardinality**: 0..1 (optional current partial row)
- **Lifecycle**: PartialDataRow exists only during active transaction
- **State**: Transitions through PartialDataRow states during transaction

### Invariant Constraints

1. **State Inference**: Transaction state is inferred from the combination of field values, not stored explicitly
2. **Mutual Exclusion**: `empty` and `last` cannot both be non-nil simultaneously
3. **Empty Transaction Result**: After successful Begin() → Commit(), `empty` points to a NullRow, `rows` remains empty
4. **Thread Safety**: All state transitions must be atomic and mutex-protected
5. **Memory Usage**: Total memory usage remains constant regardless of transaction count

### State Machine Implementation

**Begin() Process:**
1. Acquire write lock
2. Validate preconditions (all fields empty/nil)
3. Create PartialDataRow with start control
4. Set `last` field to new PartialDataRow
5. Release lock

**Commit() Process:**
1. Acquire write lock
2. Validate preconditions (active state, correct partial state)
3. Create NullRow with null payload
4. Set `empty` field to created NullRow
5. Set `last` field to nil
6. Release lock
7. **Note**: `rows` slice is NOT modified for empty transactions

### Integration Points

#### Existing frozenDB Components
- **Header**: Required for row creation and validation
- **DataRow**: Existing type used in `rows` slice
- **PartialDataRow**: Enhanced with state management integration
- **NullRow**: Used for empty transaction result
- **InvalidActionError**: Existing error type for invalid operations

#### Error Handling
- All validation errors use existing `InvalidActionError` pattern
- Error messages are specific and actionable
- Errors follow frozenDB's structured error handling guidelines

#### Memory Management
- Pointer fields enable optional presence/absence
- No dynamic allocation beyond fixed struct size
- Memory usage scales with transaction count, not database size