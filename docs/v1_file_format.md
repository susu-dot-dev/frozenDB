# frozenDB v1 File Format Specification

## 1. Introduction

This document defines the frozenDB v1 on-disk file format. frozenDB is an immutable key-value store with append-only storage and UUIDv7 keys. The v1 format is fully text-based to facilitate human readability and debugging.

### 1.1. Conformance and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

### 1.2. File Encoding

All text in a frozenDB v1 file SHALL be encoded using UTF-8. Implementations MUST accept UTF-8 encoded input and MUST generate UTF-8 encoded output.

### 1.3. Terminology

This section defines key terms and constants used throughout this specification.

- **ROW_START**: The byte value 0x1F (unit separator) that marks the beginning of a data row
- **ROW_END**: The byte value 0x0A (newline character) that marks the end of a data row
- **start_control**: The first byte after ROW_START that identifies the row type; SHALL be a single uppercase alphanumeric character (A-Z, 0-9)
- **end_control**: Exactly 2 bytes immediately preceding the parity bytes that identify the row type termination; each SHALL be an uppercase alphanumeric character (A-Z, 0-9)
- **parity_bytes**: Exactly 2 UTF-8 characters (uppercase hexadecimal digits 0-9, A-F) representing the longitudinal redundancy check (LRC) of row data
- **LRC (Longitudinal Redundancy Check)**: An error detection method that computes the XOR of all bytes in a specified range
- **CRC32**: A 32-bit cyclic redundancy check using the IEEE polynomial 0xedb88320
- **checksum_row**: A row type containing CRC32 data for integrity verification of subsequent rows

## 2. File Structure

A frozenDB v1 file consists of a fixed-width header followed by a checksum row, then zero or more data rows, with additional checksum rows inserted at regular intervals.

```
File Offset:
0            64          64+row_size     64+2*row_size
├────────────┼────────────┼──────────────┼──────────────┼
│   Header   │Checksum Row│   Data Row 0 │   Data Row 1 │
└────────────┴────────────┴──────────────┴──────────────┴
```

The header occupies bytes 0 through 63 (inclusive). The first checksum row occupies bytes 64 through (63 + row_size). Subsequent rows begin at offsets calculated as multiples of row_size added to 64.

A valid frozenDB file MUST have a checksum row immediately following the header. The file structure SHALL be: Header → Checksum Row → Data Rows, with additional checksum rows inserted after every 10,000 data rows. The pattern SHALL be: Header → Checksum Row → (up to 10,000 Data Rows) → Checksum Row → (up to 10,000 Data Rows) → Checksum Row → ..., where the file may end after any number of data rows.

## 3. Header Specification

### 3.1. Header Structure

The header SHALL be exactly 64 bytes in length. The header SHALL contain a JSON object with specific key ordering, followed by padding characters and terminated by a newline character.

### 3.2. Header Format

The header SHALL follow this exact format:

```
{"sig":"fDB","ver":1,"row_size":<size>,"skew_ms":<skew>}\x00\x00\x00\x00\x00\x00\n
```

Where:
- `<size>` SHALL be an integer between 128 and 65536 (inclusive)
- `<skew>` SHALL be an integer between 0 and 86400000 (inclusive)
- The four keys SHALL appear in exactly this order: `sig`, `ver`, `row_size`, `skew_ms`
- All JSON keys and string values SHALL use double quotes
- Padding SHALL consist of null characters (U+0000) as needed to fill to 63 bytes
- Byte 63 SHALL be a newline character (U+000A)

### 3.3. Header Fields

#### 3.3.1. Signature Field (sig)

The `sig` field SHALL contain the string value `"fDB"`. This field identifies the file as a frozenDB v1 file. Implementations MUST reject files where the signature field is missing or has any value other than `"fDB"`.

#### 3.3.2. Version Field (ver)

The `ver` field SHALL contain the integer value `1`. This field identifies the format version. Implementations MUST reject files where the version field is missing or has any value other than `1`. Future versions of this specification may define additional valid values.

#### 3.3.3. Row Size Field (row_size)

The `row_size` field SHALL contain an integer value between 128 and 65536 (inclusive). This value specifies the total size of each data row in bytes. The row_size includes all row components: sentinel characters, user data of variable length up to the maximum allowed, and padding characters. The value represents bytes, not bits.

Each row MUST reserve exactly TODO bytes for sentinel characters (including start sentinel, end sentinel, and any transaction markers). The remaining bytes after the sentinel characters are available for user data and padding. Implementations MUST reject files where the row_size field is missing, not an integer, or outside the specified range.

#### 3.3.4. Skew Milliseconds Field (skew_ms)

The `skew_ms` field SHALL contain an integer value between 0 and 86400000 (inclusive). This value specifies the maximum time skew window in milliseconds for UUIDv7 key lookups. The range of 0 to 86400000 represents 0 to 24 hours. Implementations MUST reject files where the skew_ms field is missing, not an integer, or outside the specified range.

