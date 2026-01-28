package fields

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"testing"
)

// Test_S_003_FR_001_ChecksumRowStructure tests FR-001: System MUST implement a ChecksumRow struct with fields matching v1_file_format.md section 6.1 specification
func Test_S_003_FR_001_ChecksumRowStructure(t *testing.T) {
	dataBytes := []byte("test data for checksum")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	// Verify ChecksumRow has required structure
	if checksumRow == nil {
		t.Fatal("ChecksumRow should not be nil")
	}

	// Verify checksum can be retrieved
	checksum := checksumRow.GetChecksum()
	if checksum == 0 {
		t.Error("Checksum should not be zero")
	}
}

// Test_S_003_FR_002_SerializationLayout tests FR-002: System MUST provide serialization method that outputs exact byte layout: ROW_START, start_control='C', crc32_base64 (8 bytes), NULL_BYTE padding, end_control='CS', parity_bytes (2 bytes), ROW_END
func Test_S_003_FR_002_SerializationLayout(t *testing.T) {
	dataBytes := []byte("test data for checksum calculation")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify exact byte layout per v1_file_format.md section 6.1
	rowSize := 1024
	if len(rowBytes) != rowSize {
		t.Errorf("Row size mismatch: expected %d, got %d", rowSize, len(rowBytes))
	}

	// Position [0]: ROW_START (0x1F)
	if rowBytes[0] != ROW_START {
		t.Errorf("ROW_START mismatch: expected 0x%02X, got 0x%02X", ROW_START, rowBytes[0])
	}

	// Position [1]: start_control='C'
	if rowBytes[1] != byte(CHECKSUM_ROW) {
		t.Errorf("Start control mismatch: expected 'C' (0x%02X), got 0x%02X", byte(CHECKSUM_ROW), rowBytes[1])
	}

	// Positions [2..9]: crc32_base64 (8 bytes)
	crc32Base64 := rowBytes[2:10]
	if len(crc32Base64) != 8 {
		t.Errorf("CRC32 Base64 length mismatch: expected 8, got %d", len(crc32Base64))
	}

	// Positions [10..N-6]: NULL_BYTE padding
	for i := 10; i < rowSize-6; i++ {
		if rowBytes[i] != NULL_BYTE {
			t.Errorf("Padding byte at position %d should be NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, rowBytes[i])
		}
	}

	// Positions [N-5..N-4]: end_control='CS'
	endControl := rowBytes[rowSize-5 : rowSize-3]
	if endControl[0] != 'C' || endControl[1] != 'S' {
		t.Errorf("End control mismatch: expected 'CS', got '%c%c'", endControl[0], endControl[1])
	}

	// Positions [N-3..N-2]: parity_bytes (2 bytes)
	parityBytes := rowBytes[rowSize-3 : rowSize-1]
	if len(parityBytes) != 2 {
		t.Errorf("Parity bytes length mismatch: expected 2, got %d", len(parityBytes))
	}

	// Position [N-1]: ROW_END (0x0A)
	if rowBytes[rowSize-1] != ROW_END {
		t.Errorf("ROW_END mismatch: expected 0x%02X, got 0x%02X", ROW_END, rowBytes[rowSize-1])
	}
}

// Test_S_003_FR_003_Base64Encoding tests FR-003: System MUST encode 4-byte CRC32 values as 8-character Base64 strings with standard padding
func Test_S_003_FR_003_Base64Encoding(t *testing.T) {
	dataBytes := []byte("test data for CRC32 calculation")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Extract CRC32 Base64 from positions [2..9]
	crc32Base64 := rowBytes[2:10]
	if len(crc32Base64) != 8 {
		t.Fatalf("CRC32 Base64 should be 8 bytes, got %d", len(crc32Base64))
	}

	// Verify Base64 encoding with standard padding
	crc32Str := string(crc32Base64)
	if len(crc32Str) != 8 {
		t.Errorf("CRC32 Base64 string should be 8 characters, got %d", len(crc32Str))
	}

	// Verify it ends with "==" padding (standard Base64 for 4-byte input)
	if crc32Str[6:] != "==" {
		t.Errorf("CRC32 Base64 should end with '==', got '%s'", crc32Str[6:])
	}

	// Verify it's valid Base64
	decoded, err := base64.StdEncoding.DecodeString(crc32Str)
	if err != nil {
		t.Errorf("CRC32 Base64 is not valid Base64: %v", err)
	}

	// Verify decoded length is 4 bytes
	if len(decoded) != 4 {
		t.Errorf("Decoded CRC32 should be 4 bytes, got %d", len(decoded))
	}

	// Verify CRC32 matches expected calculation
	expectedCRC32 := crc32.ChecksumIEEE(dataBytes)
	actualCRC32 := uint32(decoded[0])<<24 | uint32(decoded[1])<<16 | uint32(decoded[2])<<8 | uint32(decoded[3])
	if actualCRC32 != expectedCRC32 {
		t.Errorf("CRC32 mismatch: expected 0x%08X, got 0x%08X", expectedCRC32, actualCRC32)
	}
}

// Test_S_003_FR_004_ParityCalculation tests FR-004: System MUST calculate LRC parity bytes using XOR algorithm on bytes [0] through [row_size-4]
func Test_S_003_FR_004_ParityCalculation(t *testing.T) {
	dataBytes := []byte("test data for parity calculation")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	rowSize := 1024
	parityBytes := rowBytes[rowSize-3 : rowSize-1]

	// Calculate expected parity using XOR on bytes [0] through [row_size-4] (inclusive)
	var expectedXOR byte = 0
	for i := 0; i < rowSize-3; i++ {
		expectedXOR ^= rowBytes[i]
	}

	// Parity bytes are stored as 2-character uppercase hex
	// Convert parity bytes to hex value
	parityHex := string(parityBytes)
	if len(parityHex) != 2 {
		t.Fatalf("Parity bytes should be 2 characters, got %d", len(parityHex))
	}

	// Parse hex to byte
	var actualParity byte
	_, err = fmt.Sscanf(parityHex, "%02X", &actualParity)
	if err != nil {
		t.Fatalf("Failed to parse parity hex '%s': %v", parityHex, err)
	}

	if actualParity != expectedXOR {
		t.Errorf("Parity mismatch: expected 0x%02X, got 0x%02X (hex: %s)", expectedXOR, actualParity, parityHex)
	}
}

// Test_S_003_FR_005_ParityValidation tests FR-005: System MUST validate all data rows' parity before calculating block checksums as required by v1_file_format.md section 7.4
func Test_S_003_FR_005_ParityValidation(t *testing.T) {
	dataBytes := []byte("test data for checksum")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify parity is correct by validating the row
	var parsedRow ChecksumRow
	if err := parsedRow.UnmarshalText(rowBytes); err != nil {
		t.Fatalf("UnmarshalText failed on valid row: %v", err)
	}

	// Corrupt parity bytes and verify validation fails
	corruptedBytes := make([]byte, len(rowBytes))
	copy(corruptedBytes, rowBytes)
	corruptedBytes[len(corruptedBytes)-3] = 0xFF // Corrupt first parity byte
	corruptedBytes[len(corruptedBytes)-2] = 0xFF // Corrupt second parity byte

	var corruptedRow ChecksumRow
	if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
		t.Error("UnmarshalText should fail on row with corrupted parity bytes")
	} else {
		// Verify it's an InvalidInputError for structural validation failure
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError for corrupted parity, got %T", err)
		}
	}
}

