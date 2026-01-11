# frozenDB Validation Contract Summary

## Overview

This document defines the API contracts for frozenDB's struct validation and immutability patterns. The contracts ensure consistent validation behavior across all database components.

## Core Validation Interface

All structs that can have invalid states implement the `ValidatableStruct` interface:

- **Validate() error method**: Validates struct fields and contextual requirements
- **Construction paths**: Support for direct initialization, constructor functions, and text unmarshaling
- **Idempotent validation**: Multiple calls return the same result

## Database Core Structs

### Header
- **Purpose**: Database file metadata
- **Validation**: Signature ("fDB"), version (1), rowSize (128-65536), skewMs (0-86400000)
- **Field access**: Unexported fields with getter functions
- **Constructor**: `NewHeader([]byte) (*Header, error)`

### FrozenDB
- **Purpose**: Database connection management
- **Validation**: File handle non-nil, header valid and non-nil, mode valid
- **Field access**: All fields unexported (internal use only)
- **Constructor**: `Open(string) (*FrozenDB, error)`

### CreateConfig
- **Purpose**: Database creation configuration
- **Validation**: Path non-empty, rowSize (128-65536), skewMs (0-86400000)
- **Field access**: Unexported fields with getter functions
- **Constructor**: `Create(CreateConfig) error`

### SudoContext
- **Purpose**: Privileged execution context
- **Validation**: User non-empty, UID > 0, GID > 0
- **Field access**: Unexported fields with getter functions
- **Constructor**: `DetectSudoContext() (*SudoContext, error)`

## Row Processing Structs

### StartControl
- **Purpose**: Row start transaction marker
- **Validation**: Valid control character (T, R, C)
- **Context**: In ChecksumRow, must be 'C'
- **Unmarshaling**: `UnmarshalText([]byte) error`

### EndControl
- **Purpose**: Row end transaction sequence
- **Validation**: Valid sequence (TC, RE, SC, SE, R0-R9, S0-S9)
- **Context**: Transaction state machine compliance
- **Unmarshaling**: `UnmarshalText([]byte) error`

### Checksum
- **Purpose**: CRC32 integrity value
- **Validation**: uint32 range (always valid)
- **Usage**: Data integrity verification
- **Unmarshaling**: `UnmarshalText([]byte) error`

### ChecksumRow
- **Purpose**: Block integrity verification
- **Validation**: BaseRow valid, checksum matches calculated CRC32, header valid
- **Context**: Requires StartControl='C', EndControl='CS'
- **Constructor**: `NewChecksumRow(*Header, []byte) (*ChecksumRow, error)`

## Error Handling Contracts

### Validation Error Hierarchy
```
FrozenDBError (base)
├── InvalidInputError (field validation)
├── CorruptDatabaseError (file corruption)
└── WriteError (operation failures)
```

### Error Context
- **Field name**: Which field failed validation
- **Invalid value**: The value that caused failure
- **Context**: Additional error details
- **Wrap pattern**: Errors wrap underlying causes

## Construction Patterns

### 1. Direct Initialization
```go
s := &Struct{field: value}
err := s.Validate()  // User responsibility
```

### 2. Constructor Function
```go
s, err := NewStruct(input)  // Automatic validation
```

### 3. Text Unmarshaling
```go
s := &Struct{}
err := s.UnmarshalText(data)  // Automatic validation
```

## Field Immutability Rules

### Exported Fields → Unexported + Getters
- **Header**: All fields unexported with GetX() methods
- **CreateConfig**: All fields unexported with GetX() methods
- **SudoContext**: All fields unexported with GetX() methods
- **FrozenDB**: Already unexported (no getters needed)

### Getter Function Pattern
```go
func (s *Struct) GetField() FieldType {
    return s.field  // Direct access, no allocation
}
```

## Parent-Child Validation

### Child Validation Responsibilities
- Validate own fields during construction
- Called in child's constructor or UnmarshalText
- Parent assumes child validity

### Parent Validation Responsibilities
- Validate primitive fields
- Check child field presence (non-nil)
- Validate contextual child requirements

## Performance Guarantees

### Validation Costs
- **Constructor validation**: One-time cost
- **Getter functions**: Zero allocation
- **Repeated validation**: Idempotent, minimal overhead

### Memory Usage
- **Validation methods**: No additional allocations
- **Field immutability**: No defensive copying needed
- **Error context**: Structured, minimal overhead

## Compliance Requirements

### Constitutional Principles
- ✅ **Immutability First**: Fields immutable after construction
- ✅ **Data Integrity**: Validation prevents invalid state
- ✅ **Correctness Over Performance**: Validation prioritized
- ✅ **Spec Test Compliance**: All FR requirements covered

### File Format Compliance
- Header validation enforces v1_file_format.md requirements
- Row validation enforces transaction state machine
- Checksum validation maintains data integrity

This contract ensures that all frozenDB structs maintain data integrity through consistent validation patterns while providing efficient, immutable access to validated state.