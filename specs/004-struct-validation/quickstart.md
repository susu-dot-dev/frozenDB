# Quickstart Guide: Struct Validation and Immutability

This guide shows you how to work with frozenDB's struct validation and immutability patterns to ensure data integrity and prevent invalid state.

## Core Concepts

### Validation Pattern
Every struct that can have invalid fields implements a `Validate() error` method. This method validates the struct's fields and contextual requirements.

### Three Constructor Paths
1. **Direct Initialization**: `struct{field: value}` - You must call `Validate()`
2. **Constructor Function**: `NewStruct()` - Automatically calls `Validate()`
3. **UnmarshalText**: Text deserialization - Automatically calls `Validate()`

### Field Immutability
- Critical fields are unexported (lowercase)
- Public access provided through getter functions
- No post-construction modification allowed

## Quick Examples

### 1. Header Struct - Database File Metadata

```go
// ✅ RECOMMENDED: Use constructor function
header, err := frozendb.NewHeader(headerBytes)
if err != nil {
    return fmt.Errorf("invalid header: %w", err)
}

// ✅ ALTERNATIVE: Direct initialization with validation
header := &frozendb.Header{
    signature: "fDB",
    version: 1,
    rowSize: 1024,
    skewMs: 5000,
}
if err := header.Validate(); err != nil {
    return fmt.Errorf("header validation failed: %w", err)
}

// ✅ READ ACCESS: Use getter functions
fmt.Printf("Version: %d\n", header.GetVersion())
fmt.Printf("Row Size: %d\n", header.GetRowSize())
```

### 2. CreateConfig - Database Creation

```go
// ✅ RECOMMENDED: Use constructor with validation
config := frozendb.CreateConfig{
    Path:    "/tmp/mydb.fdb",
    RowSize: 2048,
    SkewMs:  1000,
}

if err := frozendb.Create(config); err != nil {
    return fmt.Errorf("database creation failed: %w", err)
}

// ✅ READ ACCESS: Getters for configuration
fmt.Printf("Database path: %s\n", config.GetPath())
```

### 3. Row Controls - Transaction Management

```go
// ✅ UNMARSHAL: Automatic validation
var startControl frozendb.StartControl
err := startControl.UnmarshalText([]byte("T"))
if err != nil {
    return fmt.Errorf("invalid start control: %w", err)
}

// ✅ CONTEXT VALIDATION: Parent validates child requirements
checksumRow, err := frozendb.NewChecksumRow(header, dataBytes)
if err != nil {
    // This will fail if StartControl is not 'C' in checksum row context
    return fmt.Errorf("checksum row validation failed: %w", err)
}
```

### 4. SudoContext - Privileged Operations

```go
// ✅ DETECTION: Automatic validation in constructor
sudoCtx, err := frozendb.DetectSudoContext()
if err != nil {
    return fmt.Errorf("sudo context invalid: %w", err)
}

// ✅ READ ACCESS: Getters for security information
fmt.Printf("Sudo user: %s (UID: %d)\n", sudoCtx.GetUser(), sudoCtx.GetUID())
```

## Common Patterns

### Pattern 1: Constructor Function
```go
func NewValidStruct(input Data) (*ValidStruct, error) {
    // Parse/setup fields
    s := &ValidStruct{
        field1: parseField1(input),
        field2: parseField2(input),
        child:  NewChild(input.childData), // Child validates itself
    }
    
    // Validate before returning
    return s, s.Validate()
}
```

### Pattern 2: UnmarshalText Integration
```go
func (s *ValidStruct) UnmarshalText(text []byte) error {
    // Parse text into fields
    if err := parseText(text, s); err != nil {
        return err
    }
    
    // Validate parsed content
    return s.Validate()
}
```

### Pattern 3: Parent-Child Validation
```go
func (p *ParentStruct) Validate() error {
    // Assume child structs are already valid (validated during construction)
    
    // Validate parent's primitive fields
    if p.parentField < 0 {
        return NewInvalidInputError("parentField must be non-negative", nil)
    }
    
    // Validate contextual requirements for children
    if p.requiresSpecialChild && p.child.GetType() != "special" {
        return NewInvalidInputError("special child required", nil)
    }
    
    return nil
}
```

## Error Handling

### Validation Error Types
```go
// Field validation errors
err := header.Validate()
if errors.Is(err, &frozendb.InvalidInputError{}) {
    // Field value is invalid (range, type, etc.)
}

// Database corruption errors  
err := frozendb.Open(path)
if errors.Is(err, &frozendb.CorruptDatabaseError{}) {
    // Database file is corrupted
}

// Operation errors
err := frozendb.Create(config)
if errors.Is(err, &frozendb.WriteError{}) {
    // Write operation failed validation
}
```

