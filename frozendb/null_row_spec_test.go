package frozendb

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
)

// NilUUIDBase64 is the expected Base64 encoding of uuid.Nil
const NilUUIDBase64 = "AAAAAAAAAAAAAAAAAAAAAA=="

// Test_S_010_FR_001_NullRowStartControlField tests FR-001: NullRow struct MUST have start_control field always set to 'T' (transaction begin)
func Test_S_010_FR_001_NullRowStartControlField(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	// Create a valid NullRow
	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	// Validate that start_control is 'T'
	if nullRow.StartControl != START_TRANSACTION {
		t.Errorf("NullRow start_control must be 'T', got '%c'", nullRow.StartControl)
	}

	// Validate should pass
	if err := nullRow.Validate(); err != nil {
		t.Errorf("Valid NullRow should pass validation: %v", err)
	}
}

// Test_S_010_FR_002_NullRowEndControlField tests FR-002: NullRow struct MUST have end_control field always set to 'NR' (null row)
func Test_S_010_FR_002_NullRowEndControlField(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	// Create a valid NullRow
	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	// Validate that end_control is 'NR'
	if nullRow.EndControl != NULL_ROW_CONTROL {
		t.Errorf("NullRow end_control must be 'NR', got '%s'", nullRow.EndControl.String())
	}

	// Validate should pass
	if err := nullRow.Validate(); err != nil {
		t.Errorf("Valid NullRow should pass validation: %v", err)
	}
}

// Test_S_010_FR_003_NullRowUUIDNilBase64 tests FR-003: NullRow struct MUST use uuid.Nil encoded as Base64 "AAAAAAAAAAAAAAAAAAAAAA=="
func Test_S_010_FR_003_NullRowUUIDNilBase64(t *testing.T) {
	// Verify uuid.Nil encodes to expected Base64
	nilBytes := uuid.Nil[:]
	encoded := base64.StdEncoding.EncodeToString(nilBytes)
	if encoded != NilUUIDBase64 {
		t.Errorf("uuid.Nil Base64 encoding mismatch: expected %s, got %s", NilUUIDBase64, encoded)
	}

	// Test NullRowPayload with uuid.Nil
	payload := &NullRowPayload{Key: uuid.Nil}
	marshaledPayload, err := payload.MarshalText()
	if err != nil {
		t.Fatalf("NullRowPayload.MarshalText() failed: %v", err)
	}

	// Should produce exactly 24 bytes matching the Base64 encoding
	if len(marshaledPayload) != 24 {
		t.Errorf("NullRowPayload marshaled length mismatch: expected 24, got %d", len(marshaledPayload))
	}

	if string(marshaledPayload) != NilUUIDBase64 {
		t.Errorf("NullRowPayload.MarshalText() mismatch: expected %s, got %s", NilUUIDBase64, string(marshaledPayload))
	}
}

// Test_S_010_FR_004_NullRowValidateMethod tests FR-004: NullRow struct MUST have a Validate() method that verifies all required field values
func Test_S_010_FR_004_NullRowValidateMethod(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	t.Run("valid_nullrow_passes_validation", func(t *testing.T) {
		nullRow := &NullRow{
			baseRow: baseRow[*NullRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   NULL_ROW_CONTROL,
				RowPayload:   &NullRowPayload{Key: uuid.Nil},
			},
		}

		err := nullRow.Validate()
		if err != nil {
			t.Errorf("Valid NullRow should pass validation: %v", err)
		}
	})

	t.Run("validate_is_idempotent", func(t *testing.T) {
		nullRow := &NullRow{
			baseRow: baseRow[*NullRowPayload]{
				Header:       header,
				StartControl: START_TRANSACTION,
				EndControl:   NULL_ROW_CONTROL,
				RowPayload:   &NullRowPayload{Key: uuid.Nil},
			},
		}

		// Call Validate multiple times - should always return same result
		err1 := nullRow.Validate()
		err2 := nullRow.Validate()
		err3 := nullRow.Validate()

		if err1 != nil || err2 != nil || err3 != nil {
			t.Errorf("Validate() should be idempotent and always pass for valid NullRow")
		}
	})
}

