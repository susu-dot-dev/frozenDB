package frozendb

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func generateValidUUIDv7() uuid.UUID {
	key, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return key
}

func isInvalidActionError(err error) bool {
	_, ok := err.(*InvalidActionError)
	return ok
}

func isInvalidInputError(err error) bool {
	_, ok := err.(*InvalidInputError)
	return ok
}

func Test_S_009_FR_001_PartialDataRowCreation(t *testing.T) {
	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithStartControl {
		t.Errorf("Expected state PartialDataRowWithStartControl, got %v", pdr.GetState())
	}

	if err := pdr.Validate(); err != nil {
		t.Errorf("Validation should pass for new PartialDataRow: %v", err)
	}

	t.Run("with ROW_CONTINUE control", func(t *testing.T) {
		pdr2, err := NewPartialDataRow(512, ROW_CONTINUE)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if pdr2.d.StartControl != ROW_CONTINUE {
			t.Errorf("Expected StartControl ROW_CONTINUE, got %v", pdr2.d.StartControl)
		}
		if pdr2.GetState() != PartialDataRowWithStartControl {
			t.Errorf("Expected state PartialDataRowWithStartControl, got %v", pdr2.GetState())
		}
	})
}

func Test_S_009_FR_002_AddRowStateTransition(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test","value":123}`)

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	if err := pdr.AddRow(key, jsonValue); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithPayload {
		t.Errorf("Expected state PartialDataRowWithPayload after AddRow, got %v", pdr.GetState())
	}
}

func Test_S_009_FR_003_SavepointStateTransition(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test","value":123}`)

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	if err := pdr.AddRow(key, jsonValue); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	if err := pdr.Savepoint(); err != nil {
		t.Fatalf("Savepoint failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithSavepoint {
		t.Errorf("Expected state PartialDataRowWithSavepoint after Savepoint, got %v", pdr.GetState())
	}
}

func Test_S_009_FR_004_PreventInvalidStateTransitions(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test","value":123}`)

	t.Run("State1_CannotCallSavepoint", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		err = pdr.Savepoint()
		if err == nil {
			t.Error("Savepoint should fail from State1")
		}
		if err != nil && !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %v", err)
		}
	})

	t.Run("State2_CannotCallAddRowAgain", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		key2 := generateValidUUIDv7()
		err = pdr.AddRow(key2, jsonValue)
		if err == nil {
			t.Error("AddRow should fail from State2")
		}
		if err != nil && !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %v", err)
		}
	})

	t.Run("State3_CannotCallSavepointAgain", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		err = pdr.Savepoint()
		if err == nil {
			t.Error("Savepoint should fail from State3")
		}
		if err != nil && !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %v", err)
		}
	})

	t.Run("State3_CannotCallAddRow", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		key2 := generateValidUUIDv7()
		err = pdr.AddRow(key2, jsonValue)
		if err == nil {
			t.Error("AddRow should fail from State3")
		}
		if err != nil && !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %v", err)
		}
	})
}

func Test_S_009_FR_005_RevalidateAfterTransition(t *testing.T) {

	t.Run("ValidTransition_RevalidationPasses", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		if err := pdr.Validate(); err != nil {
			t.Errorf("Validation should pass after valid transition: %v", err)
		}

		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		if err := pdr.Validate(); err != nil {
			t.Errorf("Validation should pass after Savepoint: %v", err)
		}
	})
}

func Test_S_009_FR_006_ValidateFunction(t *testing.T) {

	t.Run("State1_ValidationRequiresStartControl", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.Validate(); err != nil {
			t.Errorf("Validation should pass for State1 with valid start_control: %v", err)
		}
	})

	t.Run("State2_ValidationRequiresUUIDAndJSON", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		if err := pdr.Validate(); err != nil {
			t.Errorf("Validation should pass for State2 with valid UUID and JSON: %v", err)
		}
	})

	t.Run("State3_ValidationRequiresSCharacter", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		if err := pdr.Validate(); err != nil {
			t.Errorf("Validation should pass for State3 with 'S' character: %v", err)
		}
	})
}

