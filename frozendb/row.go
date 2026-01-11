package frozendb

import (
	"bytes"
	"encoding"
	"fmt"
	"reflect"
)

// Sentinel byte constants
const (
	ROW_START = 0x1F // Unit separator (U+001F)
	ROW_END   = 0x0A // Newline (U+000A)
	NULL_BYTE = 0x00 // Null character for padding
)

// StartControl represents single-byte control characters at row position [1]
type StartControl byte

// StartControl constants represent valid control characters
const (
	// START_TRANSACTION marks the beginning of a new transaction
	START_TRANSACTION StartControl = 'T'

	// ROW_CONTINUE marks the continuation of an existing transaction
	ROW_CONTINUE StartControl = 'R'

	// CHECKSUM_ROW marks a checksum integrity row
	CHECKSUM_ROW StartControl = 'C'
)

// MarshalText converts StartControl to single byte
func (sc StartControl) MarshalText() ([]byte, error) {
	return []byte{byte(sc)}, nil
}

// Validate validates the StartControl value
// This method is idempotent and can be called multiple times with the same result
func (sc StartControl) Validate() error {
	switch sc {
	case START_TRANSACTION, ROW_CONTINUE, CHECKSUM_ROW:
		return nil
	default:
		return NewInvalidInputError(fmt.Sprintf("invalid StartControl byte: 0x%02X", byte(sc)), nil)
	}
}

// UnmarshalText parses single byte and validates StartControl
func (sc *StartControl) UnmarshalText(text []byte) error {
	if len(text) != 1 {
		return NewInvalidInputError("StartControl must be exactly 1 byte", nil)
	}
	b := text[0]
	switch StartControl(b) {
	case START_TRANSACTION, ROW_CONTINUE, CHECKSUM_ROW:
		*sc = StartControl(b)
		// Call Validate() after unmarshaling
		return sc.Validate()
	default:
		return NewInvalidInputError(fmt.Sprintf("invalid StartControl byte: 0x%02X", b), nil)
	}
}

// EndControl represents two-byte control sequence at row positions [N-5:N-4]
type EndControl [2]byte

// Constants for common control sequences as byte arrays
var (
	// Data row end controls
	TRANSACTION_COMMIT = EndControl{'T', 'C'} // Transaction commit, no savepoint
	ROW_END_CONTROL    = EndControl{'R', 'E'} // Transaction continue, no savepoint
	SAVEPOINT_COMMIT   = EndControl{'S', 'C'} // Transaction commit with savepoint
	SAVEPOINT_CONTINUE = EndControl{'S', 'E'} // Transaction continue with savepoint
	FULL_ROLLBACK      = EndControl{'R', '0'} // Full rollback to savepoint 0

	// Checksum row end controls
	CHECKSUM_ROW_CONTROL = EndControl{'C', 'S'} // Checksum-specific end control
)

// MarshalText converts EndControl 2-byte array to slice
func (ec EndControl) MarshalText() ([]byte, error) {
	return ec[:], nil
}

// Validate validates the EndControl sequence
// This method is idempotent and can be called multiple times with the same result
func (ec EndControl) Validate() error {
	// Check exact matches against known constants
	switch ec {
	case TRANSACTION_COMMIT, ROW_END_CONTROL, CHECKSUM_ROW_CONTROL,
		SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK:
		return nil
	}

	// Special case: R0-R9 and S0-S9 rollback patterns
	first := ec[0]
	second := ec[1]
	if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
		return nil
	}

	return NewInvalidInputError(fmt.Sprintf("invalid EndControl: '%c%c'", first, second), nil)
}

// UnmarshalText parses 2-byte sequence into EndControl array with validation
func (ec *EndControl) UnmarshalText(text []byte) error {
	if len(text) != 2 {
		return NewInvalidInputError("EndControl must be exactly 2 bytes", nil)
	}

	candidate := EndControl{text[0], text[1]}

	// Check exact matches against known constants
	switch candidate {
	case TRANSACTION_COMMIT, ROW_END_CONTROL, CHECKSUM_ROW_CONTROL,
		SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK:
		copy(ec[:], text)
		// Call Validate() after unmarshaling
		return ec.Validate()
	}

	// Special case: R0-R9 and S0-S9 rollback patterns
	first := text[0]
	second := text[1]
	if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
		copy(ec[:], text)
		// Call Validate() after unmarshaling
		return ec.Validate()
	}

	return NewInvalidInputError(fmt.Sprintf("invalid EndControl: '%c%c'", first, second), nil)
}

