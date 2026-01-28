package finder

import (
	"errors"

	"github.com/google/uuid"
)

// FuzzyBinarySearch finds the index of target (UUIDv7) in a logically ordered
// sequence of UUIDv7 keys that may be out of order within skewMs. It uses a
// three-way partitioned binary search plus a linear scan over the indeterminate
// range. get(i) returns the UUIDv7 key at index i or an error (KeyNotFoundError
// propagated as-is; others wrapped as ReadError).
//
// Time: O(log n) + k where k = count of entries in [target_timestamp-skewMs, target_timestamp+skewMs].
// Space: O(1).
func FuzzyBinarySearch(target uuid.UUID, skewMs, numKeys int64, get func(int64) (uuid.UUID, error)) (int64, error) {
	// Validate target is a valid UUIDv7
	if err := ValidateUUIDv7(target); err != nil {
		return -1, err
	}
	if skewMs < 0 || skewMs > 86400000 {
		return -1, NewInvalidInputError("skewMs must be in [0, 86400000]", nil)
	}
	if numKeys < 0 {
		return -1, NewInvalidInputError("numKeys must be non-negative", nil)
	}
	if get == nil {
		return -1, NewInvalidInputError("get must not be nil", nil)
	}
	if numKeys == 0 {
		return -1, NewKeyNotFoundError("empty array", nil)
	}

	// Extract timestamp from target UUIDv7
	targetTimestamp := ExtractUUIDv7Timestamp(target)
	lower := targetTimestamp - skewMs
	upper := targetTimestamp + skewMs
	lo, hi := int64(0), numKeys-1

	for lo <= hi {
		mid := lo + (hi-lo)/2
		v, err := get(mid)
		if err != nil {
			return -1, propagateGetError(err)
		}

		// Validate returned UUID is valid UUIDv7
		if err := ValidateUUIDv7(v); err != nil {
			return -1, err
		}

		// Extract timestamp from UUIDv7 for comparison
		vTimestamp := ExtractUUIDv7Timestamp(v)

		if vTimestamp < lower {
			lo = mid + 1
			continue
		}
		if vTimestamp > upper {
			hi = mid - 1
			continue
		}
		if v == target {
			return mid, nil
		}
		// Linear scan left
		for i := mid - 1; i >= 0; i-- {
			v, err := get(i)
			if err != nil {
				return -1, propagateGetError(err)
			}
			if ValidateUUIDv7(v) != nil {
				return -1, NewInvalidInputError("invalid UUIDv7 in array", nil)
			}
			vTimestamp := ExtractUUIDv7Timestamp(v)
			if vTimestamp < lower-skewMs {
				break
			}
			if vTimestamp >= lower && vTimestamp <= upper && v == target {
				return i, nil
			}
		}
		// Linear scan right
		for i := mid + 1; i < numKeys; i++ {
			v, err := get(i)
			if err != nil {
				return -1, propagateGetError(err)
			}
			if ValidateUUIDv7(v) != nil {
				return -1, NewInvalidInputError("invalid UUIDv7 in array", nil)
			}
			vTimestamp := ExtractUUIDv7Timestamp(v)
			if vTimestamp > upper+skewMs {
				break
			}
			if vTimestamp >= lower && vTimestamp <= upper && v == target {
				return i, nil
			}
		}
		return -1, NewKeyNotFoundError("target not found in indeterminate range", nil)
	}
	return -1, NewKeyNotFoundError("target not found", nil)
}

func propagateGetError(err error) error {
	var keyErr *KeyNotFoundError
	if errors.As(err, &keyErr) {
		return err
	}
	return NewReadError("get callback failed", err)
}
