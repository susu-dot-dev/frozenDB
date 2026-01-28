package frozendb

import (
	"github.com/susu-dot-dev/frozenDB/internal/fields"
	"github.com/susu-dot-dev/frozenDB/internal/fileio"
	"github.com/susu-dot-dev/frozenDB/internal/finder"
)

// Re-export internal types for use within frozendb package
// These are NOT part of the public API

type RowUnion = fields.RowUnion
type NullRow = fields.NullRow
type NullRowPayload = fields.NullRowPayload
type PartialDataRow = fields.PartialDataRow
type ChecksumRow = fields.ChecksumRow

type DBFile = fileio.DBFile
type Data = fileio.Data

type Finder = finder.Finder

// Re-export constructor functions
var NewChecksumRow = fields.NewChecksumRow
var NewPartialDataRow = fields.NewPartialDataRow
var NewNullRow = fields.NewNullRow
var NewDataRow = fields.NewDataRow
var NewDBFile = fileio.NewDBFile

// Re-export state constants
const (
	PartialDataRowWithStartControl = fields.PartialDataRowWithStartControl
	PartialDataRowWithPayload      = fields.PartialDataRowWithPayload
	PartialDataRowWithSavepoint    = fields.PartialDataRowWithSavepoint
)

// Re-export sentinel constants for tests
const (
	ROW_START = fields.ROW_START
	ROW_END   = fields.ROW_END
	NULL_BYTE = fields.NULL_BYTE
)

// Re-export UUID helper functions
var ValidateUUIDv7 = fields.ValidateUUIDv7
var ExtractUUIDv7Timestamp = fields.ExtractUUIDv7Timestamp
var CreateNullRowUUID = fields.CreateNullRowUUID
var IsNullRowUUID = fields.IsNullRowUUID
