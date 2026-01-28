package fields

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// DataRow represents a single key-value data row with UUIDv7 key and json.RawMessage value.
// DataRow follows the v1_file_format.md specification and uses baseRow for common
// file format handling. DataRows can be created manually or deserialized from bytes.
type DataRow struct {
	baseRow[*DataRowPayload] // Embedded generic foundation
}

// NewDataRow creates a new DataRow with the specified parameters.
// All fields are validated before creating the row.
//
// Parameters:
//   - rowSize: Size of each row in bytes (128-65536)
//   - startControl: Start control byte ('T' or 'R')
//   - endControl: End control bytes (TC, RE, SC, SE, etc.)
//   - key: UUIDv7 key for time ordering
//   - value: Raw JSON bytes
//
// Returns:
//   - *DataRow: A fully initialized DataRow ready for serialization
//   - error: InvalidInputError for invalid parameters
func NewDataRow(rowSize int, startControl StartControl, endControl EndControl, key uuid.UUID, value json.RawMessage) (*DataRow, error) {
	// Create payload first
	payload := &DataRowPayload{
		Key:   key,
		Value: value,
	}

	// Create DataRow
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      rowSize,
			StartControl: startControl,
			EndControl:   endControl,
			RowPayload:   payload,
		},
	}

	// Validate complete DataRow structure
	if err := dataRow.Validate(); err != nil {
		return nil, NewInvalidInputError("NewDataRow validation failed", err)
	}

	return dataRow, nil
}

// GetKey retrieves the UUIDv7 key from the DataRow.
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
func (dr *DataRow) GetKey() uuid.UUID {
	return dr.RowPayload.Key
}

// GetValue retrieves the raw JSON bytes from the DataRow.
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
func (dr *DataRow) GetValue() json.RawMessage {
	return dr.RowPayload.Value
}

// MarshalText serializes DataRow to exact byte format per v1_file_format.md specification.
// The output includes ROW_START, start_control, Base64-encoded UUID, raw JSON bytes with
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
	if err := dr.baseRow.Validate(); err != nil {
		return err
	}

	if err := validateStartControlForDataRow(dr.StartControl); err != nil {
		return err
	}

	if err := validatePayloadSize(dr.RowPayload, dr.RowSize); err != nil {
		return err
	}

	return validateEndControlForDataRow(dr.EndControl)
}

func validateStartControlForDataRow(startControl StartControl) error {
	if startControl != START_TRANSACTION && startControl != ROW_CONTINUE {
		return NewInvalidInputError(fmt.Sprintf("data row must have start_control='T' or 'R', got '%c'", startControl), nil)
	}
	return nil
}

func validatePayloadSize(payload *DataRowPayload, rowSize int) error {
	payloadSize := 24 + len(payload.Value)
	requiredSize := payloadSize + 7
	if requiredSize > rowSize {
		return NewInvalidInputError(fmt.Sprintf("payload size (%d bytes) exceeds ROW_SIZE (%d bytes); maximum payload size is %d bytes", payloadSize, rowSize, rowSize-7), nil)
	}
	return nil
}

func validateEndControlForDataRow(endControl EndControl) error {
	first := endControl[0]
	second := endControl[1]

	switch endControl {
	case TRANSACTION_COMMIT, ROW_END_CONTROL, SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK:
		return nil
	default:
		if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
			return nil
		}
		return NewInvalidInputError(fmt.Sprintf("data row must have valid end_control, got '%c%c'", first, second), nil)
	}
}

func validateStartControl(startControl StartControl) error {
	if err := startControl.Validate(); err != nil {
		return NewInvalidInputError("invalid start_control", err)
	}
	return validateStartControlForDataRow(startControl)
}

func validatePayload(payload *DataRowPayload, rowSize int) error {
	if payload == nil {
		return NewInvalidInputError("RowPayload is required", nil)
	}
	if err := payload.Validate(); err != nil {
		return err
	}
	return validatePayloadSize(payload, rowSize)
}
