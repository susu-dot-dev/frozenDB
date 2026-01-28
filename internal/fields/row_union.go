package fields

// RowUnion holds pointers to all possible row types.
// Exactly one pointer will be non-nil after unmarshaling.
// Header must be set before calling UnmarshalText.
type RowUnion struct {
	DataRow     *DataRow
	NullRow     *NullRow
	ChecksumRow *ChecksumRow
}

// UnmarshalText unmarshals a row by examining control bytes first.
// Reads start_control at position [1] and end_control at [rowSize-5:rowSize-4].
// Unmarshals directly into the correct type - no trial and error.
// The bytes must match exactly one row (rowSize bytes).
func (ru *RowUnion) UnmarshalText(rowBytes []byte) error {
	rowSize := len(rowBytes)
	if rowSize == 0 {
		return NewInvalidInputError("row bytes cannot be empty", nil)
	}

	// Read start_control at position [1]
	startControl := StartControl(rowBytes[1])

	// Read end_control at positions [rowSize-5:rowSize-4]
	endControlStart := rowSize - 5
	endControl := EndControl{rowBytes[endControlStart], rowBytes[endControlStart+1]}

	// Determine row type from control bytes
	if startControl == CHECKSUM_ROW && endControl == CHECKSUM_ROW_CONTROL {
		// ChecksumRow: start_control='C', end_control='CS'
		ru.ChecksumRow = &ChecksumRow{
			baseRow[*Checksum]{
				RowSize: rowSize,
			},
		}
		if err := ru.ChecksumRow.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError("failed to unmarshal checksum row", err)
		}
	} else if startControl == START_TRANSACTION && endControl == NULL_ROW_CONTROL {
		// NullRow: start_control='T', end_control='NR'
		ru.NullRow = &NullRow{
			baseRow[*NullRowPayload]{
				RowSize: rowSize,
			},
		}
		if err := ru.NullRow.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError("failed to unmarshal null row", err)
		}
	} else if startControl == START_TRANSACTION || startControl == ROW_CONTINUE {
		// DataRow: start_control='T' or 'R'
		ru.DataRow = &DataRow{
			baseRow[*DataRowPayload]{
				RowSize: rowSize,
			},
		}
		if err := ru.DataRow.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError("failed to unmarshal data row", err)
		}
	} else {
		return NewCorruptDatabaseError("unknown row type", nil)
	}
	return ru.Validate()
}

func (ru *RowUnion) Validate() error {
	nonNilRows := 0
	if ru.DataRow != nil {
		nonNilRows++
	}
	if ru.NullRow != nil {
		nonNilRows++
	}
	if ru.ChecksumRow != nil {
		nonNilRows++
	}
	if nonNilRows != 1 {
		return NewInvalidInputError("exactly one row must be non-nil", nil)
	}
	return nil
}

func (ru *RowUnion) ValidateRowSize(rowSize int) error {
	if err := ru.Validate(); err != nil {
		return err
	}
	if ru.DataRow != nil && ru.DataRow.RowSize != rowSize {
		return NewInvalidInputError("data row size mismatch", nil)
	}
	if ru.NullRow != nil && ru.NullRow.RowSize != rowSize {
		return NewInvalidInputError("null row size mismatch", nil)
	}
	if ru.ChecksumRow != nil && ru.ChecksumRow.RowSize != rowSize {
		return NewInvalidInputError("checksum row size mismatch", nil)
	}
	return NewInvalidInputError("No row found", nil)
}
