package frozendb

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
)

// Checksum represents a CRC32 checksum value
type Checksum uint32

// MarshalText converts Checksum to 8-character Base64 string with "==" padding
func (c Checksum) MarshalText() ([]byte, error) {
	// Convert uint32 to 4 bytes (big-endian)
	bytes := []byte{
		byte(c >> 24),
		byte(c >> 16),
		byte(c >> 8),
		byte(c),
	}
	// Encode to Base64 (4 bytes -> 8 characters with "==" padding)
	encoded := base64.StdEncoding.EncodeToString(bytes)
	if len(encoded) != 8 {
		return nil, NewInvalidInputError(fmt.Sprintf("Base64 encoding should produce 8 characters, got %d", len(encoded)), nil)
	}
	return []byte(encoded), nil
}

// UnmarshalText parses 8-character Base64 string to Checksum
func (c *Checksum) UnmarshalText(text []byte) error {
	if len(text) != 8 {
		return NewInvalidInputError(fmt.Sprintf("Checksum Base64 must be exactly 8 bytes, got %d", len(text)), nil)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return NewInvalidInputError("invalid Base64 encoding for checksum", err)
	}
	if len(decoded) != 4 {
		return NewInvalidInputError(fmt.Sprintf("decoded checksum must be 4 bytes, got %d", len(decoded)), nil)
	}
	// Convert 4 bytes to uint32 (big-endian)
	*c = Checksum(uint32(decoded[0])<<24 | uint32(decoded[1])<<16 | uint32(decoded[2])<<8 | uint32(decoded[3]))
	return nil
}

// ChecksumRow represents a checksum integrity row in frozenDB
type ChecksumRow struct {
	baseRow[Checksum] // Embedded with typed Checksum payload
}

// NewChecksumRow creates a new checksum row from header and data bytes
// The header must already be validated by its creator (e.g., parseHeader or NewHeader).
// This function only checks that header is non-nil.
func NewChecksumRow(header *Header, dataBytes []byte) (*ChecksumRow, error) {
	if header == nil {
		return nil, NewInvalidInputError("Header is required", nil)
	}

	if len(dataBytes) == 0 {
		return nil, NewInvalidInputError("dataBytes cannot be empty", nil)
	}

	// Calculate CRC32 using IEEE polynomial
	crc32Value := crc32.ChecksumIEEE(dataBytes)
	checksum := Checksum(crc32Value)

	// Create checksum row
	cr := &ChecksumRow{
		baseRow[Checksum]{
			Header:       header,
			StartControl: CHECKSUM_ROW,
			EndControl:   CHECKSUM_ROW_CONTROL,
			RowPayload:   &checksum,
		},
	}

	// Validate the row structure
	if err := cr.validate(); err != nil {
		return nil, err
	}

	return cr, nil
}

// GetChecksum extracts the CRC32 checksum value (no type assertion needed)
func (cr *ChecksumRow) GetChecksum() Checksum {
	if cr.RowPayload == nil {
		return 0
	}
	return *cr.RowPayload
}

// MarshalText serializes ChecksumRow to exact byte format per v1_file_format.md
func (cr *ChecksumRow) MarshalText() ([]byte, error) {
	return cr.baseRow.MarshalText()
}

// UnmarshalText deserializes ChecksumRow from byte array with validation
func (cr *ChecksumRow) UnmarshalText(text []byte) error {
	// Unmarshal using baseRow (Header must be set - programmer error if nil)
	// This will parse StartControl and EndControl from the text
	if err := cr.baseRow.UnmarshalText(text); err != nil {
		return err
	}

	// Validate that the parsed control values match checksum row expectations
	return cr.validate()
}

// validate performs comprehensive validation of checksum row structure
func (cr *ChecksumRow) validate() error {
	// Validate base row structure
	if err := cr.baseRow.validate(); err != nil {
		return err
	}

	// Validate start_control is 'C' for checksum rows
	if cr.StartControl != CHECKSUM_ROW {
		return NewInvalidInputError(fmt.Sprintf("checksum row must have start_control='C', got '%c'", cr.StartControl), nil)
	}

	// Validate end_control is 'CS' for checksum rows
	if cr.EndControl != CHECKSUM_ROW_CONTROL {
		return NewInvalidInputError(fmt.Sprintf("checksum row must have end_control='CS', got '%s'", cr.EndControl.String()), nil)
	}

	// Validate payload is not nil
	if cr.RowPayload == nil {
		return NewInvalidInputError("checksum row payload cannot be nil", nil)
	}

	return nil
}
