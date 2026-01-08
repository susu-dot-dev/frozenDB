# Package Contracts

## Public API Contract

### pkg/frozendb package

```go
// Package frozendb provides immutable append-only key-value database functionality.
// 
// The Create function initializes new database files with proper immutability protection
// and atomic operations. All database files maintain append-only semantics and cannot
// be modified or deleted after creation.
package frozendb

// CreateConfig holds configuration for creating a new frozenDB database file
type CreateConfig struct {
    Path     string // Filesystem path for database file
    RowSize  int    // Size of each data row in bytes (128-65536)
    SkewMs   int    // Time skew window in milliseconds (0-86400000)
}

// Validate validates the CreateConfig and returns appropriate error types
func (cfg *CreateConfig) Validate() error

// Create creates a new frozenDB database file with the given configuration.
//
// The function performs atomic file creation with proper error handling and cleanup.
// Files are created with append-only protection to ensure immutability.
//
// Parameters:
//   - config: Configuration containing path, rowSize, and skewMs
//             Use NewCreateConfig() for simple parameter-based construction
//
// Requirements:
//   - Must be run with sudo privileges to set append-only attribute
//   - Parent directory must exist and be writable
//   - Target file must not already exist
//
// Returns:
//   - error: nil on success, or one of:
//     * InvalidInputError: for invalid input parameters
//     * PathError: for filesystem path issues  
//     * WriteError: for file operations, sudo context, or attribute setting failures
//
// Example:
//   config := frozendb.NewCreateConfig("/var/lib/app/database.fdb", 1024, 5000)
//   err := frozendb.Create(config)
//   if err != nil {
//       log.Fatal(err)
//   }
func Create(config CreateConfig) error


```

## Error Type Contracts

### Base Error Interface

```go
// FrozenDBError is the base error type for all frozenDB operations.
// All error types must embed this struct to maintain constitutional requirements.
type FrozenDBError struct {
    Code    string // Error code for programmatic handling
    Message string // Human-readable error message
    Err     error  // Underlying error (optional)
}

// Error returns the formatted error message.
func (e *FrozenDBError) Error() string

// Unwrap returns the underlying error for error chaining.
func (e *FrozenDBError) Unwrap() error
```

### Specific Error Types

```go
// InvalidInputError is returned for input validation failures.
// Used for: empty path, invalid parameter ranges, wrong file extension.
type InvalidInputError struct {
    FrozenDBError
}

// PathError is returned for filesystem path issues.
// Used for: parent directory missing, path not writable, file already exists.
type PathError struct {
    FrozenDBError
}

// WriteError is returned for file operation failures.
// Used for: sudo context issues, header write failures, attribute setting failures.
type WriteError struct {
    FrozenDBError
}
```

## Internal Function Contracts

### Input Validation

```go
## Implementation Notes

The Create function implementation will contain all necessary internal helper functions as private (lowercase) functions within the single create.go file. Internal functions will be implemented according to the requirements outlined in the functional specification and data model.
    O_CREAT_EXCL = syscall.O_CREAT | syscall.O_EXCL
    
    // File permissions: 0644 (owner read/write, group/others read)
    FilePermissions = 0644
)
```

## Data Format Contracts

### frozenDB v1 Header Structure

```go
// Header represents frozenDB v1 text-based header format
// Header is exactly 64 bytes: JSON content + null padding + newline
type Header struct {
    Signature string    // Always "fDB" 
    Version   int      // Always 1 for v1 format
    RowSize   int      // Size of each data row in bytes (128-65536)
    SkewMs    int      // Time skew window in milliseconds (0-86400000)
}

// Header constants
const (
    HeaderSize     = 64                // Fixed header size in bytes
    HeaderSignature = "fDB"            // Signature string for format validation
    MinRowSize     = 128               // Minimum allowed row size
    MaxRowSize     = 65536             // Maximum allowed row size
    MaxSkewMs      = 86400000          // Maximum time skew (24 hours)
    PaddingChar     = '\x00'            // Null character for header padding
    HeaderNewline   = '\n'             // Byte 63 must be newline
)

// Header format string for generating JSON content
const HeaderFormat = `{sig:"fDB",ver:1,row_size:%d,skew_ms:%d}`
```

## Performance Contracts

### Memory Usage

```go
// The implementation must use fixed memory regardless of input parameters:
// - Header writing: fixed 64-byte buffer
// - Path validation: O(1) operations
// - File operations: single pass, no buffering of entire file
```

### Disk Operations

```go
// The implementation must minimize disk operations:
// 1. Single file creation (O_CREAT|O_EXCL)
// 2. Single header write (64 bytes)
// 3. Single data flush (fdatasync)
// 4. Single attribute setting (ioctl)
// 5. Single ownership change (chown, if applicable)
// 6. Single file close
```

### Error Speed

```go
// All validation failures must be detected before any filesystem operations:
// - Input validation: O(1) string checks and range validation
// - Path validation: filesystem checks only after input validation passes
// - Sudo validation: environment variable checks only if sudo required
```

## Thread Safety Contracts

### Function-Level Thread Safety

```go
// Create function must be thread-safe for concurrent calls with different paths.
// No shared mutable state between function calls.
// All operations use file descriptor-level locking, not global state.
```

### Process-Level Atomicity

```go
// File creation must be atomic across processes:
// - O_CREAT|O_EXCL prevents race conditions with other processes
// - Single header write prevents partial file states
// - Cleanup on failure prevents orphaned files
```