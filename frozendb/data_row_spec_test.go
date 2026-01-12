package frozendb

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// Test_S_005_FR_001_DataRowCreation tests FR-001: System MUST allow DataRow creation through manual struct initialization with Header, UUIDv7 key, and JSON string value
func Test_S_005_FR_001_DataRowCreation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	// Generate a valid UUIDv7 key
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	// JSON string value (no syntax validation at this layer)
	value := `{"name":"John Doe","age":30}`

	// Create DataRow with manual struct initialization
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	// Validate the constructed DataRow
	if err := dataRow.Validate(); err != nil {
		t.Fatalf("DataRow validation failed: %v", err)
	}

	// Verify key and value can be retrieved
	retrievedKey := dataRow.GetKey()
	if retrievedKey != key {
		t.Errorf("Key mismatch: expected %s, got %s", key, retrievedKey)
	}

	retrievedValue := dataRow.GetValue()
	if retrievedValue != value {
		t.Errorf("Value mismatch: expected %s, got %s", value, retrievedValue)
	}
}

// Test_S_005_FR_002_UUIDv7Validation tests FR-002: System MUST validate that input UUID is UUIDv7 version
func Test_S_005_FR_002_UUIDv7Validation(t *testing.T) {
	// Test with valid UUIDv7
	t.Run("valid_uuidv7", func(t *testing.T) {
		key, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to generate UUIDv7: %v", err)
		}

		validationErr := ValidateUUIDv7(key)
		if validationErr != nil {
			t.Errorf("Valid UUIDv7 should not return error, got: %v", validationErr)
		}
	})

	// Test with UUIDv4 (invalid version)
	t.Run("invalid_uuidv4", func(t *testing.T) {
		key := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000") // v4
		err := ValidateUUIDv7(key)
		if err == nil {
			t.Error("UUIDv4 should be rejected")
		}
		if err == nil || err.Code != "invalid_input" {
			t.Errorf("Expected InvalidInputError, got %v", err)
		}
	})

	// Test with UUIDv1 (invalid version)
	t.Run("invalid_uuidv1", func(t *testing.T) {
		key := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // v1
		err := ValidateUUIDv7(key)
		if err == nil {
			t.Error("UUIDv1 should be rejected")
		}
		if err == nil || err.Code != "invalid_input" {
			t.Errorf("Expected InvalidInputError, got %v", err)
		}
	})

	// Test with zero UUID
	t.Run("zero_uuid", func(t *testing.T) {
		key := uuid.Nil
		err := ValidateUUIDv7(key)
		if err == nil {
			t.Error("Zero UUID should be rejected")
		}
		if err == nil || err.Code != "invalid_input" {
			t.Errorf("Expected InvalidInputError, got %v", err)
		}
	})
}

