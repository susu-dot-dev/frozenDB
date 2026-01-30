package frozendb

import (
	"fmt"
	"hash/crc32"
	"os"
)

// Verify validates the integrity of a frozenDB file.
//
// Verify performs comprehensive validation using a two-pass approach:
//
// Pass 1 - Checksum Validation:
//   - Validate initial checksum at offset 64 covers header [0..64)
//   - For each expected checksum position (every 10,000 data/null rows):
//   - Read checksum row, parse with ChecksumRow.UnmarshalText()
//   - Calculate byte range covered by this checksum
//   - Read bytes, calculate CRC32, compare to checksum value
//
// Pass 2 - Row Validation:
//   - Validate header with Header.UnmarshalText()
//   - For each row in file:
//   - Call UnmarshalText() to validate structure and parity
//   - Works for all row types (data, null, checksum)
//   - If file doesn't end on row boundary, validate as PartialDataRow
//
// Verify validates:
//   - Header structure and field values (64-byte header, signature, version, row_size, skew_ms)
//   - All checksum blocks (initial checksum covering header, subsequent checksums every 10,000 rows)
//   - Parity bytes for all rows after the last checksum block
//   - Row format compliance (ROW_START, ROW_END, control bytes, UUID format, JSON validity, padding)
//   - Partial data row validity if present as the last row
//
// Verify does NOT validate:
//   - Transaction nesting or state relationships between rows
//   - UUID timestamp ordering constraints
//   - Savepoint numbering or rollback semantics
func Verify(path string) error {
	// Validate input
	if path == "" {
		return NewInvalidInputError("path cannot be empty", nil)
	}

	// Open file for reading
	file, err := os.Open(path)
	if err != nil {
		return NewReadError(fmt.Sprintf("failed to open file: %s", path), err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = NewReadError("failed to close file", closeErr)
		}
	}()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return NewReadError("failed to get file info", err)
	}
	fileSize := fileInfo.Size()

	// Minimum file size: 64-byte header + 1 checksum row (128 bytes minimum)
	if fileSize < 64 {
		return NewCorruptDatabaseError("file too small: must be at least 64 bytes for header", nil)
	}

	// Read and validate header first (needed to get row_size)
	headerBytes := make([]byte, HEADER_SIZE)
	n, err := file.ReadAt(headerBytes, 0)
	if err != nil || n != HEADER_SIZE {
		return NewCorruptDatabaseError("failed to read header: file must be at least 64 bytes", err)
	}

	var header Header
	if err := header.UnmarshalText(headerBytes); err != nil {
		return NewCorruptDatabaseError(fmt.Sprintf("invalid header at offset 0: %v", err), err)
	}

	rowSize := header.GetRowSize()

	// Validate minimum file size for initial checksum
	if fileSize < int64(HEADER_SIZE+rowSize) {
		return NewCorruptDatabaseError(fmt.Sprintf("file too small: must have at least header (64 bytes) + initial checksum row (%d bytes)", rowSize), nil)
	}

	// PASS 1: Validate All Checksums (initial + subsequent)
	if err := validateAllChecksums(file, fileSize, rowSize); err != nil {
		return err
	}

	// PASS 2: Validate All Rows (structure and parity for rows after last checksum)
	if err := validateAllRows(file, fileSize, rowSize); err != nil {
		return err
	}

	return nil
}

