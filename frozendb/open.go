package frozendb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

// headerJSON is used for JSON unmarshaling
type headerJSON struct {
	Sig     string `json:"sig"`
	Ver     int    `json:"ver"`
	RowSize int    `json:"row_size"`
	SkewMs  int    `json:"skew_ms"`
}

// UnmarshalText parses a frozenDB v1 header from 64-byte buffer
// This method automatically calls Validate() before returning
func (h *Header) UnmarshalText(headerBytes []byte) error {
	// Validate fixed 64-byte size
	if len(headerBytes) != HEADER_SIZE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("header must be exactly %d bytes, got %d", HEADER_SIZE, len(headerBytes)),
			nil,
		)
	}

	// Verify byte 63 is newline
	if headerBytes[63] != HEADER_NEWLINE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("byte 63 must be newline, got 0x%02x", headerBytes[63]),
			nil,
		)
	}

	// Find null terminator position
	nullPos := bytes.IndexByte(headerBytes, PADDING_CHAR)
	if nullPos == -1 {
		return NewCorruptDatabaseError("no null terminator found in header", nil)
	}

	// Extract JSON content (before null terminator)
	jsonContent := headerBytes[:nullPos]

	// Validate padding region (null bytes from nullPos to 62, newline at 63)
	for i := nullPos; i < 63; i++ {
		if headerBytes[i] != PADDING_CHAR {
			return NewCorruptDatabaseError(
				fmt.Sprintf("padding byte at position %d must be null, got 0x%02x", i, headerBytes[i]),
				nil,
			)
		}
	}

	// Parse JSON using standard library
	var hdr headerJSON
	if err := json.Unmarshal(jsonContent, &hdr); err != nil {
		return NewCorruptDatabaseError("failed to parse JSON header", err)
	}

	// Set Header struct fields
	h.signature = hdr.Sig
	h.version = hdr.Ver
	h.rowSize = hdr.RowSize
	h.skewMs = hdr.SkewMs

	// Validate field values (UnmarshalText automatically calls Validate())
	return h.Validate()
}

// validateHeaderFields validates header field values against specification
func (h *Header) Validate() error {
	// Validate signature
	if h.signature != HEADER_SIGNATURE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("invalid signature: expected '%s', got '%s'", HEADER_SIGNATURE, h.signature),
			nil,
		)
	}

	// Validate version
	if h.version != 1 {
		return NewCorruptDatabaseError(
			fmt.Sprintf("unsupported version: expected 1, got %d", h.version),
			nil,
		)
	}

	// Validate row size range
	if h.rowSize < MIN_ROW_SIZE || h.rowSize > MAX_ROW_SIZE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("row_size must be between %d and %d, got %d", MIN_ROW_SIZE, MAX_ROW_SIZE, h.rowSize),
			nil,
		)
	}

	// Validate skew_ms range
	if h.skewMs < 0 || h.skewMs > MAX_SKEW_MS {
		return NewCorruptDatabaseError(
			fmt.Sprintf("skew_ms must be between 0 and %d, got %d", MAX_SKEW_MS, h.skewMs),
			nil,
		)
	}

	return nil
}

// acquireFileLock acquires a file lock with specified mode
// mode: syscall.LOCK_SH for shared (read), syscall.LOCK_EX for exclusive (write)
// Returns error on lock acquisition failure
func acquireFileLock(file *os.File, mode int, blocking bool) error {
	lockMode := mode
	if !blocking {
		lockMode |= syscall.LOCK_NB // Non-blocking
	}

	err := syscall.Flock(int(file.Fd()), lockMode)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return NewWriteError("another process has the database locked", err)
		}
		return NewWriteError("failed to acquire file lock", err)
	}

	return nil
}

// releaseFileLock releases the file lock
func releaseFileLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err != nil {
		return NewWriteError("failed to release file lock", err)
	}
	return nil
}