// Test_S_010_FR_005_NullRowMarshalMethod tests FR-005: NullRow struct MUST have a Marshal() method that produces binary data matching v1 file format
func Test_S_010_FR_005_NullRowMarshalMethod(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	rowBytes, err := nullRow.MarshalText()
	if err != nil {
		t.Fatalf("NullRow.MarshalText() failed: %v", err)
	}

	// Verify exact byte layout per v1_file_format.md section 8.7
	rowSize := header.GetRowSize()
	if len(rowBytes) != rowSize {
		t.Errorf("Row size mismatch: expected %d, got %d", rowSize, len(rowBytes))
	}

	// Position [0]: ROW_START (0x1F)
	if rowBytes[0] != ROW_START {
		t.Errorf("ROW_START mismatch: expected 0x%02X, got 0x%02X", ROW_START, rowBytes[0])
	}

	// Position [1]: start_control='T'
	if rowBytes[1] != byte(START_TRANSACTION) {
		t.Errorf("Start control mismatch: expected 'T' (0x%02X), got 0x%02X", byte(START_TRANSACTION), rowBytes[1])
	}

	// Positions [2..25]: uuid_base64 (24 bytes) = "AAAAAAAAAAAAAAAAAAAAAA=="
	uuidBase64 := string(rowBytes[2:26])
	if uuidBase64 != NilUUIDBase64 {
		t.Errorf("UUID Base64 mismatch: expected %s, got %s", NilUUIDBase64, uuidBase64)
	}

	// Positions [26..N-6]: NULL_BYTE padding (no JSON value for NullRow)
	for i := 26; i < rowSize-5; i++ {
		if rowBytes[i] != NULL_BYTE {
			t.Errorf("Padding byte at position %d should be NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, rowBytes[i])
		}
	}

	// Positions [N-5..N-4]: end_control = 'NR'
	endControl := string(rowBytes[rowSize-5 : rowSize-3])
	if endControl != "NR" {
		t.Errorf("End control mismatch: expected 'NR', got '%s'", endControl)
	}

	// Positions [N-3..N-2]: parity_bytes (2 uppercase hex chars)
	parityBytes := rowBytes[rowSize-3 : rowSize-1]
	for i, b := range parityBytes {
		if (b < '0' || b > '9') && (b < 'A' || b > 'F') {
			t.Errorf("Parity byte at position %d is not valid hex: 0x%02X", i, b)
		}
	}

	// Position [N-1]: ROW_END (0x0A)
	if rowBytes[rowSize-1] != ROW_END {
		t.Errorf("ROW_END mismatch: expected 0x%02X, got 0x%02X", ROW_END, rowBytes[rowSize-1])
	}
}

// Test_S_010_FR_006_NullRowUnmarshalMethod tests FR-006: NullRow struct MUST have an Unmarshal() method that can parse binary data into a NullRow instance
func Test_S_010_FR_006_NullRowUnmarshalMethod(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	// Create and marshal a valid NullRow
	originalRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	rowBytes, err := originalRow.MarshalText()
	if err != nil {
		t.Fatalf("Original NullRow.MarshalText() failed: %v", err)
	}

	// Unmarshal into a new NullRow
	deserializedRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header: header,
		},
	}

	if err := deserializedRow.UnmarshalText(rowBytes); err != nil {
		t.Fatalf("NullRow.UnmarshalText() failed: %v", err)
	}

	// Validate deserialized row
	if err := deserializedRow.Validate(); err != nil {
		t.Errorf("Deserialized NullRow should pass validation: %v", err)
	}

	// Verify round-trip preservation
	if deserializedRow.GetKey() != uuid.Nil {
		t.Errorf("Deserialized key mismatch: expected uuid.Nil, got %s", deserializedRow.GetKey())
	}

	if deserializedRow.StartControl != START_TRANSACTION {
		t.Errorf("Deserialized StartControl mismatch: expected 'T', got '%c'", deserializedRow.StartControl)
	}

	if deserializedRow.EndControl != NULL_ROW_CONTROL {
		t.Errorf("Deserialized EndControl mismatch: expected 'NR', got '%s'", deserializedRow.EndControl.String())
	}

	// Verify re-marshaling produces identical bytes
	remarshaled, err := deserializedRow.MarshalText()
	if err != nil {
		t.Fatalf("Re-marshaling failed: %v", err)
	}

	if len(remarshaled) != len(rowBytes) {
		t.Errorf("Re-marshaled length mismatch: expected %d, got %d", len(rowBytes), len(remarshaled))
	}

	for i := 0; i < len(rowBytes); i++ {
		if remarshaled[i] != rowBytes[i] {
			t.Errorf("Byte mismatch at position %d: expected 0x%02X, got 0x%02X", i, rowBytes[i], remarshaled[i])
		}
	}
}

