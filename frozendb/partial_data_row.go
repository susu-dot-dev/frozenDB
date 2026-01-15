package frozendb

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

type PartialRowState int

const (
	PartialDataRowWithStartControl PartialRowState = iota
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

func (pdr *PartialDataRow) GetState() PartialRowState {
	return pdr.state
}

func (pdr *PartialDataRow) AddRow(key uuid.UUID, json string) error {
	if pdr.state != PartialDataRowWithStartControl {
		return NewInvalidActionError("AddRow() can only be called from PartialDataRowWithStartControl", nil)
	}

	if err := ValidateUUIDv7(key); err != nil {
		return err
	}

	if json == "" {
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
	if pdr.state != PartialDataRowWithPayload {
		return NewInvalidActionError("Savepoint() can only be called from PartialDataRowWithPayload", nil)
	}

	pdr.state = PartialDataRowWithSavepoint

	return pdr.Validate()
}

func (pdr *PartialDataRow) Validate() error {
	switch pdr.state {
	case PartialDataRowWithStartControl:
		return validateHeaderAndStartControl(pdr.d.Header, pdr.d.StartControl)

	case PartialDataRowWithPayload:
		if err := validateHeaderAndStartControl(pdr.d.Header, pdr.d.StartControl); err != nil {
			return err
		}
		return validateHeaderAndPayload(pdr.d.Header, pdr.d.RowPayload)

	case PartialDataRowWithSavepoint:
		if err := validateHeaderAndStartControl(pdr.d.Header, pdr.d.StartControl); err != nil {
			return err
		}
		return validateHeaderAndPayload(pdr.d.Header, pdr.d.RowPayload)

	default:
		return NewInvalidInputError(fmt.Sprintf("unknown PartialDataRow state: %d", pdr.state), nil)
	}
}

func (pdr *PartialDataRow) MarshalText() ([]byte, error) {
	switch pdr.state {
	case PartialDataRowWithStartControl:
		return pdr.marshalWithStartControl()
	case PartialDataRowWithPayload:
		return pdr.marshalWithPayload()
	case PartialDataRowWithSavepoint:
		return pdr.marshalWithSavepoint()
	default:
		return nil, NewInvalidInputError(fmt.Sprintf("unknown PartialDataRow state: %d", pdr.state), nil)
	}
}

func (pdr *PartialDataRow) marshalWithStartControl() ([]byte, error) {
	if pdr.d.Header == nil {
		return nil, NewInvalidInputError("Header is required", nil)
	}

	result := make([]byte, 2)
	result[0] = ROW_START
	result[1] = byte(pdr.d.StartControl)

	return result, nil
}

func (pdr *PartialDataRow) marshalWithPayload() ([]byte, error) {
	state1Bytes, err := pdr.marshalWithStartControl()
	if err != nil {
		return nil, err
	}

	if pdr.d.Header == nil {
		return nil, NewInvalidInputError("Header is required", nil)
	}

	rowSize := pdr.d.Header.GetRowSize()
	uuidBytes := pdr.d.RowPayload.Key[:]
	uuidBase64 := base64.StdEncoding.EncodeToString(uuidBytes)
	jsonBytes := []byte(pdr.d.RowPayload.Value)
	paddingLen := rowSize - 7 - 24 - len(jsonBytes)

	state2Len := 2 + 24 + len(jsonBytes) + paddingLen
	result := make([]byte, state2Len)

	copy(result, state1Bytes)
	copy(result[2:26], uuidBase64)
	copy(result[26:], jsonBytes)

	paddingStart := 26 + len(jsonBytes)
	for i := paddingStart; i < paddingStart+paddingLen; i++ {
		result[i] = NULL_BYTE
	}

	return result, nil
}

func (pdr *PartialDataRow) marshalWithSavepoint() ([]byte, error) {
	state2Bytes, err := pdr.marshalWithPayload()
	if err != nil {
		return nil, err
	}

	state3Bytes := append(state2Bytes, 'S')
	return state3Bytes, nil
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

	if err := pdr.Validate(); err != nil {
		return NewCorruptDatabaseError("validation failed", err)
	}

	return nil
}

func (pdr *PartialDataRow) Commit() (*DataRow, error) {
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

func (pdr *PartialDataRow) complete(endControl EndControl) (*DataRow, error) {
	if pdr.d.RowPayload == nil {
		return nil, NewInvalidInputError("cannot complete PartialDataRow without payload", nil)
	}

	dataRow := &DataRow{
		baseRow[*DataRowPayload]{
			Header:       pdr.d.Header,
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
