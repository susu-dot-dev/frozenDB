package frozendb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

// Test_RowUnion tests row type detection using RowUnion
func Test_RowUnion(t *testing.T) {
	// Test with valid DataRow
	t.Run("valid_data_row", func(t *testing.T) {
		key := uuid.Must(uuid.NewV7())
		value := json.RawMessage(`{"test":"value"}`)
		payload := &DataRowPayload{Key: key, Value: value}
		dataRow := &DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      256,
				StartControl: START_TRANSACTION,
				EndControl:   TRANSACTION_COMMIT,
				RowPayload:   payload,
			},
		}

		rowBytes, err := dataRow.MarshalText()
		if err != nil {
			t.Fatalf("Failed to marshal data row: %v", err)
		}

		var rowUnion RowUnion
		err = rowUnion.UnmarshalText(rowBytes)
		if err != nil {
			t.Errorf("RowUnion should succeed for valid DataRow, got error: %v", err)
		}
		if rowUnion.DataRow == nil {
			t.Error("RowUnion should have DataRow set")
		}
	})

	// Test with valid ChecksumRow
	t.Run("valid_checksum_row", func(t *testing.T) {
		checksum := Checksum(0x12345678)
		checksumRow := &ChecksumRow{
			baseRow[*Checksum]{
				RowSize:      256,
				StartControl: CHECKSUM_ROW,
				EndControl:   CHECKSUM_ROW_CONTROL,
				RowPayload:   &checksum,
			},
		}

		rowBytes, err := checksumRow.MarshalText()
		if err != nil {
			t.Fatalf("Failed to marshal checksum row: %v", err)
		}

		var rowUnion RowUnion
		err = rowUnion.UnmarshalText(rowBytes)
		if err != nil {
			t.Errorf("RowUnion should succeed for valid ChecksumRow, got error: %v", err)
		}
		if rowUnion.ChecksumRow == nil {
			t.Error("RowUnion should have ChecksumRow set")
		}
	})

	// Test with invalid row
	t.Run("invalid_row", func(t *testing.T) {
		invalidRow := make([]byte, 256)
		// Fill with garbage data
		for i := range invalidRow {
			invalidRow[i] = byte(i)
		}

		var rowUnion RowUnion
		err := rowUnion.UnmarshalText(invalidRow)
		if err == nil {
			t.Error("RowUnion should fail for invalid row")
		}
	})
}

// Test_Verify_EmptyPath tests that Verify rejects empty path
func Test_Verify_EmptyPath(t *testing.T) {
	err := Verify("")
	if err == nil {
		t.Error("Verify should fail for empty path")
	}

	var inputErr *InvalidInputError
	if !errors.As(err, &inputErr) {
		t.Errorf("Expected InvalidInputError for empty path, got %T", err)
	}
}

// Test_Verify_NonExistentFile tests that Verify handles non-existent files
func Test_Verify_NonExistentFile(t *testing.T) {
	err := Verify("/nonexistent/path/to/database.db")
	if err == nil {
		t.Error("Verify should fail for non-existent file")
	}

	var readErr *ReadError
	if !errors.As(err, &readErr) {
		t.Errorf("Expected ReadError for non-existent file, got %T", err)
	}
}

// Test_Verify_FileTooSmall tests that Verify rejects files smaller than minimum size
func Test_Verify_FileTooSmall(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "frozendb_verify_small_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write only 32 bytes (less than header size of 64)
	if _, err := tmpFile.Write(make([]byte, 32)); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	tmpFile.Close()

	err = Verify(tmpPath)
	if err == nil {
		t.Error("Verify should fail for file smaller than 64 bytes")
	}

	var corruptErr *CorruptDatabaseError
	if !errors.As(err, &corruptErr) {
		t.Errorf("Expected CorruptDatabaseError for file too small, got %T", err)
	}
}

// ============================================================================
// Phase 2: Integration Tests for Checksum Edge Cases
// ============================================================================

