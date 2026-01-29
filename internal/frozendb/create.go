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
	path    string // Filesystem path for the database file
	rowSize int    // Size of each data row in bytes (128-65536)
	skewMs  int    // Time skew window in milliseconds (0-86400000)
}

// NewCreateConfig creates a new CreateConfig with the specified parameters.
// This constructor allows external packages (like examples) to create config instances.
//
// Parameters:
//   - path: Filesystem path for the database file (must end with .fdb)
//   - rowSize: Size of each data row in bytes (128-65536)
//   - skewMs: Time skew window in milliseconds (0-86400000)
//
// Returns a CreateConfig instance. Call Validate() or Create() to check validity.
func NewCreateConfig(path string, rowSize int, skewMs int) CreateConfig {
	return CreateConfig{
		path:    path,
		rowSize: rowSize,
		skewMs:  skewMs,
	}
}

// GetPath returns the filesystem path for the database file
func (cfg *CreateConfig) GetPath() string {
	return cfg.path
}

// GetRowSize returns the row size in bytes
func (cfg *CreateConfig) GetRowSize() int {
	return cfg.rowSize
}

// GetSkewMs returns the time skew window in milliseconds
func (cfg *CreateConfig) GetSkewMs() int {
	return cfg.skewMs
}

// SudoContext contains information about the sudo environment
type SudoContext struct {
	user string // Original username from SUDO_USER
	uid  int    // Original user ID from SUDO_UID
	gid  int    // Original group ID from SUDO_GID
}

// GetUser returns the original username from SUDO_USER
func (sc *SudoContext) GetUser() string {
	return sc.user
}

// GetUID returns the original user ID from SUDO_UID
func (sc *SudoContext) GetUID() int {
	return sc.uid
}

// GetGID returns the original group ID from SUDO_GID
func (sc *SudoContext) GetGID() int {
	return sc.gid
}

// File system constants
const (
	// File permissions: 0644 (owner rw, group/others r)
	FILE_PERMISSIONS = 0644

	// Atomic file creation flags
	O_CREAT_EXCL = syscall.O_CREAT | syscall.O_EXCL

	// File extension requirement
	FILE_EXTENSION = ".fdb"
)

// Linux syscall constants for append-only attribute
const (
	FS_IOC_GETFLAGS = 0x80086601 // Get file flags
	FS_IOC_SETFLAGS = 0x40086602 // Set file flags
	FS_APPEND_FL    = 0x00000020 // Append-only flag
)

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

	ctx := &SudoContext{
		user: sudoUser,
		uid:  uid,
		gid:  gid,
	}

	// Validate the SudoContext before returning
	if err := ctx.Validate(); err != nil {
		return nil, err
	}

	return ctx, nil
}

// Validate validates the SudoContext struct fields
// This method is idempotent and can be called multiple times with the same result
func (sc *SudoContext) Validate() error {
	// Validate user is not empty
	if sc.user == "" {
		return NewWriteError("SudoContext user cannot be empty", nil)
	}

	// Validate UID is positive
	if sc.uid <= 0 {
		return NewWriteError("SudoContext UID must be greater than 0", nil)
	}

	// Validate GID is positive
	if sc.gid <= 0 {
		return NewWriteError("SudoContext GID must be greater than 0", nil)
	}

	return nil
}

// Validate validates the CreateConfig and returns appropriate error types
func (cfg *CreateConfig) Validate() error {
	// Validate rowSize and skewMs by creating a Header struct and validating it
	header := &Header{
		signature: HEADER_SIGNATURE,
		version:   1,
		rowSize:   cfg.rowSize,
		skewMs:    cfg.skewMs,
	}

	if err := header.Validate(); err != nil {
		return err
	}

	// Validate path and filesystem preconditions
	return validatePath(cfg.path)
}

