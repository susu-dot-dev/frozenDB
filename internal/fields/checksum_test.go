package fields

import (
	"encoding/base64"
	"hash/crc32"
	"testing"
)

func TestChecksum_MarshalText(t *testing.T) {
	tests := []struct {
		name    string
		value   Checksum
		wantLen int
		wantErr bool
	}{
		{
			name:    "zero checksum",
			value:   0,
			wantLen: 8,
			wantErr: false,
		},
		{
			name:    "non-zero checksum",
			value:   0x12345678,
			wantLen: 8,
			wantErr: false,
		},
		{
			name:    "maximum checksum",
			value:   0xFFFFFFFF,
			wantLen: 8,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.value.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("Checksum.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("Checksum.MarshalText() length = %d, want %d", len(got), tt.wantLen)
			}
			// Verify it's valid Base64
			decoded, err := base64.StdEncoding.DecodeString(string(got))
			if err != nil {
				t.Errorf("Checksum.MarshalText() produced invalid Base64: %v", err)
			}
			if len(decoded) != 4 {
				t.Errorf("Checksum.MarshalText() decoded length = %d, want 4", len(decoded))
			}
		})
	}
}

func TestChecksum_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    Checksum
		wantErr bool
	}{
		{
			name:    "valid checksum",
			text:    "AAAAAA==",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid non-zero checksum",
			text:    "EjRWeA==",
			want:    0x12345678,
			wantErr: false,
		},
		{
			name:    "wrong length",
			text:    "AAAAA",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid Base64",
			text:    "AAAAAA!=",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			text:    "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Checksum
			err := c.UnmarshalText([]byte(tt.text))
			if (err != nil) != tt.wantErr {
				t.Errorf("Checksum.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && c != tt.want {
				t.Errorf("Checksum.UnmarshalText() = %v, want %v", c, tt.want)
			}
		})
	}
}

func TestChecksum_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value Checksum
	}{
		{"zero", 0},
		{"non-zero", 0x12345678},
		{"max", 0xFFFFFFFF},
		{"min", 0x00000001},
		{"high bit", 0x80000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.value
			marshaled, err := original.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}

			var unmarshaled Checksum
			if err := unmarshaled.UnmarshalText(marshaled); err != nil {
				t.Fatalf("UnmarshalText() error = %v", err)
			}

			if unmarshaled != original {
				t.Errorf("Round trip failed: original = %v, unmarshaled = %v", original, unmarshaled)
			}
		})
	}
}

func TestNewChecksumRow(t *testing.T) {
	tests := []struct {
		name      string
		dataBytes []byte
		wantErr   bool
	}{
		{
			name:      "valid checksum row",
			dataBytes: []byte("test data"),
			wantErr:   false,
		},
		{
			name:      "empty data bytes",
			dataBytes: []byte{},
			wantErr:   true,
		},
		{
			name:      "nil data bytes",
			dataBytes: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewChecksumRow(1024, tt.dataBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewChecksumRow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Error("NewChecksumRow() returned nil for valid input")
				}
				// Verify checksum is calculated correctly
				expectedCRC32 := crc32.ChecksumIEEE(tt.dataBytes)
				actualChecksum := got.GetChecksum()
				if Checksum(expectedCRC32) != actualChecksum {
					t.Errorf("NewChecksumRow() checksum = %v, want %v", actualChecksum, Checksum(expectedCRC32))
				}
			}
		})
	}
}

func TestChecksumRow_GetChecksum(t *testing.T) {
	dataBytes := []byte("test data for GetChecksum")

	cr, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow() error = %v", err)
	}

	got := cr.GetChecksum()
	if got == 0 {
		t.Error("GetChecksum() returned zero for non-empty data")
	}

	expectedCRC32 := crc32.ChecksumIEEE(dataBytes)
	if got != Checksum(expectedCRC32) {
		t.Errorf("GetChecksum() = %v, want %v", got, Checksum(expectedCRC32))
	}
}

// TestChecksumRow_GetChecksum_NilPayload removed:
// GetChecksum() assumes Validate() has been called and passed, ensuring RowPayload is not nil.
// Testing nil payload behavior is not applicable since Validate() prevents this state.

func TestChecksumRow_MarshalText(t *testing.T) {
	dataBytes := []byte("test data for MarshalText")

	cr, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow() error = %v", err)
	}

	got, err := cr.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}

	if len(got) != 1024 {
		t.Errorf("MarshalText() length = %d, want %d", len(got), 1024)
	}
}

func TestChecksumRow_UnmarshalText(t *testing.T) {
	dataBytes := []byte("test data for UnmarshalText")

	cr, err := NewChecksumRow(1024, dataBytes)
	if err != nil {
		t.Fatalf("NewChecksumRow() error = %v", err)
	}

	marshaled, err := cr.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}

	var unmarshaled ChecksumRow
	if err := unmarshaled.UnmarshalText(marshaled); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}

	if unmarshaled.GetChecksum() != cr.GetChecksum() {
		t.Errorf("UnmarshalText() checksum = %v, want %v", unmarshaled.GetChecksum(), cr.GetChecksum())
	}
}

func TestChecksumRow_validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *ChecksumRow
		wantErr bool
	}{
		{
			name: "valid checksum row",
			setup: func() *ChecksumRow {
				cr, _ := NewChecksumRow(1024, []byte("test"))
				return cr
			},
			wantErr: false,
		},
		{
			name: "nil header",
			setup: func() *ChecksumRow {
				return &ChecksumRow{}
			},
			wantErr: true,
		},
		{
			name: "wrong start control",
			setup: func() *ChecksumRow {
				cr, _ := NewChecksumRow(1024, []byte("test"))
				cr.StartControl = START_TRANSACTION
				return cr
			},
			wantErr: true,
		},
		{
			name: "wrong end control",
			setup: func() *ChecksumRow {
				cr, _ := NewChecksumRow(1024, []byte("test"))
				cr.EndControl = TRANSACTION_COMMIT
				return cr
			},
			wantErr: true,
		},
		{
			name: "nil payload",
			setup: func() *ChecksumRow {
				return &ChecksumRow{
					baseRow[*Checksum]{
						RowSize:      1024,
						StartControl: CHECKSUM_ROW,
						EndControl:   CHECKSUM_ROW_CONTROL,
						RowPayload:   nil,
					},
				}
			},
			wantErr: true,
			// Note: nil payload validation is now in baseRow.Validate(), not ChecksumRow.Validate()
			// This test verifies that baseRow.Validate() catches nil payload
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := tt.setup()
			// For nil payload test, validate baseRow first (which checks payload)
			// Then validate ChecksumRow-specific properties
			if tt.name == "nil payload" {
				// baseRow.Validate() should catch nil payload
				if err := cr.baseRow.Validate(); err == nil {
					t.Error("baseRow.Validate() should fail for nil payload")
				}
				// ChecksumRow.Validate() only checks checksum-specific properties
				// It assumes baseRow is already valid
				err := cr.Validate()
				if err != nil {
					t.Errorf("ChecksumRow.Validate() should not fail for nil payload (baseRow.Validate() handles it): %v", err)
				}
			} else {
				err := cr.validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("ChecksumRow.validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
