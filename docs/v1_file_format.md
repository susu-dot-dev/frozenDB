# frozenDB v1 File Format Specification

## 1. Introduction

This document defines the frozenDB v1 on-disk file format. frozenDB is an immutable key-value store designed for simplicity, correctness, and performance.

### 1.1. Design Philosophy

frozenDB uses an **append-only file with fixed-width rows**. This design imposes constraints but provides significant benefits:

**Append-Only Immutability:**
- Data is never modified in place—only appended
- Enables safe concurrent reads during writes
- Simplifies crash recovery (no partial overwrites)
- Provides natural audit trail of all operations

**Fixed-Width Rows:**
- Enables O(1) seeking to any row by index
- Allows binary search on sorted keys
- Eliminates need for index files or offset tables
- Simplifies memory-mapped access patterns

These constraints require careful handling of transactions and rollbacks, which cannot delete data but must instead mark rows as invalid.

### 1.2. Conformance and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

### 1.3. File Encoding

All text in a frozenDB v1 file SHALL be encoded using UTF-8. Implementations MUST accept UTF-8 encoded input and MUST generate UTF-8 encoded output.

## 2. Conceptual Model

### 2.1. Transactions

All database writes MUST occur within a transaction. A transaction provides atomicity—either all rows in the transaction are committed, or none are.

**Transaction Lifecycle:**
1. **Begin**: Start a new transaction
2. **Add**: Insert one or more key-value pairs (each becomes a row)
3. **Commit** or **Rollback**: End the transaction

```
Begin() → Add(k1,v1) → Add(k2,v2) → Commit()
```

**Transaction Constraints:**
- Transactions cannot be nested. A new transaction cannot begin until the previous transaction has ended.
- A transaction cannot contain more than 100 data rows.
- A transaction can contain up to 9 user-defined savepoints.
- A transaction must contain exactly one transaction-ending command (commit or rollback).

**Transaction Validity:**
- A committed transaction makes all rows from the transaction start through the commit command (inclusive) valid.
- A rolled back transaction makes rows from the start through a specified savepoint (inclusive) valid, and invalidates all rows after that savepoint through the rollback command (inclusive).
- A full rollback (to savepoint 0) invalidates all rows in the transaction, including the rollback command itself.

### 2.2. Savepoints

Savepoints allow partial rollbacks within a transaction. When a savepoint is created, the current row is marked, and a later rollback can return to that point.

**Key insight**: Since the file is append-only, "rollback" doesn't delete rows—it marks them as invalid. Readers parse the transaction to its end, check for rollback markers, and exclude invalidated rows.

**Savepoint numbering**: Savepoints are numbered 1-9 in creation order within a transaction. Savepoint 0 represents the transaction start (full rollback). A transaction can contain up to 9 user-defined savepoints.

**Example with savepoint:**
```
Begin() → Add(k1,v1) → Savepoint() → Add(k2,v2) → Rollback(1)
```
Result: k1 is committed, k2 is invalidated. The rollback to savepoint 1 commits everything up to and including the savepoint, and invalidates everything after.

**Example with full rollback:**
```
Begin() → Add(k1,v1) → Add(k2,v2) → Rollback(0)
```
Result: Both k1 and k2 are invalidated. Rollback(0) invalidates the entire transaction.

### 2.3. Partial Data Rows

A PartialDataRow is an incomplete data row that can ONLY exist as the last row in a frozenDB file. PartialDataRows and DataRows are mutually exclusive - a PartialDataRow cannot be fully complete.

**PartialDataRow States:**
PartialDataRows exist in one of three progressive states:

1. **State 1**: ROW_START & START_Control bytes only
2. **State 2**: State 1 + key UUID + value JSON
3. **State 3**: State 2 + 'S' first character of END_CONTROL (but not the second byte)

A PartialDataRow in state 3 indicates that the row will eventually include a savepoint, but the final disposition (commit, rollback, or continuation) is not yet determined.

**Key Properties:**
- **Exclusivity**: PartialDataRows and DataRows are mutually exclusive
- **Position**: MUST be the last row in the file; cannot exist at any other position
- **Size**: MUST stop exactly at the state boundary with no padding or additional data
- **Completion**: When a PartialDataRow is completed beyond state 3, it becomes a DataRow

**Reader Behavior:**
- PartialDataRows are not committed and thus never appear in query results
- Savepoint flags from PartialDataRows should be visible for transaction state queries
- Invalid PartialDataRows are treated as disk corruption

**Transaction Counting:**
- PartialDataRows count toward transaction limits (e.g., 100-row maximum)
- PartialDataRows are excluded from the 10,000-row checksum interval (only complete DataRows and NullRows count)