// Test_S_010_FR_007_NullRowParityBytesCalculation tests FR-007: NullRow struct MUST calculate correct parity bytes for marshaled data
func Test_S_010_FR_007_NullRowParityBytesCalculation(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	rowBytes, err := nullRow.MarshalText()
	if err != nil {
		t.Fatalf("NullRow.MarshalText() failed: %v", err)
	}

	rowSize := header.GetRowSize()

	// Calculate expected parity: XOR all bytes from [0] through [rowSize-4] (inclusive)
	var xor byte = 0
	for i := 0; i < rowSize-3; i++ {
		xor ^= rowBytes[i]
	}

	// Encode XOR result as uppercase hex
	expectedParityHigh := (xor >> 4) & 0x0F
	expectedParityLow := xor & 0x0F

	expectedParity := [2]byte{}
	if expectedParityHigh < 10 {
		expectedParity[0] = '0' + expectedParityHigh
	} else {
		expectedParity[0] = 'A' + expectedParityHigh - 10
	}
	if expectedParityLow < 10 {
		expectedParity[1] = '0' + expectedParityLow
	} else {
		expectedParity[1] = 'A' + expectedParityLow - 10
	}

	// Extract actual parity from marshaled bytes
	actualParity := [2]byte{rowBytes[rowSize-3], rowBytes[rowSize-2]}

	if actualParity != expectedParity {
		t.Errorf("Parity mismatch: expected [%c, %c], got [%c, %c]", expectedParity[0], expectedParity[1], actualParity[0], actualParity[1])
	}

	// Verify parity bytes are valid uppercase hex
	for i, b := range actualParity {
		if (b < '0' || b > '9') && (b < 'A' || b > 'F') {
			t.Errorf("Parity byte %d is not valid uppercase hex: 0x%02X ('%c')", i, b, b)
		}
	}
}

// Test_S_010_FR_008_NullRowPaddingCorrectness tests FR-008: NullRow struct MUST handle padding correctly to match fixed row width
func Test_S_010_FR_008_NullRowPaddingCorrectness(t *testing.T) {
	// Test with multiple row sizes to verify padding calculation
	rowSizes := []int{128, 256, 512, 1024, 2048}

	for _, rowSize := range rowSizes {
		t.Run("row_size_"+string(rune('0'+rowSize/100)), func(t *testing.T) {
			header := &Header{
				signature: "fDB",
				version:   1,
				rowSize:   rowSize,
				skewMs:    5000,
			}

			nullRow := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			}

			rowBytes, err := nullRow.MarshalText()
			if err != nil {
				t.Fatalf("NullRow.MarshalText() failed for rowSize=%d: %v", rowSize, err)
			}

			// Verify exact row length
			if len(rowBytes) != rowSize {
				t.Errorf("Row length mismatch: expected %d, got %d", rowSize, len(rowBytes))
			}

			// Verify padding bytes: positions [26..rowSize-6] should all be NULL_BYTE
			// Per spec: padding_bytes = row_size - 31
			// Where 31 = 1 (ROW_START) + 1 (start_control) + 24 (UUID) + 2 (end_control) + 2 (parity) + 1 (ROW_END)
			expectedPaddingLength := rowSize - 31
			actualPaddingStart := 26
			actualPaddingEnd := rowSize - 5

			for i := actualPaddingStart; i < actualPaddingEnd; i++ {
				if rowBytes[i] != NULL_BYTE {
					t.Errorf("Padding byte at position %d should be NULL_BYTE (0x%02X), got 0x%02X", i, NULL_BYTE, rowBytes[i])
				}
			}

			// Verify padding length matches expected
			actualPaddingLength := actualPaddingEnd - actualPaddingStart
			if actualPaddingLength != expectedPaddingLength {
				t.Errorf("Padding length mismatch: expected %d, got %d", expectedPaddingLength, actualPaddingLength)
			}
		})
	}
}

