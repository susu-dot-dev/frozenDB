# Data Model: Enhanced File Creation and Validation

**Branch**: 007-file-validation  
**Date**: 2025-01-13  
**Purpose**: Define enhanced validation logic and security checks for file creation and loading

## Enhanced Validation Logic

All security validation failures use the existing `FileCorruptedError` with descriptive messages indicating the specific security issue detected.

**Security Scenarios:**
- Buffer overflow attempts - read would exceed file boundaries
- Integer overflow detection - arithmetic operation would overflow  
- Malicious row_size - row size indicates potential attack
- Incomplete checksum row - checksum row truncated or missing

## Enhanced Creation and Validation Logic

### Enhanced File Creation Flow

```
1. Validate CreateConfig parameters (row_size, skew_ms bounds)
2. Create valid FileHeader structure
3. Calculate initial CRC32 checksum for header bytes [0..63]
4. Create ChecksumRow with calculated checksum
5. Validate complete file structure (header + checksum row)
6. Atomic write: header + checksum row in single operation
7. Post-write validation to ensure atomicity
```

**Key Enhancement:** Calculate checksum row completely before any disk write, then perform atomic write of both header and checksum row.

### Enhanced File Loading Flow

```
1. Validate file size >= 64 bytes (minimum header size)
2. Read and validate header with bounds checking
3. Validate file size >= 64 + row_size (header + checksum row)
4. Security check: row_size bounds (128-65536) before seeking
5. Read checksum row at offset row_size with overflow protection
6. Validate checksum row structure (control bytes, parity, etc.)
7. Verify CRC32 covers header bytes [0..63]
8. Validate parity bytes match row content
9. Return validated database instance
```

**Key Enhancement:** All buffer boundary checks before trusting row_size from potentially corrupted header.

## Security Validation Rules

### Bounds-Checked Operations

```go
func safeSeekAndRead(file *os.File, offset int64, length int64, maxSize int64) ([]byte, error) {
    // Check for negative values
    if offset < 0 || length < 0 {
        return nil, NewFileCorruptedError("invalid bounds detected")
    }
    
    // Check for integer overflow in offset + length
    if offset > math.MaxInt64-length {
        return nil, NewFileCorruptedError("integer overflow detected in bounds calculation")
    }
    
    // Check file boundaries before seeking
    if offset+length > maxSize {
        return nil, NewFileCorruptedError("buffer overflow would exceed file boundaries")
    }
    
    // Safe seek and read
    _, err := file.Seek(offset, io.SeekStart)
    if err != nil {
        return nil, err
    }
    
    buf := make([]byte, length)
    _, err = io.ReadFull(file, buf)
    return buf, err
}
```

### Row Size Security Validation

```go
func validateRowSizeSecurity(rowSize int, fileSize int64) error {
    // Check standard bounds
    if rowSize < 128 || rowSize > 65536 {
        return NewFileCorruptedError("row_size outside valid range (128-65536)")
    }
    
    // Check if row_size would cause file to be too small
    if int64(64+rowSize) > fileSize {
        return NewFileCorruptedError("file too small to contain checksum row")
    }
    
    // Check for malicious row_size values
    if rowSize > int(fileSize-64) {
        return NewFileCorruptedError("malicious row_size would cause buffer overflow")
    }
    
    return nil
}
```

## Implementation Requirements

### Enhanced Create() Function

The existing `Create()` function must be modified to:
1. Calculate complete checksum row before any disk writes
2. Perform atomic write of header + checksum row together
3. Validate the complete structure post-write
4. Handle checksum calculation failures gracefully

### Enhanced Load/Validation Functions

The existing file opening functions must be modified to:
1. Validate file size before trusting any header content
2. Use bounds-checked reads for all file operations
3. Validate row_size before seeking to checksum row
4. Verify checksum row structure and integrity
5. Prevent buffer overflows from malicious row_size values

### Security Validation Integration

1. **All file reads must be bounds-checked**
2. **All arithmetic must be overflow-safe**
3. **Row_size validation before any seeking operations**
4. **CRC32 verification before trusting checksum row content**
5. **Parity validation despite potential file corruption**

## Security Constraints

### Memory Constraints
- Fixed buffer sizes (max row_size) regardless of file content
- No memory allocation during validation loops
- Buffer reuse for all read operations

### Performance Constraints
- File creation <100ms (including checksum calculation)
- File validation <50ms for typical file sizes
- No performance impact when security mode is disabled

### Access Constraints
- File locking during write operations (existing)
- Concurrent read support during validation
- Atomic operations for header + checksum row write

## Integration Points

### Existing Functions to Modify
- `Create()` - Add checksum row calculation and atomic write
- File opening/validation functions - Add security validation
- Header parsing - Add bounds checking and overflow protection

### Existing Functions to Preserve
- All existing data structures (Header, ChecksumRow, etc.)
- All existing validation logic (enhanced, not replaced)
- All existing API functions (security is additive)

This data model focuses on the specific enhancements needed: integrating checksum row writing into file creation and adding comprehensive security validation to file loading operations.

This data model provides the foundation for implementing secure file creation and validation while maintaining frozenDB's core principles of immutability, data integrity, and performance with fixed memory usage.