### 2.4. Null Rows

A NullRow is a fixed-width row that represents a null operation with no user data. NullRows are single-row transactions that cannot appear within existing transactions.

**NullRow Structure:**
- **start_control**: Always `T` (transaction begin)
- **uuid**: A UUIDv7 with timestamp component equal to the `max_timestamp` of the database **at the time of insertion**, with all other fields (random components) set to zero. This timestamp is fixed and immutable once the NullRow is written to the database. For an empty database (no complete rows), the timestamp is 0.
- **value**: No user value (immediate padding after UUID)
- **end_control**: Always `NR` (null row)

**Key Properties:**
- **Fixed-width**: NullRows are always exactly `row_size` bytes (no partial equivalent)
- **Single-row transactions**: Each NullRow is a complete transaction by definition
- **Transaction boundaries**: NullRows can only appear where a new transaction could start
- **Non-appearable as partial**: A PartialDataRow cannot be a NullRow

**Transaction Placement Rules:**
- NullRows can appear as the first row after header & checksum
- Multiple NullRows can appear in succession
- NullRows cannot appear when a prior transaction is open (previous end_control is `RE` or `SE`)
- NullRows are treated as standalone transactions for all transaction rules

**Reader Behavior:**
- NullRows are not committed transactions and thus never appear in query results
- Invalid NullRows are treated as disk corruption

**Transaction Counting:**
- NullRows count toward the 10,000-row checksum interval

**UUID and Ordering Rules:**
- NullRows use a UUIDv7 with timestamp equal to the `max_timestamp` of the database **at the time of insertion**, with all other fields set to zero. This timestamp is fixed and immutable once the NullRow is written to the database, and does not change even if later rows have higher timestamps.
- DataRows cannot use uuid.Nil as a valid key
- NullRows MUST follow the same timestamp ordering requirements as DataRows (see section 8.4)

### 2.4. End Control Character Design

The end_control is a 2-character sequence that encodes both savepoint creation and transaction termination in a space-efficient manner:

| First Char | Meaning |
|------------|---------|
| `T` or `R` | No savepoint on this row |
| `S`        | Savepoint created on this row |
| `N`        | Null Row|

| Second Char | Meaning |
|-------------|---------|
| `C`         | Commit transaction |
| `E`         | Continue (more rows follow) |
| `0-9`       | Rollback to savepoint N (terminates transaction) |
| `R`        | Null Row|

**Combined sequences:**

| Sequence | Meaning |
|----------|---------|
| `TC`     | Commit, no savepoint |
| `RE`     | Continue, no savepoint |
| `SC`     | Commit + savepoint on this row |
| `SE`     | Continue + savepoint on this row |
| `R0-R9`  | Rollback to savepoint N, no savepoint on this row |
| `S0-S9`  | Rollback to savepoint N + savepoint on this row |
| `NR`     | Null row |

### 2.4. Transaction Examples

**Simple commit (two rows):**
```
Begin() → Add(k1,v1) → Add(k2,v2) → Commit()
```
- Row 1: `T...RE` (k1, continue)
- Row 2: `R...TC` (k2, commit)
- Result: k1 and k2 committed

**Single row with savepoint and commit:**
```
Begin() → Add(k1,v1) → Savepoint() → Commit()
```
- Row 1: `T...SC` (k1, savepoint 1, commit)
- Result: k1 committed

**Partial rollback:**
```
Begin() → Add(k1,v1) → Savepoint() → Add(k2,v2) → Add(k3,v3) → Rollback(1)
```
- Row 1: `T...SE` (k1, savepoint 1, continue)
- Row 2: `R...RE` (k2, continue)
- Row 3: `R...R1` (k3, rollback to savepoint 1)
- Result: k1 committed; k2 and k3 invalidated

**Full rollback:**
```
Begin() → Add(k1,v1) → Add(k2,v2) → Rollback(0)
```
- Row 1: `T...RE` (k1, continue)
- Row 2: `R...R0` (k2, full rollback)
- Result: k1 and k2 invalidated

**Empty transaction:**
```
Begin() → Commit()
Begin() → Rollback(0)
```
- Row 1: `T...NR`
- Row 2: `T...NR`
- Result: Both rows are null rows


### 2.5. Reading Transactions

When reading a frozenDB file, implementations MUST:

1. Parse each transaction from its first row (transaction start) to its terminating row (transaction-ending command)
2. Check the terminating row's transaction-ending command:
   - If commit: Include all rows in the transaction from start through commit (inclusive)
   - If rollback to 0: Exclude all rows in the transaction (entire transaction rolled back)
   - If rollback to N where N > 0: Include rows from start through savepoint N (inclusive); exclude all rows after savepoint N through the rollback command (inclusive)
