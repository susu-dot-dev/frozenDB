package fields

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/susu-dot-dev/frozenDB/pkg/types"
)

// Re-export error constructors (add to what row.go already exports)
var NewInvalidActionError = types.NewInvalidActionError

type PartialRowState int

const (
	PartialDataRowWithStartControl PartialRowState = iota + 1
	PartialDataRowWithPayload
	PartialDataRowWithSavepoint
)

func (s PartialRowState) String() string {
	switch s {
	case PartialDataRowWithStartControl:
		return "PartialDataRowWithStartControl"
	case PartialDataRowWithPayload:
		return "PartialDataRowWithPayload"
	case PartialDataRowWithSavepoint:
		return "PartialDataRowWithSavepoint"
	default:
		return fmt.Sprintf("PartialRowState(%d)", s)
	}
}

type PartialDataRow struct {
	state PartialRowState
	d     DataRow
}

func NewPartialDataRow(rowSize int, startControl StartControl) (*PartialDataRow, error) {
	pd := &PartialDataRow{
		state: PartialDataRowWithStartControl,
		d: DataRow{
			baseRow[*DataRowPayload]{
				RowSize:      rowSize,
				StartControl: startControl,
			},
		},
	}
	if err := pd.Validate(); err != nil {
		return nil, err
	}
	return pd, nil
}

func (pdr *PartialDataRow) GetState() PartialRowState {
	return pdr.state
}

// SetRowSize sets the row size for the internal DataRow.
// This is used during recovery when unmarshaling a PartialDataRow.
func (pdr *PartialDataRow) SetRowSize(rowSize int) {
	pdr.d.RowSize = rowSize
}

func (pdr *PartialDataRow) AddRow(key uuid.UUID, json json.RawMessage) error {
	if pdr.d.RowSize == -1 {
		return NewInvalidActionError("RowSize is not set", nil)
	}

	if pdr.state != PartialDataRowWithStartControl {
		return NewInvalidActionError("AddRow() can only be called from PartialDataRowWithStartControl", nil)
	}

	if err := ValidateUUIDv7(key); err != nil {
		return err
	}

	if len(json) == 0 {
		return NewInvalidInputError("value cannot be empty", nil)
	}

	pdr.d.RowPayload = &DataRowPayload{
		Key:   key,
		Value: json,
	}

	pdr.state = PartialDataRowWithPayload

	return pdr.Validate()
}

func (pdr *PartialDataRow) Savepoint() error {
	if pdr.d.RowSize == -1 {
		return NewInvalidActionError("RowSize is not set", nil)
	}
	if pdr.state != PartialDataRowWithPayload {
		return NewInvalidActionError("Savepoint() can only be called from PartialDataRowWithPayload", nil)
	}

	pdr.state = PartialDataRowWithSavepoint

	return pdr.Validate()
}

func (pdr *PartialDataRow) Validate() error {
	if pdr.state == 0 {
		return NewInvalidInputError("PartialDataRow state is required", nil)
	}
	if err := validateStartControl(pdr.d.StartControl); err != nil {
		return err
	}
	if pdr.state == PartialDataRowWithStartControl {
		return nil
	}
	if pdr.d.RowSize != -1 {
		err := validatePayload(pdr.d.RowPayload, pdr.d.RowSize)
		if err != nil {
			return err
		}
	}
	return nil
}

func (pdr *PartialDataRow) MarshalText() ([]byte, error) {
	if pdr.d.RowSize == -1 {
		return nil, NewInvalidActionError("RowSize is not set", nil)
	}
	switch pdr.state {
	case PartialDataRowWithStartControl:
		return pdr.d.BuildRowStartAndControl()
	case PartialDataRowWithPayload:
		payloadBytes, err := pdr.d.RowPayload.MarshalText()
		if err != nil {
			return nil, NewInvalidInputError("failed to marshal row payload", err)
		}
		return pdr.d.BuildRowStartControlAndPayload(payloadBytes)
	case PartialDataRowWithSavepoint:
		payloadBytes, err := pdr.d.RowPayload.MarshalText()
		if err != nil {
			return nil, NewInvalidInputError("failed to marshal row payload", err)
		}
		state2Bytes, err := pdr.d.BuildRowStartControlAndPayload(payloadBytes)
		if err != nil {
			return nil, err
		}
		return append(state2Bytes, 'S'), nil
	default:
		return nil, NewInvalidInputError(fmt.Sprintf("unknown PartialDataRow state: %d", pdr.state), nil)
	}
}

