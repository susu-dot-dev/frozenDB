package frozendb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	HEADER_SIZE      = 64
	HEADER_SIGNATURE = "fDB"
	MIN_ROW_SIZE     = 128
	MAX_ROW_SIZE     = 65536
	MAX_SKEW_MS      = 86400000
	PADDING_CHAR     = '\x00'
	HEADER_NEWLINE   = '\n'
)

const HEADER_FORMAT = `{"sig":"fDB","ver":1,"row_size":%d,"skew_ms":%d}`

type headerJSON struct {
	Sig     string `json:"sig"`
	Ver     int    `json:"ver"`
	RowSize int    `json:"row_size"`
	SkewMs  int    `json:"skew_ms"`
}

type Header struct {
	signature string
	version   int
	rowSize   int
	skewMs    int
}

func (h *Header) GetSignature() string {
	return h.signature
}

func (h *Header) GetVersion() int {
	return h.version
}

func (h *Header) GetRowSize() int {
	return h.rowSize
}

func (h *Header) GetSkewMs() int {
	return h.skewMs
}

func (h *Header) UnmarshalText(headerBytes []byte) error {
	if len(headerBytes) != HEADER_SIZE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("header must be exactly %d bytes, got %d", HEADER_SIZE, len(headerBytes)),
			nil,
		)
	}

	if headerBytes[63] != HEADER_NEWLINE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("byte 63 must be newline, got 0x%02x", headerBytes[63]),
			nil,
		)
	}

	nullPos := bytes.IndexByte(headerBytes, PADDING_CHAR)
	if nullPos == -1 {
		return NewCorruptDatabaseError("no null terminator found in header", nil)
	}

	jsonContent := headerBytes[:nullPos]

	for i := nullPos; i < 63; i++ {
		if headerBytes[i] != PADDING_CHAR {
			return NewCorruptDatabaseError(
				fmt.Sprintf("padding byte at position %d must be null, got 0x%02x", i, headerBytes[i]),
				nil,
			)
		}
	}

	var hdr headerJSON
	if err := json.Unmarshal(jsonContent, &hdr); err != nil {
		return NewCorruptDatabaseError("failed to parse JSON header", err)
	}

	h.signature = hdr.Sig
	h.version = hdr.Ver
	h.rowSize = hdr.RowSize
	h.skewMs = hdr.SkewMs

	return h.Validate()
}

func (h *Header) Validate() error {
	if h.signature != HEADER_SIGNATURE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("invalid signature: expected '%s', got '%s'", HEADER_SIGNATURE, h.signature),
			nil,
		)
	}

	if h.version != 1 {
		return NewCorruptDatabaseError(
			fmt.Sprintf("unsupported version: expected 1, got %d", h.version),
			nil,
		)
	}

	if h.rowSize < MIN_ROW_SIZE || h.rowSize > MAX_ROW_SIZE {
		return NewCorruptDatabaseError(
			fmt.Sprintf("row_size must be between %d and %d, got %d", MIN_ROW_SIZE, MAX_ROW_SIZE, h.rowSize),
			nil,
		)
	}

	if h.skewMs < 0 || h.skewMs > MAX_SKEW_MS {
		return NewCorruptDatabaseError(
			fmt.Sprintf("skew_ms must be between 0 and %d, got %d", MAX_SKEW_MS, h.skewMs),
			nil,
		)
	}

	return nil
}

func (h *Header) MarshalText() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}

	jsonContent := fmt.Sprintf(HEADER_FORMAT, h.rowSize, h.skewMs)

	contentLength := len(jsonContent)
	if contentLength > 58 {
		return nil, NewInvalidInputError("header content too long", nil)
	}

	paddingLength := 63 - contentLength
	padding := strings.Repeat(string(PADDING_CHAR), paddingLength)

	header := jsonContent + padding + string(HEADER_NEWLINE)

	return []byte(header), nil
}
