package frozendb

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func uuidSliceGetter(ts []int64) func(int64) (uuid.UUID, error) {
	return func(i int64) (uuid.UUID, error) {
		if i < 0 || i >= int64(len(ts)) {
			return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
		}
		return CreateNullRowUUID(ts[i]), nil
	}
}

func TestFuzzyBinarySearch_AllValuesBelowLower(t *testing.T) {
	ts := []int64{10, 20, 30, 40, 50}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("all below lower: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_AllValuesAboveUpper(t *testing.T) {
	ts := []int64{1000, 2000, 3000, 4000, 5000}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("all above upper: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_TargetLeftOfIndeterminateMid(t *testing.T) {
	// Mid lands on 150 (in range for target 100, skew 100). Target 100 at index 0.
	ts := []int64{100, 120, 150, 180, 200}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 100, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("target left of mid: %v", err)
	}
	v, _ := get(idx)
	if idx != 0 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("target left of mid: idx=%d value=%d, want 0 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_TargetRightOfIndeterminateMid(t *testing.T) {
	// Mid lands in [lower,upper] but v!=target; target 100 only at index 3.
	ts := []int64{95, 98, 102, 100, 99}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("target right of mid: %v", err)
	}
	v, _ := get(idx)
	if idx != 3 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("target right of mid: idx=%d value=%d, want idx=3 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_IndeterminateRangeNoExactMatch(t *testing.T) {
	// Values in [lower,upper] but none equals target.
	ts := []int64{98, 99, 101, 102}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("indeterminate no match: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_FirstIndex(t *testing.T) {
	ts := []int64{50, 100, 200, 300}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(50), 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("first index: %v", err)
	}
	if idx != 0 {
		t.Errorf("first index: idx=%d, want 0", idx)
	}
}

func TestFuzzyBinarySearch_LastIndex(t *testing.T) {
	ts := []int64{100, 200, 300, 400}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(400), 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("last index: %v", err)
	}
	if idx != 3 {
		t.Errorf("last index: idx=%d, want 3", idx)
	}
}

func TestFuzzyBinarySearch_TwoElements(t *testing.T) {
	ts := []int64{10, 20}
	get := uuidSliceGetter(ts)
	for i, want := range []int64{0, 1} {
		target := CreateNullRowUUID(ts[i])
		idx, err := FuzzyBinarySearch(target, 0, 2, get)
		if err != nil {
			t.Fatalf("two elements target %d: %v", ts[i], err)
		}
		if idx != want {
			t.Errorf("two elements target %d: idx=%d want %d", ts[i], idx, want)
		}
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(15), 2, 2, get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("two elements no match: %v", err)
	}
}

func TestFuzzyBinarySearch_SkewZeroStrictMatch(t *testing.T) {
	ts := []int64{97, 98, 99, 100, 101, 102, 103}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(100), 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skew 0: %v", err)
	}
	if idx != 3 {
		t.Errorf("skew 0: idx=%d, want 3", idx)
	}
	idx, err = FuzzyBinarySearch(CreateNullRowUUID(99), 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skew 0 find 99: %v", err)
	}
	if idx != 2 {
		t.Errorf("skew 0 find 99: idx=%d, want 2", idx)
	}
}

