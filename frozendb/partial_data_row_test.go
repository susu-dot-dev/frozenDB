package frozendb

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestPartialDataRow_InvalidStartControl(t *testing.T) {

	t.Run("CHECKSUM_ROW_StartControl_ShouldFail", func(t *testing.T) {
		pdr := &PartialDataRow{
			state: PartialDataRowWithStartControl,
			d: DataRow{
				baseRow: baseRow[*DataRowPayload]{
					RowSize:      512,
					StartControl: CHECKSUM_ROW,
				},
			},
		}

		err := pdr.Validate()
		if err == nil {
			t.Error("PartialDataRow with CHECKSUM_ROW start_control should fail validation")
		}
	})

	t.Run("InvalidStartControlByte_ShouldFail", func(t *testing.T) {
		pdr := &PartialDataRow{
			state: PartialDataRowWithStartControl,
			d: DataRow{
				baseRow: baseRow[*DataRowPayload]{
					RowSize:      512,
					StartControl: StartControl('X'),
				},
			},
		}

		err := pdr.Validate()
		if err == nil {
			t.Error("PartialDataRow with invalid start_control byte should fail validation")
		}
	})
}

func TestPartialDataRow_ValidationWithNilRowSize(t *testing.T) {
	t.Run("State1_UnsetRowSize_MarshalTextFails", func(t *testing.T) {
		pdr := &PartialDataRow{
			state: PartialDataRowWithStartControl,
			d: DataRow{
				baseRow: baseRow[*DataRowPayload]{
					RowSize:      -1,
					StartControl: START_TRANSACTION,
				},
			},
		}

		// MarshalText should fail when RowSize is -1 (unset)
		_, err := pdr.MarshalText()
		if err == nil {
			t.Error("MarshalText should fail when RowSize is -1 (unset)")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("State2_UnsetRowSize_ShouldFail", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr := &PartialDataRow{
			state: PartialDataRowWithPayload,
			d: DataRow{
				baseRow: baseRow[*DataRowPayload]{
					RowSize:      -1,
					StartControl: START_TRANSACTION,
					RowPayload: &DataRowPayload{
						Key:   key,
						Value: json.RawMessage(`{"name":"test"}`),
					},
				},
			},
		}

		// Operations should fail when RowSize is -1
		_, err := pdr.MarshalText()
		if err == nil {
			t.Error("MarshalText should fail when RowSize is -1 (unset)")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})
}

func TestPartialDataRow_State2ValidationRequiresPayload(t *testing.T) {

	t.Run("State2_WithNilPayload_ShouldFail", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		pdr.state = PartialDataRowWithPayload
		pdr.d.RowPayload = nil

		err = pdr.Validate()
		if err == nil {
			t.Error("PartialDataRow in State2 with nil payload should fail validation")
		}
	})

	t.Run("State2_WithInvalidPayload_ShouldFail", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		pdr.state = PartialDataRowWithPayload
		pdr.d.RowPayload = &DataRowPayload{
			Key:   uuid.Nil,
			Value: json.RawMessage("test"),
		}

		err = pdr.Validate()
		if err == nil {
			t.Error("PartialDataRow in State2 with invalid payload (zero UUID) should fail validation")
		}
	})

	t.Run("State2_WithEmptyJSON_ShouldFail", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		pdr.state = PartialDataRowWithPayload
		pdr.d.RowPayload = &DataRowPayload{
			Key:   key,
			Value: json.RawMessage(""),
		}

		err = pdr.Validate()
		if err == nil {
			t.Error("PartialDataRow in State2 with empty JSON should fail validation")
		}
	})
}

func TestPartialDataRow_UnmarshalTextValidation(t *testing.T) {

	t.Run("InvalidROWSTART_ShouldFail", func(t *testing.T) {
		var pdr PartialDataRow
		invalidBytes := []byte{0x00, 'T'}

		err := pdr.UnmarshalText(invalidBytes)
		if err == nil {
			t.Error("UnmarshalText with invalid ROW_START should fail")
		}
	})

	t.Run("TruncatedBytes_ShouldFail", func(t *testing.T) {
		var pdr PartialDataRow

		err := pdr.UnmarshalText([]byte{ROW_START})
		if err == nil {
			t.Error("UnmarshalText with truncated bytes should fail")
		}
	})
}