// Test_S_005_FR_003_SerializationFormat tests FR-003: System MUST serialize DataRow to exact byte format per v1_file_format.md specification
func Test_S_005_FR_003_SerializationFormat(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"name":"Test"}`
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	if err := dataRow.Validate(); err != nil {
		t.Fatalf("DataRow validation failed: %v", err)
	}

	rowBytes, err := dataRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify exact byte layout per v1_file_format.md section 8.1
	rowSize := header.GetRowSize()
	if len(rowBytes) != rowSize {
		t.Errorf("Row size mismatch: expected %d, got %d", rowSize, len(rowBytes))
	}

	// Position [0]: ROW_START (0x1F)
	if rowBytes[0] != ROW_START {
		t.Errorf("ROW_START mismatch: expected 0x%02X, got 0x%02X", ROW_START, rowBytes[0])
	}

	// Position [1]: start_control='T' or 'R'
	if rowBytes[1] != byte(START_TRANSACTION) && rowBytes[1] != byte(ROW_CONTINUE) {
		t.Errorf("Start control mismatch: expected 'T' (0x%02X) or 'R' (0x%02X), got 0x%02X", byte(START_TRANSACTION), byte(ROW_CONTINUE), rowBytes[1])
	}

	// Positions [2..25]: uuid_base64 (24 bytes)
	uuidBase64 := rowBytes[2:26]
	if len(uuidBase64) != 24 {
		t.Errorf("UUID Base64 length mismatch: expected 24, got %d", len(uuidBase64))
	}

	// Verify Base64 encoding is valid
	decoded, err := base64.StdEncoding.DecodeString(string(uuidBase64))
	if err != nil {
		t.Errorf("Invalid Base64 encoding for UUID: %v", err)
	}
	if len(decoded) != 16 {
		t.Errorf("Decoded UUID length mismatch: expected 16, got %d", len(decoded))
	}

	// Verify decoded UUID matches original
	decodedUUID, err := uuid.FromBytes(decoded)
	if err != nil {
		t.Errorf("Failed to parse decoded UUID: %v", err)
	}
	if decodedUUID != key {
		t.Errorf("Decoded UUID mismatch: expected %s, got %s", key, decodedUUID)
	}

	// Positions [26..N-6]: json_payload + NULL_BYTE padding
	// Find first null byte to identify payload end
	payloadStart := 26
	payloadEnd := payloadStart
	for i := payloadStart; i < rowSize-6; i++ {
		if rowBytes[i] == NULL_BYTE {
			payloadEnd = i
			break
		}
	}

	// Verify JSON payload matches original value
	jsonPayload := string(rowBytes[payloadStart:payloadEnd])
	if jsonPayload != value {
		t.Errorf("JSON payload mismatch: expected %s, got %s", value, jsonPayload)
	}

	// Verify padding bytes are NULL_BYTE
	for i := payloadEnd; i < rowSize-6; i++ {
		if rowBytes[i] != NULL_BYTE {
			t.Errorf("Padding byte at position %d should be NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, rowBytes[i])
		}
	}

	// Positions [N-5..N-4]: end_control
	endControl := rowBytes[rowSize-5 : rowSize-3]
	if endControl[0] != 'T' || endControl[1] != 'C' {
		t.Errorf("End control mismatch: expected 'TC', got '%c%c'", endControl[0], endControl[1])
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

// Test_S_005_FR_004_DeserializationValidation tests FR-004: System MUST deserialize DataRow from byte array with complete validation
func Test_S_005_FR_004_DeserializationValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"name":"Deserialize Test","count":42}`
	originalRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	if err := originalRow.Validate(); err != nil {
		t.Fatalf("Original DataRow validation failed: %v", err)
	}

	// Serialize to bytes
	rowBytes, err := originalRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Deserialize from bytes
	deserializedRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header: header,
		},
	}

	if err := deserializedRow.UnmarshalText(rowBytes); err != nil {
		t.Fatalf("UnmarshalText failed: %v", err)
	}

	// Validate deserialized row
	if err := deserializedRow.Validate(); err != nil {
		t.Fatalf("Deserialized DataRow validation failed: %v", err)
	}

	// Verify round-trip preservation
	deserializedKey := deserializedRow.GetKey()
	if deserializedKey != key {
		t.Errorf("Key mismatch after round-trip: expected %s, got %s", key, deserializedKey)
	}

	deserializedValue := deserializedRow.GetValue()
	if deserializedValue != value {
		t.Errorf("Value mismatch after round-trip: expected %s, got %s", value, deserializedValue)
	}

	// Verify control characters
	if deserializedRow.StartControl != START_TRANSACTION {
		t.Errorf("StartControl mismatch: expected %c, got %c", START_TRANSACTION, deserializedRow.StartControl)
	}

	if deserializedRow.EndControl != TRANSACTION_COMMIT {
		t.Errorf("EndControl mismatch: expected %s, got %s", TRANSACTION_COMMIT.String(), deserializedRow.EndControl.String())
	}
}

// Test_S_005_FR_005_StartControlValidation tests FR-005: System MUST handle proper start control characters (basic validation for single rows)
func Test_S_005_FR_005_StartControlValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`

	// Test valid start controls
	t.Run("valid_T", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err != nil {
			t.Errorf("Valid start control 'T' should pass validation: %v", err)
		}
	})

	t.Run("valid_R", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: ROW_CONTINUE,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err != nil {
			t.Errorf("Valid start control 'R' should pass validation: %v", err)
		}
	})

	// Test invalid start control
	t.Run("invalid_C", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: CHECKSUM_ROW,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Invalid start control 'C' should fail validation")
		}
	})
}

// Test_S_005_FR_006_EndControlValidation tests FR-006: System MUST handle basic end control character validation for single rows
func Test_S_005_FR_006_EndControlValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`

	// Test valid end controls
	validEndControls := []EndControl{
		TRANSACTION_COMMIT,
		ROW_END_CONTROL,
		SAVEPOINT_COMMIT,
		SAVEPOINT_CONTINUE,
		FULL_ROLLBACK,
		EndControl{'R', '5'}, // R5 rollback
		EndControl{'S', '3'}, // S3 savepoint rollback
	}

	for _, endControl := range validEndControls {
		t.Run("valid_"+endControl.String(), func(t *testing.T) {
			dataRow := &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   endControl,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: value,
					},
				},
			}
			if err := dataRow.Validate(); err != nil {
				t.Errorf("Valid end control '%s' should pass validation: %v", endControl.String(), err)
			}
		})
	}

	// Test invalid end control
	t.Run("invalid_CS", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   CHECKSUM_ROW_CONTROL,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Invalid end control 'CS' should fail validation")
		}
	})
}

