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

// parseHeader validates and parses a frozenDB v1 header from 64-byte buffer
// Returns Header struct on success, CorruptDatabaseError on validation failure
func parseHeader(headerBytes []byte) (*Header, error) {
	// Validate fixed 64-byte size
	if len(headerBytes) != HeaderSize {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("header must be exactly %d bytes, got %d", HeaderSize, len(headerBytes)),
			nil,
		)
	}

	// Verify byte 63 is newline
	if headerBytes[63] != HeaderNewline {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("byte 63 must be newline, got 0x%02x", headerBytes[63]),
			nil,
		)
	}

	// Find null terminator position
	nullPos := bytes.IndexByte(headerBytes, PaddingChar)
	if nullPos == -1 {
		return nil, NewCorruptDatabaseError("no null terminator found in header", nil)
	}

	// Extract JSON content (before null terminator)
	jsonContent := headerBytes[:nullPos]

	// Validate padding region (null bytes from nullPos to 62, newline at 63)
	for i := nullPos; i < 63; i++ {
		if headerBytes[i] != PaddingChar {
			return nil, NewCorruptDatabaseError(
				fmt.Sprintf("padding byte at position %d must be null, got 0x%02x", i, headerBytes[i]),
				nil,
			)
		}
	}

	// Parse JSON using standard library
	var hdr headerJSON
	if err := json.Unmarshal(jsonContent, &hdr); err != nil {
		return nil, NewCorruptDatabaseError("failed to parse JSON header", err)
	}

	// Convert to Header struct
	header := &Header{
		Signature: hdr.Sig,
		Version:   hdr.Ver,
		RowSize:   hdr.RowSize,
		SkewMs:    hdr.SkewMs,
	}

	// Validate field values
	if err := validateHeaderFields(header); err != nil {
		return nil, err // Already a CorruptDatabaseError
	}

	return header, nil
}

// validateHeaderFields validates header field values against specification
func validateHeaderFields(header *Header) error {
	// Validate signature
	if header.Signature != HeaderSignature {
		return NewCorruptDatabaseError(
			fmt.Sprintf("invalid signature: expected '%s', got '%s'", HeaderSignature, header.Signature),
			nil,
		)
	}

	// Validate version
	if header.Version != 1 {
		return NewCorruptDatabaseError(
			fmt.Sprintf("unsupported version: expected 1, got %d", header.Version),
			nil,
		)
	}

	// Validate row size range
	if header.RowSize < MinRowSize || header.RowSize > MaxRowSize {
		return NewCorruptDatabaseError(
			fmt.Sprintf("row_size must be between %d and %d, got %d", MinRowSize, MaxRowSize, header.RowSize),
			nil,
		)
	}

	// Validate skew_ms range
	if header.SkewMs < 0 || header.SkewMs > MaxSkewMs {
		return NewCorruptDatabaseError(
			fmt.Sprintf("skew_ms must be between 0 and %d, got %d", MaxSkewMs, header.SkewMs),
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