func TestPartialDataRow_CommitFromInvalidStates(t *testing.T) {

	t.Run("CommitFromState1_ShouldFail", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		_, err = pdr.Commit()
		if err == nil {
			t.Error("Commit from State1 should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("CommitFromState2_ValidPayload_ShouldSucceed", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Commit()
		if err != nil {
			t.Errorf("Commit from State2 with valid payload should succeed, got: %v", err)
		}
		if dataRow == nil {
			t.Error("Commit should return non-nil DataRow")
		}
	})

	t.Run("CommitFromState3_ValidPayload_ShouldSucceed", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		dataRow, err := pdr.Commit()
		if err != nil {
			t.Errorf("Commit from State3 with valid payload should succeed, got: %v", err)
		}
		if dataRow == nil {
			t.Error("Commit should return non-nil DataRow")
		} else if dataRow.EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Expected end_control SC, got: %v", dataRow.EndControl)
		}
	})
}

func TestPartialDataRow_RollbackValidation(t *testing.T) {
	key := generateValidUUIDv7()

	t.Run("RollbackFromState1_ShouldFail", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		_, err = pdr.Rollback(0)
		if err == nil {
			t.Error("Rollback from State1 should fail")
		}
	})

	t.Run("RollbackWithInvalidId_TooHigh", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		_, err = pdr.Rollback(10)
		if err == nil {
			t.Error("Rollback with savepointId 10 should fail")
		}
	})

	t.Run("RollbackWithInvalidId_Negative", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		_, err = pdr.Rollback(-1)
		if err == nil {
			t.Error("Rollback with negative savepointId should fail")
		}
	})

	t.Run("RollbackFromState2_WithValidPayload_ShouldSucceed", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Rollback(0)
		if err != nil {
			t.Errorf("Rollback from State2 should succeed, got: %v", err)
		}
		if dataRow == nil {
			t.Error("Rollback should return non-nil DataRow")
		} else {
			expected := EndControl{'R', '0'}
			if dataRow.EndControl != expected {
				t.Errorf("Expected end_control R0, got: %v", dataRow.EndControl)
			}
		}
	})
}

func TestPartialDataRow_EndRowValidation(t *testing.T) {
	key := generateValidUUIDv7()

	t.Run("EndRowFromState1_ShouldFail", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		_, err = pdr.EndRow()
		if err == nil {
			t.Error("EndRow from State1 should fail")
		}
	})

	t.Run("EndRowFromState2_WithValidPayload_ShouldSucceed", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.EndRow()
		if err != nil {
			t.Errorf("EndRow from State2 should succeed, got: %v", err)
		}
		if dataRow == nil {
			t.Error("EndRow should return non-nil DataRow")
		} else if dataRow.EndControl != ROW_END_CONTROL {
			t.Errorf("Expected end_control RE, got: %v", dataRow.EndControl)
		}
	})
}

func TestPartialDataRow_GetState(t *testing.T) {
	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithStartControl {
		t.Errorf("Expected PartialDataRowWithStartControl, got %v", pdr.GetState())
	}

	key := generateValidUUIDv7()
	if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithPayload {
		t.Errorf("Expected PartialDataRowWithPayload, got %v", pdr.GetState())
	}

	if err := pdr.Savepoint(); err != nil {
		t.Fatalf("Savepoint failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithSavepoint {
		t.Errorf("Expected PartialDataRowWithSavepoint, got %v", pdr.GetState())
	}
}

func TestPartialDataRow_SavepointFromState1_ShouldFail(t *testing.T) {

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	err = pdr.Savepoint()
	if err == nil {
		t.Error("Savepoint from State1 should fail")
	}
	if !isInvalidActionError(err) {
		t.Errorf("Expected InvalidActionError, got: %T", err)
	}
}

func TestPartialDataRow_AddRowFromState2_ShouldFail(t *testing.T) {
	key := generateValidUUIDv7()

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}
	if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	key2 := generateValidUUIDv7()
	err = pdr.AddRow(key2, json.RawMessage(`{"name":"test2"}`))
	if err == nil {
		t.Error("AddRow from State2 should fail")
	}
	if !isInvalidActionError(err) {
		t.Errorf("Expected InvalidActionError, got: %T", err)
	}
}

func TestPartialDataRow_AddRowFromState3_ShouldFail(t *testing.T) {
	key := generateValidUUIDv7()

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}
	if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}
	if err := pdr.Savepoint(); err != nil {
		t.Fatalf("Savepoint failed: %v", err)
	}

	key2 := generateValidUUIDv7()
	err = pdr.AddRow(key2, json.RawMessage(`{"name":"test2"}`))
	if err == nil {
		t.Error("AddRow from State3 should fail")
	}
	if !isInvalidActionError(err) {
		t.Errorf("Expected InvalidActionError, got: %T", err)
	}
}

