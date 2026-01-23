package frozendb

import (
	"testing"
)

// Test_S_004_FR_004_UnmarshalTextCallsValidate tests FR-004: System MUST call Validate() in all UnmarshalText() methods before returning from unmarshaling
func Test_S_004_FR_004_UnmarshalTextCallsValidate(t *testing.T) {
	// Test StartControl UnmarshalText() calls Validate()
	var sc StartControl
	validText := []byte{'T'}
	err := sc.UnmarshalText(validText)
	if err != nil {
		t.Errorf("StartControl.UnmarshalText() with valid input should succeed: %v", err)
	}
	// Verify the value was set correctly
	if sc != START_TRANSACTION {
		t.Errorf("Expected START_TRANSACTION, got: %c", sc)
	}
	// Verify Validate() can be called after UnmarshalText
	err = sc.Validate()
	if err != nil {
		t.Errorf("StartControl.Validate() should succeed after valid UnmarshalText: %v", err)
	}

	// Test invalid StartControl - UnmarshalText() should call Validate() and fail
	var invalidSc StartControl
	invalidText := []byte{'X'}
	err = invalidSc.UnmarshalText(invalidText)
	if err == nil {
		t.Error("StartControl.UnmarshalText() with invalid input should fail validation")
	}

	// Test EndControl UnmarshalText() calls Validate()
	var ec EndControl
	validEndText := []byte{'T', 'C'}
	err = ec.UnmarshalText(validEndText)
	if err != nil {
		t.Errorf("EndControl.UnmarshalText() with valid input should succeed: %v", err)
	}
	// Verify the value was set correctly
	if ec[0] != 'T' || ec[1] != 'C' {
		t.Errorf("Expected {'T','C'}, got: {'%c','%c'}", ec[0], ec[1])
	}
	// Verify Validate() can be called after UnmarshalText
	err = ec.Validate()
	if err != nil {
		t.Errorf("EndControl.Validate() should succeed after valid UnmarshalText: %v", err)
	}

	// Test invalid EndControl - UnmarshalText() should call Validate() and fail
	var invalidEc EndControl
	invalidEndText := []byte{'X', 'Y'}
	err = invalidEc.UnmarshalText(invalidEndText)
	if err == nil {
		t.Error("EndControl.UnmarshalText() with invalid input should fail validation")
	}

	// Test Checksum UnmarshalText() - Checksum is universally valid (uint32 is always valid)
	var checksum Checksum
	// Valid Base64 checksum (8 bytes with "==" padding)
	validChecksumText := []byte("AAAAAA==")
	err = checksum.UnmarshalText(validChecksumText)
	if err != nil {
		t.Errorf("Checksum.UnmarshalText() with valid input should succeed: %v", err)
	}

	// Test invalid Checksum - UnmarshalText() should fail on invalid Base64
	var invalidChecksum Checksum
	invalidChecksumText := []byte("invalid")
	err = invalidChecksum.UnmarshalText(invalidChecksumText)
	if err == nil {
		t.Error("Checksum.UnmarshalText() with invalid input should fail")
	}

	// Test ChecksumRow UnmarshalText() calls Validate()
	// First create a valid checksum row to marshal
	cr, err := NewChecksumRow(1024, []byte("test data"))
	if err != nil {
		t.Fatalf("Failed to create ChecksumRow: %v", err)
	}

	// Marshal it to get valid bytes
	rowBytes, err := cr.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal ChecksumRow: %v", err)
	}

	// Now unmarshal - should call Validate() internally
	unmarshaledCr := &ChecksumRow{}
	err = unmarshaledCr.UnmarshalText(rowBytes)
	if err != nil {
		t.Errorf("ChecksumRow.UnmarshalText() with valid input should succeed: %v", err)
	}

	// Test invalid ChecksumRow - UnmarshalText() should call Validate() and fail
	invalidCr := &ChecksumRow{}
	// Create invalid row bytes (wrong start control)
	invalidRowBytes := make([]byte, 1024)
	invalidRowBytes[0] = ROW_START
	invalidRowBytes[1] = 'T' // Wrong: should be 'C' for checksum
	err = invalidCr.UnmarshalText(invalidRowBytes)
	if err == nil {
		t.Error("ChecksumRow.UnmarshalText() with invalid input should fail validation")
	}
}

// Test_S_004_FR_013_ChildValidatedDuringConstruction tests FR-013: System MUST call Validate() on child structs during their construction (in NewStruct() or UnmarshalText()) before parent validation
func Test_S_004_FR_013_ChildValidatedDuringConstruction(t *testing.T) {
	// Test that NewChecksumRow() calls Validate() on child structs during construction

	// NewChecksumRow creates Checksum (child) during construction
	// Checksum is universally valid (uint32 is always valid), so no validation needed
	cr, err := NewChecksumRow(1024, []byte("test data"))
	if err != nil {
		t.Fatalf("NewChecksumRow should succeed with valid inputs: %v", err)
	}

	// Verify that child (Checksum) was validated during construction
	// If Checksum was invalid, NewChecksumRow would have failed
	if cr.RowPayload == nil {
		t.Error("ChecksumRow.RowPayload should not be nil after construction")
	}

	// Verify that baseRow children (StartControl, EndControl) were validated
	// during construction - if they were invalid, NewChecksumRow would have failed
	if cr.StartControl != CHECKSUM_ROW {
		t.Errorf("StartControl should be 'C', got: %c", cr.StartControl)
	}
	if cr.EndControl != CHECKSUM_ROW_CONTROL {
		t.Errorf("EndControl should be 'CS', got: %s", cr.EndControl.String())
	}

	// Test that UnmarshalText() calls Validate() on child structs
	rowBytes, err := cr.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal ChecksumRow: %v", err)
	}

	// UnmarshalText() should call Validate() on child structs (StartControl, EndControl, Checksum)
	unmarshaledCr := &ChecksumRow{}
	err = unmarshaledCr.UnmarshalText(rowBytes)
	if err != nil {
		t.Errorf("UnmarshalText() should succeed and validate children: %v", err)
	}

	// Verify children were validated (if they were invalid, UnmarshalText would have failed)
	if unmarshaledCr.StartControl != CHECKSUM_ROW {
		t.Error("StartControl should be validated during UnmarshalText()")
	}
	if unmarshaledCr.EndControl != CHECKSUM_ROW_CONTROL {
		t.Error("EndControl should be validated during UnmarshalText()")
	}
	if unmarshaledCr.RowPayload == nil {
		t.Error("Checksum should be validated during UnmarshalText()")
	}
}
