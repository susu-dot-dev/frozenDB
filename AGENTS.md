# frozenDB - Agent Guidelines

This document contains build commands and coding standards for agentic development of frozenDB, an immutable key-value store built in Go.

## Build & Development Commands

This is a Go 1.25.5 project using only standard library dependencies:

```bash
# Run all tests (unit + spec)
go test ./...

# Run tests with verbose output
go test -v ./...

# Run spec tests only (Test_S_ prefix)
go test -v ./... -run "^Test_S_"

# Run unit tests only (exclude Test_S_ prefix)
go test -v ./... -run "^Test[^_]"

# Run a specific test
go test -run TestFunctionName ./path/to/package

# Run tests with coverage
go test -cover ./...

# Run spec tests with coverage
go test -cover ./... -run "^Test_S_"

# Run benchmarks
go test -bench=. ./...

# Format code (gofmt)
gofmt -w .

# Vet code for potential issues
go vet ./...
```

## Project Structure

```
frozenDB/
├── frozendb/      # Core database package (public API)
├── cmd/           # CLI applications
├── docs/          # Documentation including file format specs
├── specs/         # Feature specifications and requirements
└── test/          # Integration tests
```

## Essential Context Files

**CRITICAL:** When implementing any database file or in-memory structure features, ALWAYS load:
- `docs/v1_file_format.md` - Complete file format specification

**Additional context for implementation:**
- `docs/spec_testing.md` - Spec testing guidelines and requirements
- Relevant spec files in `specs/` directory for feature requirements
- `AGENTS.md` - This file for coding standards and build commands

## Code Style Guidelines

### General Principles
- Follow standard Go formatting and conventions
- Use `gofmt` for consistent formatting
- Keep functions small and focused
- Prefer composition over inheritance
- Design for concurrency where applicable

### Naming Conventions
- **Package names**: short, lowercase, single words when possible
- **Functions**: camelCase, exported functions start with capital letter
- **Variables**: camelCase, prefer descriptive names over abbreviations
- **Constants**: UPPER_SNAKE_CASE for exported constants, camelCase for unexported
- **Interfaces**: typically end in "er" suffix (Reader, Writer) or describe behavior
- **Error types**: should end with "Error" suffix

### Error Handling
- Always handle errors explicitly
- All errors should be structured, deriving from the base FrozenDBError struct
- Different error types should only be made when callers are expected to have different behavior for treating each issue
- Otherwise, add descriptive errors to the Message property of the error

Example:
```go
// Base error struct
type FrozenDBError struct {
    Code    string
    Message string
    Err     error // underlying error
}

func (e *FrozenDBError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *FrozenDBError) Unwrap() error {
    return e.Err
}

// Specific error type embedding base
type InvalidInputError struct {
    FrozenDBError
}

// Constructor for specific error
func NewInvalidInputError(message string, err error) *InvalidInputError {
    return &InvalidInputError{
        FrozenDBError: FrozenDBError{
            Code:    "invalid_input",
            Message: message,
            Err:     err,
        },
    }
}

// Usage in functions
func (db *DB) Add(key uuid.UUID, value interface{}) error {
    if err := db.validateKey(key); err != nil {
        return NewInvalidInputError("key validation failed", err)
    }
    // ... implementation
}
```

### Function Documentation
- Exported functions must have Go doc comments
- Follow standard Go documentation format
- Include parameter descriptions, return values, and error conditions

Example:
```go
// Add stores a key-value pair in the database.
// The key must be a UUIDv7 for proper time ordering.
// Returns an error if the transaction cannot be completed.
func (db *DB) Add(key uuid.UUID, value interface{}) error {
    // implementation
}
```

### Types and Interfaces
- Use concrete types when implementation is fixed
- Define interfaces for behavior that needs to be mocked or swapped
- Keep interfaces small and focused (interface segregation)
- Use struct embedding carefully

### Concurrency
- Design for concurrent reads where possible
- Use proper synchronization (mutexes, channels) for shared state
- Consider using context.Context for cancellation and timeouts

### Testing
- Write table-driven tests for multiple scenarios
- Use subtests for related test cases
- Mock external dependencies using interfaces
- Test both success and error paths
- Include benchmarks for performance-critical code