3. Savepoints are numbered by counting savepoint-creating rows within the transaction, in order (first = 1, second = 2, etc.)
4. PartialDataRows are not committed and MUST NOT be included in transaction results. However, savepoint intent from State 3 PartialDataRows should be considered when determining the current savepoint count for transaction state queries.

## 3. File Structure

A frozenDB v1 file consists of:
1. A 64-byte header
2. A checksum row (required)
3. Zero or more data rows
4. Optional: One PartialDataRow (only as the very last row)
5. Additional checksum rows inserted every 10,000 complete data rows

```
Offset:    0          64        64+row_size   64+2*row_size               end
            ├──────────┼─────────┼─────────────┼─────────────┬─────────────┤
            │  Header  │Checksum │  Data Row 0 │  Data Row 1 │ PartialData │
            └──────────┴─────────┴─────────────┴─────────────┴─────────────┘
                                                        (optional, only last)
```

### 3.1. Terminology and Byte Definitions

**Row Structure:**
- **ROW_START**: Byte value 0x1F (UTF-8: U+001F, unit separator) marking row beginning
- **ROW_END**: Byte value 0x0A (UTF-8: U+000A, newline) marking row end
- **start_control**: Single byte representing an uppercase alphanumeric character (UTF-8: U+0030-U+0039 for digits 0-9, U+0041-U+005A for letters A-Z) identifying row type
- **end_control**: Two bytes, each representing an uppercase alphanumeric character (same range as start_control) indicating row termination
- **parity_bytes**: Two bytes representing uppercase hexadecimal digits (UTF-8: U+0030-U+0039, U+0041-U+0046) for LRC parity calculations

**Padding Characters:**
- **NULL_BYTE**: Byte value 0x00 (UTF-8: U+0000, null character) used for padding

## 4. Header Specification

### 4.1. Header Structure

The header SHALL be exactly 64 bytes:

```
{"sig":"fDB","ver":1,"row_size":<size>,"skew_ms":<skew>}<null padding>\n
```

| Field | Type | Valid Range | Description |
|-------|------|-------------|-------------|
| `sig` | string | `"fDB"` | File signature |
| `ver` | integer | `1` | Format version |
| `row_size` | integer | 128-65536 | Bytes per row |
| `skew_ms` | integer | 0-86400000 | Time skew window for UUIDv7 lookups (ms) |

### 4.2. Header Format Requirements

- Keys MUST appear in order: `sig`, `ver`, `row_size`, `skew_ms`
- Padding: NULL_BYTE characters fill bytes after JSON to position 62
- Byte 63 MUST be NEWLINE
- JSON content: 49-58 bytes; padding: 5-14 bytes

### 4.3. Header Parsing

Implementations SHALL:
1. Read exactly 64 bytes from file start
2. Verify byte 63 is newline
3. Extract JSON from bytes [0..first null - 1]
4. Validate all fields per section 4.1
5. Verify bytes between JSON end and byte 62 are null

## 5. Row Structure

### 5.1. Generic Row Layout

All rows share this structure:

```
Position:  [0]    [1]      [2..N-6]         [N-5..N-4]    [N-3..N-2]   [N-1]
           ├──────┼────────┼────────────────┼─────────────┼────────────┼──────┤
           │ROW_  │ start  │  Row Content   │    end      │   parity   │ROW_  │
           │START │control │  (+ padding)   │  control    │   bytes    │END   │
           └──────┴────────┴────────────────┴─────────────┴────────────┴──────┘
```

Where N = `row_size` from header. All positions use zero-based indexing.

### 5.2. Parity Bytes

Parity provides per-row integrity checking using Longitudinal Redundancy Check (LRC):

1. XOR all bytes from [0] through [N-4] (inclusive)
2. Encode result as 2-character uppercase hex string

Example: XOR result 0xA3 → "A3"

## 6. Checksum Row (C/CS)

### 6.1. Format

```
Position:  [0]    [1]    [2..9]        [10..N-6]      [N-5..N-4]    [N-3..N-2]   [N-1]
           ├──────┼──────┼─────────────┼──────────────┼─────────────┼────────────┼──────┤
           │ROW_  │start │ crc32_base64│   padding    │    end      │   parity   │ROW_  │
           │START │ctrl  │   (8 bytes) │  (NULL_BYTE) │  control    │   bytes    │END   │
           └──────┴──────┴─────────────┴──────────────┴─────────────┴────────────┴──────┘
```

Where N = `row_size` from header. All positions use zero-based indexing.

For checksum rows: start_control = `C`, end_control = `CS`

### 6.2. CRC32 Calculation