// Test_S_010_FR_009_ValidateFailsInvalidStartControl tests FR-009: NullRow validation MUST fail if start_control is not 'T'
func Test_S_010_FR_009_ValidateFailsInvalidStartControl(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	testCases := []struct {
		name         string
		startControl StartControl
		shouldFail   bool
	}{
		{"valid_T", START_TRANSACTION, false},
		{"invalid_R", ROW_CONTINUE, true},
		{"invalid_C", CHECKSUM_ROW, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nullRow := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: tc.startControl,
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			}

			err := nullRow.Validate()
			if tc.shouldFail {
				if err == nil {
					t.Errorf("NullRow with start_control='%c' should fail validation", tc.startControl)
				}
				// Verify it's an InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("NullRow with valid start_control='%c' should pass validation: %v", tc.startControl, err)
				}
			}
		})
	}
}

// Test_S_010_FR_010_ValidateFailsInvalidEndControl tests FR-010: NullRow validation MUST fail if end_control is not 'NR'
func Test_S_010_FR_010_ValidateFailsInvalidEndControl(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	testCases := []struct {
		name       string
		endControl EndControl
		shouldFail bool
	}{
		{"valid_NR", NULL_ROW_CONTROL, false},
		{"invalid_TC", TRANSACTION_COMMIT, true},
		{"invalid_RE", ROW_END_CONTROL, true},
		{"invalid_SC", SAVEPOINT_COMMIT, true},
		{"invalid_SE", SAVEPOINT_CONTINUE, true},
		{"invalid_CS", CHECKSUM_ROW_CONTROL, true},
		{"invalid_R0", FULL_ROLLBACK, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nullRow := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   tc.endControl,
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			}

			err := nullRow.Validate()
			if tc.shouldFail {
				if err == nil {
					t.Errorf("NullRow with end_control='%s' should fail validation", tc.endControl.String())
				}
				// Verify it's an InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("NullRow with valid end_control='%s' should pass validation: %v", tc.endControl.String(), err)
				}
			}
		})
	}
}

// Test_S_010_FR_011_ValidateFailsInvalidUUID tests FR-011: NullRow validation MUST fail if UUID is not uuid.Nil
// Per 004-struct-validation FR-006, NullRow.Validate() assumes child structs are already valid.
// UUID validation is performed by NullRowPayload.Validate(), which the constructing code must call.
func Test_S_010_FR_011_ValidateFailsInvalidUUID(t *testing.T) {
	// Generate a non-nil UUID
	nonNilUUID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	testCases := []struct {
		name       string
		key        uuid.UUID
		shouldFail bool
	}{
		{"valid_uuid_nil", uuid.Nil, false},
		{"invalid_uuidv7", nonNilUUID, true},
		{"invalid_uuidv4", uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test NullRowPayload.Validate() - this is where UUID validation occurs
			payload := &NullRowPayload{Key: tc.key}
			err := payload.Validate()

			if tc.shouldFail {
				if err == nil {
					t.Errorf("NullRowPayload with UUID=%s should fail validation", tc.key)
				}
				// Verify it's an InvalidInputError
				if _, ok := err.(*InvalidInputError); !ok {
					t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("NullRowPayload with valid UUID=%s should pass validation: %v", tc.key, err)
				}
			}
		})
	}
}