Example test structure:
```go
func TestDB_Add(t *testing.T) {
    tests := []struct {
        name    string
        key     uuid.UUID
        value   interface{}
        wantErr bool
    }{
        {
            name:    "valid key-value pair",
            key:     uuid.MustParse("0189b3c0-3c1b-7b8b-8b8b-8b8b8b8b8b8b"),
            value:   "test value",
            wantErr: false,
        },
        // more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            db := setupTestDB(t)
            err := db.Add(tt.key, tt.value)
            if (err != nil) != tt.wantErr {
                t.Errorf("DB.Add() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Performance Considerations
- Profile memory usage - should be fixed, not scale with database size
- Optimize disk reads (correspond to sector size when possible)
- Use binary search for key lookups within time skew windows
- Implement caching strategies to reduce disk seeks
- Minimize allocations in hot paths

### Security & Reliability
- Use OS-level file locks (flocks) for write mode exclusivity
- Implement sentinel bytes for transaction integrity
- Validate UUIDv7 keys to ensure time ordering
- Handle partial write failures gracefully
- Design for concurrent reads during write operations

## frozenDB Specific Guidelines

### UUIDv7 Key Handling
- All keys must be UUIDv7 for proper time ordering
- Validate UUID version on insertion
- Leverage time component for binary search optimization
- Handle configurable time skew for distributed systems

### Transaction Management
- Use transaction headers for atomicity guarantees
- Implement proper begin/commit/abort semantics
- Detect incomplete transactions via sentinel validation
- Allow reads to ignore in-progress transactions

### File Storage
- Maintain append-only immutability
- Use fixed-width rows for direct seeking
- Implement corruption detection via row sentinels
- Support both read and read+write modes with proper locking

### API Design Principles
- Keep the public API simple and focused
- Provide clear error messages for debugging
- Support JSON-serializable values
- Enable efficient enumeration and counting operations

## File Format Implementation Requirements

**CRITICAL:** When implementing any database file or in-memory structure features, you MUST understand:

### Append-Only Architecture
- Data is never modified in place—only appended
- Enables safe concurrent reads during writes
- Simplifies crash recovery (no partial overwrites)
- Provides natural audit trail of all operations

### Fixed-Width Rows
- Enables O(1) seeking to any row by index
- Allows binary search on sorted keys
- Eliminates need for index files or offset tables
- Simplifies memory-mapped access patterns

### Transaction Semantics
- All writes occur within transactions
- Use start_control `T` for transaction begin, `R` for continuation
- End_control encodes savepoints and termination (TC, RE, SC, SE, R0-R9, S0-S9)
- "Rollback" marks rows as invalid, doesn't delete them
- Savepoints numbered 1-9, with 0 representing full rollback

### Row Structure
- ROW_START (0x1F) and ROW_END (0x0A) sentinels
- Base64-encoded UUIDv7 keys (24 bytes)
- JSON payload with NULL_BYTE padding
- Parity bytes for integrity (LRC checksum)
- Checksum rows every 10,000 data rows (CRC32)

### Key Implementation Points
- Validate UUIDv7 timestamp ordering with configurable skew_ms
- Handle incomplete transaction detection via sentinels
- Implement two-tier integrity: checksum blocks + per-row parity
- Support both read and read+write modes with proper OS file locking

## Active Technologies
- Go 1.25.5 + Standard library only (os, syscall, encoding/json, sync)
- Single file-based database with append-only immutability
- UUIDv7 for time-ordered keys
- CRC32 checksums and LRC parity for data integrity
- Go 1.25.5 + Standard library only (encoding/base64, encoding/json, hash/crc32) (003-checksum-row)
- Single file-based frozenDB database (.fdb extension) (003-checksum-row)
- Go 1.25.5 + Go standard library only (os, encoding/json, sync, etc.) (004-struct-validation)
- Single-file append-only database (.fdb extension) (004-struct-validation)
- Go 1.25.5 + github.com/google/uuid, Go standard library only (005-data-row-handling)
- Single-file frozenDB database (.fdb extension) (005-data-row-handling)
- Go 1.25.5 + Go standard library only, github.com/google/uuid (006-transaction-struct)
- Go 1.25.5 + Go standard library only, github.com/google/uuid for UUIDv7 handling (007-file-validation)
- Single-file frozenDB database (.fdb extension) with append-only architecture (007-file-validation)

## Recent Changes
- Updated with v1_file_format.md concepts for append-only architecture
- Added transaction semantics and row structure requirements
