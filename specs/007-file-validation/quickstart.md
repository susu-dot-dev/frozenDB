# Quickstart Guide: frozenDB File Validation Security

**Version**: 1.0  
**Date**: 2025-01-13  
**Branch**: 007-file-validation

## Overview

This quickstart guide demonstrates how to use frozenDB's enhanced file validation security features. These features protect against malicious file manipulation, corruption, and buffer overflow attacks while maintaining the database's core performance characteristics.

## Basic Setup

### Import Statement

```go
import (
    "github.com/susu-dot-dev/frozenDB/frozendb"
    "time"
)
```

### Security-Enabled Database Creation

```go
// Create a new database with security validation enabled
func main() {
    config := frozendb.CreateConfig{
        RowSize:      1024,              // 1KB rows (128-65536)
        SkewMs:       1000,              // 1 second time skew (0-86400000)
        SecurityMode: true,              // Enable security validation
        Timeout:      5 * time.Second,   // Operation timeout
    }
    
    db, err := frozendb.Create("mydb.fdb", config)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    fmt.Println("Secure database created successfully")
}
```

### Security-Enabled Database Opening

```go
// Open existing database with security validation
func openSecureDatabase(path string) (*frozendb.DB, error) {
    return frozendb.Open(path,
        frozendb.WithSecurityMode(true),
        frozendb.WithTimeout(5*time.Second),
        frozendb.WithCRCValidation(true),
    )
}

// Usage
db, err := openSecureDatabase("existing.fdb")
if err != nil {
    // Handle security validation errors
    var validationErr *frozendb.ValidationError
    if errors.As(err, &validationErr) {
        fmt.Printf("Security validation failed: %s (code: %s)\n", 
            validationErr.Message, validationErr.Code)
        return nil, err
    }
    panic(err)
}
defer db.Close()
```

## Security Validation Examples

### Comprehensive File Integrity Check

```go
func validateDatabaseIntegrity(db *frozendb.DB) error {
    // Enable enhanced security mode
    db.EnableSecurityMode()
    
    // Perform comprehensive validation
    err := db.VerifyIntegrity()
    if err != nil {
        return fmt.Errorf("integrity check failed: %w", err)
    }
    
    // Get validation status
    status := db.GetValidationStatus()
    fmt.Printf("File size: %d bytes\n", status.FileSize)
    fmt.Printf("Rows: %d\n", status.RowCount)
    fmt.Printf("Last validated: %s\n", status.LastValidated)
    fmt.Printf("Validation time: %s\n", status.ValidationTime)
    
    return nil
}
```

### Header Validation

```go
func validateFileHeader(db *frozendb.DB) error {
    header, err := db.GetHeader()
    if err != nil {
        return fmt.Errorf("header validation failed: %w", err)
    }
    
    fmt.Printf("Signature: %s\n", header.Signature)
    fmt.Printf("Version: %d\n", header.Version)
    fmt.Printf("Row Size: %d bytes\n", header.RowSize)
    fmt.Printf("Time Skew: %d ms\n", header.SkewMs)
    
    // Manual header validation
    err = db.ValidateHeader()
    if err != nil {
        return fmt.Errorf("header integrity check failed: %w", err)
    }
    
    return nil
}
```

### Checksum Validation

```go
func validateChecksums(db *frozendb.DB) error {
    // Validate checksum row structure
    err := db.ValidateChecksumRow()
    if err != nil {
        return fmt.Errorf("checksum validation failed: %w", err)
    }
    
    // Calculate and verify CRC32 for header
    crc32, err := db.CalculateCRC32(0, 64) // Header bytes [0..63]
    if err != nil {
        return fmt.Errorf("CRC32 calculation failed: %w", err)
    }
    
    fmt.Printf("Header CRC32: %08x\n", crc32)
    
    return nil
}
```

## Security Error Handling

### Handling Validation Errors

```go
func handleSecurityErrors(err error) {
    var validationErr *frozendb.ValidationError
    if errors.As(err, &validationErr) {
        switch validationErr.Code {
        case "header_invalid_signature":
            fmt.Println("❌ Invalid file signature - not a frozenDB file")
            
        case "header_invalid_row_size":
            fmt.Println("❌ Invalid row size - possible corruption")
            
        case "file_corrupted":
            fmt.Printf("❌ File corruption detected at offset %d\n", 
                validationErr.Offset)
            
        case "buffer_overflow_detected":
            fmt.Println("🛡️  Buffer overflow attempt prevented")
            
        case "integer_overflow_detected":
            fmt.Println("🛡️  Integer overflow attempt prevented")
            
        case "validation_timeout":
            fmt.Println("⏰ Validation timed out - possible DoS attempt")
            
        default:
            fmt.Printf("❌ Security validation failed: %s\n", 
                validationErr.Message)
        }
        
        if validationErr.Cause != nil {
            fmt.Printf("Underlying cause: %v\n", validationErr.Cause)
        }
        return
    }
    
    // Non-validation error
    fmt.Printf("Unexpected error: %v\n", err)
}
```