// Test_S_005_FR_007_ParityByteCalculation tests FR-007: System MUST calculate and validate LRC parity bytes for row integrity
func Test_S_005_FR_007_ParityByteCalculation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	if err := dataRow.Validate(); err != nil {
		t.Fatalf("DataRow validation failed: %v", err)
	}

	// Serialize to get parity bytes
	rowBytes, err := dataRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify parity bytes are present
	rowSize := header.GetRowSize()
	parityBytes := rowBytes[rowSize-3 : rowSize-1]
	if len(parityBytes) != 2 {
		t.Errorf("Parity bytes length mismatch: expected 2, got %d", len(parityBytes))
	}

	// Verify parity bytes are valid hex characters
	for i, b := range parityBytes {
		if (b < '0' || b > '9') && (b < 'A' || b > 'F') {
			t.Errorf("Parity byte at position %d is not valid hex: 0x%02X", i, b)
		}
	}

	// Verify parity calculation: XOR all bytes from [0] through [rowSize-4]
	var xor byte = 0
	for i := 0; i < rowSize-3; i++ {
		xor ^= rowBytes[i]
	}

	// Encode XOR result as uppercase hex
	expectedParity := [2]byte{
		byte((xor >> 4) & 0x0F),
		byte(xor & 0x0F),
	}
	// Convert to ASCII hex characters
	if expectedParity[0] < 10 {
		expectedParity[0] += '0'
	} else {
		expectedParity[0] += 'A' - 10
	}
	if expectedParity[1] < 10 {
		expectedParity[1] += '0'
	} else {
		expectedParity[1] += 'A' - 10
	}

	if parityBytes[0] != expectedParity[0] || parityBytes[1] != expectedParity[1] {
		t.Errorf("Parity mismatch: expected [%c, %c], got [%c, %c]", expectedParity[0], expectedParity[1], parityBytes[0], parityBytes[1])
	}

	// Test that corrupted parity is detected during unmarshaling
	t.Run("corrupted_parity_detection", func(t *testing.T) {
		corruptedBytes := make([]byte, len(rowBytes))
		copy(corruptedBytes, rowBytes)
		// Corrupt parity byte
		corruptedBytes[rowSize-3] = 'X'

		deserializedRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header: header,
			},
		}
		if err := deserializedRow.UnmarshalText(corruptedBytes); err == nil {
			t.Error("Corrupted parity should be detected during unmarshaling")
		}
	})
}

