package fields

import (
	"bytes"
	"encoding"
	"fmt"
	"reflect"

	"github.com/susu-dot-dev/frozenDB/pkg/types"
)

// Sentinel byte constants
const (
	ROW_START = 0x1F // Unit separator (U+001F)
	ROW_END   = 0x0A // Newline (U+000A)
	NULL_BYTE = 0x00 // Null character for padding
)

// Type aliases for shared control types
type StartControl = types.StartControl
type EndControl = types.EndControl

// Re-export constants for convenience
const (
	START_TRANSACTION = types.START_TRANSACTION
	ROW_CONTINUE      = types.ROW_CONTINUE
	CHECKSUM_ROW      = types.CHECKSUM_ROW
)

var (
	TRANSACTION_COMMIT   = types.TRANSACTION_COMMIT
	ROW_END_CONTROL      = types.ROW_END_CONTROL
	SAVEPOINT_COMMIT     = types.SAVEPOINT_COMMIT
	SAVEPOINT_CONTINUE   = types.SAVEPOINT_CONTINUE
	FULL_ROLLBACK        = types.FULL_ROLLBACK
	CHECKSUM_ROW_CONTROL = types.CHECKSUM_ROW_CONTROL
	NULL_ROW_CONTROL     = types.NULL_ROW_CONTROL
)

// Re-export error constructors
var NewInvalidInputError = types.NewInvalidInputError
var NewCorruptDatabaseError = types.NewCorruptDatabaseError

// Re-export error types for type assertions in tests
type InvalidInputError = types.InvalidInputError
type CorruptDatabaseError = types.CorruptDatabaseError
type InvalidActionError = types.InvalidActionError

// Validator defines the interface for types that can validate themselves
type Validator interface {
	Validate() error
}

// RowPayload defines the interface for row-specific payload data
type RowPayload interface {
	encoding.TextMarshaler
	encoding.TextUnmarshaler
	Validator
}

// baseRow provides the generic foundation for all frozenDB row types.
// T must implement RowPayload. The RowPayload field stores T directly.
type baseRow[T RowPayload] struct {
	RowSize      int          // Row size in bytes
	StartControl StartControl // Single byte control character (position 1)
	EndControl   EndControl   // Two-byte end control sequence (positions N-5,N-4)
	RowPayload   T            // Typed payload data, validated after structural checks
}

// PaddingLength calculates the required null byte padding length
// Fixed overhead: ROW_START(1) + start_control(1) + end_control(2) + parity(2) + ROW_END(1) = 7
// Takes the marshaled payload bytes to determine actual payload size
func (br *baseRow[T]) PaddingLength(payloadBytes []byte) int {
	payloadSize := len(payloadBytes)
	return br.RowSize - 7 - payloadSize
}

// BuildRowStartAndControl builds just the ROW_START and start_control bytes (positions [0] and [1])
func (br *baseRow[T]) BuildRowStartAndControl() ([]byte, error) {
	startControlBytes, err := br.StartControl.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal start_control", err)
	}

	result := make([]byte, 2)
	result[0] = ROW_START
	result[1] = startControlBytes[0]

	return result, nil
}

// BuildRowStartControlAndPayload builds bytes from ROW_START through padding (positions [0] through [rowSize-6])
// This includes: ROW_START, start_control, payload, NULL_BYTE padding
func (br *baseRow[T]) BuildRowStartControlAndPayload(payloadBytes []byte) ([]byte, error) {
	startAndControl, err := br.BuildRowStartAndControl()
	if err != nil {
		return nil, err
	}

	paddingLen := br.PaddingLength(payloadBytes)
	if paddingLen < 0 {
		return nil, NewInvalidInputError("row_size too small for required fields", nil)
	}

	// Build bytes [0] through [rowSize-6] inclusive = rowSize-5 bytes total
	result := make([]byte, br.RowSize-5)

	// Copy ROW_START and start_control
	copy(result, startAndControl)

	// Positions [2..2+payloadSize-1]: payload
	payloadStart := 2
	payloadEnd := payloadStart + len(payloadBytes)
	copy(result[payloadStart:payloadEnd], payloadBytes)

	// Positions [payloadEnd..N-6]: NULL_BYTE padding
	for i := payloadEnd; i < br.RowSize-5; i++ {
		result[i] = NULL_BYTE
	}

	return result, nil
}

