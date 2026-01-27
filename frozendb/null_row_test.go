package frozendb

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// TestNullRowPayload_MarshalText tests NullRowPayload marshaling edge cases
func TestNullRowPayload_MarshalText(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var nrp *NullRowPayload
		_, err := nrp.MarshalText()
		if err == nil {
			t.Error("MarshalText on nil receiver should return error")
		}
	})

	t.Run("valid_nullrow_uuid_produces_correct_base64", func(t *testing.T) {
		// Use valid NullRow UUID with timestamp 0
		nullRowUUID := CreateNullRowUUID(0)
		nrp := &NullRowPayload{Key: nullRowUUID}
		b, err := nrp.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}
		// Verify it's valid Base64 encoding (24 bytes)
		if len(b) != 24 {
			t.Errorf("Expected 24 bytes, got %d", len(b))
		}
	})
}

// TestNullRowPayload_UnmarshalText tests NullRowPayload unmarshaling edge cases
func TestNullRowPayload_UnmarshalText(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var nrp *NullRowPayload
		err := nrp.UnmarshalText([]byte("AAAAAAAAAAAAAAAAAAAAAA=="))
		if err == nil {
			t.Error("UnmarshalText on nil receiver should return error")
		}
	})

	t.Run("wrong_length", func(t *testing.T) {
		nrp := &NullRowPayload{}
		err := nrp.UnmarshalText([]byte("short"))
		if err == nil {
			t.Error("UnmarshalText with wrong length should return error")
		}
	})

	t.Run("invalid_base64", func(t *testing.T) {
		nrp := &NullRowPayload{}
		err := nrp.UnmarshalText([]byte("!!!!!!!!!!!!!!!!!!!!!!!!"))
		if err == nil {
			t.Error("UnmarshalText with invalid Base64 should return error")
		}
	})

	t.Run("valid_nullrow_uuid", func(t *testing.T) {
		// Create a valid NullRow UUID and encode it
		nullRowUUID := CreateNullRowUUID(0)
		encoded := base64.StdEncoding.EncodeToString(nullRowUUID[:])

		nrp := &NullRowPayload{}
		err := nrp.UnmarshalText([]byte(encoded))
		if err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		if nrp.Key != nullRowUUID {
			t.Errorf("Expected %s, got %s", nullRowUUID, nrp.Key)
		}
	})

	t.Run("valid_non_nil_uuid", func(t *testing.T) {
		// Create a non-nil UUID and encode it
		nonNilUUID, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("Failed to create UUIDv7: %v", err)
		}
		encoded := base64.StdEncoding.EncodeToString(nonNilUUID[:])

		nrp := &NullRowPayload{}
		err = nrp.UnmarshalText([]byte(encoded))
		if err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		if nrp.Key != nonNilUUID {
			t.Errorf("Expected %s, got %s", nonNilUUID, nrp.Key)
		}
	})
}

// TestNullRowPayload_Validate tests NullRowPayload validation edge cases
func TestNullRowPayload_Validate(t *testing.T) {
	t.Run("nil_receiver", func(t *testing.T) {
		var nrp *NullRowPayload
		err := nrp.Validate()
		if err == nil {
			t.Error("Validate on nil receiver should return error")
		}
	})

	t.Run("valid_nullrow_uuid", func(t *testing.T) {
		nullRowUUID := CreateNullRowUUID(0)
		nrp := &NullRowPayload{Key: nullRowUUID}
		err := nrp.Validate()
		if err != nil {
			t.Errorf("Validate should pass for valid NullRow UUID: %v", err)
		}
	})

	t.Run("invalid_non_nil_uuid", func(t *testing.T) {
		nonNilUUID, _ := uuid.NewV7()
		nrp := &NullRowPayload{Key: nonNilUUID}
		err := nrp.Validate()
		if err == nil {
			t.Error("Validate should fail for non-nil UUID")
		}
	})
}

// TestNullRow_RoundTrip tests complete marshal/unmarshal cycles
func TestNullRow_RoundTrip(t *testing.T) {
	rowSizes := []int{128, 256, 512, 1024}

	for _, rowSize := range rowSizes {
		t.Run(fmt.Sprintf("rowSize_%d", rowSize), func(t *testing.T) {
			// Create original NullRow with valid NullRow UUID
			nullRowUUID := CreateNullRowUUID(0)
			original := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					RowSize:      rowSize,
					StartControl: START_TRANSACTION,
					EndControl:   NULL_ROW_CONTROL,
					RowPayload:   &NullRowPayload{Key: nullRowUUID},
				},
			}

			// Marshal
			marshaled, err := original.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText failed: %v", err)
			}

			// Verify length
			if len(marshaled) != rowSize {
				t.Errorf("Marshaled length mismatch: expected %d, got %d", rowSize, len(marshaled))
			}

			// Unmarshal into new NullRow
			restored := &NullRow{
				baseRow: baseRow[*NullRowPayload]{
					RowSize: 512,
				},
			}

			err = restored.UnmarshalText(marshaled)
			if err != nil {
				t.Fatalf("UnmarshalText failed: %v", err)
			}

			// Verify fields
			if restored.GetKey() != nullRowUUID {
				t.Errorf("Key mismatch: expected %s, got %s", nullRowUUID, restored.GetKey())
			}

			if restored.StartControl != START_TRANSACTION {
				t.Errorf("StartControl mismatch: expected 'T', got '%c'", restored.StartControl)
			}

			if restored.EndControl != NULL_ROW_CONTROL {
				t.Errorf("EndControl mismatch: expected 'NR', got '%s'", restored.EndControl.String())
			}

			// Re-marshal and compare
			remarshaled, err := restored.MarshalText()
			if err != nil {
				t.Fatalf("Re-marshal failed: %v", err)
			}

			if len(remarshaled) != len(marshaled) {
				t.Errorf("Re-marshaled length mismatch: expected %d, got %d", len(marshaled), len(remarshaled))
			}

			for i := 0; i < len(marshaled); i++ {
				if remarshaled[i] != marshaled[i] {
					t.Errorf("Byte mismatch at position %d: expected 0x%02X, got 0x%02X", i, marshaled[i], remarshaled[i])
					break
				}
			}
		})
	}
}

// TestNullRow_GetKey tests the GetKey accessor method
func TestNullRow_GetKey(t *testing.T) {
	nullRowUUID := CreateNullRowUUID(0)
	nullRow := &NullRow{
		baseRow: baseRow[*NullRowPayload]{
			RowSize:      512,
			StartControl: START_TRANSACTION,
			EndControl:   NULL_ROW_CONTROL,
			RowPayload:   &NullRowPayload{Key: nullRowUUID},
		},
	}

	key := nullRow.GetKey()
	if key != nullRowUUID {
		t.Errorf("GetKey should return %s, got %s", nullRowUUID, key)
	}
}