- Algorithm: IEEE CRC32 (polynomial 0xedb88320)
- Input: All bytes covered since previous checksum row, including the previous checksum row itself (or from the beginning of the file for first checksum)
- Encoding: Standard Base64 of 4-byte CRC32 value (8 bytes output with "==" padding)

**Coverage Details:**
- First checksum row (at offset 64): Covers bytes [0..63] (the 64-byte header)
- Second checksum row: Covers bytes starting at byte 65 (the first checksum row) through the end of the next 10,000 complete Data Rows or Null Rows (in any combination)
- Subsequent checksum rows: Each covers the previous checksum row, plus the next 10,000 complete Data or Null rows

This ensures complete coverage of the file without gaps, as each checksum row includes the previous checksum row in its calculation.

### 6.3. Placement Rules

1. First checksum row: Immediately after header (offset 64). This checksum row MUST be present and MUST be validated when reading the file. Since there is no previous row, this checksum MUST cover bytes [0..63] (length 64) to cover the entire header
2. Subsequent: After every 10,000 complete Data Rows or Null Rows (in any combination). A checksum row MUST be placed before the 10,001st complete Data Row or Null Row is written. Implementations MAY choose to write the checksum immediately after writing the 10,000th complete Data Row or Null Row, or defer it until just before writing the 10,001st complete Data Row or Null Row.
3. File may end after any number of complete Data Rows or Null Rows. If a file ends with fewer than 10,000 complete Data Rows or Null Rows since the last checksum, no final checksum is required. A file may optionally end with a single PartialDataRow as the very last row; this PartialDataRow is excluded from the 10,000-row count.


## 7. Data Corruption Detection

### 7.1. Initial Checksum Row Validation

When reading a frozenDB file, implementations MUST parse and validate the checksum row that immediately follows the header (at offset 64). This checksum row covers the header and MUST be validated to ensure data integrity.

### 7.2. Row Coverage and Validation Strategy

frozenDB uses a two-tier integrity checking system:

1. **Checksum rows**: Provide CRC32 validation for blocks of up to 10,000 data rows
2. **Parity bytes**: Provide per-row LRC validation for all rows

**Coverage Rules:**

When performing data validation (see section 7.3 for when validation is optional):

- For rows covered by both a checksum and parity bytes (e.g., the first 10,000 rows in a file with 12,000 rows), the checksum SHALL have precedence over parity bits. If a checksum is available for a block of rows, implementations SHALL use the checksum for validation and MAY ignore the parity bits for those rows.
- For rows not covered by a checksum (e.g., rows 10,001-12,000 in the above example), implementations SHALL use parity bytes for validation if validation is being performed.

**Example:** For a file with 12,000 data rows:
- Rows 0-9,999: If validated, use checksum (parity may be ignored)
- Rows 10,000-11,999: Use parity bytes for each row

### 7.3. Validation Requirements

This specification does NOT require implementations to validate parity bytes or checksums outside of the initial checksum row (see section 7.1) during normal read operations. Implementations MAY choose to:

- Validate all checksums and parity bytes for maximum data integrity
- Validate only checksums for performance
- Validate only when explicitly requested by the caller
- Skip validation entirely for maximum speed

The choice of validation strategy is a tradeoff between speed and data integrity that implementations SHALL make based on their use case and caller preferences.

### 7.4. Checksum Calculation Requirements

When calculating a new checksum for a block of rows (e.g., 10,000 rows), implementations MUST validate the parity of all rows in that block before calculating the checksum. If any row's parity validation fails during checksum calculation, the database MUST be considered corrupt and an error SHALL be returned to the caller.

This parity validation during checksum calculation ensures data integrity at the time of checksum creation, which is why parity bits can be ignored later when the checksum is used for validation.

**Rationale:** By validating parity during checksum calculation, the checksum becomes a trusted integrity marker for the entire block. Subsequent reads can rely solely on the checksum without re-validating individual row parity bits.

### 7.5. PartialDataRow Corruption Detection

Invalid PartialDataRows MUST be treated as disk corruption. A PartialDataRow is invalid if:

1. **Invalid Field Content**: Any present field violates DataRow validation rules (invalid ROW_START, invalid start_control, invalid UUID_base64, invalid JSON, etc.)
2. **Invalid State Structure**: The PartialDataRow does not match one of the three defined state patterns
3. **Incorrect Positioning**: A PartialDataRow exists anywhere other than as the very last row in the file
4. **Unexpected Data**: Any data exists beyond the state boundary (PartialDataRows cannot have padding or additional bytes)
5. **State 3 Violations**: State 3 has a character other than 'S' for the END_CONTROL first byte