// buildRowBytesUpToParity builds row bytes from ROW_START through end_control (positions [0] through [rowSize-4] inclusive)
// This includes: ROW_START, start_control, payload, padding, end_control
// Returns the bytes and an error if marshaling fails
func (br *baseRow[T]) buildRowBytesUpToParity() ([]byte, error) {
	// Marshal payload (T implements RowPayload)
	payloadBytes, err := br.RowPayload.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal row payload", err)
	}

	// Build bytes [0] through [rowSize-6] inclusive
	startControlAndPayload, err := br.BuildRowStartControlAndPayload(payloadBytes)
	if err != nil {
		return nil, err
	}

	// Build full row up to parity: startControlAndPayload + end_control
	rowBytes := make([]byte, br.RowSize-3)
	copy(rowBytes, startControlAndPayload)

	// Positions [N-5..N-4]: end_control
	endControlBytes, err := br.EndControl.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal end_control", err)
	}
	copy(rowBytes[br.RowSize-5:br.RowSize-3], endControlBytes)

	return rowBytes, nil
}

// GetParity calculates LRC parity bytes using XOR algorithm on bytes [0] through [row_size-4] (inclusive)
// Serializes the row from ROW_START through end_control, but not including parity or ROW_END
// Returns exactly 2 bytes (uppercase hex characters) and an error if marshaling fails
func (br *baseRow[T]) GetParity() ([2]byte, error) {
	rowBytes, err := br.buildRowBytesUpToParity()
	if err != nil {
		return [2]byte{}, err
	}

	// XOR all bytes from [0] through [rowSize-4] (inclusive)
	// rowBytes contains positions [0] through [rowSize-4], so XOR all of them
	var xor byte = 0
	for i := 0; i < len(rowBytes); i++ {
		xor ^= rowBytes[i]
	}

	// Encode XOR result as 2-character uppercase hex string (e.g., 0xA3 → "A3")
	hexStr := fmt.Sprintf("%02X", xor)
	if len(hexStr) != 2 {
		return [2]byte{}, NewInvalidInputError("parity hex encoding failed", nil)
	}

	return [2]byte{hexStr[0], hexStr[1]}, nil
}

// MarshalText serializes baseRow to exact byte format per v1_file_format.md
func (br *baseRow[T]) MarshalText() ([]byte, error) {
	// Build row bytes up to but not including parity
	rowBytesUpToParity, err := br.buildRowBytesUpToParity()
	if err != nil {
		return nil, err
	}

	// Build full row bytes
	rowBytes := make([]byte, br.RowSize)
	copy(rowBytes, rowBytesUpToParity)

	// Positions [N-3..N-2]: parity_bytes (calculated after all other bytes are set)
	parity, err := br.GetParity()
	if err != nil {
		return nil, err
	}
	copy(rowBytes[br.RowSize-3:br.RowSize-1], parity[:])

	// Position [N-1]: ROW_END
	rowBytes[br.RowSize-1] = ROW_END

	return rowBytes, nil
}

