# Data Model: Struct Validation and Immutability

## Core Entities

### ValidatableStruct

**Purpose**: Base abstraction for any struct that requires field validation

**Properties**:
- Fields that can have invalid states
- Validate() method implementation
- Construction path validation integration

**Relationships**:
- *Contains* → ChildStruct (composition)
- *Validated by* → Validate() method
- *Created via* → ConstructorPath

**Validation Rules**:
- Must implement Validate() error method
- Validate() must be called in all constructor paths
- Fields must be validated for type and range constraints

### ConstructorPath

**Purpose**: Enumeration of valid struct construction methods

**Types**:
1. **Direct Initialization**: `struct{field: value}` pattern
2. **Constructor Function**: `NewStruct()` pattern  
3. **Unmarshaling**: `UnmarshalText()` pattern

**Validation Integration**:
- Direct: User must call Validate() explicitly
- Constructor: Automatically calls Validate() before return
- UnmarshalText: Automatically calls Validate() before return

### FieldVisibility

**Purpose**: Controls field access and immutability

**Types**:
- **Exported**: Capitalized field name, external access allowed
- **Unexported**: Lowercase field name, package-private access only
- **Getter Access**: Public function provides read-only access

**Immutability Rules**:
- Validation-critical fields must be unexported
- External access requires getter functions
- No setters for immutable validated state

## Struct-Specific Entities

### Header (ValidatableStruct)

**Purpose**: Database file header with format metadata

**Fields**:
```
signature: string    // "fDB" - file signature
version: int        // 1 - format version  
rowSize: int        // 128-65536 - bytes per row
skewMs: int         // 0-86400000 - time skew window
```

**Validation Rules**:
- signature must equal "fDB"
- version must equal 1
- rowSize must be in range [128, 65536]
- skewMs must be in range [0, 86400000]

**Parent-Child Context**:
- No child structs
- Standalone validation requirements

**Required Getters**:
- GetSignature() string
- GetVersion() int
- GetRowSize() int
- GetSkewMs() int

### FrozenDB (ValidatableStruct)

**Purpose**: Main database connection and state management

**Fields**:
```
file: *os.File      // database file handle
mode: string        // access mode ("r" or "rw")
header: *Header     // parsed file header
mu: sync.Mutex      // concurrency control
closed: bool        // connection state
```

**Validation Rules**:
- file must not be nil
- header must not be nil
- header must be valid (calls header.Validate())
- mode must be "r" or "rw"

**Parent-Child Context**:
- Contains Header child struct
- Assumes Header validity, validates header field presence

**Field Visibility**: All fields already unexported (no getters needed)

### CreateConfig (ValidatableStruct)

**Purpose**: Database creation configuration

**Fields**:
```
path: string       // database file path
rowSize: int       // row size in bytes
skewMs: int        // time skew tolerance
```

**Validation Rules**:
- path must be non-empty string
- rowSize must be in range [128, 65536]
- skewMs must be in range [0, 86400000]

**Parent-Child Context**:
- No child structs
- Standalone validation requirements

**Required Getters**:
- GetPath() string
- GetRowSize() int
- GetSkewMs() int

### SudoContext (ValidatableStruct)

**Purpose**: Privileged execution environment information

**Fields**:
```
user: string       // sudo username
uid: int          // user ID
gid: int          // group ID
```

**Validation Rules**:
- user must be non-empty string
- uid must be > 0
- gid must be > 0

**Parent-Child Context**:
- No child structs
- Security validation requirements

**Required Getters**:
- GetUser() string
- GetUID() int
- GetGID() int

### StartControl (ValidatableStruct)

**Purpose**: Row start transaction control byte

**Fields**:
```
value: byte        // control character ('T', 'R', 'C')
```

**Validation Rules**:
- Must be valid start control character
- Valid values: START_TRANSACTION('T'), ROW_CONTINUE('R'), CHECKSUM_ROW('C')

**Parent-Child Context**:
- Child struct in ChecksumRow (context-specific validation)
- In ChecksumRow: must equal 'C' for checksum rows
- Standalone: any valid start control allowed

**Field Visibility**: Unexported type, no getters needed

### EndControl (ValidatableStruct)

**Purpose**: Row end transaction control sequence