// Test_S_005_FR_008_NullBytePadding tests FR-008: System MUST pad JSON string value with NULL_BYTE to fill remaining row space
func Test_S_005_FR_008_NullBytePadding(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	// Test with small JSON value to ensure padding
	value := `{"test":"value"}`
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	if err := dataRow.Validate(); err != nil {
		t.Fatalf("DataRow validation failed: %v", err)
	}

	rowBytes, err := dataRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	rowSize := header.GetRowSize()
	// Find first null byte starting from position 26 (after UUID)
	payloadStart := 26
	payloadEnd := payloadStart
	for i := payloadStart; i < rowSize-6; i++ {
		if rowBytes[i] == NULL_BYTE {
			payloadEnd = i
			break
		}
	}

	// Verify JSON payload is present
	jsonPayload := string(rowBytes[payloadStart:payloadEnd])
	if jsonPayload != value {
		t.Errorf("JSON payload mismatch: expected %s, got %s", value, jsonPayload)
	}

	// Verify all padding bytes are NULL_BYTE
	for i := payloadEnd; i < rowSize-6; i++ {
		if rowBytes[i] != NULL_BYTE {
			t.Errorf("Padding byte at position %d should be NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, rowBytes[i])
		}
	}
}

// Test_S_005_FR_009_RowSizeValidation tests FR-009: System MUST ensure overall row length matches Header's row_size exactly
func Test_S_005_FR_009_RowSizeValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`
	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   TRANSACTION_COMMIT,
			RowPayload: &DataRowPayload{
				Key:   key,
				Value: value,
			},
		},
	}

	if err := dataRow.Validate(); err != nil {
		t.Fatalf("DataRow validation failed: %v", err)
	}

	rowBytes, err := dataRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Verify row length matches header row_size exactly
	expectedSize := header.GetRowSize()
	if len(rowBytes) != expectedSize {
		t.Errorf("Row size mismatch: expected %d, got %d", expectedSize, len(rowBytes))
	}

	// Test with different row sizes
	rowSizes := []int{128, 256, 1024, 2048}
	for _, size := range rowSizes {
		t.Run(fmt.Sprintf("row_size_%d", size), func(t *testing.T) {
			testHeader := &Header{
				signature: "fDB",
				version:   1,
				rowSize:   size,
				skewMs:    5000,
			}

			testRow := &DataRow{
				baseRow[*DataRowPayload]{
					Header:       testHeader,
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: value,
					},
				},
			}

			if err := testRow.Validate(); err != nil {
				t.Fatalf("DataRow validation failed: %v", err)
			}

			testBytes, err := testRow.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText failed: %v", err)
			}

			if len(testBytes) != size {
				t.Errorf("Row size mismatch: expected %d, got %d", size, len(testBytes))
			}
		})
	}

	// Test that validation rejects payloads that would exceed ROW_SIZE
	t.Run("rejects_payload_exceeding_row_size", func(t *testing.T) {
		testHeader := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    5000,
		}

		// Calculate maximum JSON value size: row_size - 7 (fixed overhead) - 24 (Base64 UUID) = 481 bytes
		maxJsonValueSize := testHeader.GetRowSize() - 7 - 24

		// Create a JSON value that exceeds the maximum by 1 byte
		excessiveValue := make([]byte, maxJsonValueSize+1)
		for i := range excessiveValue {
			excessiveValue[i] = 'x'
		}

		excessiveDataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       testHeader,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: string(excessiveValue),
				},
			},
		}

		// Validate should reject this payload
		err := excessiveDataRow.Validate()
		if err == nil {
			t.Error("Validate() should reject payload that exceeds ROW_SIZE")
		}
		if err != nil {
			// Verify it's an InvalidInputError
			if invalidErr, ok := err.(*InvalidInputError); !ok {
				t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
			} else if invalidErr.Code != "invalid_input" {
				t.Errorf("Expected error code 'invalid_input', got '%s'", invalidErr.Code)
			}
		}
	})

	// Test that validation accepts payloads that are exactly at the maximum allowed size
	t.Run("accepts_payload_at_maximum_size", func(t *testing.T) {
		testHeader := &Header{
			signature: "fDB",
			version:   1,
			rowSize:   512,
			skewMs:    5000,
		}

		// Calculate maximum JSON value size: row_size - 7 (fixed overhead) - 24 (Base64 UUID) = 481 bytes
		maxJsonValueSize := testHeader.GetRowSize() - 7 - 24

		// Create a JSON value that is exactly at the maximum
		maxValue := make([]byte, maxJsonValueSize)
		for i := range maxValue {
			maxValue[i] = 'x'
		}

		maxDataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       testHeader,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: string(maxValue),
				},
			},
		}

		// Validate should accept this payload
		err := maxDataRow.Validate()
		if err != nil {
			t.Errorf("Validate() should accept payload at maximum allowed size, got error: %v", err)
		}

		// Verify it can be marshaled successfully
		rowBytes, err := maxDataRow.MarshalText()
		if err != nil {
			t.Errorf("MarshalText() should succeed for payload at maximum size, got error: %v", err)
		}
		if len(rowBytes) != testHeader.GetRowSize() {
			t.Errorf("Marshaled row size mismatch: expected %d, got %d", testHeader.GetRowSize(), len(rowBytes))
		}
	})
}

// Test_S_005_FR_010_ControlCharacterValidation tests FR-010: System MUST validate that start and end control characters are valid single-byte characters for single row context
func Test_S_005_FR_010_ControlCharacterValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`

	// Test valid single-byte start controls
	validStartControls := []StartControl{START_TRANSACTION, ROW_CONTINUE}
	for _, sc := range validStartControls {
		t.Run("valid_start_"+string(sc), func(t *testing.T) {
			dataRow := &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: sc,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: value,
					},
				},
			}
			if err := dataRow.Validate(); err != nil {
				t.Errorf("Valid start control '%c' should pass validation: %v", sc, err)
			}
		})
	}

	// Test that end controls are 2-byte sequences
	validEndControls := []EndControl{
		TRANSACTION_COMMIT,   // TC
		ROW_END_CONTROL,      // RE
		SAVEPOINT_COMMIT,     // SC
		SAVEPOINT_CONTINUE,   // SE
		FULL_ROLLBACK,        // R0
		EndControl{'R', '9'}, // R9
		EndControl{'S', '1'}, // S1
	}
	for _, ec := range validEndControls {
		t.Run("valid_end_"+ec.String(), func(t *testing.T) {
			dataRow := &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   ec,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: value,
					},
				},
			}
			if err := dataRow.Validate(); err != nil {
				t.Errorf("Valid end control '%s' should pass validation: %v", ec.String(), err)
			}
		})
	}
}

