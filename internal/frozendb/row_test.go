package frozendb

import (
	"testing"
)

func TestStartControl_MarshalText(t *testing.T) {
	tests := []struct {
		name    string
		sc      StartControl
		want    byte
		wantErr bool
	}{
		{
			name:    "START_TRANSACTION",
			sc:      START_TRANSACTION,
			want:    'T',
			wantErr: false,
		},
		{
			name:    "ROW_CONTINUE",
			sc:      ROW_CONTINUE,
			want:    'R',
			wantErr: false,
		},
		{
			name:    "CHECKSUM_ROW",
			sc:      CHECKSUM_ROW,
			want:    'C',
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.sc.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("StartControl.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != 1 {
				t.Errorf("StartControl.MarshalText() length = %d, want 1", len(got))
			}
			if len(got) > 0 && got[0] != tt.want {
				t.Errorf("StartControl.MarshalText() = %c, want %c", got[0], tt.want)
			}
		})
	}
}

func TestStartControl_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		text    []byte
		want    StartControl
		wantErr bool
	}{
		{
			name:    "valid START_TRANSACTION",
			text:    []byte{'T'},
			want:    START_TRANSACTION,
			wantErr: false,
		},
		{
			name:    "valid ROW_CONTINUE",
			text:    []byte{'R'},
			want:    ROW_CONTINUE,
			wantErr: false,
		},
		{
			name:    "valid CHECKSUM_ROW",
			text:    []byte{'C'},
			want:    CHECKSUM_ROW,
			wantErr: false,
		},
		{
			name:    "invalid byte",
			text:    []byte{'X'},
			wantErr: true,
		},
		{
			name:    "wrong length - too short",
			text:    []byte{},
			wantErr: true,
		},
		{
			name:    "wrong length - too long",
			text:    []byte{'T', 'T'},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sc StartControl
			err := sc.UnmarshalText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartControl.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sc != tt.want {
				t.Errorf("StartControl.UnmarshalText() = %c, want %c", sc, tt.want)
			}
		})
	}
}

func TestStartControl_RoundTrip(t *testing.T) {
	controls := []struct {
		name  string
		value StartControl
	}{
		{"START_TRANSACTION", START_TRANSACTION},
		{"ROW_CONTINUE", ROW_CONTINUE},
		{"CHECKSUM_ROW", CHECKSUM_ROW},
	}

	for _, tt := range controls {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.value
			marshaled, err := original.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}

			var unmarshaled StartControl
			if err := unmarshaled.UnmarshalText(marshaled); err != nil {
				t.Fatalf("UnmarshalText() error = %v", err)
			}

			if unmarshaled != original {
				t.Errorf("Round trip failed: original = %c, unmarshaled = %c", original, unmarshaled)
			}
		})
	}
}

func TestEndControl_MarshalText(t *testing.T) {
	tests := []struct {
		name    string
		ec      EndControl
		want    [2]byte
		wantErr bool
	}{
		{
			name:    "TRANSACTION_COMMIT",
			ec:      TRANSACTION_COMMIT,
			want:    [2]byte{'T', 'C'},
			wantErr: false,
		},
		{
			name:    "ROW_END_CONTROL",
			ec:      ROW_END_CONTROL,
			want:    [2]byte{'R', 'E'},
			wantErr: false,
		},
		{
			name:    "CHECKSUM_ROW_CONTROL",
			ec:      CHECKSUM_ROW_CONTROL,
			want:    [2]byte{'C', 'S'},
			wantErr: false,
		},
		{
			name:    "SAVEPOINT_COMMIT",
			ec:      SAVEPOINT_COMMIT,
			want:    [2]byte{'S', 'C'},
			wantErr: false,
		},
		{
			name:    "SAVEPOINT_CONTINUE",
			ec:      SAVEPOINT_CONTINUE,
			want:    [2]byte{'S', 'E'},
			wantErr: false,
		},
		{
			name:    "FULL_ROLLBACK",
			ec:      FULL_ROLLBACK,
			want:    [2]byte{'R', '0'},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ec.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("EndControl.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != 2 {
				t.Errorf("EndControl.MarshalText() length = %d, want 2", len(got))
			}
			if len(got) >= 2 && (got[0] != tt.want[0] || got[1] != tt.want[1]) {
				t.Errorf("EndControl.MarshalText() = [%c, %c], want [%c, %c]", got[0], got[1], tt.want[0], tt.want[1])
			}
		})
	}
}