### 3.4. Padding Requirements

Padding characters SHALL be used as needed to ensure the header occupies exactly 64 bytes. Padding SHALL consist solely of null characters (U+0000). The number of padding characters SHALL be calculated as 63 minus the length of the JSON content (excluding the newline).

### 3.5. Header Termination

Byte 63 SHALL be a newline character (U+000A). Implementations MUST verify the presence of this newline character when parsing the header.

### 3.6. Header Size Constraints

The JSON content (excluding padding and newline) SHALL be between 49 and 58 bytes in length:
- Minimum: `{"sig":"fDB","ver":1,"row_size":128,"skew_ms":0}` (49 bytes)
- Maximum: `{"sig":"fDB","ver":1,"row_size":65536,"skew_ms":86400000}` (58 bytes)

Therefore, the padding SHALL be between 5 and 14 characters in length.

### 3.7. Header Parsing Requirements

Implementations SHALL:
1. Read exactly 64 bytes from the file start
2. Verify byte 63 is a newline character (U+000A)
3. Find the first null character (U+0000) and extract bytes [0, N-1] as JSON content
4. Parse the JSON content using standard JSON parsing libraries with proper double-quoted keys and values
5. Verify that the JSON object contains exactly four keys in the order: `sig`, `ver`, `row_size`, `skew_ms`
6. Validate each field value according to sections 3.3.1 through 3.3.4
7. Verify that all characters between the end of the JSON content and byte 62 are null characters (U+0000)

### 3.8. Header Error Handling

Implementations SHALL reject files and report an error if any of the following conditions occur:
- File contains fewer than 64 bytes
- Bytes [0..X-1] do not contain valid JSON, where X is the index of the first null character (U+0000)
- Bytes [X..63] are not U+0000 (null)
- Byte 63 is not a newline character
- JSON object does not contain exactly four keys
- Keys are not in the required order sig,ver,row_size
- JSON string contains a newline character
- Any field value is missing, of incorrect type, or outside valid ranges
- The signature field is not `"fDB"`
- The version field is not `1`

## 4. Data Row Specification

### 4.1. Row Structure

Each data row in a frozenDB v1 file SHALL follow this basic structure:

```
Byte Layout (all ranges are inclusive, zero-based indexing):
[0]           ROW_START (0x1F)
[1]           Start Control Character (1 byte)
[2..N-6]      Row Content (varies by row type, includes padding)
[N-5..N-4]    End Control Characters (2 bytes)
[N-3..N-2]    Parity Bytes (2-byte UTF-8 hex string "00"-"FF")
[N-1]         ROW_END (0x0A)
```

Where:
- N is the total row size specified in the header's `row_size` field
- Start Control Character identifies the row type (see section 4.3)
- End Control Characters provide row type termination validation (see section 4.3)
- Row Content varies by specific row type and is defined in subsequent sections
- All row types MUST conform to this basic structure with parity protection

### 4.2. Parity Calculation

The parity bytes SHALL be computed using a longitudinal redundancy check (LRC) algorithm:

1. Compute the XOR of all bytes from `[0..N-4]`
2. Convert the resulting byte value to a 2-character uppercase hexadecimal string
3. Encode this string as UTF-8 characters in the parity_bytes field `[N-2..N-1]`

The parity bytes MUST be exactly 2 UTF-8 characters representing the hexadecimal value of the computed checksum:
- Characters SHALL be uppercase hexadecimal digits (0-9, A-F)
- The string MUST NOT include any prefix (no "0x" or similar)
- The string MUST NOT include any suffix (no "h" or similar)
- Examples: "00", "1F", "A3", "FF"

The parity calculation includes all bytes between ROW_START and the end control characters, including any padding bytes that may be present in the row content.

Example: For a row where bytes [0] through [N-4] result in an XOR value of 0x1F, the parity bytes SHALL be the UTF-8 string "1F" (bytes 0x31 and 0x46).

### 4.3. Row Types

#### 4.3.1. Start Control Characters

The byte at position [1] immediately following ROW_START SHALL be the start_control character. Start_control characters SHALL be single UTF-8 bytes representing uppercase alphanumeric characters (A-Z, 0-9). The valid byte range for start_control characters SHALL be 0x30-0x39 (digits 0-9) and 0x41-0x5A (uppercase letters A-Z).

The following start_control characters are defined:

- **C (0x43)**: Checksum row type
- Additional start_control characters MAY be defined in future specifications, limited to the alphanumeric range specified above

Implementations MUST reject rows containing start_control characters outside the defined alphanumeric range or containing undefined start_control characters within the valid range.

#### 4.3.2. End Control Characters

The bytes at positions [N-5] and [N-4] immediately preceding the parity bytes SHALL be the end_control characters. End_control characters SHALL be two UTF-8 bytes representing uppercase alphanumeric characters (A-Z, 0-9). The valid byte range for each end_control character SHALL be 0x30-0x39 (digits 0-9) and 0x41-0x5A (uppercase letters A-Z). All byte positions use inclusive, zero-based indexing.

