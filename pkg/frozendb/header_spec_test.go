package frozendb

import (
	"os"
	"testing"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Test_S_008_FR_001_HeaderGoFileCreation(t *testing.T) {
	exists := fileExists("header.go")
	if !exists {
		t.Error("header.go file should exist in frozendb package directory")
	}
}

func Test_S_008_FR_002_HeaderMarshalTextMethod(t *testing.T) {
	header, _ := NewHeader(HEADER_SIGNATURE, 1, 1024, 5000)

	if err := header.Validate(); err != nil {
		t.Errorf("Validate() should succeed for valid Header: %v", err)
	}

	headerBytes, err := header.MarshalText()
	if err != nil {
		t.Errorf("MarshalText() should succeed for valid Header: %v", err)
	}

	if len(headerBytes) != 64 {
		t.Errorf("MarshalText() should return exactly 64 bytes, got %d", len(headerBytes))
	}

	if headerBytes[63] != '\n' {
		t.Errorf("Byte 63 should be newline, got 0x%02x", headerBytes[63])
	}
}

func Test_S_008_FR_003_DirectStructValidationPattern(t *testing.T) {
	header, _ := NewHeader(HEADER_SIGNATURE, 1, 1024, 5000)

	err := header.Validate()
	if err != nil {
		t.Errorf("Validate() should succeed for valid Header: %v", err)
	}

	invalidHeader, _ := NewHeader("INVALID", 1, 1024, 5000)

	err = invalidHeader.Validate()
	if err == nil {
		t.Error("Validate() should fail for invalid signature")
	}
}

func Test_S_008_FR_004_SingleHeaderCreationPattern(t *testing.T) {
	header, _ := NewHeader(HEADER_SIGNATURE, 1, 1024, 5000)

	if err := header.Validate(); err != nil {
		t.Errorf("Validate() should succeed: %v", err)
	}

	headerBytes, err := header.MarshalText()
	if err != nil {
		t.Errorf("MarshalText() should succeed: %v", err)
	}

	if len(headerBytes) != 64 {
		t.Errorf("Expected 64 bytes from MarshalText(), got %d", len(headerBytes))
	}
}

func Test_S_008_FR_005_IdenticalByteFormatCompatibility(t *testing.T) {
	rowSize := 1024
	skewMs := 5000

	header, _ := NewHeader(HEADER_SIGNATURE, 1, rowSize, skewMs)

	if err := header.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	newBytes, err := header.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() failed: %v", err)
	}

	if len(newBytes) != 64 {
		t.Errorf("Expected 64 bytes, got %d", len(newBytes))
	}

	expected := []byte(`{"sig":"fDB","ver":1,"row_size":1024,"skew_ms":5000}`)
	if string(newBytes[:len(expected)]) != string(expected) {
		t.Errorf("JSON content mismatch")
	}

	if newBytes[63] != '\n' {
		t.Errorf("Expected newline at byte 63, got 0x%02x", newBytes[63])
	}
}

func Test_S_008_FR_006_RemoveGenerateHeaderFunction(t *testing.T) {
	header, _ := NewHeader(HEADER_SIGNATURE, 1, 1024, 5000)

	if err := header.Validate(); err != nil {
		t.Errorf("Validate() should succeed: %v", err)
	}

	_, err := header.MarshalText()
	if err != nil {
		t.Errorf("MarshalText() should work as replacement for generateHeader(): %v", err)
	}
}

func Test_S_008_FR_009_MaintainExistingHeaderGetterMethods(t *testing.T) {
	header, _ := NewHeader(HEADER_SIGNATURE, 1, 1024, 5000)

	if header.GetSignature() != HEADER_SIGNATURE {
		t.Errorf("GetSignature() = %s, want %s", header.GetSignature(), HEADER_SIGNATURE)
	}

	if header.GetVersion() != 1 {
		t.Errorf("GetVersion() = %d, want 1", header.GetVersion())
	}

	if header.GetRowSize() != 1024 {
		t.Errorf("GetRowSize() = %d, want 1024", header.GetRowSize())
	}

	if header.GetSkewMs() != 5000 {
		t.Errorf("GetSkewMs() = %d, want 5000", header.GetSkewMs())
	}
}

func Test_S_008_FR_011_Exact64ByteHeaderFormatCompatibility(t *testing.T) {
	testCases := []struct {
		name    string
		rowSize int
		skewMs  int
	}{
		{"minimum values", 128, 0},
		{"typical values", 1024, 5000},
		{"maximum row size", 65536, 86400000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			header, _ := NewHeader(HEADER_SIGNATURE, 1, tc.rowSize, tc.skewMs)

			if err := header.Validate(); err != nil {
				t.Fatalf("Validate() failed: %v", err)
			}

			headerBytes, err := header.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() failed: %v", err)
			}

			if len(headerBytes) != 64 {
				t.Errorf("MarshalText() returned %d bytes, want 64", len(headerBytes))
			}

			if headerBytes[63] != '\n' {
				t.Errorf("Byte 63 = 0x%02x, want 0x0A (newline)", headerBytes[63])
			}

			parsedHeader := &Header{}
			if err := parsedHeader.UnmarshalText(headerBytes); err != nil {
				t.Fatalf("UnmarshalText() failed: %v", err)
			}

			if parsedHeader.GetRowSize() != tc.rowSize {
				t.Errorf("Round-trip rowSize = %d, want %d", parsedHeader.GetRowSize(), tc.rowSize)
			}

			if parsedHeader.GetSkewMs() != tc.skewMs {
				t.Errorf("Round-trip skewMs = %d, want %d", parsedHeader.GetSkewMs(), tc.skewMs)
			}
		})
	}
}