func TestEndControl_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		text    []byte
		want    EndControl
		wantErr bool
	}{
		{
			name:    "valid TRANSACTION_COMMIT",
			text:    []byte{'T', 'C'},
			want:    TRANSACTION_COMMIT,
			wantErr: false,
		},
		{
			name:    "valid ROW_END_CONTROL",
			text:    []byte{'R', 'E'},
			want:    ROW_END_CONTROL,
			wantErr: false,
		},
		{
			name:    "valid CHECKSUM_ROW_CONTROL",
			text:    []byte{'C', 'S'},
			want:    CHECKSUM_ROW_CONTROL,
			wantErr: false,
		},
		{
			name:    "valid SAVEPOINT_COMMIT",
			text:    []byte{'S', 'C'},
			want:    SAVEPOINT_COMMIT,
			wantErr: false,
		},
		{
			name:    "valid SAVEPOINT_CONTINUE",
			text:    []byte{'S', 'E'},
			want:    SAVEPOINT_CONTINUE,
			wantErr: false,
		},
		{
			name:    "valid FULL_ROLLBACK",
			text:    []byte{'R', '0'},
			want:    FULL_ROLLBACK,
			wantErr: false,
		},
		{
			name:    "valid R1 rollback",
			text:    []byte{'R', '1'},
			want:    EndControl{'R', '1'},
			wantErr: false,
		},
		{
			name:    "valid R9 rollback",
			text:    []byte{'R', '9'},
			want:    EndControl{'R', '9'},
			wantErr: false,
		},
		{
			name:    "valid S0 rollback",
			text:    []byte{'S', '0'},
			want:    EndControl{'S', '0'},
			wantErr: false,
		},
		{
			name:    "valid S1 rollback",
			text:    []byte{'S', '1'},
			want:    EndControl{'S', '1'},
			wantErr: false,
		},
		{
			name:    "valid S9 rollback",
			text:    []byte{'S', '9'},
			want:    EndControl{'S', '9'},
			wantErr: false,
		},
		{
			name:    "invalid - AA",
			text:    []byte{'A', 'A'},
			wantErr: true,
		},
		{
			name:    "invalid - XX",
			text:    []byte{'X', 'X'},
			wantErr: true,
		},
		{
			name:    "invalid - T followed by wrong char",
			text:    []byte{'T', 'Z'},
			wantErr: true,
		},
		{
			name:    "invalid - T followed by E",
			text:    []byte{'T', 'E'},
			wantErr: true,
		},
		{
			name:    "invalid - R followed by wrong char",
			text:    []byte{'R', 'Z'},
			wantErr: true,
		},
		{
			name:    "invalid - R followed by C",
			text:    []byte{'R', 'C'},
			wantErr: true,
		},
		{
			name:    "invalid - S followed by wrong char",
			text:    []byte{'S', 'Z'},
			wantErr: true,
		},
		{
			name:    "invalid - C followed by wrong char",
			text:    []byte{'C', 'C'},
			wantErr: true,
		},
		{
			name:    "invalid - C followed by E",
			text:    []byte{'C', 'E'},
			wantErr: true,
		},
		{
			name:    "invalid - R followed by non-digit non-E",
			text:    []byte{'R', 'A'},
			wantErr: true,
		},
		{
			name:    "invalid - S followed by non-digit non-C non-E",
			text:    []byte{'S', 'A'},
			wantErr: true,
		},
		{
			name:    "wrong length - too short",
			text:    []byte{'T'},
			wantErr: true,
		},
		{
			name:    "wrong length - too long",
			text:    []byte{'T', 'C', 'X'},
			wantErr: true,
		},
		{
			name:    "empty",
			text:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ec EndControl
			err := ec.UnmarshalText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("EndControl.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ec != tt.want {
				t.Errorf("EndControl.UnmarshalText() = [%c, %c], want [%c, %c]", ec[0], ec[1], tt.want[0], tt.want[1])
			}
		})
	}
}