func (pdr *PartialDataRow) UnmarshalText(text []byte) error {
	if len(text) < 2 {
		return NewInvalidInputError("PartialDataRow text must be at least 2 bytes", nil)
	}

	if text[0] != ROW_START {
		return NewInvalidInputError(fmt.Sprintf("expected ROW_START (0x1F), got 0x%02X", text[0]), nil)
	}

	if err := pdr.d.StartControl.UnmarshalText(text[1:2]); err != nil {
		return NewCorruptDatabaseError("invalid start_control", err)
	}

	if len(text) == 2 {
		pdr.state = PartialDataRowWithStartControl
		pdr.d.RowPayload = nil
		return nil
	}

	isState3 := false
	if text[len(text)-1] == 'S' {
		isState3 = true
		text = text[:len(text)-1]
	}

	nullIndex := bytes.IndexByte(text[2:], NULL_BYTE)
	if nullIndex == -1 {
		return NewInvalidInputError("no null byte found in PartialDataRow payload", nil)
	}
	nullIndex += 2

	payloadBytes := text[2:nullIndex]

	if nullIndex < len(text) && text[nullIndex] == 'S' {
		pdr.state = PartialDataRowWithSavepoint
	} else if isState3 {
		pdr.state = PartialDataRowWithSavepoint
	} else {
		pdr.state = PartialDataRowWithPayload
	}

	pdr.d.RowPayload = &DataRowPayload{}
	if err := pdr.d.RowPayload.UnmarshalText(payloadBytes); err != nil {
		return NewCorruptDatabaseError("failed to unmarshal payload", err)
	}

	pdr.d.RowSize = -1 // We don't know what the row size is from UnmarshalText, it needs to be set by the caller
	if err := pdr.Validate(); err != nil {
		return NewCorruptDatabaseError("validation failed", err)
	}

	return nil
}

func (pdr *PartialDataRow) Commit() (*DataRow, error) {
	if pdr.d.RowSize == -1 {
		return nil, NewInvalidActionError("RowSize is not set", nil)
	}
	if pdr.state == PartialDataRowWithStartControl {
		return nil, NewInvalidActionError("Commit() cannot be called from PartialDataRowWithStartControl", nil)
	}

	var endControl EndControl
	if pdr.state == PartialDataRowWithPayload {
		endControl = TRANSACTION_COMMIT
	} else {
		endControl = SAVEPOINT_COMMIT
	}

	return pdr.complete(endControl)
}

func (pdr *PartialDataRow) Rollback(savepointId int) (*DataRow, error) {
	if pdr.d.RowSize == -1 {
		return nil, NewInvalidActionError("RowSize is not set", nil)
	}
	if pdr.state == PartialDataRowWithStartControl {
		return nil, NewInvalidActionError("Rollback() cannot be called from PartialDataRowWithStartControl", nil)
	}

	if savepointId < 0 || savepointId > 9 {
		return nil, NewInvalidActionError("savepointId must be between 0-9", nil)
	}

	var endControl EndControl
	if pdr.state == PartialDataRowWithPayload {
		endControl = EndControl{'R', byte('0' + savepointId)}
	} else {
		endControl = EndControl{'S', byte('0' + savepointId)}
	}

	return pdr.complete(endControl)
}

func (pdr *PartialDataRow) EndRow() (*DataRow, error) {
	if pdr.d.RowSize == -1 {
		return nil, NewInvalidActionError("RowSize is not set", nil)
	}
	if pdr.state == PartialDataRowWithStartControl {
		return nil, NewInvalidActionError("EndRow() cannot be called from PartialDataRowWithStartControl", nil)
	}

	var endControl EndControl
	if pdr.state == PartialDataRowWithPayload {
		endControl = ROW_END_CONTROL
	} else {
		endControl = SAVEPOINT_CONTINUE
	}

	return pdr.complete(endControl)
}

func (pdr *PartialDataRow) String() string {
	bytes, err := pdr.MarshalText()
	if err == nil {
		return string(bytes)
	}
	return err.Error()
}

func (pdr *PartialDataRow) complete(endControl EndControl) (*DataRow, error) {
	if pdr.d.RowPayload == nil {
		return nil, NewInvalidInputError("cannot complete PartialDataRow without payload", nil)
	}

	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			RowSize:      pdr.d.RowSize,
			StartControl: pdr.d.StartControl,
			EndControl:   endControl,
			RowPayload:   pdr.d.RowPayload,
		},
	}

	if err := dataRow.Validate(); err != nil {
		return nil, NewInvalidInputError("completed DataRow validation failed", err)
	}

	return dataRow, nil
}
