package frozendb

import (
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

// DataRowPayload contains the key-value data for a DataRow.
// The Key must be a UUIDv7 for proper time ordering, and Value is a JSON string
// (no syntax validation is performed at this layer).
type DataRowPayload struct {
	Key   uuid.UUID // UUIDv7 key for time ordering
	Value string    // JSON string value (no syntax validation at this layer)
}

// MarshalText serializes DataRowPayload to bytes: Base64-encoded UUID (24 bytes) + JSON value
func (drp *DataRowPayload) MarshalText() ([]byte, error) {
	if drp == nil {
		return nil, NewInvalidInputError("DataRowPayload cannot be nil", nil)
	}

	// Encode UUID to Base64 (16 bytes -> 24 bytes with "=" padding)
	uuidBytes := drp.Key[:]
	uuidBase64 := base64.StdEncoding.EncodeToString(uuidBytes)
	if len(uuidBase64) != 24 {
		return nil, NewInvalidInputError(fmt.Sprintf("Base64 UUID encoding should produce 24 characters, got %d", len(uuidBase64)), nil)
	}

	// Combine: Base64 UUID (24 bytes) + JSON value
	result := make([]byte, 24+len(drp.Value))
	copy(result[0:24], []byte(uuidBase64))
	copy(result[24:], []byte(drp.Value))

	return result, nil
}

// UnmarshalText deserializes DataRowPayload from bytes: Base64-encoded UUID (24 bytes) + JSON value
func (drp *DataRowPayload) UnmarshalText(text []byte) error {
	if drp == nil {
		return NewInvalidInputError("DataRowPayload cannot be nil", nil)
	}

	if len(text) < 24 {
		return NewInvalidInputError(fmt.Sprintf("DataRowPayload must be at least 24 bytes for UUID, got %d", len(text)), nil)
	}

	// Extract Base64 UUID (first 24 bytes)
	uuidBase64 := string(text[0:24])
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

	// Extract JSON value (remaining bytes)
	value := string(text[24:])

	drp.Key = key
	drp.Value = value

	return nil
}

// Validate validates the DataRowPayload
func (drp *DataRowPayload) Validate() error {
	if drp == nil {
		return NewInvalidInputError("DataRowPayload cannot be nil", nil)
	}

	// Validate UUIDv7
	if err := ValidateUUIDv7(drp.Key); err != nil {
		return err
	}

	// Validate value is non-empty
	if drp.Value == "" {
		return NewInvalidInputError("DataRowPayload.Value cannot be empty", nil)
	}

	return nil
}

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

// DataRow represents a single key-value data row with UUIDv7 key and JSON string value.
// DataRow follows the v1_file_format.md specification and uses baseRow for common
// file format handling. DataRows can be created manually or deserialized from bytes.
type DataRow struct {
	baseRow[*DataRowPayload] // Embedded generic foundation
}

// GetKey retrieves the UUIDv7 key from the DataRow.
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
func (dr *DataRow) GetKey() uuid.UUID {
	return dr.RowPayload.Key
}

// GetValue retrieves the JSON string value from the DataRow.
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
func (dr *DataRow) GetValue() string {
	return dr.RowPayload.Value
}

// MarshalText serializes DataRow to exact byte format per v1_file_format.md specification.
// The output includes ROW_START, start_control, Base64-encoded UUID, JSON value with
// NULL_BYTE padding, end_control, parity bytes, and ROW_END.
// Returns an error if serialization fails.
func (dr *DataRow) MarshalText() ([]byte, error) {
	return dr.baseRow.MarshalText()
}

// UnmarshalText deserializes DataRow from byte array with comprehensive validation.
// Validates sentinels, control characters, Base64 UUID encoding, parity bytes, and
// payload structure. The Header must be set before calling this method.
// Returns an error if deserialization or validation fails.
func (dr *DataRow) UnmarshalText(text []byte) error {
	// Unmarshal using baseRow (Header must be set - programmer error if nil)
	// This will parse StartControl and EndControl from the text
	// baseRow.UnmarshalText() will call baseRow.Validate() internally
	if err := dr.baseRow.UnmarshalText(text); err != nil {
		return err
	}

	// Validate DataRow-specific properties (StartControl='T'/'R', EndControl valid for DataRow, payload not nil)
	return dr.Validate()
}

// Validate performs validation of DataRow-specific properties.
// This method calls baseRow.Validate() first to validate structure and payload,
// then validates that start_control is 'T' or 'R' and end_control is valid for data rows.
// This method is idempotent and can be called multiple times with the same result.
// Returns an error if validation fails.
func (dr *DataRow) Validate() error {
	// First validate baseRow structure (header, controls, payload not nil, payload validation)
	if err := dr.baseRow.Validate(); err != nil {
		return err
	}

	// Validate that payload size does not exceed ROW_SIZE
	// Fixed overhead: ROW_START(1) + start_control(1) + end_control(2) + parity(2) + ROW_END(1) = 7 bytes
	// Payload: Base64 UUID (24 bytes) + JSON value (len(Value) bytes)
	payloadSize := 24 + len(dr.RowPayload.Value)
	rowSize := dr.Header.GetRowSize()
	requiredSize := payloadSize + 7
	if requiredSize > rowSize {
		return NewInvalidInputError(fmt.Sprintf("payload size (%d bytes) exceeds ROW_SIZE (%d bytes); maximum payload size is %d bytes", payloadSize, rowSize, rowSize-7), nil)
	}

	// Validate start_control is 'T' or 'R' for data rows (context-specific validation)
	if dr.StartControl != START_TRANSACTION && dr.StartControl != ROW_CONTINUE {
		return NewInvalidInputError(fmt.Sprintf("data row must have start_control='T' or 'R', got '%c'", dr.StartControl), nil)
	}

	// Validate end_control is valid for data rows
	// Valid end controls: TC, RE, SC, SE, R0-R9, S0-S9
	first := dr.EndControl[0]
	second := dr.EndControl[1]

	// Check exact matches
	switch dr.EndControl {
	case TRANSACTION_COMMIT, ROW_END_CONTROL, SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK:
		// Valid
	default:
		// Check for R0-R9 and S0-S9 patterns
		if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
			// Valid rollback pattern
		} else {
			return NewInvalidInputError(fmt.Sprintf("data row must have valid end_control, got '%c%c'", first, second), nil)
		}
	}

	return nil
}
