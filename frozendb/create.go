package frozendb

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// fsOperations provides abstraction for OS and filesystem operations to enable mocking
type fsOperations struct {
	Getuid func() int
	Lookup func(username string) (*user.User, error)
	Open   func(name string, flag int, perm os.FileMode) (*os.File, error)
	Stat   func(name string) (os.FileInfo, error)
	Mkdir  func(path string, perm os.FileMode) error
	Chown  func(name string, uid, gid int) error
	Ioctl  func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (r1 uintptr, r2 uintptr, err syscall.Errno)
}

// Default implementations using real OS functions
var defaultFSOps = fsOperations{
	Getuid: os.Getuid,
	Lookup: user.Lookup,
	Open:   os.OpenFile,
	Stat:   os.Stat,
	Mkdir:  os.Mkdir,
	Chown:  os.Chown,
	Ioctl:  syscall.Syscall,
}

// Global variable to allow tests to inject mock filesystem operations
var fsInterface = &defaultFSOps

// SetFSInterface allows tests to inject custom filesystem operation implementations
func SetFSInterface(ops fsOperations) {
	fsInterface = &ops
}

// restoreRealFS restores the original filesystem interface
func restoreRealFS() {
	fsInterface = &defaultFSOps
}

// setupMockFS creates a mock filesystem interface with real defaults and custom overrides
// This makes it easy to mock only specific functions while keeping others as real implementations
func setupMockFS(overrides fsOperations) {
	mockOps := fsOperations{
		Getuid: defaultFSOps.Getuid,
		Lookup: defaultFSOps.Lookup,
		Open:   defaultFSOps.Open,
		Stat:   defaultFSOps.Stat,
		Mkdir:  defaultFSOps.Mkdir,
		Chown:  defaultFSOps.Chown,
		Ioctl:  defaultFSOps.Ioctl,
	}

	// Apply overrides
	if overrides.Getuid != nil {
		mockOps.Getuid = overrides.Getuid
	}
	if overrides.Lookup != nil {
		mockOps.Lookup = overrides.Lookup
	}
	if overrides.Open != nil {
		mockOps.Open = overrides.Open
	}
	if overrides.Stat != nil {
		mockOps.Stat = overrides.Stat
	}
	if overrides.Mkdir != nil {
		mockOps.Mkdir = overrides.Mkdir
	}
	if overrides.Chown != nil {
		mockOps.Chown = overrides.Chown
	}
	if overrides.Ioctl != nil {
		mockOps.Ioctl = overrides.Ioctl
	}

	SetFSInterface(mockOps)
}

// CreateConfig holds configuration for creating a new frozenDB database file
type CreateConfig struct {
	Path    string // Filesystem path for the database file
	RowSize int    // Size of each data row in bytes (128-65536)
	SkewMs  int    // Time skew window in milliseconds (0-86400000)
}

// Header represents frozenDB v1 text-based header format
// Header is exactly 64 bytes: JSON content + null padding + newline
type Header struct {
	Signature string // Always "fDB"
	Version   int    // Always 1 for v1 format
	RowSize   int    // Size of each data row in bytes (128-65536)
	SkewMs    int    // Time skew window in milliseconds (0-86400000)
}

// SudoContext contains information about the sudo environment
type SudoContext struct {
	User string // Original username from SUDO_USER
	UID  int    // Original user ID from SUDO_UID
	GID  int    // Original group ID from SUDO_GID
}

// Constants for frozenDB v1 format
const (
	HeaderSize      = 64       // Fixed header size in bytes
	HeaderSignature = "fDB"    // Signature string for format identification
	MinRowSize      = 128      // Minimum allowed row size
	MaxRowSize      = 65536    // Maximum allowed row size
	MaxSkewMs       = 86400000 // Maximum time skew (24 hours)
	PaddingChar     = '\x00'   // Null character for header padding
	HeaderNewline   = '\n'     // Byte 63 must be newline
)

// Header format string for generating JSON content
const HeaderFormat = `{sig:"fDB",ver:1,row_size:%d,skew_ms:%d}`

// File system constants
const (
	// File permissions: 0644 (owner rw, group/others r)
	FilePermissions = 0644

	// Atomic file creation flags
	O_CREAT_EXCL = syscall.O_CREAT | syscall.O_EXCL

	// File extension requirement
	FileExtension = ".fdb"
)

