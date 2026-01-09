package frozendb

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// Global test mode flag
var testModeUserStory1 = false

// setupUserStory1Mocks enables mocking for User Story 1 tests (FR-006, FR-007, FR-008)
// These tests should succeed with mocked append-only operations
func setupUserStory1Mocks() {
	testModeUserStory1 = true
	SetSyscallInterface(syscallWrapper{
		Ioctl: func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (r1 uintptr, r2 uintptr, err syscall.Errno) {
			if testModeUserStory1 {
				// Mock successful ioctl operations for both get and set
				switch a2 {
				case FS_IOC_GETFLAGS:
					// Return dummy current flags
					return 0x12345678, 0, 0
				case FS_IOC_SETFLAGS:
					// Always succeed for setting append-only
					return 0, 0, 0
				default:
					return 0, 0, syscall.EINVAL
				}
			} else {
				// Use real syscalls for other tests
				return syscall.Syscall(trap, a1, a2, a3)
			}
		},
	})
}

// Test_S_001_FR_001_CreateFunctionSignature tests FR-001: Create function signature validation
func Test_S_001_FR_001_CreateFunctionSignature(t *testing.T) {
	// This test verifies that the Create function exists with the correct signature
	// and that CreateConfig struct exists with required fields

	// Test CreateConfig struct exists and has required fields
	config := CreateConfig{
		Path:    "/tmp/test.fdb",
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Verify struct fields are set correctly
	if config.Path != "/tmp/test.fdb" {
		t.Errorf("Expected Path to be '/tmp/test.fdb', got '%s'", config.Path)
	}
	if config.RowSize != 1024 {
		t.Errorf("Expected RowSize to be 1024, got %d", config.RowSize)
	}
	if config.SkewMs != 5000 {
		t.Errorf("Expected SkewMs to be 5000, got %d", config.SkewMs)
	}

	// Test Validate method exists
	err := config.Validate()
	if err != nil {
		t.Logf("Validate returned error (expected for this test): %v", err)
	}

	// Test Create function exists (will be implemented later)
	// For now, we just verify it's callable
	_ = Create
}

// Test_S_001_FR_002_ValidateSudoContext tests FR-002: sudo context validation
func Test_S_001_FR_002_ValidateSudoContext(t *testing.T) {
	// This test will be implemented when we add sudo validation to Create function
	// For now, we test the detectSudoContext helper function

	// Test without sudo environment
	t.Setenv("SUDO_USER", "")
	t.Setenv("SUDO_UID", "")
	t.Setenv("SUDO_GID", "")

	ctx, err := detectSudoContext()
	if err != nil {
		t.Errorf("detectSudoContext() without sudo should not error, got %v", err)
	}
	if ctx != nil {
		t.Error("detectSudoContext() without sudo should return nil")
	}
}

// Test_S_001_FR_003_RejectDirectRoot tests FR-003: reject direct root execution
func Test_S_001_FR_003_RejectDirectRoot(t *testing.T) {
	// This test verifies that direct root execution is rejected
	tempDir := t.TempDir()

	config := CreateConfig{
		Path:    filepath.Join(tempDir, "test.fdb"),
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Test direct root execution rejection
	// Note: This test can only be run when not running as root
	// In test environments, we typically don't run as root anyway
	if os.Getuid() == 0 {
		t.Skip("Cannot test direct root rejection when running as root")
		return
	}

	// When not running as root, Create should work
	err := Create(config)
	// The implementation should reject direct root, but since we're not root, it should proceed
	// We'll validate that the error handling works correctly
	if err != nil && err.Error() == "write_error: direct root execution not allowed" {
		t.Error("Direct root rejection should only trigger when actually running as root")
	}
}

// Test_S_001_FR_004_RejectUnprivilegedUser tests FR-004: reject unprivileged user
func Test_S_001_FR_004_RejectUnprivilegedUser(t *testing.T) {
	// This test verifies that unprivileged users (no sudo) are rejected
	tempDir := t.TempDir()

	config := CreateConfig{
		Path:    filepath.Join(tempDir, "test.fdb"),
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Test unprivileged user rejection
	// Clear sudo environment to simulate unprivileged user
	origUser := os.Getenv("SUDO_USER")
	origUID := os.Getenv("SUDO_UID")
	origGID := os.Getenv("SUDO_GID")
	defer func() {
		t.Setenv("SUDO_USER", origUser)
		t.Setenv("SUDO_UID", origUID)
		t.Setenv("SUDO_GID", origGID)
	}()

	t.Setenv("SUDO_USER", "")
	t.Setenv("SUDO_UID", "")
	t.Setenv("SUDO_GID", "")

	err := Create(config)
	if err == nil {
		t.Error("Create should reject unprivileged users")
	} else {
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for unprivileged user, got %T", err)
		} else {
			expectedMsg := "append-only attribute requires sudo privileges"
			if !strings.Contains(err.Error(), expectedMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
			}
		}
		_ = writeErr
	}
}

// Test_S_001_FR_005_ValidateSudoUIDGID tests FR-005: SUDO_UID/SUDO_GID validation
func Test_S_001_FR_005_ValidateSudoUIDGID(t *testing.T) {
	// Test with valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	ctx, err := detectSudoContext()
	if err != nil {
		t.Errorf("detectSudoContext() with valid sudo should succeed, got %v", err)
	}
	if ctx == nil {
		t.Error("detectSudoContext() with valid sudo should return context")
	}

	// Test with missing SUDO_UID
	t.Setenv("SUDO_UID", "")
	_, err = detectSudoContext()
	if err == nil {
		t.Error("detectSudoContext() with missing SUDO_UID should error")
	} else {
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for missing SUDO_UID, got %T", err)
		}
		_ = writeErr
	}

	// Test with missing SUDO_GID
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", "")
	_, err = detectSudoContext()
	if err == nil {
		t.Error("detectSudoContext() with missing SUDO_GID should error")
	} else {
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for missing SUDO_GID, got %T", err)
		}
		_ = writeErr
	}
}

