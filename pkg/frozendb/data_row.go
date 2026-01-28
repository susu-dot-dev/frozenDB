package frozendb

import (
	"github.com/susu-dot-dev/frozenDB/internal/fields"
)

// DataRow is re-exported from internal/fields for public API access.
// DataRow represents a single key-value data row with UUIDv7 key and json.RawMessage value.
// See internal/fields/data_row.go for implementation details.
type DataRow = fields.DataRow

// DataRowPayload is re-exported from internal/fields for public API access.
// DataRowPayload contains the key-value data for a DataRow.
// See internal/fields/data_row_payload.go for implementation details.
type DataRowPayload = fields.DataRowPayload
