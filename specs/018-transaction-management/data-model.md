# Data Model: Transaction State Management

## Core Entities

### FrozenDB

**Enhanced FrozenDB Struct:**
```go
type FrozenDB struct {
    file      DBFile       // DBFile interface for file operations
    header    *Header      // Parsed header information
    activeTx  *Transaction // Current active transaction (nil if none)
    txMu      sync.RWMutex // Mutex for transaction state management
}
```

**Fields:**
- `file`: Existing file operations interface
- `header`: Existing parsed header information  
- `activeTx`: New field for tracking current active transaction
- `txMu`: New field for thread-safe transaction state access

### Transaction

**Existing Transaction Struct (no changes needed):**
```go
type Transaction struct {
    rows            []DataRow       // Single slice of DataRow objects (max 100)
    empty           *NullRow        // Empty null row after successful commit
    last            *PartialDataRow // Current partial data row being built
    Header          *Header         // Header reference for row creation
    maxTimestamp    int64           // Maximum timestamp seen in this transaction
    mu              sync.RWMutex    // Mutex for thread safety
    writeChan       chan<- Data     // Write channel for sending Data structs to FileManager
    rowBytesWritten int             // Tracks bytes written for current PartialDataRow
    tombstone       bool            // Tombstone flag set when write operation fails
    db              DBFile          // File manager interface for reading rows and calculating checksums
}
```

## Transaction State Model

### Active Transaction Detection

**State Determination Logic:**
1. **Open Transaction**: Last data row has end_control `RE` or `SE`
2. **Closed Transaction**: Last data row has end_control ending in `C` or `0-9`
3. **Partial Transaction**: File ends with PartialDataRow in any state

**Transaction Recovery Process:**
1. **File Boundary Check**: If file size doesn't land on row boundary, last item is PartialDataRow → create active Transaction
2. **Read Last Row**: Read only the last row to determine transaction state
3. **Early Exit**: If last row indicates closed transaction (TC, SC, R0-R9, S0-S9, NR) → no active Transaction needed
4. **Batch Scan**: If last row indicates open transaction (RE, SE), read the last 100 rows in single operation
5. **Transaction Start**: Analyze the batch to find where the transaction begins
6. **State Recovery**: Create Transaction object with recovered state and store in FrozenDB.activeTx

**Optimization Notes:**
- Minimizes I/O by avoiding row-by-row scanning
- Single batch read of 100 rows when transaction recovery is needed
- Immediate detection of closed transactions without unnecessary scanning

### Transaction Lifecycle States

| State | Condition | GetActiveTx() Returns | BeginTx() Behavior |
|-------|-----------|----------------------|--------------------|
| No Transaction | No active rows or committed/rolled back | nil | Creates new transaction |
| Active Transaction | Last row ends with RE/SE or PartialDataRow exists | *Transaction reference | Returns error |
| Committed Transaction | Transaction ended with TC/SC | nil | Creates new transaction |
| Rolled Back Transaction | Transaction ended with R0-R9/S0-S9 | nil | Creates new transaction |

## File Format Integration

### Row Type Detection

**RowUnion Processing:**
- Uses control bytes to determine row type
- Supports DataRow, NullRow, ChecksumRow detection
- `UnmarshalText()` method for automatic type detection

**End Control Character Mapping:**
| End Control | Transaction State | Action Required |
|-------------|-------------------|-----------------|
| RE, SE | Open | Create active Transaction |
| TC, SC | Committed | No active Transaction |
| R0-R9, S0-S9 | Rolled Back | No active Transaction |
| NR | Null Row (single transaction) | No active Transaction |

### PartialDataRow Handling

**PartialDataRow States:**
1. **State 1**: ROW_START + START_Control only
2. **State 2**: State 1 + key UUID + value JSON  
3. **State 3**: State 2 + 'S' first character of END_CONTROL

**Recovery Behavior:**
- State 3 indicates savepoint intent for transaction queries
- All states indicate active transaction requiring recovery
- Completion follows existing Transaction continuation patterns

## Validation Rules

### Transaction State Validation

**Must Reject:**
- Corrupted transaction state (invalid end control sequences)
- Invalid PartialDataRow state
- Multiple transaction endings (file corruption)
- Rows with invalid structure during detection

**Must Accept:**
- Files ending with checksum rows but no data rows (no active transaction)
- Valid PartialDataRow in any state (indicates active transaction)
- Properly formed transaction endings (TC, SC, R0-R9, S0-S9, NR)

### Concurrency Rules

**Thread Safety Requirements:**
- FrozenDB.txMu protects activeTx field access
- Transaction.mu protects internal transaction state
- GetActiveTx() uses read lock for thread-safe access
- BeginTx() uses write lock for exclusive modification

**Mutex Hierarchy:**
1. FrozenDB.txMu (outer lock for transaction state)
2. Transaction.mu (inner lock for transaction operations)

## Error Scenarios

### Transaction Detection Errors

**CorruptDatabaseError**: Invalid transaction state detected
- Invalid end control sequences
- Invalid PartialDataRow structure  
- Multiple apparent transaction endings

**InvalidActionError**: Transaction conflict operations
- BeginTx() called when active transaction exists
- Operations on tombstoned transactions

### Recovery Validation

**State Validation Requirements:**
- PartialDataRow must be at file end (else corruption)
- Transaction start must be found within 100 rows backward
- All row structures must follow v1 format specification
- CRC32 checksums must validate for covered sections