// Test_S_010_FR_012_MarshalReturnsInvalidInputError tests FR-012: Marshal() method MUST return InvalidInputError if row structure is invalid
func Test_S_010_FR_012_MarshalReturnsInvalidInputError(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	testCases := []struct {
		name     string
		nullRow  *NullRow
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "invalid_start_control",
			nullRow: &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: ROW_CONTINUE, // Invalid for NullRow
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			},
			wantErr: true,
			errCheck: func(err error) bool {
				_, ok := err.(*InvalidInputError)
				return ok
			},
		},
		{
			name: "invalid_end_control",
			nullRow: &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT, // Invalid for NullRow
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			},
			wantErr: true,
			errCheck: func(err error) bool {
				_, ok := err.(*InvalidInputError)
				return ok
			},
		},
		// Note: invalid_uuid case removed - per FR-006 from 004-struct-validation,
		// NullRow.Validate() assumes child structs (NullRowPayload) are already valid.
		// UUID validation is the responsibility of NullRowPayload.Validate(), which
		// must be called by the code constructing the payload.
		{
			name: "nil_header",
			nullRow: &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       nil, // Invalid
					StartControl: START_TRANSACTION,
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   &NullRowPayload{Key: uuid.Nil},
				},
			},
			wantErr: true,
			errCheck: func(err error) bool {
				_, ok := err.(*InvalidInputError)
				return ok
			},
		},
		{
			name: "nil_payload",
			nullRow: &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   nil, // Invalid
				},
			},
			wantErr: true,
			errCheck: func(err error) bool {
				_, ok := err.(*InvalidInputError)
				return ok
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.nullRow.MarshalText()
			if tc.wantErr {
				if err == nil {
					t.Error("MarshalText() should return error for invalid NullRow")
				} else if !tc.errCheck(err) {
					t.Errorf("Expected InvalidInputError, got %T: %v", err, err)
				}
			} else {
				if err != nil {
					t.Errorf("MarshalText() should not return error: %v", err)
				}
			}
		})
	}
}

// Test_S_010_FR_013_UnmarshalReturnsCorruptDatabaseError tests FR-013: Unmarshal() method MUST return CorruptDatabaseError wrapping validation errors if input data format is invalid
func Test_S_010_FR_013_UnmarshalReturnsCorruptDatabaseError(t *testing.T) {
	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	// Create a valid NullRow and marshal it
	validRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			Header:       header,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: uuid.Nil},
		},
	}

	validBytes, err := validRow.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal valid NullRow: %v", err)
	}

	rowSize := header.GetRowSize()

	testCases := []struct {
		name    string
		corrupt func([]byte) []byte
	}{
		{
			name: "invalid_row_start",
			corrupt: func(b []byte) []byte {
				corrupted := make([]byte, len(b))
				copy(corrupted, b)
				corrupted[0] = 0xFF // Invalid ROW_START
				return corrupted
			},
		},
		{
			name: "invalid_start_control",
			corrupt: func(b []byte) []byte {
				corrupted := make([]byte, len(b))
				copy(corrupted, b)
				corrupted[1] = 'R' // Invalid for NullRow (should be 'T')
				return corrupted
			},
		},
		{
			name: "invalid_end_control",
			corrupt: func(b []byte) []byte {
				corrupted := make([]byte, len(b))
				copy(corrupted, b)
				corrupted[rowSize-5] = 'T' // Change 'N' to 'T'
				corrupted[rowSize-4] = 'C' // Change 'R' to 'C' (making it 'TC')
				return corrupted
			},
		},
		{
			name: "invalid_parity",
			corrupt: func(b []byte) []byte {
				corrupted := make([]byte, len(b))
				copy(corrupted, b)
				corrupted[rowSize-3] = 'X' // Invalid parity byte
				return corrupted
			},
		},
		{
			name: "invalid_row_end",
			corrupt: func(b []byte) []byte {
				corrupted := make([]byte, len(b))
				copy(corrupted, b)
				corrupted[rowSize-1] = 0xFF // Invalid ROW_END
				return corrupted
			},
		},
		{
			name: "wrong_length",
			corrupt: func(b []byte) []byte {
				return b[:len(b)-10] // Truncated
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			corruptedBytes := tc.corrupt(validBytes)

			deserializedRow := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					Header: header,
				},
			}

			err := deserializedRow.UnmarshalText(corruptedBytes)
			if err == nil {
				t.Error("UnmarshalText() should return error for corrupted data")
			}

			// Verify it's a CorruptDatabaseError
			if _, ok := err.(*CorruptDatabaseError); !ok {
				t.Errorf("Expected CorruptDatabaseError, got %T: %v", err, err)
			}
		})
	}
}
