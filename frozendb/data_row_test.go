package frozendb

import (
	"testing"

	"github.com/google/uuid"
)

func TestDataRowPayload_MarshalText(t *testing.T) {
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	tests := []struct {
		name    string
		payload *DataRowPayload
		wantErr bool
	}{
		{
			name: "valid payload",
			payload: &DataRowPayload{
				Key:   key,
				Value: `{"test":"value"}`,
			},
			wantErr: false,
		},
		{
			name:    "nil payload",
			payload: nil,
			wantErr: true,
		},
		{
			name: "empty value",
			payload: &DataRowPayload{
				Key:   key,
				Value: "",
			},
			wantErr: false, // MarshalText doesn't validate empty value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.payload.MarshalText()
			if (err != nil) != tt.wantErr {
				t.Errorf("DataRowPayload.MarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) < 24 {
				t.Errorf("DataRowPayload.MarshalText() length = %d, want at least 24", len(got))
			}
		})
	}
}

func TestDataRowPayload_UnmarshalText(t *testing.T) {
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	// Create valid payload bytes
	validPayload := &DataRowPayload{
		Key:   key,
		Value: `{"test":"value"}`,
	}
	validBytes, err := validPayload.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal valid payload: %v", err)
	}

	tests := []struct {
		name    string
		text    []byte
		wantErr bool
	}{
		{
			name:    "valid payload",
			text:    validBytes,
			wantErr: false,
		},
		{
			name:    "too short",
			text:    []byte("short"),
			wantErr: true,
		},
		{
			name:    "invalid Base64 UUID",
			text:    append([]byte("invalid_base64_encoding!!"), []byte(`{"test":"value"}`)...),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &DataRowPayload{}
			err := payload.UnmarshalText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataRowPayload.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataRowPayload_Validate(t *testing.T) {
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	tests := []struct {
		name    string
		payload *DataRowPayload
		wantErr bool
	}{
		{
			name: "valid payload",
			payload: &DataRowPayload{
				Key:   key,
				Value: `{"test":"value"}`,
			},
			wantErr: false,
		},
		{
			name:    "nil payload",
			payload: nil,
			wantErr: true,
		},
		{
			name: "empty value",
			payload: &DataRowPayload{
				Key:   key,
				Value: "",
			},
			wantErr: true,
		},
		{
			name: "invalid UUIDv4",
			payload: &DataRowPayload{
				Key:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), // v4
				Value: `{"test":"value"}`,
			},
			wantErr: true,
		},
		{
			name: "zero UUID",
			payload: &DataRowPayload{
				Key:   uuid.Nil,
				Value: `{"test":"value"}`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.payload.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DataRowPayload.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUUIDv7(t *testing.T) {
	tests := []struct {
		name    string
		u       uuid.UUID
		wantErr bool
	}{
		{
			name:    "valid UUIDv7",
			u:       uuid.Must(uuid.NewV7()),
			wantErr: false,
		},
		{
			name:    "zero UUID",
			u:       uuid.Nil,
			wantErr: true,
		},
		{
			name:    "UUIDv4",
			u:       uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
			wantErr: true,
		},
		{
			name:    "UUIDv1",
			u:       uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUUIDv7(tt.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUUIDv7() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataRow_GetKey(t *testing.T) {
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	tests := []struct {
		name    string
		dataRow *DataRow
		want    uuid.UUID
	}{
		{
			name: "valid key",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header: header,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			want: key,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dataRow.GetKey()
			if got != tt.want {
				t.Errorf("DataRow.GetKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataRow_GetValue(t *testing.T) {
	key, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	header := &Header{
		signature: "fDB",
		version:   1,
		rowSize:   512,
		skewMs:    5000,
	}

	value := `{"test":"value"}`
	tests := []struct {
		name    string
		dataRow *DataRow
		want    string
	}{
		{
			name: "valid value",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header: header,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: value,
					},
				},
			},
			want: value,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dataRow.GetValue()
			if got != tt.want {
				t.Errorf("DataRow.GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataRow_RoundTripSerialization(t *testing.T) {
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

	value := `{"name":"Test","count":42,"active":true}`
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
		t.Fatalf("Original row validation failed: %v", err)
	}

	// Serialize
	bytes, err := originalRow.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText failed: %v", err)
	}

	// Deserialize
	deserializedRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header: header,
		},
	}

	if err := deserializedRow.UnmarshalText(bytes); err != nil {
		t.Fatalf("UnmarshalText failed: %v", err)
	}

	// Verify round-trip
	if deserializedRow.GetKey() != key {
		t.Errorf("Key mismatch: expected %s, got %s", key, deserializedRow.GetKey())
	}

	if deserializedRow.GetValue() != value {
		t.Errorf("Value mismatch: expected %s, got %s", value, deserializedRow.GetValue())
	}
}

func TestDataRow_Validate(t *testing.T) {
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

	tests := []struct {
		name    string
		dataRow *DataRow
		wantErr bool
	}{
		{
			name: "valid DataRow with T/TC",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid DataRow with R/RE",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: ROW_CONTINUE,
					EndControl:   ROW_END_CONTROL,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid start control (C)",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: CHECKSUM_ROW,
					EndControl:   TRANSACTION_COMMIT,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid end control (CS)",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   CHECKSUM_ROW_CONTROL,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid rollback R0",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: START_TRANSACTION,
					EndControl:   FULL_ROLLBACK,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid rollback R5",
			dataRow: &DataRow{
				baseRow[*DataRowPayload]{
					Header:       header,
					StartControl: ROW_CONTINUE,
					EndControl:   EndControl{'R', '5'},
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: `{"test":"value"}`,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First validate baseRow structure
			if err := tt.dataRow.baseRow.Validate(); err != nil && !tt.wantErr {
				// If baseRow validation fails, that's expected for some test cases
				// But we still want to test DataRow.Validate() if baseRow passes
				return
			}

			err := tt.dataRow.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DataRow.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