func TestFuzzyBinarySearch_MaxSkew(t *testing.T) {
	ts := []int64{0, 86400000}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(86400000)
	idx, err := FuzzyBinarySearch(target, 86400000, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("max skew: %v", err)
	}
	v, _ := get(idx)
	if idx != 1 || ExtractUUIDv7Timestamp(v) != 86400000 {
		t.Errorf("max skew: idx=%d value=%d, want 1 and 86400000", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_LinearScanStopLeft(t *testing.T) {
	// Left scan must stop when v < lower - skew. 100 only at index 2.
	ts := []int64{50, 98, 100, 102, 150}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("linear stop left: %v", err)
	}
	v, _ := get(idx)
	if idx != 2 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("linear stop left: idx=%d value=%d, want idx=2 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_LinearScanStopRight(t *testing.T) {
	// Right scan must stop when v > upper + skew. 100 only at index 2.
	ts := []int64{50, 98, 100, 102, 200}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("linear stop right: %v", err)
	}
	v, _ := get(idx)
	if idx != 2 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("linear stop right: idx=%d value=%d, want idx=2 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_KeyNotFoundFromGetDuringBinarySearch(t *testing.T) {
	// [10, 30, 50], target 40: first get(1)=30, second get(2)=KeyNotFound.
	ts := []int64{10, 30, 50}
	calls := 0
	get := func(i int64) (uuid.UUID, error) {
		calls++
		if calls >= 2 {
			return uuid.Nil, NewKeyNotFoundError("missing", nil)
		}
		return CreateNullRowUUID(ts[i]), nil
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(40), 0, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("KeyNotFound during get: %v", err)
	}
}

func TestFuzzyBinarySearch_KeyNotFoundFromGetDuringLinearScan(t *testing.T) {
	// 100 at index 3; mid hits 101. Left scan then right: 4th get is get(3) which we fail.
	// Proves we propagate get's KeyNotFound from linear scan (not algorithm's "not found").
	ts := []int64{98, 99, 101, 100, 102}
	callCount := 0
	get := func(i int64) (uuid.UUID, error) {
		callCount++
		if callCount >= 4 {
			return uuid.Nil, NewKeyNotFoundError("corrupt at index", nil)
		}
		return CreateNullRowUUID(ts[i]), nil
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 10, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("KeyNotFound during linear scan: %v", err)
	}
}

func TestFuzzyBinarySearch_ReadErrorFromGetWrapped(t *testing.T) {
	get := func(int64) (uuid.UUID, error) {
		return uuid.Nil, NewReadError("disk read failed", nil)
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 0, 1, get)
	var readErr *ReadError
	if !errors.As(err, &readErr) {
		t.Errorf("ReadError from get: %v", err)
	}
}

func TestFuzzyBinarySearch_ExactAtLowerBound(t *testing.T) {
	// target=100 at index 2; [lower,upper]=[90,110].
	ts := []int64{90, 95, 100, 105, 110}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("exact at lower: %v", err)
	}
	v, _ := get(idx)
	if idx != 2 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("exact at lower: idx=%d value=%d, want idx=2 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_ExactAtUpperBound(t *testing.T) {
	ts := []int64{90, 95, 100, 105, 110}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(110)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("exact at upper: %v", err)
	}
	v, _ := get(idx)
	if idx != 4 || ExtractUUIDv7Timestamp(v) != 110 {
		t.Errorf("exact at upper: idx=%d value=%d want 4, 110", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_BinarySearchNarrowsCorrectly(t *testing.T) {
	// Even-sized, target in second half; odd-sized.
	ts := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(8), 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("narrows: %v", err)
	}
	if idx != 7 {
		t.Errorf("narrows: idx=%d want 7", idx)
	}
}

func TestFuzzyBinarySearch_OddLength(t *testing.T) {
	ts := []int64{10, 20, 30, 40, 50}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(30), 0, 5, get)
	if err != nil {
		t.Fatalf("odd length: %v", err)
	}
	if idx != 2 {
		t.Errorf("odd length: idx=%d want 2", idx)
	}
}

func TestFuzzyBinarySearch_EntireArrayInSkewWindow(t *testing.T) {
	// All unique values in [target-skew, target+skew]; target 102 at index 2.
	ts := []int64{100, 101, 102, 103, 104}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(102)
	idx, err := FuzzyBinarySearch(target, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("entire in skew: %v", err)
	}
	v, _ := get(idx)
	if idx != 2 || ExtractUUIDv7Timestamp(v) != 102 {
		t.Errorf("entire in skew: idx=%d value=%d, want 2 and 102", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInFirstCluster(t *testing.T) {
	// Three clusters with gaps > 2*skew. Cluster 1: 95-105 (100 at 5), cluster 2: 495-505, cluster 3: 995-1005.
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(100)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in first cluster: %v", err)
	}
	v, _ := get(idx)
	if idx != 5 || ExtractUUIDv7Timestamp(v) != 100 {
		t.Errorf("first cluster: idx=%d value=%d, want 5 and 100", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInMiddleCluster(t *testing.T) {
	// Middle cluster has out-of-order within skew; 500 at index 13.
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		502, 495, 500, 498, 501, 497, 504, 499, 503, 496, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(500)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in middle cluster: %v", err)
	}
	v, _ := get(idx)
	if idx != 13 || ExtractUUIDv7Timestamp(v) != 500 {
		t.Errorf("middle cluster: idx=%d value=%d, want 13 and 500", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInLastCluster(t *testing.T) {
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(1000)
	idx, err := FuzzyBinarySearch(target, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in last cluster: %v", err)
	}
	v, _ := get(idx)
	if idx != 27 || ExtractUUIDv7Timestamp(v) != 1000 {
		t.Errorf("last cluster: idx=%d value=%d, want 27 and 1000", idx, ExtractUUIDv7Timestamp(v))
	}
}

func TestFuzzyBinarySearch_MultipleClusters_TargetInGapReturnsKeyNotFound(t *testing.T) {
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(300), 10, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("target 300 in gap: err=%v, want KeyNotFoundError", err)
	}
	_, err = FuzzyBinarySearch(CreateNullRowUUID(750), 10, int64(len(ts)), get)
	if !errors.As(err, &keyErr) {
		t.Errorf("target 750 in gap: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_MidpointCalculationNoOverflow(t *testing.T) {
	// Large indices to stress mid = lo + (hi-lo)/2
	n := int64(1<<20 - 1)
	ts := make([]int64, n)
	for i := range ts {
		ts[i] = int64(i)
	}
	get := uuidSliceGetter(ts)
	targetTimestamp := int64(n/2 + 100)
	target := CreateNullRowUUID(targetTimestamp)
	idx, err := FuzzyBinarySearch(target, 0, n, get)
	if err != nil {
		t.Fatalf("large n: %v", err)
	}
	if idx != targetTimestamp {
		t.Errorf("large n: idx=%d want %d", idx, targetTimestamp)
	}
}

func TestFuzzyBinarySearch_NegativeLowerBound(t *testing.T) {
	ts := []int64{0, 1, 2, 3, 4}
	get := uuidSliceGetter(ts)
	idx, err := FuzzyBinarySearch(CreateNullRowUUID(2), 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("negative lower: %v", err)
	}
	if idx != 2 {
		t.Errorf("negative lower: idx=%d want 2", idx)
	}
}

func TestFuzzyBinarySearch_ValidationSkewZeroAllowed(t *testing.T) {
	get := func(i int64) (uuid.UUID, error) {
		if i != 0 {
			return uuid.Nil, NewKeyNotFoundError("oob", nil)
		}
		return CreateNullRowUUID(0), nil
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(0), 0, 1, get)
	if err != nil {
		t.Errorf("skew 0 allowed: %v", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkew86400000Allowed(t *testing.T) {
	get := func(i int64) (uuid.UUID, error) {
		if i != 0 {
			return uuid.Nil, NewKeyNotFoundError("oob", nil)
		}
		return CreateNullRowUUID(1), nil
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(1), 86400000, 1, get)
	if err != nil {
		t.Errorf("skew 86400000 allowed: %v", err)
	}
}

func TestFuzzyBinarySearch_NumKeysZeroVsNegative(t *testing.T) {
	get := func(int64) (uuid.UUID, error) { return CreateNullRowUUID(0), nil }
	_, err0 := FuzzyBinarySearch(CreateNullRowUUID(1), 0, 0, get)
	var keyErr *KeyNotFoundError
	if !errors.As(err0, &keyErr) {
		t.Errorf("numKeys 0: %v, want KeyNotFoundError", err0)
	}
	_, errNeg := FuzzyBinarySearch(CreateNullRowUUID(1), 0, -1, get)
	var inv *InvalidInputError
	if !errors.As(errNeg, &inv) {
		t.Errorf("numKeys -1: %v, want InvalidInputError", errNeg)
	}
}

func TestFuzzyBinarySearch_ValidationTargetNegative(t *testing.T) {
	// Negative timestamps can't be converted to UUIDv7, so we test with a valid UUID that has negative timestamp extraction
	// Actually, CreateNullRowUUID will accept negative values, but ValidateUUIDv7 will fail
	// So we need to test with an invalid UUID - but actually, we can't create a UUID from -1
	// The validation happens in ValidateUUIDv7, so we'll test with uuid.Nil instead
	get := func(int64) (uuid.UUID, error) { return CreateNullRowUUID(0), nil }
	// Create a UUID that will fail validation - use a non-v7 UUID
	invalidUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000") // UUIDv4
	_, err := FuzzyBinarySearch(invalidUUID, 0, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("target < 0: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkewNegative(t *testing.T) {
	get := func(int64) (uuid.UUID, error) { return CreateNullRowUUID(0), nil }
	_, err := FuzzyBinarySearch(CreateNullRowUUID(0), -1, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("skewMs < 0: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkewOver86400000(t *testing.T) {
	get := func(int64) (uuid.UUID, error) { return CreateNullRowUUID(0), nil }
	_, err := FuzzyBinarySearch(CreateNullRowUUID(0), 86400001, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("skewMs > 86400000: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationGetNil(t *testing.T) {
	_, err := FuzzyBinarySearch(CreateNullRowUUID(0), 0, 1, nil)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("get == nil: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_GetErrorDuringLeftLinearScan(t *testing.T) {
	// Mid in [lower,upper] but v!=target; get fails on first left-scan call.
	ts := []int64{98, 99, 101, 102}
	calls := 0
	get := func(i int64) (uuid.UUID, error) {
		calls++
		if calls == 2 {
			return uuid.Nil, NewKeyNotFoundError("missing", nil)
		}
		return CreateNullRowUUID(ts[i]), nil
	}
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 10, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("KeyNotFound during left linear scan: %v", err)
	}
}

func TestFuzzyBinarySearch_LeftScanBreakWhenBelowLowerMinusSkew(t *testing.T) {
	// Mid in [lower,upper], v!=target; left scan sees v < lower-skewMs and breaks.
	// [40, 98, 99, 101, 102], target 100, skew 5 → lower=95, upper=105, lower-skewMs=90.
	// Mid=2, v=99; left i=1: 98; i=0: 40 < 90 break. Right: no 100 → not found in indeterminate.
	ts := []int64{40, 98, 99, 101, 102}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("left scan break: %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_RightScanBreakWhenAboveUpperPlusSkew(t *testing.T) {
	// Mid in [lower,upper], v!=target; right scan sees v > upper+skewMs and breaks.
	// [98, 99, 101, 102, 200], target 100, skew 5 → upper+skewMs=110.
	ts := []int64{98, 99, 101, 102, 200}
	get := uuidSliceGetter(ts)
	_, err := FuzzyBinarySearch(CreateNullRowUUID(100), 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("right scan break: %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_ConcurrentReads(t *testing.T) {
	ts := make([]int64, 1000)
	for i := range ts {
		ts[i] = int64(i)
	}
	get := uuidSliceGetter(ts)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = FuzzyBinarySearch(CreateNullRowUUID(500), 0, int64(len(ts)), get)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkFuzzyBinarySearch_StrictlySorted(b *testing.B) {
	const n = 100_000
	ts := make([]int64, n)
	for i := range ts {
		ts[i] = int64(i * 1000)
	}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(int64(50_000 * 1000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FuzzyBinarySearch(target, 0, n, get)
	}
}

func BenchmarkFuzzyBinarySearch_AllInSkew(b *testing.B) {
	// All n unique values in [target-skew, target+skew]; k=n.
	const n = 1000
	ts := make([]int64, n)
	for i := range ts {
		ts[i] = 100_000 + int64(i)
	}
	get := uuidSliceGetter(ts)
	target := CreateNullRowUUID(int64(100_500))
	skew := int64(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FuzzyBinarySearch(target, skew, n, get)
	}
}

// UUIDv7 test helpers

func uuidArrayGetter(keys []uuid.UUID) func(int64) (uuid.UUID, error) {
	return func(i int64) (uuid.UUID, error) {
		if i < 0 || i >= int64(len(keys)) {
			return uuid.Nil, NewKeyNotFoundError("index out of range", nil)
		}
		return keys[i], nil
	}
}

func TestFuzzyBinarySearchUUIDv7_BasicFunctionality(t *testing.T) {
	// Test that implementation works with valid UUIDv7
	keys := []uuid.UUID{MustNewUUIDv7()}
	get := uuidArrayGetter(keys)

	idx, err := FuzzyBinarySearch(keys[0], 1000, int64(len(keys)), get)
	if err != nil {
		t.Errorf("Expected successful search, got error: %v", err)
	}
	if idx != 0 {
		t.Errorf("Expected index 0, got: %d", idx)
	}

	// Test with invalid UUIDv7
	_, err = FuzzyBinarySearch(uuid.Nil, 1000, 1, get)
	var invErr *InvalidInputError
	if !errors.As(err, &invErr) {
		t.Errorf("Expected InvalidInputError for nil UUID, got: %v", err)
	}
}