// validateAllChecksums performs Pass 1: validates all checksum rows in the file
func validateAllChecksums(file *os.File, fileSize int64, rowSize int) error {
	// Checksum positions follow this pattern:
	// Checksum 0: offset 64 (covers header bytes [0, 64))
	// Checksum 1: offset 64 + 1*(rowSize + 10000*rowSize) = 64 + 10001*rowSize
	//             (covers checksum 0 + 10,000 data rows)
	// Checksum 2: offset 64 + 2*(rowSize + 10000*rowSize) = 64 + 2*10001*rowSize
	//             (covers checksum 1 + next 10,000 data rows)
	// Checksum i: offset = 64 + i*(rowSize + 10000*rowSize) = 64 + i*10001*rowSize

	checksumIndex := 0

	for {
		// Calculate checksum position
		var checksumOffset int64
		var rangeStart int64
		var rangeLength int64

		if checksumIndex == 0 {
			// Initial checksum at offset 64
			checksumOffset = int64(HEADER_SIZE)
			rangeStart = 0
			rangeLength = HEADER_SIZE
		} else {
			// Subsequent checksums: 64 + checksumIndex * 10001 * rowSize
			checksumOffset = int64(HEADER_SIZE + checksumIndex*10001*rowSize)

			// Range starts at previous checksum offset
			previousChecksumOffset := int64(HEADER_SIZE + (checksumIndex-1)*10001*rowSize)
			rangeStart = previousChecksumOffset
			rangeLength = checksumOffset - previousChecksumOffset
		}

		// Check if this checksum should exist based on file size
		// A checksum exists if there's enough space for: the checksum row + at least 1 more row after it
		// OR if it's the last complete row in the file
		if checksumOffset+int64(rowSize) > fileSize {
			// Not enough space for a complete checksum row
			break
		}

		// Read checksum row
		checksumRowBytes := make([]byte, rowSize)
		n, err := file.ReadAt(checksumRowBytes, checksumOffset)
		if err != nil || n != rowSize {
			return NewCorruptDatabaseError(fmt.Sprintf("failed to read checksum row at offset %d", checksumOffset), err)
		}

		// Parse checksum row - this MUST succeed since we expect a checksum at this position
		var checksumRow ChecksumRow
		if err := checksumRow.UnmarshalText(checksumRowBytes); err != nil {
			return NewCorruptDatabaseError(fmt.Sprintf("invalid checksum row at offset %d: %v", checksumOffset, err), err)
		}

		// Read the bytes that should be covered by this checksum
		dataToChecksum := make([]byte, rangeLength)
		if _, err := file.ReadAt(dataToChecksum, rangeStart); err != nil {
			return NewReadError(fmt.Sprintf("failed to read data for checksum validation at offset %d", checksumOffset), err)
		}

		// Calculate expected checksum
		expectedChecksum := crc32.ChecksumIEEE(dataToChecksum)

		// Compare checksums
		if Checksum(expectedChecksum) != *checksumRow.RowPayload {
			return NewCorruptDatabaseError(
				fmt.Sprintf("checksum mismatch at offset %d (expected %08X, got %08X)",
					checksumOffset, expectedChecksum, *checksumRow.RowPayload),
				nil,
			)
		}

		checksumIndex++
	}

	return nil
}

// validateAllRows performs Pass 2: row-by-row validation
// Validates structure and parity for all rows
func validateAllRows(file *os.File, fileSize int64, rowSize int) error {
	// Start at offset 64 (after header)
	currentOffset := int64(HEADER_SIZE)

	for currentOffset < fileSize {
		remainingBytes := fileSize - currentOffset

		// Check if we have a full row
		if remainingBytes < int64(rowSize) {
			// Partial row - validate as PartialDataRow
			partialBytes := make([]byte, remainingBytes)
			if _, err := file.ReadAt(partialBytes, currentOffset); err != nil {
				return NewReadError(fmt.Sprintf("failed to read partial row at offset %d", currentOffset), err)
			}

			// Try to parse as PartialDataRow
			var partialRow PartialDataRow
			if err := partialRow.UnmarshalText(partialBytes); err != nil {
				return NewCorruptDatabaseError(
					fmt.Sprintf("invalid partial row at offset %d: %v", currentOffset, err),
					err,
				)
			}

			break
		}

		// Read full row
		rowBytes := make([]byte, rowSize)
		if _, err := file.ReadAt(rowBytes, currentOffset); err != nil {
			return NewReadError(fmt.Sprintf("failed to read row at offset %d", currentOffset), err)
		}

		// Use RowUnion to unmarshal and validate the row
		// RowUnion will automatically detect the row type from control bytes
		// This validates structure, parity, and all row-specific fields
		var rowUnion RowUnion
		if err := rowUnion.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError(
				fmt.Sprintf("invalid row at offset %d: %v", currentOffset, err),
				err,
			)
		}

		currentOffset += int64(rowSize)
	}

	return nil
}
