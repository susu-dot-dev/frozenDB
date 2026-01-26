package frozendb

import (
	"errors"
	"testing"
)

// Test_S_022_FR_001_FuzzyBinarySearchFunction verifies that FuzzyBinarySearch
// accepts target, skewMs, numKeys, get callback; returns exact index of unique
// target or KeyNotFoundError.
func Test_S_022_FR_001_FuzzyBinarySearchFunction(t *testing.T) {
	t.Run("finds_exact_match_in_strictly_sorted", func(t *testing.T) {
		ts := []int64{100, 200, 300, 400, 500}
		get := func(i int64) (int64, error) {
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("index out of range", nil)
			}
			return ts[i], nil
		}
		idx, err := FuzzyBinarySearch(300, 0, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(300): %v", err)
		}
		if idx != 2 {
			t.Errorf("FuzzyBinarySearch(300) = index %d, want 2", idx)
		}
	})

	t.Run("returns_KeyNotFoundError_when_target_not_present", func(t *testing.T) {
		ts := []int64{100, 200, 400, 500}
		get := func(i int64) (int64, error) {
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("index out of range", nil)
			}
			return ts[i], nil
		}
		_, err := FuzzyBinarySearch(300, 5, int64(len(ts)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch(300) err = %v, want KeyNotFoundError", err)
		}
	})

	t.Run("empty_dataset_returns_KeyNotFoundError", func(t *testing.T) {
		get := func(int64) (int64, error) {
			return 0, NewKeyNotFoundError("empty", nil)
		}
		_, err := FuzzyBinarySearch(100, 0, 0, get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch with numKeys=0 err = %v, want KeyNotFoundError", err)
		}
	})

	t.Run("single_element_match", func(t *testing.T) {
		get := func(i int64) (int64, error) {
			if i != 0 {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return 42, nil
		}
		idx, err := FuzzyBinarySearch(42, 0, 1, get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(42): %v", err)
		}
		if idx != 0 {
			t.Errorf("FuzzyBinarySearch(42) = index %d, want 0", idx)
		}
	})

	t.Run("single_element_no_match_returns_KeyNotFoundError", func(t *testing.T) {
		get := func(i int64) (int64, error) {
			if i != 0 {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return 42, nil
		}
		_, err := FuzzyBinarySearch(100, 10, 1, get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch(100) err = %v, want KeyNotFoundError", err)
		}
	})
}

// Test_S_022_FR_002_PerformanceComplexity verifies that the algorithm performs
// at most O(log n) + k Get() calls where k is entries within ±skew of target.
func Test_S_022_FR_002_PerformanceComplexity(t *testing.T) {
	t.Run("callback_count_bounded_for_strictly_sorted", func(t *testing.T) {
		const n = 1024
		ts := make([]int64, n)
		for i := range ts {
			ts[i] = int64(1000 + i)
		}
		calls := 0
		get := func(i int64) (int64, error) {
			calls++
			if i < 0 || i >= n {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return ts[i], nil
		}
		_, err := FuzzyBinarySearch(1000+512, 0, n, get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		// O(log n) + k with k=1 (exact match, no skew) => at most ~ceil(log2(1024))+1 = 11+1
		maxExpected := 20
		if calls > maxExpected {
			t.Errorf("callback calls = %d, expect ≤ %d (O(log n)+k)", calls, maxExpected)
		}
	})

	t.Run("callback_count_with_skew_linear_scan_bounded", func(t *testing.T) {
		// All 5 entries in [target-skew, target+skew]; k=5.
		ts := []int64{98, 99, 100, 101, 102}
		calls := 0
		get := func(i int64) (int64, error) {
			calls++
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return ts[i], nil
		}
		_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		// O(log 5) + 5 => ~3 + 5 = 8; allow some slack
		if calls > 15 {
			t.Errorf("callback calls = %d, expect O(log n)+k", calls)
		}
	})
}

// Test_S_022_FR_003_SkewWindowHandling verifies that FuzzyBinarySearch handles
// datasets where entries may be out of order within the configured skew_ms.
func Test_S_022_FR_003_SkewWindowHandling(t *testing.T) {
	t.Run("out_of_order_within_skew_finds_target", func(t *testing.T) {
		// Indices 0..4 have timestamps with reordering within skew 5; target 100 at index 2
		ts := []int64{102, 98, 100, 101, 99}
		get := func(i int64) (int64, error) {
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return ts[i], nil
		}
		idx, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(100, skew=5): %v", err)
		}
		if ts[idx] != 100 {
			t.Errorf("FuzzyBinarySearch found index %d with value %d, want 100", idx, ts[idx])
		}
	})

	t.Run("all_entries_in_skew_window_degrades_to_linear", func(t *testing.T) {
		// All unique; entire array in [target-skew, target+skew]; target 100 at index 2.
		ts := []int64{98, 99, 100, 101, 102}
		get := func(i int64) (int64, error) {
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return ts[i], nil
		}
		idx, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		if idx != 2 || ts[idx] != 100 {
			t.Errorf("found idx %d value %d, want 2 and 100", idx, ts[idx])
		}
	})

	t.Run("target_outside_array_range_returns_KeyNotFoundError", func(t *testing.T) {
		ts := []int64{1000, 2000, 3000}
		get := func(i int64) (int64, error) {
			if i < 0 || i >= int64(len(ts)) {
				return 0, NewKeyNotFoundError("out of range", nil)
			}
			return ts[i], nil
		}
		_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("target below range err = %v, want KeyNotFoundError", err)
		}
		_, err = FuzzyBinarySearch(5000, 5, int64(len(ts)), get)
		if !errors.As(err, &keyErr) {
			t.Errorf("target above range err = %v, want KeyNotFoundError", err)
		}
	})
}

// Test_S_022_FR_004_ErrorPropagation verifies that KeyNotFoundError from the
// timestamp access function is properly propagated.
func Test_S_022_FR_004_ErrorPropagation(t *testing.T) {
	t.Run("KeyNotFoundError_from_get_propagated", func(t *testing.T) {
		get := func(i int64) (int64, error) {
			return 0, NewKeyNotFoundError("key does not exist at index", nil)
		}
		_, err := FuzzyBinarySearch(100, 0, 1, get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch err = %v, want KeyNotFoundError propagated", err)
		}
	})
}