// String converts EndControl to string representation for display/debugging
func (ec EndControl) String() string {
	return string(ec[:])
}

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
	Header       *Header      // Header reference for row_size and configuration
	StartControl StartControl // Single byte control character (position 1)
	EndControl   EndControl   // Two-byte end control sequence (positions N-5,N-4)
	RowPayload   T            // Typed payload data, validated after structural checks
}

// PaddingLength calculates the required null byte padding length
// Fixed overhead: ROW_START(1) + start_control(1) + end_control(2) + parity(2) + ROW_END(1) = 7
// Takes the marshaled payload bytes to determine actual payload size
func (br *baseRow[T]) PaddingLength(payloadBytes []byte) int {
	payloadSize := len(payloadBytes)
	return br.Header.GetRowSize() - 7 - payloadSize
}

// buildRowBytesUpToParity builds row bytes from ROW_START through end_control (positions [0] through [rowSize-4] inclusive)
// This includes: ROW_START, start_control, payload, padding, end_control
// Returns the bytes and an error if marshaling fails
func (br *baseRow[T]) buildRowBytesUpToParity() ([]byte, error) {
	if br.Header == nil {
		return nil, NewInvalidInputError("Header is required (programmer error: Header must be set)", nil)
	}
	rowSize := br.Header.GetRowSize()

	// Marshal payload (T implements RowPayload)
	payloadBytes, err := br.RowPayload.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal row payload", err)
	}

	// Calculate padding length
	paddingLen := br.PaddingLength(payloadBytes)
	if paddingLen < 0 {
		return nil, NewInvalidInputError("row_size too small for required fields", nil)
	}

	// Build row bytes up to but not including parity (positions [0] through [rowSize-4] inclusive)
	// We need bytes [0] through [rowSize-4] inclusive, which is rowSize-3 bytes total
	rowBytes := make([]byte, rowSize-3)

	// Position [0]: ROW_START
	rowBytes[0] = ROW_START

	// Position [1]: start_control
	startControlBytes, err := br.StartControl.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal start_control", err)
	}
	rowBytes[1] = startControlBytes[0]

	// Positions [2..2+payloadSize-1]: payload
	payloadStart := 2
	payloadEnd := payloadStart + len(payloadBytes)
	copy(rowBytes[payloadStart:payloadEnd], payloadBytes)

	// Positions [payloadEnd..N-6]: NULL_BYTE padding
	for i := payloadEnd; i < rowSize-6; i++ {
		rowBytes[i] = NULL_BYTE
	}

	// Positions [N-5..N-4]: end_control
	endControlBytes, err := br.EndControl.MarshalText()
	if err != nil {
		return nil, NewInvalidInputError("failed to marshal end_control", err)
	}
	copy(rowBytes[rowSize-5:rowSize-3], endControlBytes)

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
	if br.Header == nil {
		return nil, NewInvalidInputError("Header is required (programmer error: Header must be set)", nil)
	}
	rowSize := br.Header.GetRowSize()

	// Build row bytes up to but not including parity
	rowBytesUpToParity, err := br.buildRowBytesUpToParity()
	if err != nil {
		return nil, err
	}

	// Build full row bytes
	rowBytes := make([]byte, rowSize)
	copy(rowBytes, rowBytesUpToParity)

	// Positions [N-3..N-2]: parity_bytes (calculated after all other bytes are set)
	parity, err := br.GetParity()
	if err != nil {
		return nil, err
	}
	copy(rowBytes[rowSize-3:rowSize-1], parity[:])

	// Position [N-1]: ROW_END
	rowBytes[rowSize-1] = ROW_END

	return rowBytes, nil
}

// UnmarshalText deserializes baseRow from byte array with validation
func (br *baseRow[T]) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return NewInvalidInputError("row bytes cannot be empty", nil)
	}

	// Validate Header is set (programmer error if nil)
	// Header must already be validated by its creator, we only check it's non-nil
	if br.Header == nil {
		return NewInvalidInputError("Header is required (programmer error: Header must be set before UnmarshalText)", nil)
	}

	rowSize := br.Header.GetRowSize()
	if len(text) != rowSize {
		return NewInvalidInputError(fmt.Sprintf("row bytes length mismatch: expected %d, got %d", rowSize, len(text)), nil)
	}

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
	if br.Header == nil {
		return NewInvalidInputError("Header is required (programmer error: Header must be set before validation)", nil)
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