func Test_S_009_FR_009_ValidateUUIDv7Format(t *testing.T) {
	validKey := generateValidUUIDv7()
	invalidKey := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	jsonValue := json.RawMessage(`{"name":"test"}`)

	t.Run("ValidUUIDv7_Accept", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(validKey, jsonValue); err != nil {
			t.Errorf("AddRow should accept valid UUIDv7: %v", err)
		}
	})

	t.Run("InvalidUUIDv4_Reject", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		err = pdr.AddRow(invalidKey, jsonValue)
		if err == nil {
			t.Error("AddRow should reject invalid UUID")
		}
		if err != nil && !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %v", err)
		}
	})

	t.Run("ZeroUUID_Reject", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		err = pdr.AddRow(uuid.Nil, jsonValue)
		if err == nil {
			t.Error("AddRow should reject zero UUID")
		}
		if err != nil && !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %v", err)
		}
	})
}

func Test_S_009_FR_010_ValidateJSONPayload(t *testing.T) {
	key := generateValidUUIDv7()

	t.Run("ValidJSONString_Accept", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		jsonValue := json.RawMessage(`{"name":"test","value":123}`)
		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Errorf("AddRow should accept valid JSON string: %v", err)
		}
	})

	t.Run("EmptyJSON_Reject", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		err = pdr.AddRow(key, json.RawMessage(""))
		if err == nil {
			t.Error("AddRow should reject empty JSON")
		}
		if err != nil && !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %v", err)
		}
	})

	t.Run("ValidUTF8NonEmpty_Accept", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		jsonValue := json.RawMessage("plain text value")
		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Errorf("AddRow should accept valid UTF-8 string: %v", err)
		}
	})
}

func Test_S_009_FR_011_StateImmutability(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	initialState := pdr.GetState()

	if err := pdr.AddRow(key, jsonValue); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithPayload {
		t.Errorf("State should have transitioned to PartialDataRowWithPayload")
	}

	if initialState == pdr.GetState() {
		t.Error("State should have changed after AddRow")
	}

	if err := pdr.Savepoint(); err != nil {
		t.Fatalf("Savepoint failed: %v", err)
	}

	if pdr.GetState() != PartialDataRowWithSavepoint {
		t.Errorf("State should have transitioned to PartialDataRowWithSavepoint")
	}

	t.Run("CannotRevertToPreviousState", func(t *testing.T) {
		pdr2, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr2.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		if err := pdr2.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		if pdr2.GetState() == PartialDataRowWithStartControl {
			t.Error("State should not be able to revert to PartialDataRowWithStartControl")
		}
		if pdr2.GetState() == PartialDataRowWithPayload {
			t.Error("State should not be able to revert to PartialDataRowWithPayload")
		}
	})
}