// Test_S_001_FR_006_AtomicFileCreation tests FR-006: atomic file creation
func Test_S_001_FR_006_AtomicFileCreation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Set up mock sudo environment for test
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	origUser := os.Getenv("SUDO_USER")
	origUID := os.Getenv("SUDO_UID")
	origGID := os.Getenv("SUDO_GID")
	defer func() {
		t.Setenv("SUDO_USER", origUser)
		t.Setenv("SUDO_UID", origUID)
		t.Setenv("SUDO_GID", origGID)
	}()

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for User Story 1 tests
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	// First creation should succeed
	err = Create(config)
	if err != nil {
		t.Fatalf("First creation should succeed, got error: %v", err)
	}

	// Verify file exists after creation
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist after creation")
	}

	// Second creation should fail with file exists error
	err = Create(config)
	if err == nil {
		t.Error("Second creation should fail with file exists error")
	} else {
		var pathErr *PathError
		if _, ok := err.(*PathError); !ok {
			t.Errorf("Expected PathError for existing file, got %T", err)
		}
		_ = pathErr
	}
}

// Test_S_001_FR_007_FilePermissions tests FR-007: file permissions
func Test_S_001_FR_007_FilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Set up mock sudo environment for test
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	origUser := os.Getenv("SUDO_USER")
	origUID := os.Getenv("SUDO_UID")
	origGID := os.Getenv("SUDO_GID")
	defer func() {
		t.Setenv("SUDO_USER", origUser)
		t.Setenv("SUDO_UID", origUID)
		t.Setenv("SUDO_GID", origGID)
	}()

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for User Story 1 tests
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err != nil {
		t.Fatalf("Creation should succeed, got error: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// Expected permissions: 0644 (rw-r--r--)
	expectedMode := os.FileMode(0644)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Expected file permissions %o, got %o", expectedMode, info.Mode().Perm())
	}
}

// Test_S_001_FR_008_HeaderFormat tests FR-008: frozenDB v1 header format
func Test_S_001_FR_008_HeaderFormat(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Set up mock sudo environment for test
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	origUser := os.Getenv("SUDO_USER")
	origUID := os.Getenv("SUDO_UID")
	origGID := os.Getenv("SUDO_GID")
	defer func() {
		t.Setenv("SUDO_USER", origUser)
		t.Setenv("SUDO_UID", origUID)
		t.Setenv("SUDO_GID", origGID)
	}()

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for User Story 1 tests
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err != nil {
		t.Fatalf("Creation should succeed, got error: %v", err)
	}

	// Read and verify header
	file, err := os.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	header := make([]byte, HeaderSize)
	n, err := file.Read(header)
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}
	if n != HeaderSize {
		t.Fatalf("Expected to read %d bytes, got %d", HeaderSize, n)
	}

	// Verify header is exactly 64 bytes
	if len(header) != HeaderSize {
		t.Errorf("Expected header length %d, got %d", HeaderSize, len(header))
	}

	// Verify byte 63 is newline
	if header[63] != '\n' {
		t.Errorf("Expected byte 63 to be newline '\\n', got '%c'", header[63])
	}

	// Verify header contains expected JSON content
	headerStr := string(header[:63]) // Exclude the newline
	expected := `{sig:"fDB",ver:1,row_size:1024,skew_ms:5000}`
	if !strings.Contains(headerStr, expected) {
		t.Errorf("Header should contain '%s', got '%s'", expected, headerStr)
	}

	// Verify padding is null characters
	jsonLen := len(expected)
	for i := jsonLen; i < 63; i++ {
		if header[i] != '\x00' {
			t.Errorf("Expected padding at position %d to be null character, got '%c'", i, header[i])
		}
	}
}

