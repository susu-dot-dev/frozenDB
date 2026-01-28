package fields

import (
	"fmt"

	"github.com/google/uuid"
)

// NullRowPayload contains the UUID for a NullRow.
// The Key must be a valid NullRow UUID (UUIDv7 with timestamp and zeroed random components).
type NullRowPayload struct {
	Key uuid.UUID // NullRow UUID (UUIDv7 with timestamp and zeroed random components)
}

// MarshalText serializes NullRowPayload to bytes: Base64-encoded NullRow UUID (24 bytes)
func (nrp *NullRowPayload) MarshalText() ([]byte, error) {
	if nrp == nil {
		return nil, NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	// Encode UUID to Base64 using centralized function
	return EncodeUUIDBase64(nrp.Key)
}

// UnmarshalText deserializes NullRowPayload from bytes: Base64-encoded NullRow UUID (24 bytes)
func (nrp *NullRowPayload) UnmarshalText(text []byte) error {
	if nrp == nil {
		return NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	// Decode UUID from Base64 using centralized function
	key, err := DecodeUUIDBase64(text)
	if err != nil {
		return err
	}

	nrp.Key = key
	return nil
}

// Validate validates the NullRowPayload.
// The Key must be a valid NullRow UUID (UUIDv7 with zeroed random components).
func (nrp *NullRowPayload) Validate() error {
	if nrp == nil {
		return NewInvalidInputError("NullRowPayload cannot be nil", nil)
	}

	// Validate UUID follows NullRow pattern (valid UUIDv7 with zeroed random components)
	if !IsNullRowUUID(nrp.Key) {
		return NewInvalidInputError(fmt.Sprintf("NullRowPayload.Key must be valid NullRow UUID (UUIDv7 with zeroed random components), got %s", nrp.Key), nil)
	}

	return nil
}

// NewNullRow creates a new NullRow with timestamp-aware UUID for the specified maxTimestamp.
// Parameters:
//   - rowSize: The fixed row size from database header (must be 128-65536)
//   - maxTimestamp: The maximum timestamp of database at insertion time (must be non-negative)
//
// Returns:
//   - *NullRow: A fully initialized NullRow ready for serialization
//   - error: InvalidInputError for invalid parameters
func NewNullRow(rowSize int, maxTimestamp int64) (*NullRow, error) {
	// Validate rowSize parameter
	if rowSize < 128 || rowSize > 65536 {
		return nil, NewInvalidInputError(fmt.Sprintf("rowSize must be between 128 and 65536, got %d", rowSize), nil)
	}

	// Validate maxTimestamp parameter
	if maxTimestamp < 0 {
		return nil, NewInvalidInputError(fmt.Sprintf("maxTimestamp must be non-negative, got %d", maxTimestamp), nil)
	}

	// Create UUIDv7 with timestamp=maxTimestamp and zeroed random components
	uuid := CreateNullRowUUID(maxTimestamp)

	// Create NullRow with proper control characters and UUID
	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			RowSize:      rowSize,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid},
		},
	}

	// Validate complete NullRow structure
	if err := nullRow.Validate(); err != nil {
		return nil, NewInvalidInputError("NewNullRow validation failed", err)
	}

	return nullRow, nil
}

// NullRow represents a null operation row in the frozenDB file format.
// NullRows are single-row transactions that mark empty transaction slots.
// Per the v1_file_format.md specification:
// - start_control: Always 'T' (transaction begin)
// - uuid: UUIDv7 with timestamp matching maxTimestamp and zeroed random components, Base64 encoded
// - value: No user value (immediate padding after UUID)
// - end_control: Always 'NR' (null row)
type NullRow struct {
	baseRow[*NullRowPayload] // Embedded generic foundation
}

// Validate performs validation of NullRow-specific properties.
// This method assumes child structs (NullRowPayload) are already valid per 004-struct-validation FR-006.
// It validates context-specific requirements: start_control='T', end_control='NR', RowPayload non-nil.
// UUID validation (NullRow UUID format) is the responsibility of NullRowPayload.Validate().
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
// The output includes ROW_START, start_control, Base64-encoded NullRow UUID with
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
// For NullRows, this returns the timestamp-aware NullRow UUID.
func (nr *NullRow) GetKey() uuid.UUID {
	return nr.RowPayload.Key
}