// Create creates a new frozenDB database file with the given configuration
// The file is created with a 64-byte header followed by an initial checksum row
// that covers the header bytes [0..63] using CRC32 IEEE polynomial
func Create(config CreateConfig) error {
	// Validate all inputs first (no side effects)
	if err := config.Validate(); err != nil {
		return err
	}

	// Detect sudo context first (required for proper operation)
	sudoCtx, err := detectSudoContext()
	if err != nil {
		return err
	}

	// Check for direct root execution - only reject if no sudo context
	if fsInterface.Getuid() == 0 && sudoCtx == nil {
		return NewWriteError("direct root execution not allowed", nil)
	}

	// Validate that we have proper sudo context for append-only setting
	if sudoCtx == nil {
		return NewWriteError("append-only attribute requires sudo privileges", nil)
	}

	// Create file atomically
	file, err := createFile(config.path)
	if err != nil {
		return err
	}

	// Defer cleanup on any error
	defer func() {
		if err != nil {
			_ = os.Remove(config.path)
		}
	}()

	// Create Header struct and generate header bytes
	header := &Header{
		signature: HEADER_SIGNATURE,
		version:   1,
		rowSize:   config.rowSize,
		skewMs:    config.skewMs,
	}

	if err := header.Validate(); err != nil {
		return err
	}

	headerBytes, err := header.MarshalText()
	if err != nil {
		return NewWriteError("failed to generate header", err)
	}

	// Calculate CRC32 for header bytes [0..63] (entire header)
	checksumRow, err := NewChecksumRow(header.GetRowSize(), headerBytes)
	if err != nil {
		return NewWriteError("failed to create checksum row", err)
	}

	// Marshal checksum row to bytes
	checksumBytes, err := checksumRow.MarshalText()
	if err != nil {
		return NewWriteError("failed to marshal checksum row", err)
	}

	// Write header and checksum row atomically in a single write operation
	// This ensures both are written together or neither is written
	totalSize := HEADER_SIZE + config.rowSize
	writeBuffer := make([]byte, totalSize)
	copy(writeBuffer[0:HEADER_SIZE], headerBytes)
	copy(writeBuffer[HEADER_SIZE:], checksumBytes)

	n, err := file.Write(writeBuffer)
	if err != nil {
		return NewWriteError("failed to write header and checksum row", err)
	}
	if n != totalSize {
		return NewWriteError(fmt.Sprintf("expected to write %d bytes, wrote %d", totalSize, n), nil)
	}

	// Sync data to disk before setting attributes (fdatasync equivalent)
	if err = file.Sync(); err != nil {
		return NewWriteError("failed to sync file data", err)
	}

	// Set ownership to original user (if running under sudo)
	if err = setOwnership(config.path, sudoCtx); err != nil {
		return err
	}

	// Set append-only attribute using ioctl (must be done while file is open)
	if err = setAppendOnlyAttr(int(file.Fd())); err != nil {
		return err
	}

	// Close file before validation - we'll re-open for reading
	if err = file.Close(); err != nil {
		return NewWriteError("failed to close file before validation", err)
	}
	return nil
}

// validatePath validates path format and filesystem preconditions
func validatePath(path string) error {
	// Validate path is not empty
	if path == "" {
		return NewInvalidInputError("path cannot be empty", nil)
	}

	// Validate path has .fdb extension
	if !strings.HasSuffix(path, FILE_EXTENSION) || len(path) <= len(FILE_EXTENSION) {
		return NewInvalidInputError("path must have .fdb extension", nil)
	}

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
	file, err := fsInterface.Open(path, O_CREAT_EXCL|syscall.O_WRONLY, FILE_PERMISSIONS)
	if err != nil {
		return nil, NewPathError("failed to create file atomically", err)
	}

	return file, nil
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
	err := fsInterface.Chown(path, sudoCtx.uid, sudoCtx.gid)
	if err != nil {
		return NewWriteError("failed to set file ownership", err)
	}

	return nil
}