func TestPartialDataRow_SavepointFromState3_ShouldFail(t *testing.T) {
	key := generateValidUUIDv7()

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}
	if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}
	if err := pdr.Savepoint(); err != nil {
		t.Fatalf("Savepoint failed: %v", err)
	}

	err = pdr.Savepoint()
	if err == nil {
		t.Error("Savepoint from State3 should fail")
	}
	if !isInvalidActionError(err) {
		t.Errorf("Expected InvalidActionError, got: %T", err)
	}
}

func TestPartialDataRow_AddRowWithInvalidUUID(t *testing.T) {

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	t.Run("WithUUIDv4_ShouldFail", func(t *testing.T) {
		invalidKey := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		err := pdr.AddRow(invalidKey, json.RawMessage(`{"name":"test"}`))
		if err == nil {
			t.Error("AddRow with UUIDv4 should fail")
		}
		if !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %T", err)
		}
	})

	t.Run("WithZeroUUID_ShouldFail", func(t *testing.T) {
		err := pdr.AddRow(uuid.Nil, json.RawMessage(`{"name":"test"}`))
		if err == nil {
			t.Error("AddRow with zero UUID should fail")
		}
		if !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %T", err)
		}
	})

	t.Run("WithEmptyJSON_ShouldFail", func(t *testing.T) {
		validKey := generateValidUUIDv7()
		err := pdr.AddRow(validKey, json.RawMessage(""))
		if err == nil {
			t.Error("AddRow with empty JSON should fail")
		}
		if !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %T", err)
		}
	})
}

func TestPartialDataRow_MarshalText(t *testing.T) {

	t.Run("State1_ShouldMarshalTo2Bytes", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		if len(bytes) != 2 {
			t.Errorf("State1 should marshal to exactly 2 bytes, got %d", len(bytes))
		}

		if bytes[0] != ROW_START {
			t.Errorf("Expected ROW_START (0x1F) at position 0, got 0x%02X", bytes[0])
		}
		if bytes[1] != byte(START_TRANSACTION) {
			t.Errorf("Expected START_CONTROL 'T' at position 1, got 0x%02X", bytes[1])
		}
	})

	t.Run("State2_ShouldIncludeUUIDAndJSON", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		if len(bytes) < 2 {
			t.Errorf("State2 should marshal to more than 2 bytes, got %d", len(bytes))
		}

		if bytes[0] != ROW_START {
			t.Errorf("Expected ROW_START at position 0, got 0x%02X", bytes[0])
		}
	})

	t.Run("State3_ShouldIncludeSCharacter", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		foundS := false
		for i := 2; i < len(bytes); i++ {
			if bytes[i] == 'S' {
				foundS = true
				break
			}
		}
		if !foundS {
			t.Error("State3 MarshalText should include 'S' character")
		}
	})
}

func TestPartialDataRow_RoundTrip(t *testing.T) {

	t.Run("State1_RoundTrip", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		pdr2.d.RowSize = 512

		if pdr2.GetState() != PartialDataRowWithStartControl {
			t.Errorf("Expected state PartialDataRowWithStartControl, got %v", pdr2.GetState())
		}
	})

	t.Run("State2_RoundTrip", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		pdr2.d.RowSize = 512

		if pdr2.GetState() != PartialDataRowWithPayload {
			t.Errorf("Expected state PartialDataRowWithPayload, got %v", pdr2.GetState())
		}
	})

	t.Run("State3_RoundTrip", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		pdr2.d.RowSize = 512

		if pdr2.GetState() != PartialDataRowWithSavepoint {
			t.Errorf("Expected state PartialDataRowWithSavepoint, got %v", pdr2.GetState())
		}
	})
}

func TestPartialDataRow_UnmarshalTextWithoutRowSize(t *testing.T) {
	t.Run("MarshalText_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize (UnmarshalText sets it to -1 for states with payload)

		_, err = pdr2.MarshalText()
		if err == nil {
			t.Error("MarshalText should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("AddRow_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize

		key2 := generateValidUUIDv7()
		err = pdr2.AddRow(key2, json.RawMessage(`{"name":"test2"}`))
		if err == nil {
			t.Error("AddRow should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("Savepoint_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize

		err = pdr2.Savepoint()
		if err == nil {
			t.Error("Savepoint should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("Commit_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize

		_, err = pdr2.Commit()
		if err == nil {
			t.Error("Commit should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("Rollback_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize

		_, err = pdr2.Rollback(0)
		if err == nil {
			t.Error("Rollback should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("EndRow_FailsWithoutRowSize", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		var pdr2 PartialDataRow
		if err := pdr2.UnmarshalText(bytes); err != nil {
			t.Fatalf("UnmarshalText failed: %v", err)
		}
		// Intentionally NOT setting RowSize

		_, err = pdr2.EndRow()
		if err == nil {
			t.Error("EndRow should fail when RowSize is not set after UnmarshalText")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})
}