func Test_S_009_FR_007_MarshalTextFunction(t *testing.T) {

	t.Run("State1_MarshalText", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		bytes, err := pdr.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText failed: %v", err)
		}

		if len(bytes) < 2 {
			t.Errorf("State1 should marshal to at least 2 bytes, got %d", len(bytes))
		}

		if bytes[0] != ROW_START {
			t.Errorf("Expected ROW_START (0x1F) at position 0, got 0x%02X", bytes[0])
		}
		if bytes[1] != byte(START_TRANSACTION) {
			t.Errorf("Expected START_CONTROL 'T' at position 1, got 0x%02X", bytes[1])
		}
	})

	t.Run("State2_MarshalText", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
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

	t.Run("State3_MarshalText", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
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

func Test_S_009_FR_008_UnmarshalTextFunction(t *testing.T) {

	t.Run("State1_UnmarshalText", func(t *testing.T) {
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

	t.Run("State2_UnmarshalText", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
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

	t.Run("State3_UnmarshalText", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		jsonValue := json.RawMessage(`{"name":"test"}`)

		if err := pdr.AddRow(key, jsonValue); err != nil {
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

func Test_S_009_FR_012_ErrorTypes(t *testing.T) {
	t.Run("InvalidActionErrorForStateTransition", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		err = pdr.Savepoint()
		if err == nil {
			t.Error("Expected error for invalid state transition")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("InvalidInputErrorForValidation", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		key := generateValidUUIDv7()
		err = pdr.AddRow(key, json.RawMessage(""))
		if err == nil {
			t.Error("Expected error for empty JSON")
		}
		if !isInvalidInputError(err) {
			t.Errorf("Expected InvalidInputError, got: %T", err)
		}
	})
}

func Test_S_009_FR_013_CommitFunction(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	t.Run("State2_Commit_ReturnsTC", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
		if dataRow == nil {
			t.Fatal("Commit should return non-nil DataRow")
		}

		if dataRow.EndControl != TRANSACTION_COMMIT {
			t.Errorf("Expected end_control TC, got: %v", dataRow.EndControl)
		}
	})

	t.Run("State3_Commit_ReturnsSC", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		dataRow, err := pdr.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
		if dataRow == nil {
			t.Fatal("Commit should return non-nil DataRow")
		}

		if dataRow.EndControl != SAVEPOINT_COMMIT {
			t.Errorf("Expected end_control SC, got: %v", dataRow.EndControl)
		}
	})
}

func Test_S_009_FR_014_RollbackFunction(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	t.Run("State2_Rollback_ReturnsR0", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
		if dataRow == nil {
			t.Fatal("Rollback should return non-nil DataRow")
		}

		expected := EndControl{'R', '0'}
		if dataRow.EndControl != expected {
			t.Errorf("Expected end_control R0, got: %v", dataRow.EndControl)
		}
	})

	t.Run("State2_Rollback_ReturnsR3", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Rollback(3)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		expected := EndControl{'R', '3'}
		if dataRow.EndControl != expected {
			t.Errorf("Expected end_control R3, got: %v", dataRow.EndControl)
		}
	})

	t.Run("State3_Rollback_ReturnsS0", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		dataRow, err := pdr.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		expected := EndControl{'S', '0'}
		if dataRow.EndControl != expected {
			t.Errorf("Expected end_control S0, got: %v", dataRow.EndControl)
		}
	})

	t.Run("State3_Rollback_ReturnsS5", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		dataRow, err := pdr.Rollback(5)
		if err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}

		expected := EndControl{'S', '5'}
		if dataRow.EndControl != expected {
			t.Errorf("Expected end_control S5, got: %v", dataRow.EndControl)
		}
	})
}

func Test_S_009_FR_015_EndRowFunction(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	t.Run("State2_EndRow_ReturnsRE", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.EndRow()
		if err != nil {
			t.Fatalf("EndRow failed: %v", err)
		}
		if dataRow == nil {
			t.Fatal("EndRow should return non-nil DataRow")
		}

		if dataRow.EndControl != ROW_END_CONTROL {
			t.Errorf("Expected end_control RE, got: %v", dataRow.EndControl)
		}
	})

	t.Run("State3_EndRow_ReturnsSE", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}
		if err := pdr.Savepoint(); err != nil {
			t.Fatalf("Savepoint failed: %v", err)
		}

		dataRow, err := pdr.EndRow()
		if err != nil {
			t.Fatalf("EndRow failed: %v", err)
		}

		if dataRow.EndControl != SAVEPOINT_CONTINUE {
			t.Errorf("Expected end_control SE, got: %v", dataRow.EndControl)
		}
	})
}