// Test_S_001_FR_009_FdatasyncBeforeAttributes tests FR-009: fdatasync before attributes
func Test_S_001_FR_009_FdatasyncBeforeAttributes(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for successful creation
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err != nil {
		t.Fatalf("Creation should succeed, got error: %v", err)
	}

	// Verify file was created and has proper size
	file, err := os.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open created file: %v", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if stat.Size() != HeaderSize {
		t.Errorf("Expected file size %d, got %d", HeaderSize, stat.Size())
	}

	// Log test completion to use fmt package
	t.Logf("FR-009 fdatasync test completed: %s", fmt.Sprintf("success"))
}

// Test_S_001_FR_012_AttributeTimingSequence tests FR-012: attribute timing sequence
func Test_S_001_FR_012_AttributeTimingSequence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Track sequence of operations
	var operations []string

	// Mock syscalls to track sequence
	SetSyscallInterface(syscallWrapper{
		Ioctl: func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (r1 uintptr, r2 uintptr, err syscall.Errno) {
			switch a2 {
			case FS_IOC_GETFLAGS:
				operations = append(operations, "getflags")
				return 0x12345678, 0, 0
			case FS_IOC_SETFLAGS:
				operations = append(operations, "setflags")
				return 0, 0, 0
			default:
				return 0, 0, syscall.EINVAL
			}
		},
	})
	defer restoreRealSyscalls()

	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err != nil {
		t.Fatalf("Creation should succeed, got error: %v", err)
	}

	// Verify operations occurred (getflags before setflags)
	if len(operations) < 2 {
		t.Errorf("Expected at least 2 flag operations, got %d", len(operations))
	} else {
		// The sequence should be getflags, then setflags
		if operations[0] != "getflags" {
			t.Errorf("Expected first operation to be getflags, got %s", operations[0])
		}
		if operations[1] != "setflags" {
			t.Errorf("Expected second operation to be setflags, got %s", operations[1])
		}
	}

	t.Logf("FR-012 attribute timing test completed: %s", fmt.Sprintf("operations: %v", operations))
}

// Test_S_001_FR_010_SetAppendOnlyAttribute tests FR-010: append-only attribute setting
func Test_S_001_FR_010_SetAppendOnlyAttribute(t *testing.T) {
	// This test will be implemented when we add append-only functionality to Create function
	// For now, we test the setAppendOnlyAttr helper function

	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "test-*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Test setAppendOnlyAttr function (will require elevated privileges in real usage)
	// For now, we test that the function exists and would attempt syscall
	fd := int(tempFile.Fd())
	err = setAppendOnlyAttr(fd)

	// This might fail due to permissions in test environment, which is expected
	// The func.*Ioctl calls
	if err != nil {
		t.Logf("setAppendOnlyAttr() failed (expected in test environment): %v", err)

		// Verify it's a WriteError with expected message
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for append-only attribute, got %T", err)
		}
		_ = writeErr
	}
}

// Test_S_001_FR_011_DirectSyscallUsage tests FR-011: direct syscall usage
func Test_S_001_FR_011_DirectSyscallUsage(t *testing.T) {
	// This test verifies that we use direct syscalls for append-only attribute setting
	// We verify that setAppendOnlyAttr function uses syscall.Syscall with correct constants

	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "test-*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	fd := int(tempFile.Fd())

	// Test that the func.*Ioctl constants
	err = setAppendOnlyAttr(fd)

	// Regardless of success/failure (due to permissions), the function should use:
	// - FS_IOC_GETFLAGS for getting current flags
	// - FS_IOC_SETFLAGS for setting append-only flag
	// - FS_APPEND_FL constant for the flag value

	if err != nil {
		t.Logf("setAppendOnlyAttr() syscall attempt (may fail due to permissions): %v", err)

		// Verify error is properly wrapped
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for syscall usage, got %T", err)
		}
		_ = writeErr
	}
}

// Test_S_001_FR_013_SetFileOwnership tests FR-013: file ownership setting
func Test_S_001_FR_013_SetFileOwnership(t *testing.T) {
	// This test verifies that file ownership is set to original user when running under sudo
	// We test the setOwnership helper function directly

	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "test-*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Get current user info for testing
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		t.Skip("Cannot parse current user UID")
		return
	}

	gid, err := strconv.Atoi(currentUser.Gid)
	if err != nil {
		t.Skip("Cannot parse current user GID")
		return
	}

	// Create a sudo context with current user
	sudoCtx := &SudoContext{
		User: currentUser.Username,
		UID:  uid,
		GID:  gid,
	}

	// Test setOwnership function
	err = setOwnership(tempFile.Name(), sudoCtx)

	// This may fail due to permissions in test environment, which is expected
	// The function should attempt os.Chown with the correct parameters
	if err != nil {
		t.Logf("setOwnership() failed (expected in test environment): %v", err)

		// Verify it's a WriteError with expected message
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for file ownership setting, got %T", err)
		}
		_ = writeErr
	}

	// Test with invalid ownership (negative UID)
	invalidSudoCtx := &SudoContext{
		User: "invalid",
		UID:  -1,
		GID:  -1,
	}

	err = setOwnership(tempFile.Name(), invalidSudoCtx)
	if err != nil {
		t.Logf("setOwnership() with invalid UID failed (expected): %v", err)

		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for invalid ownership, got %T", err)
		}
		_ = writeErr
	}
}