// Test_S_003_FR_006_SentinelBytes tests FR-006: System MUST handle sentinel bytes correctly: ROW_START (0x1F) and ROW_END (0x0A)
func Test_S_003_FR_006_SentinelBytes(t *testing.T) {
	dataBytes := []byte("test data")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify ROW_START at position [0]
	if rowBytes[0] != ROW_START {
		t.Errorf("ROW_START mismatch: expected 0x%02X, got 0x%02X", ROW_START, rowBytes[0])
	}

	// Verify ROW_END at position [row_size-1]
	rowSize := 1024
	if rowBytes[rowSize-1] != ROW_END {
		t.Errorf("ROW_END mismatch: expected 0x%02X, got 0x%02X", ROW_END, rowBytes[rowSize-1])
	}

	// Test deserialization with wrong sentinel bytes
	corruptedBytes := make([]byte, len(rowBytes))
	copy(corruptedBytes, rowBytes)
	corruptedBytes[0] = 0x00 // Wrong ROW_START

	var corruptedRow ChecksumRow
	if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
		t.Error("UnmarshalText should fail on row with wrong ROW_START")
	} else {
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError for wrong ROW_START, got %T", err)
		}
	}

	corruptedBytes = make([]byte, len(rowBytes))
	copy(corruptedBytes, rowBytes)
	corruptedBytes[rowSize-1] = 0x00 // Wrong ROW_END

	if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
		t.Error("UnmarshalText should fail on row with wrong ROW_END")
	} else {
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError for wrong ROW_END, got %T", err)
		}
	}
}

