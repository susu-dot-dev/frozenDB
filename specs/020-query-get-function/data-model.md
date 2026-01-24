# Data Model: Query Get Function

## New Entity: InvalidDataError

**Description**: Structured error type for JSON unmarshal failures in Get() operations.

**Attributes**:
- **Code**: "invalid_data" - Error code for programmatic handling
- **Message**: Human-readable error description
- **Err**: Underlying JSON unmarshal error (optional)

**Inherits from**: FrozenDBError (base error type)

## Row Visibility Model for Get Operations

### Transaction State Impact on Row Visibility

**Decision Logic for Get Operations**:
1. **Row Location**: Use Finder.GetIndex() to locate row by UUID
2. **Transaction Boundaries**: Get transaction start/end indices
3. **Transaction State Analysis**: Determine row visibility based on end_control:
   - **Committed (TC/SC)**: Row is visible to Get operations
   - **Fully rolled back (R0/S0)**: Row is hidden (KeyNotFoundError)
   - **Partially rolled back (R1-R9/S1-S9)**: Savepoint-dependent visibility
   - **Active transaction**: No ending (TransactionActiveError)

### Savepoint Visibility Logic

For partially rolled back transactions:
- Count savepoints from transaction start to target row index
- Compare count to rollback savepoint number
- Row is visible if: savepoint_count ≤ rollback_savepoint

## Data Validation Flow

### Input Validation Rules

**Destination Parameter Validation**:
- Must not be nil
- Must be a pointer type
- Must be JSON-unmarshalable

**UUID Key Validation**:
- Must be valid UUIDv7 format
- Must not be uuid.Nil
- Key existence checked via Finder.GetIndex()

### Transaction Validation Rules

**Committed Transaction Validity**:
- Transaction ends with TC or SC end_control
- All rows from start through commit are visible
- No rollback markers present

**Partial Rollback Validity**:
- Transaction ends with R1-R9 or S1-S9 end_control
- Visibility depends on savepoint position relative to rollback point
- Uses savepoint counting algorithm for determination

**Full Rollback Validity**:
- Transaction ends with R0 or S0 end_control
- All rows in transaction are hidden
- Get must return KeyNotFoundError

## Error State Mapping

### New Error Conditions

| Condition | Error Type | Trigger |
|-----------|------------|----------|
| JSON syntax error | InvalidDataError | Malformed JSON in stored data |
| Type mismatch | InvalidDataError | Destination incompatible with JSON |
| Invalid destination | InvalidInputError | Non-pointer or nil destination |
| Active transaction | TransactionActiveError | Transaction has no ending row |

### Row Visibility Errors

| Transaction State | Row Position | Result |
|------------------|---------------|---------|
| Committed | Any position | Visible |
| Partial rollback | ≤ rollback savepoint | Visible |
| Partial rollback | > rollback savepoint | Hidden (KeyNotFoundError) |
| Full rollback | Any position | Hidden (KeyNotFoundError) |
| Active | Any position | Error (TransactionActiveError) |

## Data Flow Relationships

```
UUID Key → Finder.GetIndex() → Row Index → Transaction Boundaries → State Analysis → Visibility Decision → JSON Retrieval → Unmarshaling
```

**Key Data Points**:
- Row index from Finder protocol
- Transaction boundaries from GetTransactionStart/GetTransactionEnd
- End_control byte pattern from transaction end row
- Savepoint count for partial rollback scenarios
- JSON payload from visible rows only