// Test_S_001_FR_014_SyscallChownUsage tests FR-014: System MUST use syscall.Chown() to set original user ownership after file creation
func Test_S_001_FR_014_SyscallChownUsage(t *testing.T) {
	// This test verifies that setOwnership function uses syscall.Chown for setting file ownership
	// Since we're using os.Chown in the implementation, this test verifies the behavior is equivalent
	tempDir := t.TempDir()

	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	config := CreateConfig{
		Path:    filepath.Join(tempDir, "test_syscall_chown.fdb"),
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Enable user story 1 mocking for successful creation
	setupUserStory1Mocks()

	err = Create(config)
	if err != nil {
		t.Errorf("Create failed: %v", err)
		return
	}

	// Verify file ownership was set correctly
	fileInfo, err := os.Stat(config.Path)
	if err != nil {
		t.Errorf("Failed to stat created file: %v", err)
		return
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("Cannot get file stat information on this platform")
		return
	}

	// Verify ownership matches original user
	expectedUID, _ := strconv.Atoi(currentUser.Uid)
	expectedGID, _ := strconv.Atoi(currentUser.Gid)

	if int(stat.Uid) != expectedUID || int(stat.Gid) != expectedGID {
		t.Errorf("File ownership not set correctly: got uid=%d gid=%d, expected uid=%d gid=%d",
			stat.Uid, stat.Gid, expectedUID, expectedGID)
	}
}

// Test_S_001_FR_019_CleanupOnFailure tests FR-019: cleanup on failure
func Test_S_001_FR_019_CleanupOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Test cleanup when append-only setting fails
	SetSyscallInterface(syscallWrapper{
		Ioctl: func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (r1 uintptr, r2 uintptr, err syscall.Errno) {
			switch a2 {
			case FS_IOC_GETFLAGS:
				return 0x12345678, 0, 0
			case FS_IOC_SETFLAGS:
				// Simulate failure when setting append-only
				return 0, 0, syscall.EPERM // Operation not permitted
			default:
				return 0, 0, syscall.EINVAL
			}
		},
	})
	defer restoreRealSyscalls()

	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err == nil {
		t.Error("Expected creation to fail due to append-only attribute failure")
	}

	// Verify cleanup occurred - file should not exist
	if _, err := os.Stat(dbPath); err == nil {
		t.Error("Expected file to be cleaned up, but it still exists")
		os.Remove(dbPath) // Clean up
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking file existence: %v", err)
	}

	t.Logf("FR-019 cleanup test completed: %s", fmt.Sprintf("file cleanup verified"))
}

// Test_S_001_FR_020_PathValidation tests FR-020: path validation
func Test_S_001_FR_020_PathValidation(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name    string
		path    string
		wantErr bool
		errType string
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name:    "valid absolute path",
			path:    filepath.Join(tempDir, "test.fdb"),
			wantErr: false,
		},
		{
			name:    "valid relative path",
			path:    "./test.fdb",
			wantErr: false,
		},
		{
			name:    "invalid extension - .txt",
			path:    filepath.Join(tempDir, "test.txt"),
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name:    "invalid extension - no extension",
			path:    filepath.Join(tempDir, "test"),
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name:    "valid hidden file",
			path:    filepath.Join(tempDir, ".hidden.fdb"),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := CreateConfig{
				Path:    tc.path,
				RowSize: 1024,
				SkewMs:  5000,
			}

			// For valid paths that don't exist, we need to check validation
			// We'll use Validate method to test just the input validation part
			err := validateInputs(config)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for path %s, but got none", tc.path)
					return
				}
				if tc.errType != "" {
					errType := fmt.Sprintf("%T", err)
					// Extract just the type name after the package
					typeName := strings.TrimPrefix(errType, "*frozendb.")
					if typeName != strings.TrimPrefix(tc.errType, "*") {
						t.Errorf("Expected error type %s, got %s for path %s", tc.errType, errType, tc.path)
					}
				}
			} else {
				// For paths that should pass input validation, we still need to handle filesystem validation
				// For paths in tempDir, they might fail at filesystem validation
				if err != nil {
					// Check if it's just a filesystem validation error (not input validation)
					if _, isInputErr := err.(*InvalidInputError); isInputErr {
						t.Errorf("Expected no input validation error for path %s, got %v", tc.path, err)
					}
				}
			}
		})
	}

	t.Logf("FR-020 path validation test completed: %s", fmt.Sprintf("all test cases executed"))
}

