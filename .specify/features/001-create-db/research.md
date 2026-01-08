# Research: Database File Creation Implementation

## Phase 0: Research & Dependencies

### Error Handling Architecture

Based on the constitutional requirements, the error handling system must derive all errors from a base FrozenDBError struct:

```go
// Base error struct (constitutional requirement)
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

// Specific error types for different caller behaviors
type InvalidInputError struct {
    FrozenDBError
}

type PathError struct {
    FrozenDBError
}

type WriteError struct {
    FrozenDBError
}

// Constructor functions
func NewInvalidInputError(message string, err error) *InvalidInputError
func NewPathError(message string, err error) *PathError
func NewWriteError(message string, err error) *WriteError
```

### Linux System Calls Required

**Append-only Attribute Setting:**
```go
// FS_IOC_SETFLAGS ioctl constant
const FS_IOC_SETFLAGS = 0x40086602

// FS_APPEND_FL flag for append-only attribute
const FS_APPEND_FL = 0x00000020

// syscall wrapper for setting file attributes
func SetAppendOnlyAttr(fd int) error {
    var flags uint32
    // Get current flags first
    _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), FS_IOC_GETFLAGS, uintptr(unsafe.Pointer(&flags)))
    if errno != 0 {
        return errno
    }
    
    // Set append-only flag
    flags |= FS_APPEND_FL
    _, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), FS_IOC_SETFLAGS, uintptr(unsafe.Pointer(&flags)))
    return errno
}
```

**Data Flushing:**
```go
import "golang.org/x/sys/unix"

// fdatasync wrapper for data-only flush (no metadata)
func fdatasync(fd int) error {
    return unix.Fdatasync(fd)
}
```

**File Creation with Atomic Flags:**
```go
// O_CREAT|O_EXCL for atomic creation preventing race conditions
const O_CREAT_EXCL = syscall.O_CREAT | syscall.O_EXCL

// File permissions: 0644 (owner rw, group/others r)
const FILE_PERMISSIONS = 0644
```

### Sudo Context Detection

**Environment Variables Required:**
- `SUDO_USER`: Username of original user
- `SUDO_UID`: UID of original user 
- `SUDO_GID`: GID of original user

**Detection Logic:**
```go
type SudoContext struct {
    User     string
    UID      int
    GID      int
    IsValid  bool
}

func DetectSudoContext() *SudoContext {
    sudoUser := os.Getenv("SUDO_USER")
    if sudoUser == "" {
        return &SudoContext{IsValid: false}
    }
    
    sudoUID := os.Getenv("SUDO_UID")
    sudoGID := os.Getenv("SUDO_GID")
    
    uid, err := strconv.Atoi(sudoUID)
    if err != nil {
        return &SudoContext{IsValid: false}
    }
    
    gid, err := strconv.Atoi(sudoGID)
    if err != nil {
        return &SudoContext{IsValid: false}
    }
    
    return &SudoContext{
        User:    sudoUser,
        UID:     uid,
        GID:     gid,
        IsValid: true,
    }
}
```

### frozenDB v1 File Format

**Header Structure (from docs/v1_file_format.md):**
```go
// Header represents frozenDB v1 text-based header format
// Header is exactly 64 bytes: JSON content + period padding + newline
type Header struct {
    Signature string    // Always "fDB"
    Version   int      // Always 1 for v1 format
    RowSize   int      // Size of each data row in bytes (128-65536)
    SkewMs    int      // Time skew window in milliseconds (0-86400000)
}

const HeaderSize = 64 // Fixed header size in bytes
```

**Header Format String:**
```
{sig:"fDB",ver:1,row_size:<size>,skew_ms:<skew>}....\n
```
Where:
- JSON content must be between 44-51 bytes
- Padding consists of period characters (.) 
- Byte 63 must be newline (\n)
- Total header length is exactly 64 bytes

**Header Writing with Integrity:**
```go
func WriteHeader(fd int, rowSize, skewMs int) error {
    // Generate JSON content: {sig:"fDB",ver:1,row_size:1024,skew_ms:5000}
    jsonContent := fmt.Sprintf(`{sig:"fDB",ver:1,row_size:%d,skew_ms:%d}`, rowSize, skewMs)
    
    // Calculate padding needed (63 - jsonContent length, byte 63 is newline)
    contentLength := len(jsonContent)
    if contentLength > 51 {
        return fmt.Errorf("header content too long: %d bytes", contentLength)
    }
    
    paddingLength := 63 - contentLength
    padding := strings.Repeat(".", paddingLength)
    
    // Assemble header: JSON + padding + newline
    header := jsonContent + padding + "\n"
    
    // Atomic 64-byte header write
    _, err := syscall.Write(fd, []byte(header))
    return err
}
```

### Input Validation Requirements

**Path Validation:**
- Non-empty string
- Must end with `.fdb`
- Must be valid for Linux filesystem
- Parent directory must exist and be writable
- Handle absolute/relative paths correctly
- No shell expansion (~, etc.)
- Allow hidden files (starting with .)

**Parameter Validation:**
- rowSize: 128-65536 inclusive
- skewMs: 0-86400000 inclusive (24 hours in milliseconds)

### Atomic Operation Sequence

1. **Input Validation** (no side effects)
2. **Sudo Context Validation** (no side effects) 
3. **Path Validation** (no side effects)
4. **File Creation** (O_CREAT|O_EXCL, atomic)
5. **Header Writing** (single write)
6. **Data Flush** (fdatasync)
7. **Append-Only Attribute** (ioctl)
8. **Ownership Setting** (chown, if sudo)
9. **File Close**

**Failure Cleanup:**
- Any failure after file creation must remove the file
- No partial files should remain
- No side effects on parent directory

### Performance Constraints

- **Fixed Memory**: Must use constant memory regardless of parameters
- **Minimal Disk Operations**: Single pass through the sequence above
- **Early Validation**: All validations before any filesystem operations
- **Constant Time Header Writing**: Fixed 72-byte header write