func Test_S_009_FR_016_RollbackSavepointValidation(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	pdr := &PartialDataRow{
		state: PartialDataRowWithStartControl,
		d: DataRow{
			baseRow: baseRow[*DataRowPayload]{
				RowSize:      512,
				StartControl: START_TRANSACTION,
			},
		},
	}

	if err := pdr.AddRow(key, jsonValue); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	t.Run("InvalidSavepointId_TooHigh", func(t *testing.T) {
		_, err := pdr.Rollback(10)
		if err == nil {
			t.Error("Rollback(10) should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("InvalidSavepointId_Negative", func(t *testing.T) {
		_, err := pdr.Rollback(-1)
		if err == nil {
			t.Error("Rollback(-1) should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})
}

func Test_S_009_FR_017_PreventCompletionFromState1(t *testing.T) {

	pdr, err := NewPartialDataRow(512, START_TRANSACTION)
	if err != nil {
		t.Fatalf("NewPartialDataRow failed: %v", err)
	}

	t.Run("Commit_FromState1_Fails", func(t *testing.T) {
		_, err := pdr.Commit()
		if err == nil {
			t.Error("Commit from State1 should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("Rollback_FromState1_Fails", func(t *testing.T) {
		_, err := pdr.Rollback(0)
		if err == nil {
			t.Error("Rollback from State1 should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})

	t.Run("EndRow_FromState1_Fails", func(t *testing.T) {
		_, err := pdr.EndRow()
		if err == nil {
			t.Error("EndRow from State1 should fail")
		}
		if !isInvalidActionError(err) {
			t.Errorf("Expected InvalidActionError, got: %T", err)
		}
	})
}

func Test_S_009_FR_018_CompletionReturnValues(t *testing.T) {
	key := generateValidUUIDv7()
	jsonValue := json.RawMessage(`{"name":"test"}`)

	pdr := &PartialDataRow{
		state: PartialDataRowWithStartControl,
		d: DataRow{
			baseRow: baseRow[*DataRowPayload]{
				RowSize:      512,
				StartControl: START_TRANSACTION,
			},
		},
	}

	if err := pdr.AddRow(key, jsonValue); err != nil {
		t.Fatalf("AddRow failed: %v", err)
	}

	t.Run("Commit_ReturnsDataRowAndNilError", func(t *testing.T) {
		dataRow, err := pdr.Commit()
		if err != nil {
			t.Errorf("Commit should return nil error, got: %v", err)
		}
		if dataRow == nil {
			t.Error("Commit should return non-nil DataRow")
		}
	})

	t.Run("Rollback_ReturnsDataRowAndNilError", func(t *testing.T) {
		pdr2, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr2.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr2.Rollback(0)
		if err != nil {
			t.Errorf("Rollback should return nil error, got: %v", err)
		}
		if dataRow == nil {
			t.Error("Rollback should return non-nil DataRow")
		}
	})

	t.Run("EndRow_ReturnsDataRowAndNilError", func(t *testing.T) {
		pdr3, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}
		if err := pdr3.AddRow(key, jsonValue); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr3.EndRow()
		if err != nil {
			t.Errorf("EndRow should return nil error, got: %v", err)
		}
		if dataRow == nil {
			t.Error("EndRow should return non-nil DataRow")
		}
	})
}

func Test_S_009_FR_019_DataRowValidationBeforeReturn(t *testing.T) {

	t.Run("CommitValidatesDataRow", func(t *testing.T) {
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
			t.Fatalf("Commit should not fail for valid PartialDataRow: %v", err)
		}

		if err := dataRow.Validate(); err != nil {
			t.Errorf("Returned DataRow should be valid: %v", err)
		}
	})

	t.Run("RollbackValidatesDataRow", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.Rollback(0)
		if err != nil {
			t.Fatalf("Rollback should not fail for valid PartialDataRow: %v", err)
		}

		if err := dataRow.Validate(); err != nil {
			t.Errorf("Returned DataRow should be valid: %v", err)
		}
	})

	t.Run("EndRowValidatesDataRow", func(t *testing.T) {
		key := generateValidUUIDv7()
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		dataRow, err := pdr.EndRow()
		if err != nil {
			t.Fatalf("EndRow should not fail for valid PartialDataRow: %v", err)
		}

		if err := dataRow.Validate(); err != nil {
			t.Errorf("Returned DataRow should be valid: %v", err)
		}
	})
}

func Test_S_009_GetState(t *testing.T) {

	t.Run("GetState_ReturnsCurrentState", func(t *testing.T) {
		pdr, err := NewPartialDataRow(512, START_TRANSACTION)
		if err != nil {
			t.Fatalf("NewPartialDataRow failed: %v", err)
		}

		state := pdr.GetState()
		if state != PartialDataRowWithStartControl {
			t.Errorf("Expected PartialDataRowWithStartControl, got %v", state)
		}

		key := generateValidUUIDv7()
		if err := pdr.AddRow(key, json.RawMessage(`{"name":"test"}`)); err != nil {
			t.Fatalf("AddRow failed: %v", err)
		}

		state = pdr.GetState()
		if state != PartialDataRowWithPayload {
			t.Errorf("Expected PartialDataRowWithPayload, got %v", state)
		}
	})
}