// Test_S_003_FR_007_RowSizeSupport tests FR-007: System MUST support row sizes from 128 to 65536 bytes as specified in header format
func Test_S_003_FR_007_RowSizeSupport(t *testing.T) {
	testCases := []struct {
		name    string
		rowSize int
		wantErr bool
	}{
		{"minimum row size", 128, false},
		{"medium row size", 1024, false},
		{"large row size", 4096, false},
		{"maximum row size", 65536, false},
		{"too small", 127, true},
		{"too large", 65537, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataBytes := []byte("test data")

			checksumRow, err := NewChecksumRow(tc.rowSize, dataBytes)
			if err != nil {
				t.Fatalf("NewChecksumRow failed for valid row size %d: %v", tc.rowSize, err)
			}

			rowBytes, err := checksumRow.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText failed: %v", err)
			}

			if len(rowBytes) != tc.rowSize {
				t.Errorf("Row size mismatch: expected %d, got %d", tc.rowSize, len(rowBytes))
			}
		})
	}
}

// Test_S_003_FR_008_DeserializationSupport tests FR-008: System MUST provide deserialization method that can parse checksum rows from byte arrays
func Test_S_003_FR_008_DeserializationSupport(t *testing.T) {
	dataBytes := []byte("test data for deserialization")

	// Create and serialize checksum row
	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Deserialize back
	var parsedRow ChecksumRow
	if err := parsedRow.UnmarshalText(rowBytes); err != nil {
		t.Fatalf("UnmarshalText failed: %v", err)
	}

	// Verify deserialized checksum matches original
	originalChecksum := checksumRow.GetChecksum()
	parsedChecksum := parsedRow.GetChecksum()
	if originalChecksum != parsedChecksum {
		t.Errorf("Checksum mismatch after deserialization: expected %v, got %v", originalChecksum, parsedChecksum)
	}
}

// Test_S_003_FR_009_ValidationControlBytes tests FR-009: System MUST validate control byte sequences: start_control='C' and end_control='CS' for checksum rows
func Test_S_003_FR_009_ValidationControlBytes(t *testing.T) {
	dataBytes := []byte("test data")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify start_control is 'C'
	if rowBytes[1] != byte(CHECKSUM_ROW) {
		t.Errorf("Start control mismatch: expected 'C' (0x%02X), got 0x%02X", byte(CHECKSUM_ROW), rowBytes[1])
	}

	// Verify end_control is 'CS'
	rowSize := 1024
	endControl := rowBytes[rowSize-5 : rowSize-3]
	if endControl[0] != 'C' || endControl[1] != 'S' {
		t.Errorf("End control mismatch: expected 'CS', got '%c%c'", endControl[0], endControl[1])
	}

	// Test deserialization with wrong start_control
	corruptedBytes := make([]byte, len(rowBytes))
	copy(corruptedBytes, rowBytes)
	corruptedBytes[1] = 'T' // Wrong start_control

	var corruptedRow ChecksumRow
	if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
		t.Error("UnmarshalText should fail on row with wrong start_control")
	} else {
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError for wrong start_control, got %T", err)
		}
	}

	// Test deserialization with wrong end_control
	corruptedBytes = make([]byte, len(rowBytes))
	copy(corruptedBytes, rowBytes)
	corruptedBytes[rowSize-5] = 'T' // Wrong end_control
	corruptedBytes[rowSize-4] = 'C'

	if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
		t.Error("UnmarshalText should fail on row with wrong end_control")
	} else {
		if _, ok := err.(*InvalidInputError); !ok {
			t.Errorf("Expected InvalidInputError for wrong end_control, got %T", err)
		}
	}
}