func TestEndControl_String(t *testing.T) {
	tests := []struct {
		name string
		ec   EndControl
		want string
	}{
		{
			name: "TRANSACTION_COMMIT",
			ec:   TRANSACTION_COMMIT,
			want: "TC",
		},
		{
			name: "CHECKSUM_ROW_CONTROL",
			ec:   CHECKSUM_ROW_CONTROL,
			want: "CS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ec.String()
			if got != tt.want {
				t.Errorf("EndControl.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndControl_RoundTrip(t *testing.T) {
	controls := []EndControl{
		TRANSACTION_COMMIT,
		ROW_END_CONTROL,
		CHECKSUM_ROW_CONTROL,
		SAVEPOINT_COMMIT,
		SAVEPOINT_CONTINUE,
		FULL_ROLLBACK,
	}

	for _, original := range controls {
		t.Run(original.String(), func(t *testing.T) {
			marshaled, err := original.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}

			var unmarshaled EndControl
			if err := unmarshaled.UnmarshalText(marshaled); err != nil {
				t.Fatalf("UnmarshalText() error = %v", err)
			}

			if unmarshaled != original {
				t.Errorf("Round trip failed: original = [%c, %c], unmarshaled = [%c, %c]", original[0], original[1], unmarshaled[0], unmarshaled[1])
			}
		})
	}
}

func TestBaseRow_PaddingLength(t *testing.T) {
	tests := []struct {
		name   string
		header *Header
		want   int
	}{
		{
			name: "minimum row size",
			header: &Header{
				rowSize: 128,
				skewMs:  5000,
			},
			want: 128 - 7 - 8, // row_size - overhead - payload
		},
		{
			name: "medium row size",
			header: &Header{
				rowSize: 1024,
				skewMs:  5000,
			},
			want: 1024 - 7 - 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := &baseRow[*Checksum]{
				RowSize: tt.header.GetRowSize(),
			}
			// Create a zero Checksum and marshal it to get payload bytes
			zeroChecksum := Checksum(0)
			payloadBytes, err := zeroChecksum.MarshalText()
			if err != nil {
				t.Fatalf("failed to marshal zero checksum: %v", err)
			}
			got := br.PaddingLength(payloadBytes)
			if got != tt.want {
				t.Errorf("baseRow.PaddingLength() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBaseRow_GetParity(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *baseRow[*Checksum]
		wantErr   bool
		verifyHex bool
	}{
		{
			name: "valid checksum row",
			setup: func() *baseRow[*Checksum] {
				checksum := Checksum(0x12345678)
				return &baseRow[*Checksum]{
					RowSize:      1024,
					StartControl: CHECKSUM_ROW,
					EndControl:   CHECKSUM_ROW_CONTROL,
					RowPayload:   &checksum,
				}
			},
			wantErr:   false,
			verifyHex: true,
		},
		{
			name: "nil header",
			setup: func() *baseRow[*Checksum] {
				checksum := Checksum(0x12345678)
				return &baseRow[*Checksum]{
					StartControl: CHECKSUM_ROW,
					EndControl:   CHECKSUM_ROW_CONTROL,
					RowPayload:   &checksum,
				}
			},
			wantErr:   true,
			verifyHex: false,
		},
		{
			name: "row size too small",
			setup: func() *baseRow[*Checksum] {
				checksum := Checksum(0x12345678)
				return &baseRow[*Checksum]{
					RowSize:      10,
					StartControl: CHECKSUM_ROW,
					EndControl:   CHECKSUM_ROW_CONTROL,
					RowPayload:   &checksum,
				}
			},
			wantErr:   true,
			verifyHex: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := tt.setup()
			got, err := br.GetParity()
			if (err != nil) != tt.wantErr {
				t.Errorf("baseRow.GetParity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			// Verify it returns exactly 2 bytes
			if len(got) != 2 {
				t.Errorf("baseRow.GetParity() length = %d, want 2", len(got))
			}
			// Verify it's valid hex
			if tt.verifyHex {
				for i, c := range got {
					if (c < '0' || c > '9') && (c < 'A' || c > 'F') {
						t.Errorf("baseRow.GetParity()[%d] contains invalid hex character: %c (0x%02X)", i, c, c)
					}
				}
			}
		})
	}
}

func TestBaseRow_validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *baseRow[*Checksum]
		wantErr bool
	}{
		{
			name: "valid baseRow",
			setup: func() *baseRow[*Checksum] {
				checksum := Checksum(0x12345678)
				return &baseRow[*Checksum]{
					RowSize:      1024,
					StartControl: CHECKSUM_ROW,
					EndControl:   CHECKSUM_ROW_CONTROL,
					RowPayload:   &checksum,
				}
			},
			wantErr: false,
		},
		{
			name: "nil header",
			setup: func() *baseRow[*Checksum] {
				return &baseRow[*Checksum]{}
			},
			wantErr: true,
		},
		{
			name: "invalid row size too small",
			setup: func() *baseRow[*Checksum] {
				return &baseRow[*Checksum]{
					RowSize: 127,
				}
			},
			wantErr: true,
		},
		{
			name: "invalid row size too large",
			setup: func() *baseRow[*Checksum] {
				return &baseRow[*Checksum]{
					RowSize: 65537,
				}
			},
			wantErr: true,
		},
		{
			name: "invalid StartControl",
			setup: func() *baseRow[*Checksum] {
				return &baseRow[*Checksum]{
					RowSize:      1024,
					StartControl: StartControl('X'),
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := tt.setup()
			err := br.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("baseRow.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
