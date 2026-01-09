# frozenDB - Agent Guidelines

This document contains build commands and coding standards for agentic development of frozenDB, an immutable key-value store built in Go.

## Build & Development Commands

This is a Go project. Standard Go toolchain commands:

```bash
# Run all tests (unit + spec)
go test ./...

# Run tests with verbose output
go test -v ./...

# Run spec tests only
go test -v ./... -run "^Test_S_"

# Run unit tests only
go test -v ./... -run "^Test[^S]"

# Run a specific test
go test -run TestFunctionName ./path/to/package

# Run tests with coverage
go test -cover ./...

# Run spec tests with coverage
go test -cover ./... -run "^Test_S_"

# Run benchmarks
go test -bench=. ./...
```

## Project Structure

```
frozenDB/
├── cmd/           # CLI applications
├── internal/      # Internal packages (not importable by others)
│   ├── db/        # Core database implementation
│   ├── storage/   # File storage layer
├── pkg/           # Public API packages
├── test/          # Integration tests
└── examples/      # Usage examples
```

## Context Files

During implementation, the following files should be used for context:
- `specs/001-create-db/tasks.md` - Complete task breakdown and execution plan
- `specs/001-create-db/plan.md` - Technical architecture and design decisions
- `specs/001-create-db/data-model.md` - Entity definitions and data structures
- `specs/001-create-db/contracts/api-contract.md` - API specifications and requirements
- `specs/001-create-db/research.md` - Technical research and dependency analysis
- `specs/001-create-db/quickstart.md` - Usage examples and integration scenarios
- `docs/spec_testing.md` - Spec testing guidelines and requirements
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

## Active Technologies
- Go 1.25.5 + Standard library only (os, syscall, encoding/json, sync) (002-open-frozendb)
- Single file-based database with append-only immutability (002-open-frozendb)

## Recent Changes
- 002-open-frozendb: Added Go 1.25.5 + Standard library only (os, syscall, encoding/json, sync)