Implementations MUST reject files containing invalid PartialDataRows and return a corruption error to the caller.

## 8. Data Row (T/R)

### 8.1. Format

```
Position:  [0]    [1]    [2..25]         [26..N-6]              [N-5..N-4]    [N-3..N-2]   [N-1]
           ├──────┼──────┼───────────────┼──────────────────────┼─────────────┼────────────┼──────┤
           │ROW_  │start │  uuid_base64  │ json_payload+padding │    end      │   parity   │ROW_  │
           │START | ctrl |   (24 bytes)  │   (variable)         │  control    │   bytes    │END   │
           └──────┴──────┴───────────────┴──────────────────────┴─────────────┴────────────┴──────┘
```

Where N = `row_size` from header. 
start_control = `T` (transaction begin) or `R` (row continuation); 
and end_control values are defined in section 8.3.
All positions use zero-based indexing.
- **uuid_base64**: 24 bytes, Base64 encoding of 16-byte UUIDv7
- **json_payload**: Variable length UTF-8 JSON, followed by NULL_BYTE padding to fill remaining space

### 8.2. Start Control Rules

| Code | When Valid |
|------|------------|
| `T` | First data row of file, or after a transaction-ending command (`TC`, `SC`, `R0-R9`, `S0-S9`, or `NR`). Zero or one checksum rows may appear between the transaction end and the next `T`. |
| `R` | Previous data row ended with `RE` or `SE` (transaction continues). Checksum rows do not affect this rule. |

### 8.3. End Control Rules

| Sequence | Meaning | Transaction State After |
|----------|---------|------------------------|
| `TC` | Commit | Closed |
| `RE` | Continue | Open |
| `SC` | Savepoint + Commit | Closed |
| `SE` | Savepoint + Continue | Open |
| `R0` | Full rollback | Closed |
| `R1-R9` | Rollback to savepoint N | Closed |
| `S0` | Savepoint + Full rollback | Closed |
| `S1-S9` | Savepoint + Rollback to savepoint N | Closed |
| `NR` | Null row | Closed |

**Important**: For `S0-S9` sequences, the savepoint is created on the current row first (incrementing the savepoint counter), and then the rollback is performed. This maps to user behavior: `Add()` (adds the row), `Savepoint()` (saves the current row), `Rollback()` (rolls back to a savepoint). For example, `S1` means: create a savepoint on this row, then rollback to savepoint 1. This allows saving the current row before calling rollback.

### 8.4. UUIDv7 Requirements

- MUST be globally unique
- MUST be Base64 encoded (24 bytes with "=" padding)
- Timestamp ordering MUST follow the algorithm described below to prevent unbounded decreases
- DataRows MUST NOT use uuid.Nil as a valid key
- **DataRow UUID Validation**: DataRow UUIDs MUST NOT have all zeros in the non-timestamp part (bytes 7, 9-15). This pattern indicates a NullRow UUID, which is invalid for DataRows. At least one byte in positions 7, 9-15 MUST be non-zero. Implementations MUST reject DataRows with all-zero non-timestamp parts during validation.

#### Timestamp Ordering Algorithm

To prevent unbounded timestamp decreases while accounting for clock skew, implementations MUST enforce the following algorithm when inserting new non-checksum rows (DataRows and NullRows):

1. **Track Maximum Timestamp**: Maintain the maximum timestamp observed across all committed non-checksum rows (DataRows and NullRows) in the database (`max_timestamp`)
2. **Validate New Row**: For any new non-checksum row (DataRow or NullRow) with timestamp `new_timestamp`:
   - `new_timestamp + skew_ms > max_timestamp` MUST be true
   - If this condition fails, the row MUST be rejected
3. **Update Maximum**: After successful insertion, update: `max_timestamp = max(max_timestamp, new_timestamp)`

**NullRow Timestamp Generation:**
- When inserting a NullRow, the UUID timestamp MUST be set to the `max_timestamp` of the database **at the time of insertion**
- This timestamp becomes part of the NullRow's immutable UUID and does not change after the row is written to the database
- For an empty database (no complete rows), `max_timestamp` is 0, so the NullRow timestamp is 0
- The NullRow's UUID has its timestamp component set to the insertion-time `max_timestamp` of the database, with all other UUID fields (random components) set to zero
- **Important distinction**: The NullRow's UUID timestamp is a fixed value from insertion time. It is independent of the Finder's `MaxTimestamp()` method, which tracks the current maximum timestamp across all rows in the database and may increase as new rows are added

#### Example Prevention

