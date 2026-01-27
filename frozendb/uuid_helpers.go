package frozendb

import (
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// Core UUID Functions

// ValidateUUIDv7 validates that a UUID is version 7 and RFC 4122 variant.
// Returns nil if valid, or an InvalidInputError if the UUID is invalid.
func ValidateUUIDv7(u uuid.UUID) *InvalidInputError {
	// Check for zero UUID
	if u == uuid.Nil {
		return NewInvalidInputError("UUID cannot be zero/Nil", nil)
	}

	// Validate variant is RFC 4122
	if u.Variant() != uuid.RFC4122 {
		return NewInvalidInputError(fmt.Sprintf("UUID variant must be RFC 4122, got %v", u.Variant()), nil)
	}

	// Validate version is 7
	if u.Version() != 7 {
		return NewInvalidInputError(fmt.Sprintf("UUID version must be 7, got %d", u.Version()), nil)
	}

	return nil
}

// ExtractUUIDv7Timestamp extracts 48-bit millisecond timestamp from a UUIDv7.
// The timestamp is stored in the first 6 bytes (48 bits) of UUID.
func ExtractUUIDv7Timestamp(u uuid.UUID) int64 {
	// UUIDv7 format: first 48 bits are the timestamp in milliseconds
	// Bytes 0-5 contain timestamp, big-endian
	return int64(u[0])<<40 | int64(u[1])<<32 | int64(u[2])<<24 |
		int64(u[3])<<16 | int64(u[4])<<8 | int64(u[5])
}

// NewUUIDv7 creates new UUIDv7 with current timestamp.
// Returns error if UUID generation fails.
func NewUUIDv7() (uuid.UUID, error) {
	return uuid.NewV7()
}

// MustNewUUIDv7 creates new UUIDv7 with current timestamp, panics on failure.
// For use in tests and initialization where failure is not acceptable.
func MustNewUUIDv7() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("Failed to generate UUIDv7: %v", err))
	}
	return u
}

// NullRow-Specific UUID Functions

// CreateNullRowUUID creates UUIDv7 with specified timestamp and zeroed random components for NullRows.
// The resulting UUID is deterministic for the same timestamp.
func CreateNullRowUUID(maxTimestamp int64) uuid.UUID {
	var u uuid.UUID

	// Set first 6 bytes to timestamp (big-endian)
	u[0] = byte(maxTimestamp >> 40)
	u[1] = byte(maxTimestamp >> 32)
	u[2] = byte(maxTimestamp >> 24)
	u[3] = byte(maxTimestamp >> 16)
	u[4] = byte(maxTimestamp >> 8)
	u[5] = byte(maxTimestamp)

	// Set version bits to 7 (bits 12-15 of u[6])
	// UUIDv7 version: set bits 12-15 to 0111
	u[6] = 0x70 // 0111 0000

	// Set variant bits to RFC 4122 (bits 6-7 of u[8])
	// RFC 4122 variant: set bits 6-7 to 10
	u[8] = 0x80 // 1000 0000

	// Remaining random bytes (7,9-15) stay zeroed for deterministic NullRows

	return u
}

// ValidateNullRowUUID validates UUID for NullRow usage with expected timestamp.
// Returns nil if valid, or InvalidInputError with details if invalid.
func ValidateNullRowUUID(u uuid.UUID, maxTimestamp int64) *InvalidInputError {
	// First validate it's a proper UUIDv7
	if err := ValidateUUIDv7(u); err != nil {
		return NewInvalidInputError("NullRow UUID must be valid UUIDv7", err)
	}

	// Extract timestamp from UUID
	uuidTimestamp := ExtractUUIDv7Timestamp(u)

	// Validate timestamp matches expected maxTimestamp
	if uuidTimestamp != maxTimestamp {
		return NewInvalidInputError(fmt.Sprintf("NullRow UUID timestamp %d does not match expected maxTimestamp %d", uuidTimestamp, maxTimestamp), nil)
	}

	// Validate random components are zeroed (bytes 6-15 should have specific pattern)
	// For NullRows: bytes 6 should have version=7, byte 8 should have RFC4122 variant
	// and all other random bits should be zero
	if u[6] != 0x70 {
		return NewInvalidInputError(fmt.Sprintf("NullRow UUID version byte should be 0x70, got 0x%02X", u[6]), nil)
	}

	if u[8] != 0x80 {
		return NewInvalidInputError(fmt.Sprintf("NullRow UUID variant byte should be 0x80, got 0x%02X", u[8]), nil)
	}

	// Check remaining bytes are zeroed (except version and variant bits)
	for i := 0; i < 16; i++ {
		var expected byte
		switch i {
		case 0, 1, 2, 3, 4, 5:
			// Timestamp bytes - extract from maxTimestamp
			shift := (5 - i) * 8
			expected = byte(maxTimestamp >> shift)
		case 6:
			expected = 0x70 // Version 7 with zeroed random bits
		case 7:
			expected = 0x00 // Random bits zeroed
		case 8:
			expected = 0x80 // RFC 4122 variant with zeroed random bits
		case 9, 10, 11, 12, 13, 14, 15:
			expected = 0x00 // Random bits zeroed
		}

		if u[i] != expected {
			return NewInvalidInputError(fmt.Sprintf("NullRow UUID byte %d should be 0x%02X, got 0x%02X", i, expected, u[i]), nil)
		}
	}

	return nil
}

