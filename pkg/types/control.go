package types

import "fmt"

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
	CHECKSUM_ROW_CONTROL = EndControl{'C', 'S'}

	// Null row end controls
	NULL_ROW_CONTROL = EndControl{'N', 'R'}
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
		SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK, NULL_ROW_CONTROL:
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
		SAVEPOINT_COMMIT, SAVEPOINT_CONTINUE, FULL_ROLLBACK, NULL_ROW_CONTROL:
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
