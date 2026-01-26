package frozendb

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var _ Finder = (*InMemoryFinder)(nil)

// InMemoryFinder implements Finder with O(1) GetIndex, GetTransactionStart, and
// GetTransactionEnd via in-memory maps. Memory scales ~40 bytes per row.
// Use when DB size allows full in-memory indexing; prefer SimpleFinder for
// fixed memory when DB is large.
type InMemoryFinder struct {
	uuidIndex        map[uuid.UUID]int64
	transactionStart map[int64]int64
	transactionEnd   map[int64]int64
	mu               sync.RWMutex
	dbFile           DBFile
	rowSize          int32
	size             int64
	lastTxStart      int64
}

// NewInMemoryFinder builds an InMemoryFinder by scanning the database and
// populating uuid and transaction boundary maps. O(n) init, O(1) lookups after.
func NewInMemoryFinder(dbFile DBFile, rowSize int32) (*InMemoryFinder, error) {
	if dbFile == nil {
		return nil, NewInvalidInputError("dbFile cannot be nil", nil)
	}
	if rowSize < 128 || rowSize > 65536 {
		return nil, NewInvalidInputError(fmt.Sprintf("rowSize must be between 128 and 65536, got %d", rowSize), nil)
	}
	size := dbFile.Size()
	imf := &InMemoryFinder{
		uuidIndex:        make(map[uuid.UUID]int64),
		transactionStart: make(map[int64]int64),
		transactionEnd:   make(map[int64]int64),
		dbFile:           dbFile,
		rowSize:          rowSize,
		size:             size,
		lastTxStart:      -1,
	}
	if err := imf.buildIndex(); err != nil {
		return nil, err
	}
	return imf, nil
}

func (imf *InMemoryFinder) buildIndex() error {
	totalRows := (imf.size - int64(HEADER_SIZE)) / int64(imf.rowSize)
	var currentTxStart int64 = -1
	for i := int64(0); i < totalRows; i++ {
		offset := int64(HEADER_SIZE) + i*int64(imf.rowSize)
		rowBytes, err := imf.dbFile.Read(offset, imf.rowSize)
		if err != nil {
			return NewReadError(fmt.Sprintf("failed to read row at index %d", i), err)
		}
		var ru RowUnion
		if err := ru.UnmarshalText(rowBytes); err != nil {
			return NewCorruptDatabaseError(fmt.Sprintf("failed to parse row at index %d", i), err)
		}
		if ru.ChecksumRow != nil {
			continue
		}
		if ru.DataRow != nil {
			if ru.DataRow.StartControl == START_TRANSACTION {
				currentTxStart = i
			}
			imf.transactionStart[i] = currentTxStart
			if imf.rowEndsTransaction(&ru) {
				for j := currentTxStart; j <= i; j++ {
					imf.transactionEnd[j] = i
				}
			}
			key := ru.DataRow.GetKey()
			if key != uuid.Nil {
				if err := ValidateUUIDv7(key); err == nil {
					imf.uuidIndex[key] = i
				}
			}
		} else if ru.NullRow != nil {
			currentTxStart = i
			imf.transactionStart[i] = i
			imf.transactionEnd[i] = i
		}
	}
	imf.lastTxStart = currentTxStart
	return nil
}

func (imf *InMemoryFinder) rowEndsTransaction(ru *RowUnion) bool {
	if ru.NullRow != nil {
		return true
	}
	if ru.DataRow == nil {
		return false
	}
	ec := ru.DataRow.EndControl
	if ec == TRANSACTION_COMMIT || ec == SAVEPOINT_COMMIT {
		return true
	}
	first, second := ec[0], ec[1]
	if (first == 'R' || first == 'S') && second >= '0' && second <= '9' {
		return true
	}
	return false
}

func (imf *InMemoryFinder) GetIndex(key uuid.UUID) (int64, error) {
	if key == uuid.Nil {
		return -1, NewInvalidInputError("key cannot be uuid.Nil", nil)
	}
	if err := ValidateUUIDv7(key); err != nil {
		return -1, err
	}
	imf.mu.RLock()
	defer imf.mu.RUnlock()
	idx, ok := imf.uuidIndex[key]
	if !ok {
		return -1, NewKeyNotFoundError(fmt.Sprintf("key %s not found in database", key.String()), nil)
	}
	return idx, nil
}

func (imf *InMemoryFinder) GetTransactionStart(index int64) (int64, error) {
	imf.mu.RLock()
	defer imf.mu.RUnlock()
	if err := imf.validateIndex(index); err != nil {
		return -1, err
	}
	if imf.isChecksumRow(index) {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}
	start, ok := imf.transactionStart[index]
	if !ok {
		return -1, NewCorruptDatabaseError("no transaction start found for index", nil)
	}
	return start, nil
}

func (imf *InMemoryFinder) GetTransactionEnd(index int64) (int64, error) {
	imf.mu.RLock()
	defer imf.mu.RUnlock()
	if err := imf.validateIndex(index); err != nil {
		return -1, err
	}
	if imf.isChecksumRow(index) {
		return -1, NewInvalidInputError("index points to checksum row", nil)
	}
	end, ok := imf.transactionEnd[index]
	if !ok {
		return -1, NewTransactionActiveError("transaction has no ending row", nil)
	}
	return end, nil
}

func (imf *InMemoryFinder) OnRowAdded(index int64, row *RowUnion) error {
	if row == nil {
		return NewInvalidInputError("row cannot be nil", nil)
	}
	imf.mu.Lock()
	defer imf.mu.Unlock()
	expected := (imf.size - int64(HEADER_SIZE)) / int64(imf.rowSize)
	if index < expected {
		return NewInvalidInputError(fmt.Sprintf("row index %d does not match expected position %d (existing data)", index, expected), nil)
	}
	if index > expected {
		return NewInvalidInputError(fmt.Sprintf("row index %d skips positions (expected %d)", index, expected), nil)
	}
	if row.ChecksumRow != nil {
		imf.size += int64(imf.rowSize)
		return nil
	}
	if row.DataRow != nil {
		if row.DataRow.StartControl == START_TRANSACTION {
			imf.lastTxStart = index
		}
		imf.transactionStart[index] = imf.lastTxStart
		if imf.rowEndsTransaction(row) {
			for j := imf.lastTxStart; j <= index; j++ {
				imf.transactionEnd[j] = index
			}
		}
		key := row.DataRow.GetKey()
		if key != uuid.Nil {
			if err := ValidateUUIDv7(key); err == nil {
				imf.uuidIndex[key] = index
			}
		}
	} else if row.NullRow != nil {
		imf.lastTxStart = index
		imf.transactionStart[index] = index
		imf.transactionEnd[index] = index
	}
	imf.size += int64(imf.rowSize)
	return nil
}

func (imf *InMemoryFinder) validateIndex(index int64) error {
	if index < 0 {
		return NewInvalidInputError("index cannot be negative", nil)
	}
	totalRows := (imf.size - int64(HEADER_SIZE)) / int64(imf.rowSize)
	if index >= totalRows {
		return NewInvalidInputError(fmt.Sprintf("index %d out of bounds (total rows: %d)", index, totalRows), nil)
	}
	return nil
}

func (imf *InMemoryFinder) isChecksumRow(index int64) bool {
	return index%int64(CHECKSUM_INTERVAL+1) == 0
}
