package frozendb

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"syscall"
)

func acquireFileLock(file *os.File, mode int, blocking bool) error {
	lockMode := mode
	if !blocking {
		lockMode |= syscall.LOCK_NB
	}

	err := syscall.Flock(int(file.Fd()), lockMode)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return NewWriteError("another process has the database locked", err)
		}
		return NewWriteError("failed to acquire file lock", err)
	}

	return nil
}

func releaseFileLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err != nil {
		return NewWriteError("failed to release file lock", err)
	}
	return nil
}

func validateDatabaseFile(file *os.File) (*Header, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to stat file", err)
	}

	if info.Size() < int64(HEADER_SIZE) {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("file too small for header: expected at least %d bytes, got %d",
				HEADER_SIZE, info.Size()),
			nil,
		)
	}

	headerBytes := make([]byte, HEADER_SIZE)
	n, err := file.Read(headerBytes)
	if err != nil {
		return nil, NewCorruptDatabaseError("failed to read header", err)
	}
	if n != HEADER_SIZE {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("incomplete header read: expected %d bytes, got %d", HEADER_SIZE, n),
			nil,
		)
	}

	header := &Header{}
	if err := header.UnmarshalText(headerBytes); err != nil {
		return nil, err
	}

	rowSize := header.GetRowSize()
	expectedMinSize := int64(HEADER_SIZE + rowSize)
	if info.Size() < expectedMinSize {
		return nil, NewCorruptDatabaseError(
			fmt.Sprintf("file too small: expected at least %d bytes (header + checksum row), got %d",
				expectedMinSize, info.Size()),
			nil,
		)
	}

	checksumRowBytes := make([]byte, rowSize)
	_, err = file.ReadAt(checksumRowBytes, int64(HEADER_SIZE))
	if err != nil && err != io.EOF {
		return nil, NewCorruptDatabaseError("failed to read checksum row", err)
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
	checksumBytes := make([]byte, 8)
	_, err = file.ReadAt(checksumBytes, checksumPayloadPos)
	if err != nil && err != io.EOF {
		return nil, NewCorruptDatabaseError("failed to read checksum value", err)
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

func validateOpenInputs(path string, mode string) error {
	if path == "" {
		return NewInvalidInputError("path cannot be empty", nil)
	}

	if !strings.HasSuffix(path, FILE_EXTENSION) || len(path) <= len(FILE_EXTENSION) {
		return NewInvalidInputError("path must have .fdb extension", nil)
	}

	if mode != MODE_READ && mode != MODE_WRITE {
		return NewInvalidInputError("mode must be 'read' or 'write'", nil)
	}

	return nil
}

func openDatabaseFile(path string, mode string) (*os.File, error) {
	var flags int
	if mode == MODE_READ {
		flags = os.O_RDONLY
	} else {
		flags = os.O_RDWR
	}

	file, err := fsInterface.Open(path, flags, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewPathError("database file does not exist", err)
		}
		if os.IsPermission(err) {
			return nil, NewPathError("permission denied to access database file", err)
		}
		return nil, NewPathError("failed to open database file", err)
	}

	return file, nil
}
