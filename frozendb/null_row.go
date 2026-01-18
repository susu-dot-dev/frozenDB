package frozendb

import (
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// NullRowPayload contains the UUID for a NullRow.
// The Key must always be uuid.Nil for null rows.
type NullRowPayload struct {
	Key uuid.UUID // Always uuid.Nil for null rows
}

// MarshalText serializes NullRowPayload to bytes: Base64-encoded uuid.Nil (24 bytes)
func (nrp *NullRowPayload) MarshalText() ([]byte, error) {
	if nrp == nil {
		return nil, NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	// Encode UUID to Base64 (16 bytes -> 24 bytes with "==" padding)
	uuidBytes := nrp.Key[:]
	uuidBase64 := base64.StdEncoding.EncodeToString(uuidBytes)
	if len(uuidBase64) != 24 {
		return nil, NewInvalidInputError(fmt.Sprintf("Base64 UUID encoding should produce 24 characters, got %d", len(uuidBase64)), nil)
	}

	return []byte(uuidBase64), nil
}

// UnmarshalText deserializes NullRowPayload from bytes: Base64-encoded uuid.Nil (24 bytes)
func (nrp *NullRowPayload) UnmarshalText(text []byte) error {
	if nrp == nil {
		return NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	if len(text) != 24 {
		return NewInvalidInputError(fmt.Sprintf("NullRowPayload must be exactly 24 bytes for UUID, got %d", len(text)), nil)
	}

	// Decode Base64 UUID
	uuidBase64 := string(text)
	decoded, err := base64.StdEncoding.DecodeString(uuidBase64)
	if err != nil {
		return NewInvalidInputError("invalid Base64 encoding for UUID", err)
	}
	if len(decoded) != 16 {
		return NewInvalidInputError(fmt.Sprintf("decoded UUID must be 16 bytes, got %d", len(decoded)), nil)
	}

	// Parse UUID from bytes
	key, err := uuid.FromBytes(decoded)
	if err != nil {
		return NewInvalidInputError("failed to parse UUID from bytes", err)
	}

	nrp.Key = key
	return nil
}

// Validate validates the NullRowPayload.
// The Key must be uuid.Nil for null rows.
func (nrp *NullRowPayload) Validate() error {
	if nrp == nil {
		return NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	if nrp.Key != uuid.Nil {
		return NewInvalidInputError(fmt.Sprintf("NullRowPayload.Key must be uuid.Nil, got %s", nrp.Key), nil)
	}

	return nil
}

// NullRow represents a null operation row in the frozenDB file format.
// NullRows are single-row transactions that mark empty transaction slots.
// Per the v1_file_format.md specification:
// - start_control: Always 'T' (transaction begin)
// - uuid: Always uuid.Nil, Base64 encoded as "AAAAAAAAAAAAAAAAAAAAAA=="
// - value: No user value (immediate padding after UUID)
// - end_control: Always 'NR' (null row)
type NullRow struct {
	baseRow[*NullRowPayload] // Embedded generic foundation
}

// Validate performs validation of NullRow-specific properties.
// This method assumes child structs (NullRowPayload) are already valid per 004-struct-validation FR-006.
// It validates context-specific requirements: start_control='T', end_control='NR', RowPayload non-nil.
// UUID validation (Key == uuid.Nil) is the responsibility of NullRowPayload.Validate().
// This method is idempotent and can be called multiple times with the same result.
// Returns an error if validation fails.
func (nr *NullRow) Validate() error {
	// Validate start_control is 'T' for null rows (context-specific validation)
	if nr.StartControl != START_TRANSACTION {
		return NewInvalidInputError(fmt.Sprintf("null row must have start_control='T', got '%c'", nr.StartControl), nil)
	}

	// Validate end_control is 'NR' for null rows (context-specific validation)
	if nr.EndControl != NULL_ROW_CONTROL {
		return NewInvalidInputError(fmt.Sprintf("null row must have end_control='NR', got '%s'", nr.EndControl.String()), nil)
	}

	// Validate UUID is uuid.Nil for null rows (context-specific validation per FR-009)
	if nr.RowPayload == nil {
		return NewInvalidInputError("null row must have non-nil RowPayload", nil)
	}

	return nil
}

// MarshalText serializes NullRow to exact byte format per v1_file_format.md specification.
// The output includes ROW_START, start_control, Base64-encoded uuid.Nil with
// NULL_BYTE padding, end_control, parity bytes, and ROW_END.
// Returns an error if serialization fails.
func (nr *NullRow) MarshalText() ([]byte, error) {
	// Validate before marshaling
	if err := nr.Validate(); err != nil {
		return nil, err
	}
	return nr.baseRow.MarshalText()
}

// UnmarshalText deserializes NullRow from byte array with comprehensive validation.
// Validates sentinels, control characters, Base64 UUID encoding, parity bytes, and
// payload structure. The Header must be set before calling this method.
// Returns CorruptDatabaseError wrapping validation errors if input data format is invalid.
func (nr *NullRow) UnmarshalText(text []byte) error {
	// Unmarshal using baseRow (Header must be set - programmer error if nil)
	if err := nr.baseRow.UnmarshalText(text); err != nil {
		return NewCorruptDatabaseError("failed to unmarshal null row", err)
	}

	// Validate NullRow-specific properties
	if err := nr.Validate(); err != nil {
		return NewCorruptDatabaseError("null row validation failed", err)
	}

	return nil
}

// GetKey retrieves the UUID key from the NullRow.
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
// For NullRows, this will always return uuid.Nil.
func (nr *NullRow) GetKey() uuid.UUID {
	return nr.RowPayload.Key
}