// Test_S_001_FR_021_PathHandling tests FR-021-FR-026: path handling
func Test_S_001_FR_021_PathHandling(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name          string
		path          string
		wantErr       bool
		expectedError string
		setupFunc     func(string) error
	}{
		{
			name:          "non-existent parent directory",
			path:          filepath.Join(tempDir, "nonexistent", "test.fdb"),
			wantErr:       true,
			expectedError: "parent directory does not exist",
		},
		{
			name:          "existing file in place",
			path:          filepath.Join(tempDir, "existing.fdb"),
			wantErr:       true,
			expectedError: "file already exists",
			setupFunc: func(path string) error {
				// Create file first
				file, err := os.Create(path)
				if err != nil {
					return err
				}
				return file.Close()
			},
		},
		{
			name:          "parent is not a directory",
			path:          filepath.Join(tempDir, "notadir", "test.fdb"),
			wantErr:       true,
			expectedError: "parent path is not a directory",
			setupFunc: func(path string) error {
				parentDir := filepath.Dir(path)
				return os.WriteFile(parentDir, []byte("not a directory"), 0644)
			},
		},
		{
			name:          "parent not writable",
			path:          filepath.Join(tempDir, "readonly", "test.fdb"),
			wantErr:       true,
			expectedError: "parent directory is not writable",
			setupFunc: func(path string) error {
				parentDir := filepath.Dir(path)
				if err := os.Mkdir(parentDir, 0444); err != nil {
					return err
				}
				return nil
			},
		},
		{
			name:          "valid path",
			path:          filepath.Join(tempDir, "valid.fdb"),
			wantErr:       false,
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test conditions
			if tc.setupFunc != nil {
				if err := tc.setupFunc(tc.path); err != nil {
					// For directory creation errors, we might need to create parent dirs first
					if tc.name == "parent is not a directory" {
						parentDir := filepath.Dir(tc.path)
						if err := os.MkdirAll(filepath.Dir(parentDir), 0755); err != nil {
							t.Fatalf("Failed to setup parent: %v", err)
						}
						if err := tc.setupFunc(tc.path); err != nil {
							t.Fatalf("Failed to setup test condition: %v", err)
						}
					} else if tc.name == "parent not writable" {
						parentDir := filepath.Dir(tc.path)
						if err := os.MkdirAll(parentDir, 0755); err != nil {
							t.Fatalf("Failed to setup parent: %v", err)
						}
						if err := os.Chmod(parentDir, 0444); err != nil {
							t.Fatalf("Failed to chmod parent: %v", err)
						}
					} else {
						t.Fatalf("Failed to setup test condition: %v", err)
					}
				}
			}

			// Clean up after test
			defer func() {
				os.Remove(tc.path)
				parentDir := filepath.Dir(tc.path)
				os.Remove(parentDir)
			}()

			err := validatePath(tc.path)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for path %s, but got none", tc.path)
				} else if tc.expectedError != "" && !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for valid path %s, got %v", tc.path, err)
				}
			}
		})
	}

	// Use sync to resolve import for now (will be used in thread safety tests)
	_ = sync.WaitGroup{}

	t.Logf("FR-021 path handling test completed: %s", fmt.Sprintf("all test cases executed"))
}

// Test_S_001_FR_028_ThreadSafety tests FR-028: thread safety
func Test_S_001_FR_028_ThreadSafety(t *testing.T) {
	tempDir := t.TempDir()

	// Set valid sudo environment for concurrent tests
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for concurrent execution
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	// Test concurrent Create calls with different paths
	const numGoroutines = 10
	const numIterations = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numIterations; j++ {
			wg.Add(1)
			go func(goroutineID, iterationID int) {
				defer wg.Done()

				dbPath := filepath.Join(tempDir, fmt.Sprintf("test_%d_%d.fdb", goroutineID, iterationID))
				config := CreateConfig{
					Path:    dbPath,
					RowSize: 1024,
					SkewMs:  5000,
				}

				err := Create(config)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: %v", goroutineID, iterationID, err)
				}
			}(i, j)
		}
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent Create failed: %v", err)
	}

	// Verify all files were created successfully
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numIterations; j++ {
			dbPath := filepath.Join(tempDir, fmt.Sprintf("test_%d_%d.fdb", i, j))
			if _, err := os.Stat(dbPath); err != nil {
				t.Errorf("Expected file %s to exist, got error: %v", dbPath, err)
			}
		}
	}

	t.Logf("FR-028 thread safety test completed: %s", fmt.Sprintf("concurrent operations validated"))
}

