package frozendb

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// Mock behaviors for testing
var (
	mockShouldFailGetFlags  bool
	mockShouldFailSetFlags  bool
	mockGetFlagsReturnValue uint32
)

// Mock UID/GID values used in tests to detect when mocks aren't being used
const (
	MOCK_UID  = "12345"
	MOCK_GID  = "12345"
	MOCK_USER = "testuser"
)

// Helper function to set up mock syscalls for testing
func setupMockSyscalls(failGet, failSet bool) {
	mockGetFlagsReturnValue = 0x12345678 // Some dummy flags
	mockShouldFailGetFlags = failGet
	mockShouldFailSetFlags = failSet

	SetFSInterface(fsOperations{
		Getuid: os.Getuid,
		Lookup: func(username string) (*user.User, error) {
			// Always return mock values to ensure consistency and detect when mocks aren't used
			return &user.User{Uid: MOCK_UID, Gid: MOCK_GID, Username: username}, nil
		},
		Open:  os.OpenFile,
		Stat:  os.Stat,
		Mkdir: os.Mkdir,
		Chown: func(name string, uid, gid int) error {
			return nil
		},
		Ioctl: func(trap uintptr, a1 uintptr, a2 uintptr, a3 uintptr) (uintptr, uintptr, syscall.Errno) {
			switch a2 {
			case FS_IOC_GETFLAGS:
				if mockShouldFailGetFlags {
					return 0, 0, syscall.EPERM
				}
				return uintptr(mockGetFlagsReturnValue), 0, 0
			case FS_IOC_SETFLAGS:
				if mockShouldFailSetFlags {
					return 0, 0, syscall.EPERM
				}
				// Extract flags from pointer and verify FS_APPEND_FL is set
				// This is safe for testing - we control the pointer value in our mock
				//nolint:govet // safe in test context as we control the pointer
				flagsPtr := (*uint32)(unsafe.Pointer(uintptr(a3)))
				if *flagsPtr&FS_APPEND_FL == 0 {
					return 0, 0, syscall.EINVAL
				}
				return 0, 0, 0
			default:
				return 0, 0, syscall.EINVAL
			}
		},
	})
}

// Helper function to restore real syscalls
func restoreRealSyscalls() {
	fsInterface = &defaultFSOps
}

