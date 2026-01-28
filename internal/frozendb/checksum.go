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

// Validate validates the Checksum value
// Checksum is universally valid (uint32 is always valid), so this always returns nil
func (c Checksum) Validate() error {
	return nil
}

// ChecksumRow represents a checksum integrity row in frozenDB
type ChecksumRow struct {
	baseRow[*Checksum] // Embedded with typed *Checksum payload
}

// NewChecksumRow creates a new checksum row from header and data bytes
// The header must already be validated by its creator (e.g., UnmarshalText or manual creation with Validate()).
// This function only checks that header is non-nil.
func NewChecksumRow(rowSize int, dataBytes []byte) (*ChecksumRow, error) {
	if len(dataBytes) == 0 {
		return nil, NewInvalidInputError("dataBytes cannot be empty", nil)
	}

	// Calculate CRC32 using IEEE polynomial
	crc32Value := crc32.ChecksumIEEE(dataBytes)
	checksum := Checksum(crc32Value)

	// Create checksum row
	cr := &ChecksumRow{
		baseRow[*Checksum]{
			RowSize:      rowSize,
			StartControl: CHECKSUM_ROW,
			EndControl:   CHECKSUM_ROW_CONTROL,
			RowPayload:   &checksum,
		},
	}

	// Validate baseRow structure first
	if err := cr.baseRow.Validate(); err != nil {
		return nil, err
	}

	// Validate checksum-specific properties
	if err := cr.Validate(); err != nil {
		return nil, err
	}

	return cr, nil
}

// GetChecksum extracts the CRC32 checksum value (no type assertion needed).
// This method assumes Validate() has been called and passed, ensuring RowPayload is not nil.
func (cr *ChecksumRow) GetChecksum() Checksum {
	return *cr.RowPayload
}

// MarshalText serializes ChecksumRow to exact byte format per v1_file_format.md
func (cr *ChecksumRow) MarshalText() ([]byte, error) {
	return cr.baseRow.MarshalText()
}

// UnmarshalText deserializes ChecksumRow from byte array with validation
func (cr *ChecksumRow) UnmarshalText(text []byte) error {
	// This will parse StartControl and EndControl from the text
	// baseRow.UnmarshalText() will call baseRow.Validate() internally
	if err := cr.baseRow.UnmarshalText(text); err != nil {
		return err
	}

	// Validate checksum-specific properties (StartControl='C', EndControl='CS', payload not nil)
	return cr.Validate()
}

// Validate performs validation of checksum-specific properties
// This method assumes baseRow.Validate() has already been called
// This method is idempotent and can be called multiple times with the same result
func (cr *ChecksumRow) Validate() error {
	// Validate start_control is 'C' for checksum rows (context-specific validation)
	if cr.StartControl != CHECKSUM_ROW {
		return NewInvalidInputError(fmt.Sprintf("checksum row must have start_control='C', got '%c'", cr.StartControl), nil)
	}

	// Validate end_control is 'CS' for checksum rows (context-specific validation)
	if cr.EndControl != CHECKSUM_ROW_CONTROL {
		return NewInvalidInputError(fmt.Sprintf("checksum row must have end_control='CS', got '%s'", cr.EndControl.String()), nil)
	}

	// Checksum is universally valid (uint32 is always valid), no validation needed

	return nil
}

// validate is kept for backward compatibility, calls Validate()
func (cr *ChecksumRow) validate() error {
	return cr.Validate()
}