// Helper to create database with N rows for integration tests
func createDatabaseWithRows(t *testing.T, path string, rowSize int, numRows int) {
	t.Helper()

	// Create initial database file
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	header := &Header{
		signature: HEADER_SIGNATURE,
		version:   1,
		rowSize:   rowSize,
		skewMs:    5000,
	}

	// Write header
	headerBytes, err := header.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal header: %v", err)
	}
	if _, err := file.Write(headerBytes); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Write initial checksum
	checksumRow, err := NewChecksumRow(header.GetRowSize(), headerBytes)
	if err != nil {
		t.Fatalf("Failed to create checksum row: %v", err)
	}
	checksumBytes, err := checksumRow.MarshalText()
	if err != nil {
		t.Fatalf("Failed to marshal checksum row: %v", err)
	}
	if _, err := file.Write(checksumBytes); err != nil {
		t.Fatalf("Failed to write checksum row: %v", err)
	}
	file.Close()

	if numRows == 0 {
		return
	}

	// Open for writing and add rows
	db, err := NewFrozenDB(path, MODE_WRITE, FinderStrategySimple)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add rows in batches of 100 (transaction limit)
	numBatches := (numRows + 99) / 100
	for batch := 0; batch < numBatches; batch++ {
		tx, err := db.BeginTx()
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		startIdx := batch * 100
		endIdx := startIdx + 100
		if endIdx > numRows {
			endIdx = numRows
		}

		for i := startIdx; i < endIdx; i++ {
			key := uuid.Must(uuid.NewV7())
			value := json.RawMessage(fmt.Sprintf(`{"row":%d}`, i))
			if err := tx.AddRow(key, value); err != nil {
				t.Fatalf("Failed to add row %d: %v", i, err)
			}
		}

		if err := tx.Commit(); err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}
	}
}

// Test_Verify_Exactly10000Rows tests verification of file with exactly 10,000 rows (boundary condition)
func Test_Verify_Exactly10000Rows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpFile, err := os.CreateTemp("", "frozendb_verify_10k_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create database with exactly 10,000 rows
	createDatabaseWithRows(t, tmpPath, 256, 10000)

	// Verify should succeed
	err = Verify(tmpPath)
	if err != nil {
		t.Errorf("Verify should succeed for file with exactly 10,000 rows, got error: %v", err)
	}
}

// Test_Verify_25000Rows tests verification with multiple checksum blocks
func Test_Verify_25000Rows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpFile, err := os.CreateTemp("", "frozendb_verify_25k_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create database with 25,000 rows (will have 2 checksum blocks after initial)
	createDatabaseWithRows(t, tmpPath, 256, 25000)

	// Verify should succeed
	err = Verify(tmpPath)
	if err != nil {
		t.Errorf("Verify should succeed for file with 25,000 rows, got error: %v", err)
	}
}

// Test_Verify_CorruptedSecondChecksum tests detection of corrupted second checksum block
func Test_Verify_CorruptedSecondChecksum(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpFile, err := os.CreateTemp("", "frozendb_verify_corrupt_cs2_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create database with 12,000 rows to ensure second checksum exists
	createDatabaseWithRows(t, tmpPath, 256, 12000)

	// Now corrupt the second checksum block (at position: 64 + 256 + 10000*256)
	// Second checksum is at physical row position 10001 (after initial checksum + 10000 data rows)
	secondChecksumOffset := int64(64 + 256 + 10000*256)

	file, err := os.OpenFile(tmpPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for corruption: %v", err)
	}

	// Corrupt a byte in the checksum payload area
	corruptOffset := secondChecksumOffset + 10 // Corrupt somewhere in the middle
	if _, err := file.WriteAt([]byte{0xFF}, corruptOffset); err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}
	file.Close()

	// Verify should fail
	err = Verify(tmpPath)
	if err == nil {
		t.Error("Verify should fail when second checksum block is corrupted")
	}

	var corruptErr *CorruptDatabaseError
	if !errors.As(err, &corruptErr) {
		t.Errorf("Expected CorruptDatabaseError, got %T: %v", err, err)
	}
}