### Error Context
Validation errors include detailed context about what failed:

```go
err := header.Validate()
// Error: "invalid input: row_size must be between 128 and 65536 (got: 64)"
```

## Migration Guide

### Converting From Old Validation

```go
// ❌ OLD: Separate validation function
header := parseHeader(data)
if err := validateHeaderFields(header); err != nil {
    return nil, err
}

// ✅ NEW: Integrated validation
header, err := NewHeader(data)  // Calls Validate() internally
if err != nil {
    return nil, err
}
```

### Converting From Exported Fields

```go
// ❌ OLD: Direct field access
fmt.Printf("Version: %d\n", header.Version)
header.Version = 2  // DANGEROUS: bypasses validation

// ✅ NEW: Getter access (read-only)
fmt.Printf("Version: %d\n", header.GetVersion())
// No setter - immutable after construction
```

## Best Practices

### 1. Always Use Constructor Functions
```go
// ✅ GOOD: Automatic validation
header, err := frozendb.NewHeader(data)

// ⚠️ ACCEPTABLE: Direct init with explicit validation
header := &frozendb.Header{...}
if err := header.Validate(); err != nil { ... }

// ❌ BAD: Direct init without validation
header := &frozendb.Header{...}  // May be invalid!
useHeader(header)  // Undefined behavior
```

### 2. Handle All Validation Errors
```go
header, err := frozendb.NewHeader(data)
if err != nil {
    // Always handle validation errors
    log.Printf("Header validation failed: %v", err)
    return err
}
```

### 3. Trust Getter Functions
```go
// ✅ TRUSTED: Getters provide validated, immutable access
version := header.GetVersion()  // Always valid

// ❌ UNNECESSARY: Don't re-validate after construction
if err := header.Validate(); err != nil { ... }  // Already validated
```

### 4. Understand Parent-Child Validation
```go
// Child validates itself during construction
child, err := NewChild(childData)
if err != nil {
    return nil, err
}

// Parent assumes child is valid, validates context
parent := &Parent{child: child}
return parent, parent.Validate()  // Only checks parent-specific rules
```

## Performance Considerations

### Validation Costs
- **Header validation**: One-time cost during file open
- **Row validation**: Per-row cost during database operations  
- **Getter functions**: Zero allocation, direct field access

### Memory Usage
- Validation methods use no additional memory
- Getter functions are inline-friendly
- Field immutability reduces need for repeated validation

## Troubleshooting

### Common Validation Errors

#### Header Validation Failures
```go
// "invalid input: signature must be 'fDB' (got: 'XYZ')"
// → File is not a frozenDB file

// "invalid input: version must be 1 (got: 2)"  
// → Unsupported file format version

// "invalid input: row_size must be between 128 and 65536 (got: 64)"
// → Row size too small for data
```

#### CreateConfig Validation Failures
```go
// "write: path cannot be empty"
// → Missing database file path

// "invalid input: row_size must be between 128 and 65536 (got: 100000)"
// → Row size exceeds maximum
```

#### SudoContext Validation Failures
```go
// "write: sudo user cannot be empty"
// → Sudo privileges required but user not detected
```

### Debug Tips

1. **Check validation order**: Child validation happens first, then parent
2. **Verify field visibility**: Use getters, not direct field access
3. **Handle all errors**: Validation errors contain useful context
4. **Trust constructors**: They handle validation automatically

## Integration Examples

### Database Operations
```go
// Open with automatic header validation
db, err := frozendb.Open(path)
if err != nil {
    return fmt.Errorf("database open failed: %w", err)
}
defer db.Close()

// Create with configuration validation  
err = frozendb.Create(frozendb.CreateConfig{
    Path:    path,
    RowSize: 1024,
    SkewMs:  1000,
})
if err != nil {
    return fmt.Errorf("database creation failed: %w", err)
}
```

### Row Processing
```go
// Parse row with automatic control validation
row, err := frozendb.ParseRow(rowBytes, header)
if err != nil {
    return fmt.Errorf("row parsing failed: %w", err)
}

// Validate row contextual requirements
switch r := row.(type) {
case *frozendb.ChecksumRow:
    // ChecksumRow constructor already validated StartControl='C'
    fmt.Printf("Checksum: %08x\n", r.GetChecksum().GetValue())
case *frozendb.DataRow:
    // DataRow validates its own control sequences
    fmt.Printf("Key: %s\n", r.GetKey())
}
```

This pattern ensures that all frozenDB structs are valid throughout their lifecycle, preventing data corruption and simplifying reasoning about code behavior.