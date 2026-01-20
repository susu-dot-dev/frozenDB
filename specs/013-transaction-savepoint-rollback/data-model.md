# Data Model: Transaction Savepoint and Rollback

## Core Entities

### Transaction
The main transaction management entity that orchestrates savepoints and rollback operations.

**Fields:**
- **No new fields required** - uses existing Transaction struct
- Savepoint state tracked through existing `PartialDataRow` end control patterns
- Savepoint counting determined by analyzing completed rows' end controls
- Thread safety maintained through existing `sync.RWMutex`

**State Transitions:**
1. Begin → PartialDataRowWithStartControl (new transaction)
2. Add row → PartialDataRowWithPayload (data added)
3. Savepoint → PartialDataRowWithSavepoint (savepoint created)
4. Commit/Rollback → Transaction closed

**Validation Rules:**
- Maximum 100 data rows per transaction
- Maximum 9 savepoints per transaction
- Savepoint only allowed after at least one data row
- Cannot be nested (no concurrent transactions)

### Savepoint
A marker within a transaction representing a rollback point.

**Fields:**
- **No explicit struct** - represented by end control encoding on rows
- Identified by rows where `EndControl[0] == 'S'` (savepoint created on this row)
- Numbered by counting savepoint-creating rows in transaction order
- Savepoint 0 is implicit (transaction start)

**Properties:**
- Numbered sequentially 1-9 within transaction by counting rows with savepoint flags
- Created by calling Savepoint() which marks current row with savepoint intent
- Represents a rollback target for partial rollback operations
- No additional storage needed - encoded in existing row structure

### EndControl
Two-character sequence encoding transaction termination behavior.

**Values:**
- `TC`: Commit without savepoint
- `SC`: Commit with savepoint
- `RE`: Continue without savepoint
- `SE`: Continue with savepoint
- `R0-R9`: Rollback to savepoint N without savepoint
- `S0-S9`: Rollback to savepoint N with savepoint
- `NR`: Null row

**Encoding Rules:**
- First character: `T`/`R` (no savepoint), `S` (savepoint), `N` (null row)
- Second character: `C` (commit), `E` (continue), `0-9` (rollback), `R` (null row)

### Row
Base entity for all row types in frozenDB.

**Common Fields:**
- `startControl`: Single byte (`T`, `R`, `C`, `N`)
- `endControl`: Two bytes per EndControl specification
- `uuid`: UUIDv7 key (except NullRow uses uuid.Nil)
- `value`: JSON payload (empty for NullRow)
- `sentinels`: ROW_START (0x1F) and ROW_END (0x0A)

**Row Types:**
- **DataRow**: Complete data row with user key-value pair
- **PartialDataRow**: In-progress row (3 states)
- **NullRow**: Single-row transaction with no user data
- **ChecksumRow**: Integrity checking row

## Transaction Operations

### Savepoint Creation
**Preconditions:**
- Transaction must be active
- At least one data row must exist
- Fewer than 9 savepoints currently exist (determined by counting existing rows with savepoint flags)

**Process:**
1. Validate transaction state and savepoint limit by analyzing existing rows
2. Call `PartialDataRow.Savepoint()` on current PartialDataRow
3. No explicit counter needed - count derived from row analysis
4. Transition to PartialDataRowWithSavepoint state

**Error Cases:**
- `InvalidActionError`: Savepoint on empty transaction or inactive transaction
- `InvalidActionError`: More than 9 savepoints (detected by row analysis)

### Rollback Operations
**Preconditions:**
- Transaction must be active
- Valid savepoint target (0 to current savepoint count)

**Process:**
1. Validate transaction state and savepoint target
2. For Rollback(0) on empty transaction: create NullRow
3. For Rollback(n>0): finalize current row with rollback end control
4. Close transaction and update state

**Rollback Results:**
- **Full rollback (R0/S0)**: All rows invalidated
- **Partial rollback (R1-R9/S1-S9)**: Rows up to savepoint N committed, subsequent rows invalidated

**Error Cases:**
- `InvalidActionError`: Rollback on inactive transaction
- `InvalidInputError`: Invalid savepoint number (> current savepoints)

## Data Integrity Constraints

### Append-Only Architecture
- All operations create new rows, never modify existing ones
- Rollback operations append rows with special end controls (R0-R9, S0-S9)
- Data corruption prevented through sentinel bytes and parity
- No new fields in Transaction struct - state encoded in existing row structure

### Transaction Boundaries
- Each transaction has exactly one transaction-ending command
- No nested transactions allowed
- NullRows are single-row transactions

### Savepoint Limits
- Maximum 9 user savepoints per transaction (enforced by counting savepoint rows)
- Savepoints numbered 1-9 by counting rows with savepoint flags in order
- Savepoint 0 implicit (transaction start)
- No additional storage required - information encoded in end control patterns

## Validation Rules Summary

| Operation | Preconditions | Success Result | Error Conditions |
|-----------|---------------|----------------|------------------|
| Savepoint() | Active transaction, ≥1 data row, <9 savepoints | Current row marked as savepoint, counter incremented | InvalidActionError on empty/inactive transaction or savepoint limit |
| Rollback(0) | Active transaction | All rows invalidated, transaction closed | InvalidActionError on inactive transaction |
| Rollback(n>0) | Active transaction, n ≤ current savepoints | Rows up to n committed, subsequent invalidated, transaction closed | InvalidActionError on inactive transaction, InvalidInputError on invalid savepoint |