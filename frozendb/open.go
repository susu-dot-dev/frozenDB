package frozendb

import (
	"fmt"
	"hash/crc32"
	"io"
)

func validateDatabaseFile(dbFile DBFile) (*Header, error) {
	fileSize := dbFile.Size()

	if fileSize < int64(HEADER_SIZE) {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("file too small for header: expected at least %d bytes, got %d",
				HEADER_SIZE, fileSize),
			nil,
		)
	}

	headerBytes, err := dbFile.Read(0, HEADER_SIZE)
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to read header", err)
	}
	if len(headerBytes) != HEADER_SIZE {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("incomplete header read: expected %d bytes, got %d", HEADER_SIZE, len(headerBytes)),
			nil,
		)
	}

	header := &Header{}
	if err := header.UnmarshalText(headerBytes); err != nil {
		return nil, err
	}

	rowSize := header.GetRowSize()
	expectedMinSize := int64(HEADER_SIZE + rowSize)
	if fileSize < expectedMinSize {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("file too small: expected at least %d bytes (header + checksum row), got %d",
				expectedMinSize, fileSize),
			nil,
		)
	}

	checksumRowBytes, err := dbFile.Read(int64(HEADER_SIZE), int32(rowSize))
	if err != nil && err != io.EOF {
		return nil, NewCorruptDatabaseError("failed to read checksum row", err)
	}

	if len(checksumRowBytes) < rowSize {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("incomplete checksum row read: expected %d bytes, got %d", rowSize, len(checksumRowBytes)),
			nil,
		)
	}

	if checksumRowBytes[0] != ROW_START {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("checksum row must start with ROW_START sentinel (0x%02X), got 0x%02X",
				ROW_START, checksumRowBytes[0]),
			nil,
		)
	}

	if checksumRowBytes[1] != byte(CHECKSUM_ROW) {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("checksum row start_control must be 'C', got 0x%02X", checksumRowBytes[1]),
			nil,
		)
	}

	endCtrlPos := rowSize - 5
	if checksumRowBytes[endCtrlPos] != 'C' || checksumRowBytes[endCtrlPos+1] != 'S' {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("checksum row end_control must be 'CS', got '%c%c'",
				checksumRowBytes[endCtrlPos], checksumRowBytes[endCtrlPos+1]),
			nil,
		)
	}

	if checksumRowBytes[rowSize-1] != ROW_END {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("checksum row must end with ROW_END sentinel (0x%02X), got 0x%02X",
				ROW_END, checksumRowBytes[rowSize-1]),
			nil,
		)
	}

	expectedCRC := crc32.ChecksumIEEE(headerBytes)

	checksumPayloadPos := int64(HEADER_SIZE) + 2
	checksumBytes, err := dbFile.Read(checksumPayloadPos, 8)
	if err != nil && err != io.EOF {
		return nil, NewCorruptDatabaseError("failed to read checksum value", err)
	}

	if len(checksumBytes) < 8 {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("incomplete checksum read: expected 8 bytes, got %d", len(checksumBytes)),
			nil,
		)
	}

	var storedCRC Checksum
	if err := storedCRC.UnmarshalText(checksumBytes); err != nil {
		return nil, NewCorruptDatabaseError("invalid checksum format in checksum row", err)
	}

	if uint32(storedCRC) != expectedCRC {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("CRC32 verification failed: calculated 0x%08X, stored 0x%08X",
				expectedCRC, uint32(storedCRC)),
			nil,
		)
	}

	return header, nil
}