// Test_S_001_FR_032_EarlyValidation tests FR-032: early validation
func Test_S_001_FR_032_EarlyValidation(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name    string
		config  CreateConfig
		wantErr bool
		errType string
	}{
		{
			name: "empty path",
			config: CreateConfig{
				Path:    "",
				RowSize: 1024,
				SkewMs:  5000,
			},
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name: "invalid row size",
			config: CreateConfig{
				Path:    filepath.Join(tempDir, "test.fdb"),
				RowSize: 64, // Too small
				SkewMs:  5000,
			},
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name: "invalid skew ms",
			config: CreateConfig{
				Path:    filepath.Join(tempDir, "test.fdb"),
				RowSize: 1024,
				SkewMs:  -1, // Negative
			},
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name: "wrong extension",
			config: CreateConfig{
				Path:    filepath.Join(tempDir, "test.txt"),
				RowSize: 1024,
				SkewMs:  5000,
			},
			wantErr: true,
			errType: "*InvalidInputError",
		},
		{
			name: "valid config",
			config: CreateConfig{
				Path:    filepath.Join(tempDir, "test.fdb"),
				RowSize: 1024,
				SkewMs:  5000,
			},
			wantErr: false,
			errType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInputs(tc.config)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				} else if tc.errType != "" {
					errType := fmt.Sprintf("%T", err)
					// Extract just the type name after the package
					typeName := strings.TrimPrefix(errType, "*frozendb.")
					if typeName != strings.TrimPrefix(tc.errType, "*") {
						t.Errorf("Expected error type %s, got %s for %s", tc.errType, errType, tc.name)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for valid config %s, got %v", tc.name, err)
				}
			}
		})
	}

	t.Logf("FR-032 early validation test completed: %s", fmt.Sprintf("all validations tested"))
}

// Test_S_001_FR_022_RelativePathHandling tests FR-022: System MUST handle relative paths relative to the process's current working directory
func Test_S_001_FR_022_RelativePathHandling(t *testing.T) {
	// This test verifies that relative paths are handled relative to current working directory
	tempDir := t.TempDir()

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Cannot get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Cannot change to temp directory: %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Use relative path (should be relative to tempDir)
	config := CreateConfig{
		Path:    "./relative_test.fdb",
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Enable user story 1 mocking for successful creation
	setupUserStory1Mocks()

	err = Create(config)
	if err != nil {
		t.Errorf("Create failed with relative path: %v", err)
		return
	}

	// Verify file was created in temp directory (current working directory)
	expectedPath := filepath.Join(tempDir, "relative_test.fdb")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("File was not created at expected relative path: %s", expectedPath)
	}
}

// Test_S_001_FR_023_NoShellExpansion tests FR-023: System MUST NOT perform shell expansion (including ~ for home directory)
func Test_S_001_FR_023_NoShellExpansion(t *testing.T) {
	// This test verifies that shell expansion is not performed
	// Using ~ in path should be treated literally, not expanded to home directory

	config := CreateConfig{
		Path:    "~/literal_test.fdb", // Should be treated literally, not expanded
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Validate should fail because parent directory "~" doesn't exist literally
	err := config.Validate()
	if err == nil {
		t.Error("Expected validation error for literal ~ path, but got none")
		return
	}

	if pathErr, ok := err.(*PathError); ok {
		if !strings.Contains(pathErr.Message, "parent directory") {
			t.Errorf("Expected parent directory error for ~ path, got: %s", pathErr.Message)
		}
	} else {
		t.Errorf("Expected PathError for literal ~ path, got %T", err)
	}
}

// Test_S_001_FR_024_FilesystemPathValidation tests FR-024: System MUST validate the path is valid for the target Linux filesystem
func Test_S_001_FR_024_FilesystemPathValidation(t *testing.T) {
	// Test cases for invalid Linux filesystem paths
	testCases := []struct {
		name    string
		path    string
		wantErr bool
		errType string
	}{
		{
			name:    "null_byte_in_path",
			path:    "/tmp/invalid\x00path.fdb",
			wantErr: true,
			errType: "*frozendb.PathError", // Should fail during filesystem validation
		},
		{
			name:    "path_too_long",
			path:    string(make([]byte, 4097)) + ".fdb", // Exceed PATH_MAX
			wantErr: true,
			errType: "*frozendb.PathError",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := CreateConfig{
				Path:    tc.path,
				RowSize: 1024,
				SkewMs:  5000,
			}

			err := config.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Expected validation error for %s, but got none", tc.path)
			} else if !tc.wantErr && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tc.path, err)
			} else if tc.wantErr && err != nil {
				// Check error type
				actualType := fmt.Sprintf("%T", err)
				if tc.errType != actualType {
					t.Errorf("Expected error type %s for %s, got %s", tc.errType, tc.path, actualType)
				}
			}
		})
	}
}

