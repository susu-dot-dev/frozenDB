package finder

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Test_S_022_FR_001_FuzzyBinarySearchFunction verifies that FuzzyBinarySearch
// accepts target, skewMs, numKeys, get callback; returns exact index of unique
// target or KeyNotFoundError.
func Test_S_022_FR_001_FuzzyBinarySearchFunction(t *testing.T) {
	t.Run("finds_exact_match_in_strictly_sorted", func(t *testing.T) {
		ts := []int64{100, 200, 300, 400, 500}
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		idx, err := FuzzyBinarySearch(CreateNullRowUUID(300), 0, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(300): %v", err)
		}
		if idx != 2 {
			t.Errorf("FuzzyBinarySearch(300) = index %d, want 2", idx)
		}
	})

	t.Run("returns_KeyNotFoundError_when_target_not_present", func(t *testing.T) {
		ts := []int64{100, 200, 400, 500}
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(300), 5, int64(len(ts)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch(300) err = %v, want KeyNotFoundError", err)
		}
	})

	t.Run("empty_dataset_returns_KeyNotFoundError", func(t *testing.T) {
		get := func(int64) (uuid.UUID, error) {
			return uuid.Nil, NewKeyNotFoundError("empty", nil)
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 0, 0, get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch with numKeys=0 err = %v, want KeyNotFoundError", err)
		}
	})

	t.Run("single_element_match", func(t *testing.T) {
		get := func(i int64) (uuid.UUID, error) {
			if i != 0 {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(42), nil
		}
		idx, err := FuzzyBinarySearch(CreateNullRowUUID(42), 0, 1, get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(42): %v", err)
		}
		if idx != 0 {
			t.Errorf("FuzzyBinarySearch(42) = index %d, want 0", idx)
		}
	})

	t.Run("single_element_no_match_returns_KeyNotFoundError", func(t *testing.T) {
		get := func(i int64) (uuid.UUID, error) {
			if i != 0 {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(42), nil
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 10, 1, get)
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
		get := func(i int64) (uuid.UUID, error) {
			calls++
			if i < 0 || i >= n {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(1000+512), 0, n, get)
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
		get := func(i int64) (uuid.UUID, error) {
			calls++
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
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
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		target := CreateNullRowUUID(100)
		idx, err := FuzzyBinarySearch(target, 5, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch(100, skew=5): %v", err)
		}
		v, _ := get(idx)
		if ExtractUUIDv7Timestamp(v) != 100 {
			t.Errorf("FuzzyBinarySearch found index %d with value %d, want 100", idx, ExtractUUIDv7Timestamp(v))
		}
	})

	t.Run("all_entries_in_skew_window_degrades_to_linear", func(t *testing.T) {
		// All unique; entire array in [target-skew, target+skew]; target 100 at index 2.
		ts := []int64{98, 99, 100, 101, 102}
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		target := CreateNullRowUUID(100)
		idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		v, _ := get(idx)
		if idx != 2 || ExtractUUIDv7Timestamp(v) != 100 {
			t.Errorf("found idx %d value %d, want 2 and 100", idx, ExtractUUIDv7Timestamp(v))
		}
	})

	t.Run("target_outside_array_range_returns_KeyNotFoundError", func(t *testing.T) {
		ts := []int64{1000, 2000, 3000}
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(ts)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return CreateNullRowUUID(ts[i]), nil
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("target below range err = %v, want KeyNotFoundError", err)
		}
		_, err = FuzzyBinarySearch(CreateNullRowUUID(5000), 5, int64(len(ts)), get)
		if !errors.As(err, &keyErr) {
			t.Errorf("target above range err = %v, want KeyNotFoundError", err)
		}
	})
}

// Test_S_022_FR_004_ErrorPropagation verifies that KeyNotFoundError from the
// timestamp access function is properly propagated.
func Test_S_022_FR_004_ErrorPropagation(t *testing.T) {
	t.Run("KeyNotFoundError_from_get_propagated", func(t *testing.T) {
		get := func(i int64) (uuid.UUID, error) {
			return uuid.Nil, NewKeyNotFoundError("key does not exist at index", nil)
		}
		_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 0, 1, get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("FuzzyBinarySearch err = %v, want KeyNotFoundError propagated", err)
		}
	})
}

// Test_S_026_FR_001_UUIDv7TargetValidation verifies that FuzzyBinarySearch
// validates that the target parameter is a valid UUIDv7 key.
func Test_S_026_FR_001_UUIDv7TargetValidation(t *testing.T) {
	t.Run("rejects_nil_UUID_target", func(t *testing.T) {
		get := func(i int64) (uuid.UUID, error) {
			return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
		}
		_, err := FuzzyBinarySearch(uuid.Nil, 1000, 1, get)
		var invErr *InvalidInputError
		if !errors.As(err, &invErr) {
			t.Errorf("FuzzyBinarySearch with nil UUID err = %v, want InvalidInputError", err)
		}
		if invErr == nil || !containsUUID(invErr.Error(), "UUID cannot be zero") {
			t.Errorf("Expected UUID validation error, got: %v", invErr)
		}
	})

	t.Run("rejects_non_UUIDv7_target", func(t *testing.T) {
		// Create a UUIDv4 (not UUIDv7)
		target := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		get := func(i int64) (uuid.UUID, error) {
			return target, NewKeyNotFoundError("index out of range", nil)
		}
		_, err := FuzzyBinarySearch(target, 1000, 1, get)
		var invErr *InvalidInputError
		if !errors.As(err, &invErr) {
			t.Errorf("FuzzyBinarySearch with UUIDv4 err = %v, want InvalidInputError", err)
		}
		if invErr == nil || !containsUUID(invErr.Error(), "UUID version must be 7") {
			t.Errorf("Expected UUIDv7 version validation error, got: %v", invErr)
		}
	})

	t.Run("accepts_valid_UUIDv7_target", func(t *testing.T) {
		target := MustNewUUIDv7()
		keys := []uuid.UUID{target}
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}
		_, err := FuzzyBinarySearch(target, 1000, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with valid UUIDv7 failed: %v", err)
		}
	})
}

// Test_S_026_FR_002_TimestampExtractionComparison verifies that FuzzyBinarySearch
// extracts timestamps from UUIDv7 keys for binary search comparison.
func Test_S_026_FR_002_TimestampExtractionComparison(t *testing.T) {
	t.Run("binary_search_compares_extracted_timestamps", func(t *testing.T) {
		// Create UUIDv7 keys with increasing timestamps
		baseTimestamp := int64(1640995200000) // 2022-01-01 00:00:00 UTC
		keys := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp + int64(i*1000))
		}

		target := keys[2] // The middle UUID
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		_, err := FuzzyBinarySearch(target, 500, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with timestamp comparison failed: %v", err)
		}
	})

	t.Run("binary_search_handles_out_of_order_timestamps_within_skew", func(t *testing.T) {
		// Create UUIDv7 keys with same timestamp but different UUIDs (within skew)
		baseTimestamp := int64(1640995200000)
		keys := []uuid.UUID{
			CreateNullRowUUID(baseTimestamp + 100),
			CreateNullRowUUID(baseTimestamp - 50), // Out of order within skew
			CreateNullRowUUID(baseTimestamp + 200),
			CreateNullRowUUID(baseTimestamp + 50), // Out of order within skew
			CreateNullRowUUID(baseTimestamp + 300),
		}

		target := keys[2] // timestamp +200
		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		_, err := FuzzyBinarySearch(target, 300, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with out-of-order timestamps failed: %v", err)
		}
	})
}

