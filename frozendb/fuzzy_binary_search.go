package frozendb

import "errors"

// FuzzyBinarySearch finds the index of target (Unix ms) in a logically ordered
// sequence of timestamps that may be out of order within skewMs. It uses a
// three-way partitioned binary search plus a linear scan over the indeterminate
// range. get(i) returns the timestamp at index i or an error (KeyNotFoundError
// propagated as-is; others wrapped as ReadError).
//
// Time: O(log n) + k where k = count of entries in [target-skewMs, target+skewMs].
// Space: O(1).
func FuzzyBinarySearch(target, skewMs, numKeys int64, get func(int64) (int64, error)) (int64, error) {
	if target < 0 {
		return -1, NewInvalidInputError("target must be non-negative", nil)
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

	lower := target - skewMs
	upper := target + skewMs
	lo, hi := int64(0), numKeys-1

	for lo <= hi {
		mid := lo + (hi-lo)/2
		v, err := get(mid)
		if err != nil {
			return -1, propagateGetError(err)
		}
		if v < lower {
			lo = mid + 1
			continue
		}
		if v > upper {
			hi = mid - 1
			continue
		}
		if v == target {
			return mid, nil
		}
		for i := mid - 1; i >= 0; i-- {
			v, err := get(i)
			if err != nil {
				return -1, propagateGetError(err)
			}
			if v < lower-skewMs {
				break
			}
			if v >= lower && v <= upper && v == target {
				return i, nil
			}
		}
		for i := mid + 1; i < numKeys; i++ {
			v, err := get(i)
			if err != nil {
				return -1, propagateGetError(err)
			}
			if v > upper+skewMs {
				break
			}
			if v >= lower && v <= upper && v == target {
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