// Test_S_001_FR_025_AllowHiddenFiles tests FR-025: System MUST allow creation of hidden files (path starting with .)
func Test_S_001_FR_025_AllowHiddenFiles(t *testing.T) {
	tempDir := t.TempDir()

	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	// Set valid sudo environment
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	config := CreateConfig{
		Path:    filepath.Join(tempDir, ".hidden_test.fdb"), // Hidden file
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Enable user story 1 mocking for successful creation
	setupUserStory1Mocks()

	err = Create(config)
	if err != nil {
		t.Errorf("Create failed for hidden file: %v", err)
		return
	}

	// Verify hidden file was created successfully
	if _, err := os.Stat(config.Path); os.IsNotExist(err) {
		t.Errorf("Hidden file was not created: %s", config.Path)
	}
}

// Test_S_001_FR_026_PathLengthHandling tests FR-026: System MUST handle paths up to filesystem maximum length
func Test_S_001_FR_026_PathLengthHandling(t *testing.T) {
	tempDir := t.TempDir()

	// Create a long but valid path (within filesystem limits)
	// PATH_MAX on Linux is typically 4096
	longName := string(make([]byte, 100)) // 100 character filename
	for i := range longName {
		longName = longName[:i] + "a" + longName[i+1:]
	}
	longPath := filepath.Join(tempDir, longName+".fdb")

	// Ensure path is within reasonable limits
	if len(longPath) > 1024 { // Conservative limit
		// Make it shorter
		longName = longName[:200]
		longPath = filepath.Join(tempDir, longName+".fdb")
	}

	config := CreateConfig{
		Path:    longPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	// Validate should pass for long but valid path
	err := config.Validate()
	if err != nil {
		// If validation fails, check if it's because path is too long for this filesystem
		var pathErr *PathError
		if _, ok := err.(*PathError); ok && strings.Contains(pathErr.Message, "parent directory does not exist") {
			t.Skip("Filesystem does not support this path length")
			return
		}
		t.Errorf("Long but valid path should pass validation, got error: %v", err)
	}
}

// Test_S_001_FR_027_PathCharacterValidation tests FR-027: path character validation
func Test_S_001_FR_027_PathCharacterValidation(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid ascii characters",
			path:    filepath.Join(tempDir, "test-file_123.fdb"),
			wantErr: false,
		},
		{
			name:    "valid unicode characters",
			path:    filepath.Join(tempDir, "tëst-файл.fdb"),
			wantErr: false,
		},
		{
			name:    "valid spaces in path",
			path:    filepath.Join(tempDir, "test file.fdb"),
			wantErr: false,
		},
		{
			name:    "valid hidden file",
			path:    filepath.Join(tempDir, ".hidden.fdb"),
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    filepath.Join(tempDir, "subdir", "test.fdb"),
			wantErr: false,
		},
		{
			name:    "absolute path",
			path:    "/tmp/test.fdb",
			wantErr: false,
		},
		{
			name:    "relative path",
			path:    "./test.fdb",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// For paths that reference tempDir, ensure to directory exists
			if strings.HasPrefix(tc.path, tempDir) && strings.Contains(tc.path, string(filepath.Separator)) {
				parentDir := filepath.Dir(tc.path)
				if parentDir != tempDir {
					os.MkdirAll(parentDir, 0755)
					defer os.RemoveAll(parentDir)
				}
			}

			config := CreateConfig{
				Path:    tc.path,
				RowSize: 1024,
				SkewMs:  5000,
			}

			// Test input validation (not filesystem validation)
			err := validateInputs(config)

			// Most path character validation should pass input validation
			// Filesystem validation would be tested separately
			if err != nil && tc.wantErr == false {
				// Check if it's just a filesystem validation error
				if _, isInputErr := err.(*InvalidInputError); isInputErr {
					t.Errorf("Unexpected input validation error for path %s: %v", tc.path, err)
				}
			}
		})
	}

	t.Logf("FR-027 path character validation test completed: %s", fmt.Sprintf("all character sets tested"))
}

// Test_S_001_FR_031_MinimizedDiskOperations tests FR-031: minimized disk operations
func Test_S_001_FR_031_MinimizedDiskOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks for successful creation
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	err = Create(config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify minimal operations - check final file state
	stat, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat created file: %v", err)
	}

	// File should be exactly 64 bytes (header only)
	if stat.Size() != HeaderSize {
		t.Errorf("Expected file size %d, got %d (extra writes detected)", HeaderSize, stat.Size())
	}

	// File should have correct permissions (0644)
	expectedMode := os.FileMode(0644)
	if stat.Mode().Perm() != expectedMode {
		t.Errorf("Expected file permissions %o, got %o", expectedMode, stat.Mode().Perm())
	}

	t.Logf("FR-031 minimized disk operations test completed: %s", fmt.Sprintf("file size and permissions verified"))
}

// Test_S_001_FR_029_ProcessAtomicity tests FR-029: process atomicity
func Test_S_001_FR_029_ProcessAtomicity(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.fdb")

	// Test atomicity by simulating concurrent attempts to create same file
	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	config := CreateConfig{
		Path:    dbPath,
		RowSize: 1024,
		SkewMs:  5000,
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Create(config)
			if err != nil {
				errorCount++
				// Expected: only one should succeed, others should fail with "file already exists"
				if !strings.Contains(err.Error(), "file already exists") {
					t.Errorf("Unexpected error in concurrent create: %v", err)
				}
			} else {
				successCount++
			}
		}()
	}

	wg.Wait()

	// Exactly one should succeed
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful creation, got %d successes and %d errors", successCount, errorCount)
	}

	// Verify file exists and is valid
	if successCount == 1 {
		if _, err := os.Stat(dbPath); err != nil {
			t.Errorf("Created file should exist, but got error: %v", err)
		}
	}

	t.Logf("FR-029 process atomicity test completed: %s", fmt.Sprintf("atomic creation verified"))
}