**Fields**:
```
sequence: [2]byte  // control character pair
```

**Validation Rules**:
- Must be valid end control sequence
- First char: 'T', 'R', 'S' for savepoint behavior
- Second char: 'C', 'E', '0-9' for transaction termination

**Valid Sequences**:
- TC, RE, SC, SE, R0-R9, S0-S9

**Parent-Child Context**:
- Child struct in data rows
- Contextual validation based on transaction state

**Field Visibility**: Unexported type, no getters needed

### Checksum (ValidatableStruct)

**Purpose**: CRC32 checksum value

**Fields**:
```
value: uint32      // checksum value
```

**Validation Rules**:
- uint32 is always valid within range
- Could add range constraints if needed

**Parent-Child Context**:
- Child struct in ChecksumRow
- No contextual validation requirements

**Field Visibility**: Unexported type, no getters needed

### baseRow[T] (ValidatableStruct)

**Purpose**: Generic foundation for all row types

**Fields**:
```
startControl: StartControl  // row start marker
endControl: EndControl      // row end marker
parity: [2]byte             // LRC checksum
```

**Validation Rules**:
- startControl must be valid
- endControl must be valid
- parity bytes must be valid hex characters

**Parent-Child Context**:
- Base class for all row implementations
- Child structs validate row-specific content

**Field Visibility**: Generic, no getters needed

### ChecksumRow (ValidatableStruct)

**Purpose**: Integrity verification row with CRC32 checksum

**Fields**:
```
baseRow: baseRow[ChecksumRow]  // embedded row foundation
checksum: Checksum              // CRC32 value
header: *Header                 // database header reference
dataBytes: []byte               // covered data bytes
```

**Validation Rules**:
- baseRow must be valid (calls baseRow.Validate())
- checksum must be valid
- header must not be nil and valid
- checksum value must match calculated CRC32 of dataBytes
- startControl must be 'C' (context-specific validation)
- endControl must be 'CS' (context-specific validation)

**Parent-Child Context**:
- Contains baseRow and Header child structs
- Assumes child validity, validates checksum calculation
- Contextual validation: forces startControl='C', endControl='CS'

**Field Visibility**: All fields unexported, no getters needed

## Validation Flow Relationships

### Constructor Flow
```
DirectInit → UserCallsValidate() → StructValid
NewStruct() → AutoCallValidate() → StructValid  
UnmarshalText() → AutoCallValidate() → StructValid
```

### Parent-Child Validation Flow
```
ParentStruct {
    ChildStruct child = NewChild()  // child.Validate() called internally
    Validate() {                    // parent validation assumes child valid
        // validate parent-specific context
        // validate child contextual requirements
    }
}
```

### Field Immutability Flow
```
StructConstruction {
    fields = unexported
    Validate() called
}
ExternalAccess {
    getter functions provide read-only access
    no direct field modification possible
}
```

## State Transition Diagrams

### Validation State Machine
```
Uninitialized → Validated → UsageReady
     ↓               ↓
   Error           ← ValidationFailed
```

### Construction State Machine  
```
Construction → Validation → ReadyToUse
      ↓            ↓
   ConstructionError ← ValidationError
```

### Parent-Child Validation Order
```
1. ChildStruct.Construct()
   ├── child.Validate() called internally
   └── child ready

2. ParentStruct.Construct()  
   ├── child fields already valid
   ├── parent.Validate() called
   ├── parent validates own fields
   └── parent validates child contextual requirements
```

## Data Integrity Constraints

### Referential Integrity
- FrozenDB.header must reference valid Header
- ChecksumRow.header must reference valid Header
- All parent struct references must be non-nil

### Type Safety
- All Validate() methods return error type
- Constructor functions return (*Struct, error) pairs
- No nil struct returns without error

### Immutability Guarantees
- Validated fields cannot be modified post-construction
- Getter functions provide immutable read access
- No setter functions for validated state

## Error Handling Relationships

### Validation Error Hierarchy
```
FrozenDBError (base)
├── InvalidInputError (invalid field values)
├── CorruptDatabaseError (invalid database state)
└── WriteError (operation validation failures)
```

### Error Context
- Validation errors include field name and invalid value
- Parent validation errors include child context information
- Constructor errors wrap validation errors with construction context