// Test_Verify_VariousRowSizes tests verification with different row sizes
func Test_Verify_VariousRowSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	rowSizes := []int{128, 256, 1024, 65536}

	for _, rowSize := range rowSizes {
		t.Run(fmt.Sprintf("rowSize_%d", rowSize), func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", fmt.Sprintf("frozendb_verify_rs%d_test_*.fdb", rowSize))
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close()
			defer os.Remove(tmpPath)

			// Create database with specified row size and 100 rows
			createDatabaseWithRows(t, tmpPath, rowSize, 100)

			// Verify should succeed
			err = Verify(tmpPath)
			if err != nil {
				t.Errorf("Verify should succeed for row size %d, got error: %v", rowSize, err)
			}
		})
	}
}

// Test_Verify_SecondChecksumValidation tests that the second checksum at row 10,000 is validated
func Test_Verify_SecondChecksumValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpFile, err := os.CreateTemp("", "frozendb_verify_2nd_cs_test_*.fdb")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create database with 10,001 rows to trigger second checksum
	// (10,000 rows will NOT have a second checksum - it appears AFTER the 10,000th row)
	createDatabaseWithRows(t, tmpPath, 256, 10001)

	// First verify the file is valid
	err = Verify(tmpPath)
	if err != nil {
		t.Fatalf("Initial verify should succeed, got error: %v", err)
	}

	// Now corrupt the SECOND checksum row
	// Checksum position 1: offset = 64 + 1*10001*256 = 64 + 2,560,256 = 2,560,320
	secondChecksumOffset := int64(64 + 1*10001*256)

	t.Logf("Second checksum offset: %d", secondChecksumOffset)

	file, err := os.OpenFile(tmpPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for corruption: %v", err)
	}

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	t.Logf("File size: %d bytes", fileInfo.Size())

	// Corrupt a byte in a DATA row (one of the 10,000 data rows before the second checksum)
	// We need to also fix the parity bytes so the row itself is still valid structurally
	// but the checksum will detect the change
	dataRowOffset := secondChecksumOffset - 256 // Last data row before the checksum
	t.Logf("Corrupting data row at offset: %d", dataRowOffset)

	// Read the entire row
	dataRow := make([]byte, 256)
	if _, err := file.ReadAt(dataRow, dataRowOffset); err != nil {
		t.Fatalf("Failed to read data row: %v", err)
	}

	// Corrupt a byte in the padding area (position 100, should be in padding)
	corruptPosition := 100
	originalByte := dataRow[corruptPosition]
	dataRow[corruptPosition] = originalByte ^ 0xFF
	t.Logf("Corrupted byte at position %d in row: 0x%02X -> 0x%02X", corruptPosition, originalByte, dataRow[corruptPosition])

	// Recalculate parity bytes (positions rowSize-3 and rowSize-2)
	// Parity is XOR of bytes [0] through [rowSize-4]
	var parity byte = 0x00
	for i := 0; i < 256-3; i++ {
		parity ^= dataRow[i]
	}
	parityHex := fmt.Sprintf("%02X", parity)
	dataRow[256-3] = parityHex[0]
	dataRow[256-2] = parityHex[1]
	t.Logf("Updated parity bytes to: %s", parityHex)

	// Write the corrupted row back
	if _, err := file.WriteAt(dataRow, dataRowOffset); err != nil {
		t.Fatalf("Failed to write corrupted row: %v", err)
	}
	file.Close()

	// Verify should now fail because the second checksum is corrupted
	err = Verify(tmpPath)
	if err == nil {
		t.Error("Verify should fail when second checksum is corrupted")
	} else {
		t.Logf("Verify correctly detected corruption: %v", err)
	}

	var corruptErr *CorruptDatabaseError
	if !errors.As(err, &corruptErr) {
		t.Errorf("Expected CorruptDatabaseError, got %T: %v", err, err)
	}
}