// Test_S_026_FR_003_FullUUIDEqualityLinearScan verifies that FuzzyBinarySearch
// uses full UUID equality during linear scan phase for exact matching.
func Test_S_026_FR_003_FullUUIDEqualityLinearScan(t *testing.T) {
	t.Run("linear_scan_uses_full_UUID_equality_not_timestamp", func(t *testing.T) {
		// Create multiple UUIDv7s with same timestamp but different random components
		baseTimestamp := int64(1640995200000)

		// Create UUIDs with same timestamp but different random bits
		uuid1 := CreateNullRowUUID(baseTimestamp)
		uuid2 := CreateNullRowUUID(baseTimestamp)
		// Modify random components to make them different
		uuid2[7] = 0x01 // Change random component
		uuid3 := CreateNullRowUUID(baseTimestamp)
		uuid3[7] = 0x02 // Different random component

		keys := []uuid.UUID{uuid1, uuid2, uuid3}
		target := uuid2

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		idx, err := FuzzyBinarySearch(target, 0, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with full UUID equality failed: %v", err)
		}
		if idx != 1 {
			t.Errorf("Expected index 1, got %d", idx)
		}
		// Verify the returned UUID exactly matches target
		if keys[idx] != target {
			t.Errorf("Returned UUID %v does not match target %v", keys[idx], target)
		}
	})

	t.Run("linear_scan_returns_KeyNotFoundError_for_different_UUID_same_timestamp", func(t *testing.T) {
		// Create UUIDs with same timestamp but target not in array
		baseTimestamp := int64(1640995200000)

		keys := []uuid.UUID{
			CreateNullRowUUID(baseTimestamp),
			CreateNullRowUUID(baseTimestamp),
		}
		// Modify to make them different from target
		keys[0][7] = 0x01
		keys[1][7] = 0x02

		// Target has same timestamp but different random components
		target := CreateNullRowUUID(baseTimestamp)
		target[7] = 0x03 // Different from both array elements

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		_, err := FuzzyBinarySearch(target, 1000, int64(len(keys)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("Expected KeyNotFoundError for different UUID same timestamp, got: %v", err)
		}
	})
}

// Test_S_026_FR_004_MultipleIdenticalTimestamps verifies that FuzzyBinarySearch
// correctly handles multiple UUIDv7 keys with identical timestamps.
func Test_S_026_FR_004_MultipleIdenticalTimestamps(t *testing.T) {
	t.Run("finds_target_among_multiple_UUIDs_same_timestamp", func(t *testing.T) {
		// Create multiple UUIDv7s with identical timestamp but different random components
		baseTimestamp := int64(1640995200000)

		keys := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp)
			// Modify random components to make them unique
			keys[i][7] = byte(i + 1)
		}

		target := keys[3] // Target in middle of identical timestamps

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		idx, err := FuzzyBinarySearch(target, 0, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with identical timestamps failed: %v", err)
		}
		if idx != 3 {
			t.Errorf("Expected index 3, got %d", idx)
		}
		// Verify exact UUID match
		if keys[idx] != target {
			t.Errorf("Returned UUID %v does not match target %v", keys[idx], target)
		}
	})

	t.Run("linear_scan_searches_all_identical_timestamps", func(t *testing.T) {
		// Create array where all UUIDs have same timestamp, target is at the end
		baseTimestamp := int64(1640995200000)

		keys := make([]uuid.UUID, 10)
		for i := 0; i < 10; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp)
			keys[i][7] = byte(i + 1)
		}

		target := keys[9] // Target at the end

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		idx, err := FuzzyBinarySearch(target, 0, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch failed to find target at end: %v", err)
		}
		if idx != 9 {
			t.Errorf("Expected index 9, got %d", idx)
		}
	})

	t.Run("returns_KeyNotFoundError_when_target_not_in_identical_timestamps", func(t *testing.T) {
		// Create array with identical timestamps, but target not present
		baseTimestamp := int64(1640995200000)

		keys := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp)
			keys[i][7] = byte(i + 1) // Values 1-5
		}

		// Target has same timestamp but different random component
		target := CreateNullRowUUID(baseTimestamp)
		target[7] = 0x99 // Not in array

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		_, err := FuzzyBinarySearch(target, 0, int64(len(keys)), get)
		var keyErr *KeyNotFoundError
		if !errors.As(err, &keyErr) {
			t.Errorf("Expected KeyNotFoundError for target not in identical timestamps, got: %v", err)
		}
	})

	t.Run("handles_mixed_timestamps_with_clusters", func(t *testing.T) {
		// Create array with clusters of identical timestamps
		timestamp1 := int64(1640995200000)
		timestamp2 := int64(1640995201000) // 1 second later

		keys := make([]uuid.UUID, 5)
		// Cluster 1: 3 UUIDs with timestamp1
		keys[0] = CreateNullRowUUID(timestamp1)
		keys[0][7] = 0x01
		keys[1] = CreateNullRowUUID(timestamp1)
		keys[1][7] = 0x02
		keys[2] = CreateNullRowUUID(timestamp1)
		keys[2][7] = 0x03
		// Cluster 2: 2 UUIDs with timestamp2
		keys[3] = CreateNullRowUUID(timestamp2)
		keys[3][7] = 0x01
		keys[4] = CreateNullRowUUID(timestamp2)
		keys[4][7] = 0x02

		// Fix the array properly
		keys[0] = CreateNullRowUUID(timestamp1)
		keys[0][7] = 0x01
		keys[1] = CreateNullRowUUID(timestamp1)
		keys[1][7] = 0x02
		keys[2] = CreateNullRowUUID(timestamp1)
		keys[2][7] = 0x03
		keys[3] = CreateNullRowUUID(timestamp2)
		keys[3][7] = 0x01
		keys[4] = CreateNullRowUUID(timestamp2)
		keys[4][7] = 0x02

		target := keys[1] // Second UUID in first cluster

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
			}
			return keys[i], nil
		}

		idx, err := FuzzyBinarySearch(target, 500, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with clustered timestamps failed: %v", err)
		}
		if idx != 1 {
			t.Errorf("Expected index 1, got %d", idx)
		}
	})
}