This algorithm prevents problematic scenarios like:
- skew = 5ms, row1 = 100ms, row2 = 95ms, row3 = 90ms (unbounded decrease)
- After row1 (100ms): `max_timestamp = 100ms`
- For row2 (95ms): `95ms + 5ms > 100ms` → `100ms > 100ms` → **FAILS**
- Row2 rejected, preventing the unbounded decrease

**All Non-Checksum Rows Must Follow Ordering:**
- Both DataRows and NullRows MUST follow the timestamp ordering algorithm above
- NullRows are no longer excluded from timestamp ordering validation
- All non-checksum rows (DataRows and NullRows) must be in roughly ascending timestamp order, accounting for the `skew_ms` window

### 8.5. Padding Calculation

```
padding_bytes = row_size - len(json_payload) - 31
```

Where 31 = 1 (ROW_START) + 1 (start_control) + 24 (UUID) + 2 (end_control) + 2 (parity) + 1 (ROW_END)

## 8.6. Partial Data Row

### 8.6.1. Overview

A PartialDataRow is an incomplete data row that represents an in-progress write operation. Unlike DataRows, PartialDataRows are not fixed-width and MUST stop exactly at their state boundary.

**Key Characteristics:**
- Can ONLY exist as the last row in the file
- Cannot be fully complete by definition
- All present fields must follow the same validation rules as DataRows
- No padding bytes are allowed beyond the state boundary

### 8.6.2. State Definitions

A PartialDataRow exists in exactly one of three states:

#### State 1: Start Control Only
```
Position:  [0]    [1]    [end]
             ├──────┼──────┼─────┤
             │ROW_  │START │ EOF │
             │START │CTRL  │     │
             └──────┴──────┴─────┘
```

#### State 2: Complete Key-Value Data
```
Position:  [0]    [1]    [2..25]         [26..M-1]        [M..N-1] [N]
             ├──────┼──────┼───────────────┼────────────────────┼─────────┼──────┤
             │ROW_  │start │  uuid_base64  │ json_payload       │ padding │ EOF │
             │START │ ctrl │   (24 bytes)  │  (variable)        │(NULL_BYTE)│     │
             └──────┴──────┴───────────────┴────────────────────┼─────────┴──────┘
                                                       │   (up to
                                                       │ row_size - 2)
```

#### State 3: Savepoint Intent
```
Position:  [0]    [1]    [2..25]         [26..M-1]              [M]    [N-1]
             ├──────┼──────┼───────────────┼────────────────────────┼──────┼──────┤
             │ROW_  │START │  uuid_base64  │ json_payload+padding  │ 'S'  │ EOF │
             │START │CTRL  │   (24 bytes)  │   (variable + NULL)   │      │     │
             └──────┴──────┴───────────────┴────────────────────────┼──────┴──────┘
                                                    END_CONTROL      
                                                     (first byte)  
```

### 8.6.3. Field Validation Rules

All present fields MUST follow the same validation rules as DataRows:

- **ROW_START**: MUST be byte value 0x1F (UTF-8: U+001F, unit separator)
- **start_control**: MUST be valid uppercase alphanumeric character (T or R for data rows)
- **uuid_base64**: MUST be 24-byte valid Base64 encoding of a UUIDv7. When present (State 2 and State 3), the UUID MUST follow all DataRow UUID validation rules, including the requirement that the non-timestamp part (bytes 7, 9-15) MUST NOT be all zeros (see section 8.4).
- **json_payload**: MUST be valid UTF-8 JSON string
- **'S' character**: MUST be the single character 'S' indicating savepoint intent

### 8.6.4. Size and Position Constraints

**Row Size Determination:**
When reading the last row of a frozenDB file:
- If the row is exactly `row_size` bytes: MUST be parsed as a data row or checksum row depending on the start_control
- If the row is less than `row_size` bytes: MUST be parsed as a PartialDataRow

**Strict Size Requirements:**
- PartialDataRow MUST end exactly at the state boundary
- NO bytes of any kind (padding, null bytes, or otherwise) may follow the state boundary
- This constraint forces PartialDataRows to be the last row in the file

**Positioning Rules:**
- PartialDataRow can ONLY exist as the last row in the file
- No rows (data, checksum, or otherwise) may follow a PartialDataRow
- If additional data needs to be written, the PartialDataRow must be completed as a DataRow

### 8.6.5. Transaction Behavior

**Savepoint Handling:**
- State 3 PartialDataRow with 'S' character indicates eventual savepoint creation
- The savepoint intent should be visible for transaction state queries
- The final disposition (commit, rollback, or continuation) is determined when the row is completed

**Transaction Counting:**
- PartialDataRows count toward transaction limits (e.g., maximum 100 data rows)
- PartialDataRows are excluded from the 10,000-row checksum interval calculation
- Only complete DataRows count toward checksum placement triggers