// Test_S_005_FR_011_NilInputValidation tests FR-011: System MUST reject DataRows with nil payload, nil header, or empty/zero UUID
func Test_S_005_FR_011_NilInputValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`

	// Test nil header
	t.Run("nil_header", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       nil,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Nil header should be rejected")
		}
	})

	// Test nil payload
	t.Run("nil_payload", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   nil,
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Nil payload should be rejected")
		}
	})

	// Test zero UUID
	t.Run("zero_uuid", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   uuid.Nil,
					Value: value,
				},
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Zero UUID should be rejected")
		}
	})

	// Test empty value
	t.Run("empty_value", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: "",
				},
			},
		}
		if err := dataRow.Validate(); err == nil {
			t.Error("Empty value should be rejected")
		}
	})
}

// Test_S_005_FR_012_ChildStructValidation tests FR-012: System MUST validate all child structs during construction before parent validation
func Test_S_005_FR_012_ChildStructValidation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	value := `{"test":"value"}`

	// Test that payload validation happens before DataRow validation
	t.Run("payload_validation_first", func(t *testing.T) {
		// Create payload with invalid UUIDv4
		invalidPayload := &DataRowPayload{
			Key:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), // v4
			Value: value,
		}

		// Validate payload first (should fail)
		if err := invalidPayload.Validate(); err == nil {
			t.Error("Invalid payload should fail validation")
		}

		// Create DataRow with invalid payload
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   invalidPayload,
			},
		}

		// DataRow validation should fail because payload validation fails
		if err := dataRow.Validate(); err == nil {
			t.Error("DataRow with invalid payload should fail validation")
		}
	})

	// Test that baseRow validation happens before DataRow-specific validation
	t.Run("baseRow_validation_first", func(t *testing.T) {
		// Create DataRow with invalid start control but valid payload
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: CHECKSUM_ROW, // Invalid for DataRow
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}

		// baseRow.Validate() should pass (CHECKSUM_ROW is valid for baseRow)
		// But DataRow.Validate() should fail
		if err := dataRow.baseRow.Validate(); err != nil {
			t.Logf("baseRow validation error (expected for this test setup): %v", err)
		}

		// DataRow.Validate() should fail due to invalid start control
		if err := dataRow.Validate(); err == nil {
			t.Error("DataRow with invalid start control should fail validation")
		}
	})

	// Test complete validation chain with valid data
	t.Run("complete_validation_chain", func(t *testing.T) {
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload: &DataRowPayload{
					Key:   key,
					Value: value,
				},
			},
		}

		// All validations should pass
		if err := dataRow.RowPayload.Validate(); err != nil {
			t.Errorf("Payload validation should pass: %v", err)
		}

		if err := dataRow.baseRow.Validate(); err != nil {
			t.Errorf("baseRow validation should pass: %v", err)
		}

		if err := dataRow.Validate(); err != nil {
			t.Errorf("DataRow validation should pass: %v", err)
		}
	})
}