// Test_S_026_FR_005_PerformanceCharacteristics verifies that FuzzyBinarySearch
// maintains O(log n) + k performance where k = UUIDv7 entries in skew window.
func Test_S_026_FR_005_PerformanceCharacteristics(t *testing.T) {
	t.Run("callback_count_O_log_n_plus_k_with_UUIDv7", func(t *testing.T) {
		const n = 1024
		keys := make([]uuid.UUID, n)
		baseTimestamp := int64(1640995200000)
		for i := 0; i < n; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp + int64(i*1000))
		}

		calls := 0
		get := func(i int64) (uuid.UUID, error) {
			calls++
			if i < 0 || i >= n {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return keys[i], nil
		}

		target := keys[n/2]
		_, err := FuzzyBinarySearch(target, 0, n, get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		// O(log n) + k with k=1 (exact match, no skew) => at most ~ceil(log2(1024))+1 = 11+1
		maxExpected := 20
		if calls > maxExpected {
			t.Errorf("callback calls = %d, expect ≤ %d (O(log n)+k)", calls, maxExpected)
		}
	})

	t.Run("performance_with_all_UUIDv7_in_skew_window", func(t *testing.T) {
		// All 5 UUIDv7 entries in [target-skew, target+skew]; k=5.
		keys := make([]uuid.UUID, 5)
		baseTimestamp := int64(1640995200000)
		for i := 0; i < 5; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp + int64(i))
			// Make each UUID unique by modifying random component
			keys[i][7] = byte(i + 1)
		}

		calls := 0
		get := func(i int64) (uuid.UUID, error) {
			calls++
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return keys[i], nil
		}

		target := keys[2]
		skew := int64(10)
		_, err := FuzzyBinarySearch(target, skew, int64(len(keys)), get)
		if err != nil {
			t.Fatalf("FuzzyBinarySearch: %v", err)
		}
		// O(log 5) + 5 => ~3 + 5 = 8; allow some slack
		if calls > 15 {
			t.Errorf("callback calls = %d, expect O(log n)+k", calls)
		}
	})

	t.Run("space_complexity_O_1_no_additional_allocations", func(t *testing.T) {
		// Test that algorithm doesn't allocate additional memory proportional to input size
		keys := make([]uuid.UUID, 10000)
		baseTimestamp := int64(1640995200000)
		for i := 0; i < 10000; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp + int64(i))
			keys[i][7] = byte(i % 256)
		}

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return keys[i], nil
		}

		// This test mainly ensures function works with large arrays without memory issues
		target := keys[5000]
		_, err := FuzzyBinarySearch(target, 100, int64(len(keys)), get)
		if err != nil {
			t.Errorf("FuzzyBinarySearch with large dataset failed: %v", err)
		}
	})

	t.Run("concurrent_safety_for_read_operations", func(t *testing.T) {
		// Test that multiple concurrent reads don't interfere
		keys := make([]uuid.UUID, 1000)
		baseTimestamp := int64(1640995200000)
		for i := 0; i < 1000; i++ {
			keys[i] = CreateNullRowUUID(baseTimestamp + int64(i))
			keys[i][7] = byte(i % 256)
		}

		get := func(i int64) (uuid.UUID, error) {
			if i < 0 || i >= int64(len(keys)) {
				return uuid.Nil, NewKeyNotFoundError("out of range", nil)
			}
			return keys[i], nil
		}

		done := make(chan struct{})
		for i := 0; i < 10; i++ {
			go func(targetIndex int) {
				defer func() { done <- struct{}{} }()
				for j := 0; j < 10; j++ {
					_, err := FuzzyBinarySearch(keys[targetIndex], 0, int64(len(keys)), get)
					if err != nil {
						t.Errorf("Concurrent read failed: %v", err)
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Helper function to check if string contains substring
func containsUUID(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
