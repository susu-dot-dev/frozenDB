package frozendb

import "github.com/susu-dot-dev/frozenDB/pkg/header"

// Re-export Header type and constants
type Header = header.Header

// Re-export Header constructor
var NewHeader = header.NewHeader

const (
	HEADER_SIZE      = header.HEADER_SIZE
	HEADER_SIGNATURE = header.HEADER_SIGNATURE
	HEADER_NEWLINE   = header.HEADER_NEWLINE
	MIN_ROW_SIZE     = header.MIN_ROW_SIZE
	MAX_ROW_SIZE     = header.MAX_ROW_SIZE
	MAX_SKEW_MS      = header.MAX_SKEW_MS
)