### Security Mode Configuration

```go
func configureSecurityMode(db *frozendb.DB) {
    // Enable security with custom timeout
    db.SetValidationTimeout(10 * time.Second)
    
    // Check current status
    status := db.GetValidationStatus()
    if !status.SecurityEnabled {
        fmt.Println("Enabling security mode...")
        db.EnableSecurityMode()
    }
    
    // Periodic validation
    ticker := time.NewTicker(5 * time.Minute)
    go func() {
        for range ticker.C {
            if err := db.VerifyIntegrity(); err != nil {
                handleSecurityErrors(err)
            }
        }
    }()
}
```

## Advanced Security Features

### Safe File Operations

```go
import "github.com/susu-dot-dev/frozenDB/frozendb/security"

func safeFileOperations() error {
    // Safe file reading with bounds checking
    file, err := os.Open("database.fdb")
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Get file info for bounds checking
    info, err := file.Stat()
    if err != nil {
        return err
    }
    
    // Safe read at specific offset
    data, err := security.SafeRead(file, 64, 1024) // Read 1024 bytes at offset 64
    if err != nil {
        return fmt.Errorf("safe read failed: %w", err)
    }
    
    fmt.Printf("Read %d bytes safely\n", len(data))
    return nil
}
```

### Bounds Validation

```go
func demonstrateBoundsValidation() {
    var fileSize int64 = 1024
    var offset int64 = 1000
    var length int64 = 100
    
    // Validate bounds before operation
    err := security.ValidateBounds(fileSize, offset, length)
    if err != nil {
        fmt.Printf("Bounds validation failed: %v\n", err)
        return
    }
    
    // Safe arithmetic operations
    sum, err := security.SafeAdd(offset, length)
    if err != nil {
        fmt.Printf("Safe addition failed: %v\n", err)
        return
    }
    
    fmt.Printf("Safe result: %d\n", sum)
}
```

## Performance Considerations

### Optimized Security Configuration

```go
func optimizedSecurityConfig() frozendb.CreateConfig {
    return frozendb.CreateConfig{
        RowSize:      2048,              // Optimal for most workloads
        SkewMs:       5000,              // 5 seconds for distributed systems
        SecurityMode: true,              // Always enable in production
        Timeout:      2 * time.Second,   // Reasonable timeout
    }
}
```

### Performance Monitoring

```go
func monitorSecurityPerformance(db *frozendb.DB) {
    start := time.Now()
    
    err := db.VerifyIntegrity()
    duration := time.Since(start)
    
    if err != nil {
        handleSecurityErrors(err)
        return
    }
    
    status := db.GetValidationStatus()
    fmt.Printf("Validation completed in %s\n", duration)
    fmt.Printf("Validation rate: %.2f MB/s\n", 
        float64(status.FileSize)/duration.Seconds()/1024/1024)
}
```

## Testing Security Features

### Security Test Example

```go
func TestSecurityValidation(t *testing.T) {
    // Create database with security enabled
    config := frozendb.CreateConfig{
        RowSize:      128,
        SecurityMode: true,
    }
    
    db, err := frozendb.Create("test.fdb", config)
    require.NoError(t, err)
    defer db.Close()
    
    // Verify integrity passes
    err = db.VerifyIntegrity()
    assert.NoError(t, err)
    
    // Simulate corrupted file and test detection
    corruptedFile := createCorruptedFile()
    _, err = frozendb.OpenFromBytes(corruptedFile, 
        frozendb.WithSecurityMode(true))
    assert.Error(t, err)
    
    var validationErr *frozendb.ValidationError
    assert.True(t, errors.As(err, &validationErr))
}
```

## Best Practices

### Production Deployment

1. **Always enable SecurityMode** in production environments
2. **Set appropriate timeouts** based on expected file sizes
3. **Monitor validation performance** and adjust timeouts if needed
4. **Log security violations** for monitoring and alerting
5. **Regular integrity checks** for critical data

### Development Workflow

1. **Enable security during development** to catch issues early
2. **Write security-focused tests** alongside functional tests
3. **Test with malicious files** to verify protection mechanisms
4. **Profile security overhead** and optimize as needed
5. **Document security assumptions** in code comments

This quickstart guide provides the essential information for implementing and using frozenDB's security validation features while maintaining performance and backward compatibility.