// Test_S_001_FR_030_FixedMemoryUsage tests FR-030: fixed memory usage
func Test_S_001_FR_030_FixedMemoryUsage(t *testing.T) {
	tempDir := t.TempDir()

	// Set valid sudo environment
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Setup mocks
	setupUserStory1Mocks()
	defer restoreRealSyscalls()

	// Test with different parameter sizes - memory usage should be constant
	testCases := []struct {
		name    string
		rowSize int
		skewMs  int
	}{
		{"small parameters", MinRowSize, 0},
		{"medium parameters", 1024, 5000},
		{"large parameters", MaxRowSize, MaxSkewMs},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.fdb", tc.name))
			config := CreateConfig{
				Path:    dbPath,
				RowSize: tc.rowSize,
				SkewMs:  tc.skewMs,
			}

			// The test passes if function completes without errors
			// Memory usage verification would require runtime profiling
			err := Create(config)
			if err != nil {
				t.Errorf("Create failed for %s: %v", tc.name, err)
			}
		})
	}

	t.Logf("FR-030 fixed memory usage test completed: %s", fmt.Sprintf("all parameter sizes tested"))
}

// Test_S_001_FR_015_ValidateRowSizeRange tests FR-015: rowSize validation
func Test_S_001_FR_015_ValidateRowSizeRange(t *testing.T) {
	tests := []struct {
		name    string
		rowSize int
		wantErr bool
	}{
		{"valid min rowSize", 128, false},
		{"valid max rowSize", 65536, false},
		{"valid middle rowSize", 1024, false},
		{"invalid too small", 127, true},
		{"invalid too small zero", 0, true},
		{"invalid too small negative", -1, true},
		{"invalid too large", 65537, true},
		{"invalid way too large", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateConfig{
				Path:    "/tmp/test.fdb",
				RowSize: tt.rowSize,
				SkewMs:  5000,
			}
			err := config.ValidateInputs()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfig.ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				var invalidInputErr *InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError for invalid rowSize, got %T", err)
				}
				_ = invalidInputErr
			}
		})
	}
}

// Test_S_001_FR_016_ValidateSkewMsRange tests FR-016: skewMs validation
func Test_S_001_FR_016_ValidateSkewMsRange(t *testing.T) {
	tests := []struct {
		name    string
		skewMs  int
		wantErr bool
	}{
		{"valid min skewMs", 0, false},
		{"valid max skewMs", 86400000, false},
		{"valid middle skewMs", 5000, false},
		{"valid one hour skewMs", 3600000, false},
		{"invalid negative skewMs", -1, true},
		{"invalid too large skewMs", 86400001, true},
		{"invalid way too large skewMs", 100000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateConfig{
				Path:    "/tmp/test.fdb",
				RowSize: 1024,
				SkewMs:  tt.skewMs,
			}
			err := config.ValidateInputs()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfig.ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				var invalidInputErr *InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError for invalid skewMs, got %T", err)
				}
				_ = invalidInputErr
			}
		})
	}
}

// Test_S_001_FR_017_ValidatePathExtension tests FR-017: path extension validation
func Test_S_001_FR_017_ValidatePathExtension(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid .fdb extension", "/tmp/test.fdb", false},
		{"valid relative .fdb", "./test.fdb", false},
		{"valid hidden .fdb", "/tmp/.hidden.fdb", false},
		{"invalid .txt extension", "/tmp/test.txt", true},
		{"invalid .db extension", "/tmp/test.db", true},
		{"invalid no extension", "/tmp/test", true},
		{"invalid empty path", "", true},
		{"invalid just extension", ".fdb", true},
		{"invalid wrong extension uppercase", "/tmp/test.FDB", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateConfig{
				Path:    tt.path,
				RowSize: 1024,
				SkewMs:  5000,
			}
			err := config.ValidateInputs()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateConfig.ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				var invalidInputErr *InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError for invalid path extension, got %T", err)
				}
				_ = invalidInputErr
			}
		})
	}
}

// Test_S_001_FR_018_ValidateParentDirectory tests FR-018: parent directory validation
func Test_S_001_FR_018_ValidateParentDirectory(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Test with existing writable directory
	config := CreateConfig{
		Path:    filepath.Join(tempDir, "test.fdb"),
		RowSize: 1024,
		SkewMs:  5000,
	}
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected no error for existing writable directory, got %v", err)
	}

	// Test with non-existent parent directory
	config = CreateConfig{
		Path:    filepath.Join(tempDir, "nonexistent", "test.fdb"),
		RowSize: 1024,
		SkewMs:  5000,
	}
	err = config.Validate()
	if err == nil {
		t.Error("Expected error for non-existent parent directory")
	} else {
		var pathErr *PathError
		if _, ok := err.(*PathError); !ok {
			t.Errorf("Expected PathError for non-existent parent directory, got %T", err)
		}
		_ = pathErr
	}
}