// IsNullRowUUID checks if UUID follows NullRow pattern (timestamp present, random components zeroed).
// Returns true if UUID matches NullRow pattern.
func IsNullRowUUID(u uuid.UUID) bool {
	// Must be valid UUIDv7
	if ValidateUUIDv7(u) != nil {
		return false
	}

	// Check for NullRow-specific pattern
	// Version byte should be 0x70 (version 7 with zeroed random bits)
	// Variant byte should be 0x80 (RFC 4122 with zeroed random bits)
	// Random bytes should be zeroed
	return u[6] == 0x70 && u[8] == 0x80 &&
		u[7] == 0x00 && u[9] == 0x00 && u[10] == 0x00 && u[11] == 0x00 &&
		u[12] == 0x00 && u[13] == 0x00 && u[14] == 0x00 && u[15] == 0x00
}

// UUID Base64 Utilities

// EncodeUUIDBase64 encodes UUID to Base64 with validation.
// Returns exactly 24 bytes and error if encoding fails.
func EncodeUUIDBase64(u uuid.UUID) ([]byte, error) {
	// Validate UUID is valid UUIDv7 before encoding
	if err := ValidateUUIDv7(u); err != nil {
		return nil, NewInvalidInputError("cannot encode invalid UUIDv7 to Base64", err)
	}

	// Encode UUID to Base64 (16 bytes -> 24 bytes with "==" padding)
	uuidBytes := u[:]
	base64Str := base64.StdEncoding.EncodeToString(uuidBytes)

	// Validate length
	if len(base64Str) != 24 {
		return nil, NewInvalidInputError(fmt.Sprintf("Base64 UUID encoding should produce 24 characters, got %d", len(base64Str)), nil)
	}

	return []byte(base64Str), nil
}

// DecodeUUIDBase64 decodes UUID from Base64 with validation.
// Input must be exactly 24 bytes, returns UUID and error if decoding fails.
func DecodeUUIDBase64(data []byte) (uuid.UUID, error) {
	// Validate input length
	if err := ValidateBase64UUIDLength(data); err != nil {
		return uuid.Nil, err
	}

	// Decode Base64
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return uuid.Nil, NewInvalidInputError("invalid Base64 encoding for UUID", err)
	}

	// Validate decoded length
	if len(decoded) != 16 {
		return uuid.Nil, NewInvalidInputError(fmt.Sprintf("decoded UUID must be 16 bytes, got %d", len(decoded)), nil)
	}

	// Parse UUID from bytes
	u, err := uuid.FromBytes(decoded)
	if err != nil {
		return uuid.Nil, NewInvalidInputError("failed to parse UUID from bytes", err)
	}

	// Validate resulting UUID is valid UUIDv7
	if err := ValidateUUIDv7(u); err != nil {
		return uuid.Nil, NewInvalidInputError("decoded UUID is not valid UUIDv7", err)
	}

	return u, nil
}

// ValidateBase64UUIDLength validates Base64 UUID encoding length.
// Returns nil if length is 24, error otherwise.
func ValidateBase64UUIDLength(data []byte) *InvalidInputError {
	if len(data) != 24 {
		return NewInvalidInputError(fmt.Sprintf("Base64 UUID must be exactly 24 bytes, got %d", len(data)), nil)
	}
	return nil
}