### 8.7. Null Row (T/NR)

### 8.7.1. Format

```
Position:  [0]    [1]    [2..25]         [26..N-6]        [N-5..N-4]    [N-3..N-2]   [N-1]
           ├──────┼──────┼───────────────┼─────────────────┼─────────────┼────────────┼──────┤
           │ROW_  │start │ uuid_base64   │   padding       │    end      │   parity   │ROW_  │
           │START │ ctrl │ (24 bytes)    │  (NULL_BYTE)    │  control    │   bytes    │END   │
           └──────┴──────┴───────────────┴─────────────────┴─────────────┴────────────┴──────┘
```

Where N = `row_size` from header. All positions use zero-based indexing.
- **start_control**: Always `T` (transaction begin) - position [1]
- **uuid_base64**: Always 24 bytes at positions [2..25], Base64 encoding of a UUIDv7 with timestamp component equal to the `max_timestamp` of the database **at the time of insertion** and all other fields set to zero. This timestamp is fixed and immutable once written to the database.
- **padding**: NULL_BYTE padding from position [26] through position [N-6]
- **end_control**: Always `NR` at positions [N-5..N-4]
- **parity_bytes**: At positions [N-3..N-2] 
- **ROW_END**: At position [N-1]

**Padding Calculation:**
```
padding_bytes = row_size - 31
```
Where 31 = 1 (ROW_START) + 1 (start_control) + 24 (UUID) + 2 (end_control) + 2 (parity) + 1 (ROW_END)

### 8.7.2. Validation Rules
- **UUID**: Must be a UUIDv7 with timestamp component equal to the `max_timestamp` of the database **at the time of insertion**, with all other fields (random components) set to zero. For an empty database, the timestamp is 0. Note: This timestamp is a fixed value from insertion time and does not change, even if later rows have higher timestamps. It is independent of the Finder's `MaxTimestamp()` method which tracks the current maximum across all rows.
- **Start Control**: Must be exactly `T`
- **End Control**: Must be exactly `NR`
- **Value**: No user value allowed (padding starts immediately after UUID)
- **PartialDataRow Exclusion**: A PartialDataRow cannot be a NullRow

### 8.7.3. Transaction Behavior
- **Single-row transaction**: Each NullRow is a complete, self-contained transaction
- **Transaction boundaries**: NullRows can only appear where new transactions are allowed
- **Counting**: NullRows count toward the 10,000-row checksum interval
- **UUID ordering**: NullRows MUST follow the same timestamp ordering requirements as DataRows (see section 8.4)

### 8.6.6. Reader and Recovery Behavior

**Reader Requirements:**
- PartialDataRows are not committed and MUST NOT appear in query results
- Savepoint flags from State 3 PartialDataRows should be exposed for transaction state queries
- Invalid PartialDataRows (violating field validation) MUST be treated as disk corruption

**Recovery Requirements:**
- When reopening a file with a PartialDataRow, implementations MUST resume the existing state
- To continue writing, implementations must complete the PartialDataRow as a DataRow
- PartialDataRows cannot be overwritten; they must be completed according to their current state
- Implementations MAY choose to warn about PartialDataRows, as they indicate a transaction that was not committed or rolled back by previous code. This is not invalid, but it does mean that parity byte protections are not available yet for this row, so users should generally program their applicatons to commit or rollback before shutdown.

## 9. Transaction Specification

### 9.1. Transaction Structure

A transaction SHALL be defined as follows:

1. **Transaction Start**: A transaction always starts with a data row or null row that has start_control `T` (transaction begin).

2. **Transaction Continuation**: Subsequent rows within the transaction use start_control `R` (row continuation) and end_control `RE` (continue).

3. **Transaction End**: A transaction ends with the first subsequent data row that has an end_control ending in `C` (commit) or `0-9` (rollback), or a null row with end_control `NR`. The transaction-ending end_control sequences are: `TC`, `SC`, `R0-R9`, `S0-S9` for data rows, and `NR` for null rows.

4. **Transaction Boundaries**: After a transaction ends (with any transaction-ending command), the next data row encountered MUST have start_control `T` to begin a new transaction. Zero or one checksum rows MAY appear between the end of one transaction and the start of the next transaction.

### 9.2. Transaction Constraints

Implementations MUST enforce the following constraints:

1. **No Nested Transactions**: Transactions cannot be nested. A new transaction start (`T` start_control) MUST NOT occur until the previous transaction has ended (with a transaction-ending command).

2. **Transaction Start Requirement**: After a transaction ends, the next data row (after any checksum rows) MUST have start_control `T`. There MUST NOT be any data rows with start_control `R` between transactions.