// UnmarshalText deserializes baseRow from byte array with validation
func (br *baseRow[T]) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return NewInvalidInputError("row bytes cannot be empty", nil)
	}

	rowSize := len(text)
	// Set RowSize early so it's available for GetParity() and other methods
	br.RowSize = rowSize

	// Step 1: Validate ROW_START at position [0]
	if text[0] != ROW_START {
		return NewInvalidInputError(fmt.Sprintf("invalid ROW_START: expected 0x%02X, got 0x%02X", ROW_START, text[0]), nil)
	}

	// Step 1: Parse and validate start_control at position [1]
	// UnmarshalText() will call Validate() internally
	if err := br.StartControl.UnmarshalText(text[1:2]); err != nil {
		return NewInvalidInputError("invalid start_control", err)
	}

	// Step 2: Find the first null byte starting from position [2] to identify padding start
	// Search from position [2] up to [N-6] (before end_control)
	searchRange := text[2 : rowSize-6]
	firstNullIndex := bytes.IndexByte(searchRange, NULL_BYTE)
	if firstNullIndex == -1 {
		return NewInvalidInputError("no null byte found to mark padding start", nil)
	}
	// Adjust index to account for starting at position 2
	firstNullIndex += 2

	// Step 3: Validate that bytes [2..firstNullIndex) are the valid payload
	payloadStart := 2
	payloadEnd := firstNullIndex
	payloadBytes := text[payloadStart:payloadEnd]

	// Create a new instance of T and unmarshal into it
	// Handle pointer types specially: if T is a pointer type, create a new instance of the underlying type
	var payload T
	tType := reflect.TypeOf(payload)
	if tType.Kind() == reflect.Ptr {
		// T is a pointer type, create a new instance of the underlying type
		elemType := tType.Elem()
		newElem := reflect.New(elemType)
		payload = newElem.Interface().(T)
	}
	if err := payload.UnmarshalText(payloadBytes); err != nil {
		return NewInvalidInputError("failed to unmarshal payload", err)
	}

	// Validate the unmarshaled payload
	if err := payload.Validate(); err != nil {
		return NewInvalidInputError("payload validation failed", err)
	}

	// Assign the payload
	br.RowPayload = payload

	// Step 4: Validate that bytes [firstNullIndex..N-6] are all null (padding)
	for i := firstNullIndex; i < rowSize-6; i++ {
		if text[i] != NULL_BYTE {
			return NewInvalidInputError(fmt.Sprintf("invalid padding byte at position %d: expected NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, text[i]), nil)
		}
	}

	// Step 5: Validate end_control at positions [N-5..N-4]
	// UnmarshalText() will call Validate() internally
	if err := br.EndControl.UnmarshalText(text[rowSize-5 : rowSize-3]); err != nil {
		return NewInvalidInputError("invalid end_control", err)
	}

	// Step 6: Validate parity at positions [N-3..N-2] and verify it's valid
	expectedParity, err := br.GetParity()
	if err != nil {
		return NewInvalidInputError("failed to calculate expected parity", err)
	}
	actualParity := [2]byte{text[rowSize-3], text[rowSize-2]}
	if actualParity != expectedParity {
		return NewInvalidInputError(fmt.Sprintf("parity mismatch: expected [%c, %c], got [%c, %c]", expectedParity[0], expectedParity[1], actualParity[0], actualParity[1]), nil)
	}

	// Step 7: Validate ROW_END at position [N-1]
	if text[rowSize-1] != ROW_END {
		return NewInvalidInputError(fmt.Sprintf("invalid ROW_END: expected 0x%02X, got 0x%02X", ROW_END, text[rowSize-1]), nil)
	}

	// Call Validate() after successful unmarshaling
	return br.Validate()
}

// Validate performs comprehensive validation of baseRow structure
// This method is idempotent and can be called multiple times with the same result
func (br *baseRow[T]) Validate() error {
	if br.RowSize == 0 {
		return NewInvalidInputError("RowSize is required", nil)
	}

	// Validate start_control (assumes StartControl.Validate() was called during construction)
	if err := br.StartControl.Validate(); err != nil {
		return NewInvalidInputError("invalid StartControl in baseRow", err)
	}

	// Validate end_control (assumes EndControl.Validate() was called during construction)
	if err := br.EndControl.Validate(); err != nil {
		return NewInvalidInputError("invalid EndControl in baseRow", err)
	}

	// Validate the payload itself (T implements RowPayload, so we can call methods directly)
	// Check for nil using reflection since T is a generic type parameter
	payloadValue := reflect.ValueOf(br.RowPayload)
	if payloadValue.Kind() == reflect.Ptr && payloadValue.IsNil() {
		return NewInvalidInputError("RowPayload is required (programmer error: RowPayload must be set before validation)", nil)
	}
	if err := br.RowPayload.Validate(); err != nil {
		return NewInvalidInputError("payload validation failed", err)
	}

	return nil
}

// validate is kept for backward compatibility, calls Validate()
func (br *baseRow[T]) validate() error {
	return br.Validate()
}