// Linux syscall constants for append-only attribute
const (
	FS_IOC_GETFLAGS = 0x80086601 // Get file flags
	FS_IOC_SETFLAGS = 0x40086602 // Set file flags
	FS_APPEND_FL    = 0x00000020 // Append-only flag
)

// generateHeader creates the 64-byte header string with proper padding
func generateHeader(rowSize, skewMs int) ([]byte, error) {
	// Generate JSON content
	jsonContent := fmt.Sprintf(HeaderFormat, rowSize, skewMs)

	// Calculate padding needed (total 64 bytes, minus newline at end)
	contentLength := len(jsonContent)
	if contentLength > 51 {
		return nil, NewInvalidInputError("header content too long", nil)
	}

	// Calculate padding: 63 - jsonContent length (byte 63 is newline)
	paddingLength := 63 - contentLength
	padding := strings.Repeat(string(PaddingChar), paddingLength)

	// Assemble header: JSON + padding + newline
	header := jsonContent + padding + string(HeaderNewline)

	return []byte(header), nil
}

// detectSudoContext detects and validates sudo environment
func detectSudoContext() (*SudoContext, error) {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil, nil // Not running under sudo
	}

	sudoUID := os.Getenv("SUDO_UID")
	sudoGID := os.Getenv("SUDO_GID")

	if sudoUID == "" || sudoGID == "" {
		return nil, NewWriteError("missing SUDO_UID or SUDO_GID environment variables", nil)
	}

	uid, err := strconv.Atoi(sudoUID)
	if err != nil {
		return nil, NewWriteError("invalid SUDO_UID format", err)
	}

	gid, err := strconv.Atoi(sudoGID)
	if err != nil {
		return nil, NewWriteError("invalid SUDO_GID format", err)
	}

	// Verify the user exists
	userInfo, err := fsInterface.Lookup(sudoUser)
	if err != nil {
		return nil, NewWriteError("original user not found", err)
	}

	// Verify UID matches the user
	userUID, err := strconv.Atoi(userInfo.Uid)
	if err != nil {
		return nil, NewWriteError("invalid user UID format", err)
	}

	if userUID != uid {
		return nil, NewWriteError("SUDO_UID does not match SUDO_USER", nil)
	}

	return &SudoContext{
		User: sudoUser,
		UID:  uid,
		GID:  gid,
	}, nil
}

// Validate validates the CreateConfig and returns appropriate error types
func (cfg *CreateConfig) Validate() error {
	if err := validateInputs(*cfg); err != nil {
		return err
	}
	if err := validatePath(cfg.Path); err != nil {
		return err
	}
	return nil
}

// ValidateInputs performs only input validation (no filesystem checks)
func (cfg *CreateConfig) ValidateInputs() error {
	return validateInputs(*cfg)
}

// Create creates a new frozenDB database file with the given configuration
func Create(config CreateConfig) error {
	// Validate all inputs first (no side effects)
	if err := config.Validate(); err != nil {
		return err
	}

	// Check for direct root execution (FR-003)
	if fsInterface.Getuid() == 0 {
		return NewWriteError("direct root execution not allowed", nil)
	}

	// Detect sudo context (required for proper operation)
	sudoCtx, err := detectSudoContext()
	if err != nil {
		return err
	}

	// Validate that we have proper sudo context for append-only setting
	if sudoCtx == nil {
		return NewWriteError("append-only attribute requires sudo privileges", nil)
	}

	// Create file atomically
	file, err := createFile(config.Path)
	if err != nil {
		return err
	}

	// Defer cleanup on any error
	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(config.Path) // Clean up partial file
		}
	}()

	// Write header
	if err = writeHeader(file, config); err != nil {
		return err
	}

	// Sync data to disk before setting attributes (fdatasync equivalent)
	if err = file.Sync(); err != nil {
		return NewWriteError("failed to sync file data", err)
	}

	// Set ownership to original user (if running under sudo)
	if err = setOwnership(config.Path, sudoCtx); err != nil {
		return err
	}

	// Set append-only attribute using ioctl
	if err = setAppendOnlyAttr(int(file.Fd())); err != nil {
		return err
	}

	// Close file successfully
	if err = file.Close(); err != nil {
		return NewWriteError("failed to close file", err)
	}

	// Clear defer error since we succeeded
	err = nil
	return nil
}