3. **Maximum Data Rows**: A transaction MUST NOT contain more than 100 data rows. For this constraint, both complete DataRows and PartialDataRows count toward the total.

4. **Maximum Savepoints**: A transaction MUST NOT contain more than 9 user-defined savepoints (savepoints numbered 1-9).

5. **Single Transaction-Ending Command**: A transaction MUST contain exactly one transaction-ending command. Once a row with end_control ending in `C` or `0-9` is encountered, or a null row with end_control `NR`, the transaction is ended and no further rows can be added to that transaction.

6. **NullRow Single-Row Constraint**: NullRows are inherently single-row transactions and cannot contain continuation rows or savepoints. A NullRow cannot appear within an existing transaction.

### 9.3. Transaction Validity Rules

When reading transactions, implementations SHALL apply the following validity rules:

1. **Committed Transactions**: If a transaction ends with a commit command (`TC` or `SC`), all rows from the transaction start (the row with start_control `T`) through the commit row (inclusive) are valid.

2. **Rolled Back Transactions**: If a transaction ends with a rollback command:
   - **Full Rollback (`R0` or `S0`)**: All rows from the transaction start through the rollback row (inclusive) are invalidated. The entire transaction is rolled back.
   - **Partial Rollback (`R1-R9` or `S1-S9`)**: 
     - Rows from the transaction start through and including the savepoint specified by the rollback number are valid (committed).
     - All rows after that savepoint through and including the rollback row are invalidated (reverted).

### 9.4. Savepoint Tracking

Within a transaction, savepoints are tracked as follows:

1. Count rows where the first character of end_control is `S` (savepoint created on this row).
2. The first such row creates savepoint 1, the second creates savepoint 2, etc.
3. Savepoint 0 represents the transaction start (used for full rollback).
4. A transaction MUST NOT contain more than 9 user-defined savepoints (numbered 1-9).

**Savepoint creation order for `S0-S9`**: When an end_control sequence `S0-S9` is encountered, the savepoint is created on the current row first (incrementing the savepoint counter), and then the rollback is performed. This means:
- `S1` on a row with no previous savepoints: creates savepoint 1, then rolls back to savepoint 1 (the row just created).
- `S2` on a row with one previous savepoint: creates savepoint 2, then rolls back to savepoint 2 (the row just created).
- `S1` on a row with two previous savepoints: creates savepoint 3, then rolls back to savepoint 1 (the first savepoint).

### 9.5. State Machine

Implementations SHALL track transaction state:

1. **Closed**: Expecting `T` start_control (or checksum row with start_control `C`)
2. **Open**: Expecting `R` start_control

Transitions:
- Encountering a data row with `T` start_control:
  - If end_control is `RE` or `SE` → Open state (transaction continues)
  - If end_control is `TC`, `SC`, `R0-R9`, or `S0-S9` → Closed state (single-row transaction ends)
- Encountering a data row with `R` start_control:
  - If end_control is `RE` or `SE` → remain Open (transaction continues)
  - If end_control is `TC`, `SC`, `R0-R9`, or `S0-S9` → Closed state (transaction ends)
- Encountering a null row with `T` start_control and `NR` end_control → Closed state (null row transaction ends)
- Encountering a checksum row (start_control `C`) → state unchanged (checksum rows are ignored for transaction state)

### 9.6. Invalid Sequences

Implementations MUST reject:
- `R` start_control when transaction is Closed (unless preceded by a checksum row)
- `T` start_control when transaction is Open
- Rollback to savepoint N when fewer than N savepoints exist in the transaction
- Savepoint numbers > 9 in rollback commands
- More than 100 data rows in a single transaction
- More than 9 savepoints in a single transaction
- More than one transaction-ending command in a single transaction
- NullRows with start_control other than `T`
- NullRows with end_control other than `NR`
- NullRows appearing when transaction is Open (previous end_control is `RE` or `SE`)

## 10. Algorithm Details

### 10.1. Base64 Encoding

Per RFC 4648 standard Base64:
- Alphabet: A-Z, a-z, 0-9, +, /
- Padding: "=" characters as required

| Input | Output | Use |
|-------|--------|-----|
| 4 bytes | 8 bytes (with "==") | CRC32 |
| 16 bytes | 24 bytes (with "=") | UUIDv7 |

### 10.2. LRC Parity

```
function calculateParity(row_bytes, row_size):
    result = 0x00
    for i from 0 to row_size - 4:
        result = result XOR row_bytes[i]
    return toUpperHex(result, 2)  // e.g., 0x1F → "1F"
```

### 10.3. CRC32

IEEE polynomial 0xedb88320 (LSB-first). Equivalent to Go's `crc32.ChecksumIEEE()`.
