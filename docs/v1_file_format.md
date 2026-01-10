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

The first row in a transaction uses start_control `T` (Transaction begin). Subsequent rows use start_control `R` (Row continuation). The final row's end_control indicates how the transaction ended.

### 2.2. Savepoints

Savepoints allow partial rollbacks within a transaction. When a savepoint is created, the current row is marked, and a later rollback can return to that point.

**Key insight**: Since the file is append-only, "rollback" doesn't delete rows—it marks them as invalid. Readers parse the transaction to its end, check for rollback markers, and exclude invalidated rows.

**Savepoint numbering**: Savepoints are numbered 1-9 in creation order. Savepoint 0 represents the transaction start (full rollback).

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

### 2.3. End Control Character Design

The end_control is a 2-character sequence that encodes both savepoint creation and transaction termination in a space-efficient manner:

| First Char | Meaning |
|------------|---------|
| `T` or `R` | No savepoint on this row |
| `S`        | Savepoint created on this row |

| Second Char | Meaning |
|-------------|---------|
| `C`         | Commit transaction |
| `E`         | Continue (more rows follow) |
| `0-9`       | Rollback to savepoint N (terminates transaction) |

**Combined sequences:**

| Sequence | Meaning |
|----------|---------|
| `TC`     | Commit, no savepoint |
| `RE`     | Continue, no savepoint |
| `SC`     | Commit + savepoint on this row |
| `SE`     | Continue + savepoint on this row |
| `R0-R9`  | Rollback to savepoint N, no savepoint on this row |
| `S0-S9`  | Rollback to savepoint N + savepoint on this row |

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

### 2.5. Reading Transactions

When reading a frozenDB file, implementations MUST:

1. Parse each transaction from its first row (start_control `T`) to its terminating row (end_control ending in `C` or `0-9`)
2. Check the terminating row's end_control:
   - If `*C` (commit): Include all rows in the transaction
   - If `*0` (rollback to 0): Exclude all rows in the transaction
   - If `*N` where N > 0: Include rows up to and including savepoint N; exclude rows after
3. Savepoints are numbered by counting rows with `S` as their first end_control character, in order (first = 1, second = 2, etc.)

## 3. File Structure

A frozenDB v1 file consists of:
1. A 64-byte header
2. A checksum row (required)
3. Zero or more data rows
4. Additional checksum rows inserted every 10,000 data rows

```
Offset:    0          64        64+row_size   64+2*row_size
           ├──────────┼─────────┼─────────────┼─────────────┤
           │  Header  │Checksum │  Data Row 0 │  Data Row 1 │ ...
           └──────────┴─────────┴─────────────┴─────────────┘
```

### 3.1. Terminology and Byte Definitions

**Row Structure:**
- **ROW_START**: Byte value 0x1F (UTF-8: U+001F, unit separator) marking row beginning
- **ROW_END**: Byte value 0x0A (UTF-8: U+000A, newline) marking row end
- **start_control**: Single byte representing an uppercase alphanumeric character (UTF-8: U+0030-U+0039 for digits 0-9, U+0041-U+005A for letters A-Z) identifying row type
- **end_control**: Two bytes, each representing an uppercase alphanumeric character (same range as start_control) indicating row termination
- **parity_bytes**: Two bytes representing uppercase hexadecimal digits (UTF-8: U+0030-U+0039, U+0041-U+0046) for LRC checksum

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
- Input: All bytes covered since previous checksum row (or from offset 64 for first checksum)
- Encoding: Standard Base64 of 4-byte CRC32 value (8 bytes output with "==" padding)

### 6.3. Placement Rules

1. First checksum row: Immediately after header (offset 64). This checksum row MUST be present and MUST be validated when reading the file.
2. Subsequent: After every 10,000 data rows. A checksum row MUST be placed before the 10,001st data row is written. Implementations MAY choose to write the checksum immediately after writing the 10,000th row, or defer it until just before writing the 10,001st row.
3. File may end after any number of data rows. If a file ends with fewer than 10,000 data rows since the last checksum, no final checksum is required.

## 7. Data Corruption Detection

### 7.1. Initial Checksum Row Validation

When reading a frozenDB file, implementations MUST parse and validate the checksum row that immediately follows the header (at offset 64). This checksum row covers the initial data rows and MUST be validated to ensure data integrity. The header itself does not contain a checksum, but the first checksum row MUST be present and validated.

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
- Rows 10,000-11,999: If validated, use parity bytes

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
| `T` | First row of file, or previous row ended with `*C` or `*0-9` (transaction boundary) |
| `R` | Previous row ended with `*E` (transaction continues) |

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

### 8.4. UUIDv7 Requirements

- MUST be globally unique
- MUST be Base64 encoded (24 bytes with "=" padding)
- Timestamp component minus `skew_ms` MUST be ≥ previous row's timestamp

### 8.5. Padding Calculation

```
padding_bytes = row_size - len(json_payload) - 31
```

Where 31 = 1 (ROW_START) + 1 (start_control) + 24 (UUID) + 2 (end_control) + 2 (parity) + 1 (ROW_END)

## 9. Transaction Validation

### 9.1. State Machine

Implementations SHALL track transaction state:

1. **Closed**: Expecting `T` start_control
2. **Open**: Expecting `R` start_control

Transitions:
- Encountering a row with `T` start_control → Open state
- Encountering a row with `*E` end_control → remain Open
- Encountering a row with `*C` or `*0-9` end_control → Closed state

### 9.2. Savepoint Tracking

Within a transaction:
1. Count rows with `S` as first end_control character
2. First `S` row = savepoint 1, second = savepoint 2, etc.
3. Maximum 9 savepoints per transaction

### 9.3. Invalid Sequences

Implementations MUST reject:
- `R` start_control when transaction is Closed
- `T` start_control when transaction is Open
- Rollback to savepoint N when fewer than N savepoints exist
- Savepoint numbers > 9

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