// validateInputs performs all input validation from CreateConfig (no side effects)
func validateInputs(config CreateConfig) error {
	// Validate path is not empty
	if config.Path == "" {
		return NewInvalidInputError("path cannot be empty", nil)
	}

	// Validate path has .fdb extension
	if !strings.HasSuffix(config.Path, FileExtension) || len(config.Path) <= len(FileExtension) {
		return NewInvalidInputError("path must have .fdb extension", nil)
	}

	// Validate rowSize range
	if config.RowSize < MinRowSize || config.RowSize > MaxRowSize {
		return NewInvalidInputError(
			fmt.Sprintf("rowSize must be between %d and %d, got %d", MinRowSize, MaxRowSize, config.RowSize),
			nil,
		)
	}

	// Validate skewMs range
	if config.SkewMs < 0 || config.SkewMs > MaxSkewMs {
		return NewInvalidInputError(
			fmt.Sprintf("skewMs must be between 0 and %d, got %d", MaxSkewMs, config.SkewMs),
			nil,
		)
	}

	return nil
}

// validatePath validates path and filesystem preconditions
func validatePath(path string) error {
	// Get parent directory
	parentDir := filepath.Dir(path)

	// Check if parent directory exists
	info, err := fsInterface.Stat(parentDir)
	if os.IsNotExist(err) {
		return NewPathError("parent directory does not exist", err)
	}
	if err != nil {
		return NewPathError("failed to access parent directory", err)
	}

	// Check if parent is a directory
	if !info.IsDir() {
		return NewPathError("parent path is not a directory", nil)
	}

	// Check if parent directory is writable
	if info.Mode().Perm()&0200 == 0 {
		return NewPathError("parent directory is not writable", nil)
	}

	// Check if target file already exists
	if _, err := fsInterface.Stat(path); err == nil {
		return NewPathError("file already exists", nil)
	} else if !os.IsNotExist(err) {
		return NewPathError("failed to check if file exists", err)
	}

	return nil
}

// createFile atomically creates the file with proper permissions
func createFile(path string) (*os.File, error) {
	// Create file with O_CREAT|O_EXCL for atomic creation using fsInterface
	file, err := fsInterface.Open(path, O_CREAT_EXCL|syscall.O_WRONLY, FilePermissions)
	if err != nil {
		return nil, NewPathError("failed to create file atomically", err)
	}

	return file, nil
}

// writeHeader writes the frozenDB v1 header to file
func writeHeader(file *os.File, config CreateConfig) error {
	header, err := generateHeader(config.RowSize, config.SkewMs)
	if err != nil {
		return NewWriteError("failed to generate header", err)
	}

	// Write header to file
	n, err := file.Write(header)
	if err != nil {
		return NewWriteError("failed to write header", err)
	}

	// Ensure all 64 bytes were written
	if n != HeaderSize {
		return NewWriteError(
			fmt.Sprintf("expected to write %d bytes, wrote %d", HeaderSize, n),
			nil,
		)
	}

	return nil
}

// setAppendOnlyAttr sets the append-only attribute using ioctl
func setAppendOnlyAttr(fd int) error {
	var flags uint32

	// Get current flags first
	_, _, errno := fsInterface.Ioctl(syscall.SYS_IOCTL, uintptr(fd), FS_IOC_GETFLAGS, uintptr(unsafe.Pointer(&flags)))
	if errno != 0 {
		return NewWriteError("failed to get file flags", errno)
	}

	// Set append-only flag
	flags |= FS_APPEND_FL
	_, _, errno = fsInterface.Ioctl(syscall.SYS_IOCTL, uintptr(fd), FS_IOC_SETFLAGS, uintptr(unsafe.Pointer(&flags)))
	if errno != 0 {
		return NewWriteError("failed to set append-only attribute", errno)
	}

	return nil
}

// setOwnership changes file ownership if running under sudo
func setOwnership(path string, sudoCtx *SudoContext) error {
	// Use fsInterface.Chown to change ownership to original user
	err := fsInterface.Chown(path, sudoCtx.UID, sudoCtx.GID)
	if err != nil {
		return NewWriteError("failed to set file ownership", err)
	}

	return nil
}
