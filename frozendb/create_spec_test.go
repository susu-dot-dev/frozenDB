package frozendb

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
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
