# frozenDB v1 File Format Specification

## 1. Introduction

This document defines the frozenDB v1 on-disk file format. frozenDB is an immutable key-value store with append-only storage and UUIDv7 keys. The v1 format is fully text-based to facilitate human readability and debugging.

### 1.1. Conformance and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

### 1.2. File Encoding

All text in a frozenDB v1 file SHALL be encoded using UTF-8. Implementations MUST accept UTF-8 encoded input and MUST generate UTF-8 encoded output.

## 2. File Structure

A frozenDB v1 file consists of a fixed-width header followed by zero or more fixed-width data rows.

```
File Offset:
0            64          64+row_size     64+2*row_size
├────────────┼────────────┼──────────────┼──────────────┤
│   Header   │    Row 0   │    Row 1     │    Row 2     │
└────────────┴────────────┴──────────────┴──────────────┘
```

The header occupies bytes 0 through 63 (inclusive). Data rows begin at offset 64.

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
- Bytes [0-N) do not contain valid JSON, where N is the index of the first null character (U+0000)
- Bytes [N-63) is not U+0000 (null)
- Byte 63 is not a newline character
- JSON object does not contain exactly four keys
- Keys are not in the required order sig,ver,row_size
- JSON string contains a newline character
- Any field value is missing, of incorrect type, or outside valid ranges
- The signature field is not `"fDB"`
- The version field is not `1`