// Test_S_004_FR_009_ValidatesChildContext tests FR-009: System MUST have Validate() check that child struct fields meet parent's contextual requirements (e.g., ChecksumRow requires StartControl='C')
func Test_S_004_FR_009_ValidatesChildContext(t *testing.T) {
	// Test ChecksumRow.Validate() checks contextual requirements for StartControl and EndControl
	// Create a valid ChecksumRow
	cr, err := NewChecksumRow(1024, []byte("test data"))
	if err != nil {
		t.Fatalf("Failed to create ChecksumRow: %v", err)
	}

	// Verify valid ChecksumRow passes validation
	err = cr.Validate()
	if err != nil {
		t.Errorf("Valid ChecksumRow should pass validation: %v", err)
	}

	// Test that ChecksumRow.Validate() fails when StartControl is not 'C'
	// (context-specific validation)
	invalidCr := &ChecksumRow{
		baseRow: baseRow[*Checksum]{
			RowSize:      1024,
			StartControl: START_TRANSACTION, // Wrong: should be 'C' for checksum
			EndControl:   CHECKSUM_ROW_CONTROL,
			RowPayload:   cr.RowPayload,
		},
	}
	err = invalidCr.Validate()
	if err == nil {
		t.Error("ChecksumRow.Validate() should fail when StartControl is not 'C'")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("Expected InvalidInputError, got: %T", err)
	}

	// Test that ChecksumRow.Validate() fails when EndControl is not 'CS'
	invalidCr2 := &ChecksumRow{
		baseRow: baseRow[*Checksum]{
			RowSize:      1024,
			StartControl: CHECKSUM_ROW,
			EndControl:   TRANSACTION_COMMIT, // Wrong: should be 'CS' for checksum
			RowPayload:   cr.RowPayload,
		},
	}
	err = invalidCr2.Validate()
	if err == nil {
		t.Error("ChecksumRow.Validate() should fail when EndControl is not 'CS'")
	}
	if _, ok := err.(*InvalidInputError); !ok {
		t.Errorf("Expected InvalidInputError, got: %T", err)
	}
}

// Test_S_003_FR_010_CompleteValidation tests FR-010: System MUST validate EVERY bit of the string when deserializing a checksum row, including sentinel bits, parity correctness, padding, and control characters
func Test_S_003_FR_010_CompleteValidation(t *testing.T) {
	dataBytes := []byte("test data for complete validation")

	checksumRow, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow failed: %v", err)
	}

	rowBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Test that valid row passes validation
	var validRow ChecksumRow
	if err := validRow.UnmarshalText(rowBytes); err != nil {
		t.Fatalf("Valid row should pass validation, got error: %v", err)
	}

	// Test corrupted sentinel bytes
	testCases := []struct {
		name        string
		corruptFunc func([]byte)
	}{
		{
			name:        "corrupt ROW_START",
			corruptFunc: func(b []byte) { b[0] = 0x00 },
		},
		{
			name:        "corrupt ROW_END",
			corruptFunc: func(b []byte) { b[len(b)-1] = 0x00 },
		},
		{
			name:        "corrupt start_control",
			corruptFunc: func(b []byte) { b[1] = 'T' },
		},
		{
			name: "corrupt end_control",
			corruptFunc: func(b []byte) {
				b[len(b)-5] = 'T'
				b[len(b)-4] = 'C'
			},
		},
		{
			name:        "corrupt padding",
			corruptFunc: func(b []byte) { b[10] = 0xFF },
		},
		{
			name: "corrupt parity bytes",
			corruptFunc: func(b []byte) {
				b[len(b)-3] = 0xFF
				b[len(b)-2] = 0xFF
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			corruptedBytes := make([]byte, len(rowBytes))
			copy(corruptedBytes, rowBytes)
			tc.corruptFunc(corruptedBytes)

			var corruptedRow ChecksumRow
			if err := corruptedRow.UnmarshalText(corruptedBytes); err == nil {
				t.Errorf("UnmarshalText should fail for %s", tc.name)
			} else {
				// Verify it's an InvalidInputError for structural validation failures
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError for %s, got %T: %v", tc.name, err, err)
				}
			}
		})
	}
}