func TestValidateInputs(t *testing.T) {
	tests := []struct {
		name    string
		config  CreateConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: CreateConfig{
				path:    "/tmp/test.fdb",
				rowSize: 1024,
				skewMs:  5000,
			},
			wantErr: false,
		},
		{
			name: "empty path",
			config: CreateConfig{
				path:    "",
				rowSize: 1024,
				skewMs:  5000,
			},
			wantErr: true,
		},
		{
			name: "invalid extension",
			config: CreateConfig{
				path:    "/tmp/test.txt",
				rowSize: 1024,
				skewMs:  5000,
			},
			wantErr: true,
		},
		{
			name: "rowSize too small",
			config: CreateConfig{
				path:    "/tmp/test.fdb",
				rowSize: 64,
				skewMs:  5000,
			},
			wantErr: true,
		},
		{
			name: "rowSize too large",
			config: CreateConfig{
				path:    "/tmp/test.fdb",
				rowSize: 100000,
				skewMs:  5000,
			},
			wantErr: true,
		},
		{
			name: "negative skewMs",
			config: CreateConfig{
				path:    "/tmp/test.fdb",
				rowSize: 1024,
				skewMs:  -1,
			},
			wantErr: true,
		},
		{
			name: "skewMs too large",
			config: CreateConfig{
				path:    "/tmp/test.fdb",
				rowSize: 1024,
				skewMs:  100000000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInputs(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateHeader(t *testing.T) {
	tests := []struct {
		name    string
		rowSize int
		skewMs  int
		wantErr bool
	}{
		{
			name:    "valid header",
			rowSize: 1024,
			skewMs:  5000,
			wantErr: false,
		},
		{
			name:    "minimum values",
			rowSize: 128,
			skewMs:  0,
			wantErr: false,
		},
		{
			name:    "maximum values",
			rowSize: 65536,
			skewMs:  86400000,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &Header{
				signature: HEADER_SIGNATURE,
				version:   1,
				rowSize:   tt.rowSize,
				skewMs:    tt.skewMs,
			}
			headerBytes, err := header.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("Header.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(headerBytes) != HEADER_SIZE {
					t.Errorf("Header.MarshalText() header length = %d, want %d", len(headerBytes), HEADER_SIZE)
				}

				if headerBytes[63] != '\n' {
					t.Errorf("Header.MarshalText() byte 63 = %c, want '\\n'", headerBytes[63])
				}
			}
		})
	}
}

func TestDetectSudoContext(t *testing.T) {
	// Get current user info for testing
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user for testing")
		return
	}

	// Save original environment
	origUser := os.Getenv("SUDO_USER")
	defer t.Setenv("SUDO_USER", origUser)

	origUID := os.Getenv("SUDO_UID")
	defer t.Setenv("SUDO_UID", origUID)

	origGID := os.Getenv("SUDO_GID")
	defer t.Setenv("SUDO_GID", origGID)

	// Set test environment using current user
	t.Setenv("SUDO_USER", currentUser.Username)
	t.Setenv("SUDO_UID", currentUser.Uid)
	t.Setenv("SUDO_GID", currentUser.Gid)

	// Test with sudo context
	ctx, err := detectSudoContext()
	if err != nil {
		t.Errorf("detectSudoContext() with sudo context error = %v", err)
	}

	if ctx == nil {
		t.Error("detectSudoContext() should return context when SUDO_USER is set")
	} else {
		if ctx.GetUser() != currentUser.Username {
			t.Errorf("detectSudoContext() user = %s, want %s", ctx.GetUser(), currentUser.Username)
		}
		uid, _ := strconv.Atoi(currentUser.Uid)
		if ctx.GetUID() != uid {
			t.Errorf("detectSudoContext() UID = %d, want %s", ctx.GetUID(), currentUser.Uid)
		}
		gid, _ := strconv.Atoi(currentUser.Gid)
		if ctx.GetGID() != gid {
			t.Errorf("detectSudoContext() GID = %d, want %s", ctx.GetGID(), currentUser.Gid)
		}
	}
}

func TestDetectSudoContextNoSudo(t *testing.T) {
	// Clear sudo environment
	t.Setenv("SUDO_USER", "")
	t.Setenv("SUDO_UID", "")
	t.Setenv("SUDO_GID", "")

	ctx, err := detectSudoContext()
	if err != nil {
		t.Errorf("detectSudoContext() without sudo error = %v", err)
	}

	if ctx != nil {
		t.Error("detectSudoContext() should return nil when not running under sudo")
	}
}

func TestSetAppendOnlyAttr(t *testing.T) {
	// Create a temporary file for testing
	tempFile, err := os.CreateTemp("", "test-*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	defer restoreRealSyscalls()

	// Test successful append-only attribute setting
	t.Run("successful append-only setting", func(t *testing.T) {
		setupMockSyscalls(false, false)
		defer restoreRealSyscalls()

		fd := int(tempFile.Fd())
		err = setAppendOnlyAttr(fd)
		if err != nil {
			t.Errorf("setAppendOnlyAttr() with mocked success should succeed, got: %v", err)
		}
	})

	// Test failed get flags operation
	t.Run("failed get flags operation", func(t *testing.T) {
		setupMockSyscalls(true, false)
		defer restoreRealSyscalls()

		fd := int(tempFile.Fd())
		err = setAppendOnlyAttr(fd)
		if err == nil {
			t.Error("setAppendOnlyAttr() with failed get flags should return error")
		} else {
			var writeErr *WriteError
			if _, ok := err.(*WriteError); !ok {
				t.Errorf("Expected WriteError for failed get flags, got %T", err)
			}
			if !strings.Contains(err.Error(), "failed to get file flags") {
				t.Errorf("Expected error message to contain 'failed to get file flags', got '%s'", err.Error())
			}
			_ = writeErr
		}
	})

	// Test failed set flags operation
	t.Run("failed set flags operation", func(t *testing.T) {
		setupMockSyscalls(false, true)
		defer restoreRealSyscalls()

		fd := int(tempFile.Fd())
		err = setAppendOnlyAttr(fd)
		if err == nil {
			t.Error("setAppendOnlyAttr() with failed set flags should return error")
		} else {
			var writeErr *WriteError
			if _, ok := err.(*WriteError); !ok {
				t.Errorf("Expected WriteError for failed set flags, got %T", err)
			}
			if !strings.Contains(err.Error(), "failed to set append-only attribute") {
				t.Errorf("Expected error message to contain 'failed to set append-only attribute', got '%s'", err.Error())
			}
			_ = writeErr
		}
	})
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Test setAppendOnlyAttr function
	fd := int(tempFile.Fd())
	err = setAppendOnlyAttr(fd)

	// This might fail due to permissions in test environment, which is expected
	// The function should attempt the correct ioctl syscalls
	if err != nil {
		t.Logf("setAppendOnlyAttr() failed (expected in test environment): %v", err)

		// Verify error is properly wrapped as WriteError
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for append-only attribute setting, got %T", err)
		}
		_ = writeErr
	}

	// Test with invalid file descriptor
	err = setAppendOnlyAttr(-1)
	if err != nil {
		t.Logf("setAppendOnlyAttr() with invalid fd failed (expected): %v", err)

		// Verify error is properly wrapped as WriteError
		var writeErr *WriteError
		if _, ok := err.(*WriteError); !ok {
			t.Errorf("Expected WriteError for invalid fd, got %T", err)
		}
		_ = writeErr
	}
}