The following end_control character sequences are defined:

- **CS (0x43 0x53)**: Checksum row type termination
- Additional end_control character sequences MAY be defined in future specifications, limited to the alphanumeric range specified above

Implementations MUST reject rows containing end_control characters outside the defined alphanumeric range or containing undefined end_control character sequences within the valid range.

#### 4.3.3. Row Type Validation

For each row type, the start_control character and end_control character sequence MUST correspond according to this specification. Implementations MUST validate that the start_control and end_control characters match the defined row type.

### 4.4. Checksum Row (C/CS) Specification

#### 4.4.1. Checksum Row Format

Checksum rows SHALL use start_control character C (0x43) and end_control characters CS (0x43 0x53). The format SHALL be:

```
ROW_START|C|crc32_b64_encoded|CS|parity|\n
```

Where:
- ROW_START is byte 0x1F at position [0]
- C is the start_control character (0x43) at position [1]
- crc32_b64_encoded is exactly 8 bytes containing standard base64 encoding at positions [2..9]
- CS is the end_control character sequence (0x43 0x53) at positions [10..11]
- parity is the 2-byte LRC checksum at positions [12..13]
- \n is ROW_END (0x0A) at position [N-1]
All byte positions use inclusive, zero-based indexing.

#### 4.4.2. CRC32 Calculation

The crc32_b64_encoded field SHALL contain the base64 encoding of a CRC32 checksum calculated as follows:

1. **Algorithm**: IEEE CRC32 with polynomial 0xedb88320 (LSB-first representation)
2. **Input Range**: All bytes from the ROW_START character immediately following the previous checksum row through the ROW_END character of the row immediately preceding this checksum row
   - For the first checksum row in a file: bytes from offset 64 through the ROW_END at offset (64 + row_size - 1) inclusive
   - For subsequent checksum rows: bytes from the ROW_START after the previous checksum row through the ROW_END at the last byte of the row immediately before this checksum row
3. **Output**: 32-bit unsigned integer (4 bytes)
4. **Encoding**: Standard base64 encoding (RFC 4648) resulting in exactly 8 bytes including padding

The CRC32 calculation SHALL use the same algorithm as Go's `crc32.ChecksumIEEE()` function. Implementations using other languages MUST produce identical results for identical input data.

#### 4.4.3. Base64 Encoding Requirements

The crc32_b64_encoded field SHALL be exactly 8 bytes containing standard base64 encoding:

- **Alphabet**: Standard base64 alphabet (A-Z, a-z, 0-9, +, /)
- **Padding**: Standard base64 padding with "=" characters as required
- **Input**: 4 bytes (32-bit CRC32 value)
- **Output**: Exactly 8 bytes including padding characters

Examples:
- CRC32 value 0x00000000 → Base64 "AAAAAA=="
- CRC32 value 0x12345678 → Base64 "EjRWeA=="
- CRC32 value 0xFFFFFFFF → Base64 "/////w=="

#### 4.4.4. Checksum Row Placement Requirements

Checksum rows SHALL be placed according to the following rules:

1. **First Checksum Row**: A checksum row MUST be the first row in every frozenDB file, immediately following the header at offset 64
2. **Subsequent Checksum Rows**: Additional checksum rows SHALL be inserted after every 10,000 non-checksum rows, but only if there are data rows present
3. **Row Counting**: The 10,000 row count SHALL include only non-checksum rows; checksum rows SHALL NOT be counted toward this total
4. **File Termination**: A frozenDB file MAY end after any number of data rows; a final checksum row is not required unless the 10,000-row threshold is reached
5. **Pattern**: The file structure SHALL follow the pattern: Header → Checksum Row → (0 to 10,000 Data Rows) → Checksum Row (if 10,000 data rows reached) → (0 to 10,000 Data Rows) → Checksum Row (if another 10,000 data rows reached) → ...

Row numbering examples:
- Row 1: Checksum row (required)
- Rows 2-5: Data rows (file ends here - valid)
- Row 1: Checksum row (required)
- Rows 2-10,001: Data rows (10,000 total)
- Row 10,002: Checksum row (required because 10,000 data rows reached)
- Rows 10,003-10,010: Data rows (8 total - file ends here - valid)

#### 4.4.5. Checksum Row Validation

Implementations SHALL validate checksum rows according to these requirements:

1. Verify start_control character is C (0x43)
2. Verify end_control characters are CS (0x43 0x53)
3. Verify crc32_b64_encoded field is exactly 8 bytes containing valid standard base64 encoding
4. Compute the CRC32 of the specified byte range and verify it matches the decoded value
5. Verify parity bytes using the LRC algorithm specified in section 4.2

### 4.5. Data Row Types

Additional row types for data storage (key-value pairs, metadata, etc.) SHALL be defined in future specifications. All future row types MUST conform to the basic row structure defined in section 4.1 and MUST use defined start_control and end_control character sequences.
