package fields

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// DataRowPayload contains the key-value data for a DataRow.
// The Key must be a UUIDv7 for proper time ordering, and Value is a json.RawMessage
// that stores raw JSON bytes without validation at this layer.
type DataRowPayload struct {
	Key   uuid.UUID       // UUIDv7 key for time ordering
	Value json.RawMessage // Raw JSON bytes (no syntax validation at this layer)
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
	value := json.RawMessage(text[24:])

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

	// FR-009: Reject UUIDs where the non-timestamp part (bytes 7, 9-15) are all zeros
	// This pattern indicates a NullRow UUID, which is invalid for DataRows
	if IsNullRowUUID(drp.Key) {
		return NewInvalidInputError("DataRow UUID cannot be a NullRow UUID (non-timestamp parts bytes 7, 9-15 must not be all zeros)", nil)
	}

	// Validate value is non-empty
	if len(drp.Value) == 0 {
		return NewInvalidInputError("DataRowPayload.Value cannot be empty", nil)
	}

	return nil
}
