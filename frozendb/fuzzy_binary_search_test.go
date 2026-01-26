package frozendb

import (
	"errors"
	"testing"
)

func sliceGetter(ts []int64) func(int64) (int64, error) {
	return func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		return ts[i], nil
	}
}

func TestFuzzyBinarySearch_AllValuesBelowLower(t *testing.T) {
	ts := []int64{10, 20, 30, 40, 50}
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("all below lower: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_AllValuesAboveUpper(t *testing.T) {
	ts := []int64{1000, 2000, 3000, 4000, 5000}
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("all above upper: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_TargetLeftOfIndeterminateMid(t *testing.T) {
	// Mid lands on 150 (in range for target 100, skew 100). Target 100 at index 0.
	ts := []int64{100, 120, 150, 180, 200}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 100, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("target left of mid: %v", err)
	}
	if idx != 0 || ts[idx] != 100 {
		t.Errorf("target left of mid: idx=%d value=%d, want 0 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_TargetRightOfIndeterminateMid(t *testing.T) {
	// Mid lands in [lower,upper] but v!=target; target 100 only at index 3.
	ts := []int64{95, 98, 102, 100, 99}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("target right of mid: %v", err)
	}
	if idx != 3 || ts[idx] != 100 {
		t.Errorf("target right of mid: idx=%d value=%d, want idx=3 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_IndeterminateRangeNoExactMatch(t *testing.T) {
	// Values in [lower,upper] but none equals target.
	ts := []int64{98, 99, 101, 102}
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("indeterminate no match: err = %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_FirstIndex(t *testing.T) {
	ts := []int64{50, 100, 200, 300}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(50, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("first index: %v", err)
	}
	if idx != 0 {
		t.Errorf("first index: idx=%d, want 0", idx)
	}
}

func TestFuzzyBinarySearch_LastIndex(t *testing.T) {
	ts := []int64{100, 200, 300, 400}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(400, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("last index: %v", err)
	}
	if idx != 3 {
		t.Errorf("last index: idx=%d, want 3", idx)
	}
}

func TestFuzzyBinarySearch_TwoElements(t *testing.T) {
	ts := []int64{10, 20}
	get := sliceGetter(ts)
	for i, want := range []int64{0, 1} {
		idx, err := FuzzyBinarySearch(ts[i], 0, 2, get)
		if err != nil {
			t.Fatalf("two elements target %d: %v", ts[i], err)
		}
		if idx != want {
			t.Errorf("two elements target %d: idx=%d want %d", ts[i], idx, want)
		}
	}
	_, err := FuzzyBinarySearch(15, 2, 2, get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("two elements no match: %v", err)
	}
}

func TestFuzzyBinarySearch_SkewZeroStrictMatch(t *testing.T) {
	ts := []int64{97, 98, 99, 100, 101, 102, 103}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skew 0: %v", err)
	}
	if idx != 3 {
		t.Errorf("skew 0: idx=%d, want 3", idx)
	}
	idx, err = FuzzyBinarySearch(99, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skew 0 find 99: %v", err)
	}
	if idx != 2 {
		t.Errorf("skew 0 find 99: idx=%d, want 2", idx)
	}
}

func TestFuzzyBinarySearch_MaxSkew(t *testing.T) {
	ts := []int64{0, 86400000}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(86400000, 86400000, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("max skew: %v", err)
	}
	if idx != 1 || ts[idx] != 86400000 {
		t.Errorf("max skew: idx=%d value=%d, want 1 and 86400000", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_LinearScanStopLeft(t *testing.T) {
	// Left scan must stop when v < lower - skew. 100 only at index 2.
	ts := []int64{50, 98, 100, 102, 150}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("linear stop left: %v", err)
	}
	if idx != 2 || ts[idx] != 100 {
		t.Errorf("linear stop left: idx=%d value=%d, want idx=2 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_LinearScanStopRight(t *testing.T) {
	// Right scan must stop when v > upper + skew. 100 only at index 2.
	ts := []int64{50, 98, 100, 102, 200}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("linear stop right: %v", err)
	}
	if idx != 2 || ts[idx] != 100 {
		t.Errorf("linear stop right: idx=%d value=%d, want idx=2 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_KeyNotFoundFromGetDuringBinarySearch(t *testing.T) {
	// [10, 30, 50], target 40: first get(1)=30, second get(2)=KeyNotFound.
	ts := []int64{10, 30, 50}
	calls := 0
	get := func(i int64) (int64, error) {
		calls++
		if calls >= 2 {
			return 0, NewKeyNotFoundError("missing", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(40, 0, int64(len(ts)), get)
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
	get := func(i int64) (int64, error) {
		callCount++
		if callCount >= 4 {
			return 0, NewKeyNotFoundError("corrupt at index", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("KeyNotFound during linear scan: %v", err)
	}
}

func TestFuzzyBinarySearch_ReadErrorFromGetWrapped(t *testing.T) {
	get := func(int64) (int64, error) {
		return 0, NewReadError("disk read failed", nil)
	}
	_, err := FuzzyBinarySearch(100, 0, 1, get)
	var readErr *ReadError
	if !errors.As(err, &readErr) {
		t.Errorf("ReadError from get: %v", err)
	}
}

func TestFuzzyBinarySearch_ExactAtLowerBound(t *testing.T) {
	// target=100 at index 2; [lower,upper]=[90,110].
	ts := []int64{90, 95, 100, 105, 110}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("exact at lower: %v", err)
	}
	if idx != 2 || ts[idx] != 100 {
		t.Errorf("exact at lower: idx=%d value=%d, want idx=2 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_ExactAtUpperBound(t *testing.T) {
	ts := []int64{90, 95, 100, 105, 110}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(110, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("exact at upper: %v", err)
	}
	if idx != 4 || ts[idx] != 110 {
		t.Errorf("exact at upper: idx=%d value=%d want 4, 110", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_BinarySearchNarrowsCorrectly(t *testing.T) {
	// Even-sized, target in second half; odd-sized.
	ts := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(8, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("narrows: %v", err)
	}
	if idx != 7 {
		t.Errorf("narrows: idx=%d want 7", idx)
	}
}

func TestFuzzyBinarySearch_OddLength(t *testing.T) {
	ts := []int64{10, 20, 30, 40, 50}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(30, 0, 5, get)
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
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(102, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("entire in skew: %v", err)
	}
	if idx != 2 || ts[idx] != 102 {
		t.Errorf("entire in skew: idx=%d value=%d, want 2 and 102", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInFirstCluster(t *testing.T) {
	// Three clusters with gaps > 2*skew. Cluster 1: 95-105 (100 at 5), cluster 2: 495-505, cluster 3: 995-1005.
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in first cluster: %v", err)
	}
	if idx != 5 || ts[idx] != 100 {
		t.Errorf("first cluster: idx=%d value=%d, want 5 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInMiddleCluster(t *testing.T) {
	// Middle cluster has out-of-order within skew; 500 at index 13.
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		502, 495, 500, 498, 501, 497, 504, 499, 503, 496, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(500, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in middle cluster: %v", err)
	}
	if idx != 13 || ts[idx] != 500 {
		t.Errorf("middle cluster: idx=%d value=%d, want 13 and 500", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_MultipleClusters_FindInLastCluster(t *testing.T) {
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(1000, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("find in last cluster: %v", err)
	}
	if idx != 27 || ts[idx] != 1000 {
		t.Errorf("last cluster: idx=%d value=%d, want 27 and 1000", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_MultipleClusters_TargetInGapReturnsKeyNotFound(t *testing.T) {
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105,
		495, 496, 497, 498, 499, 500, 501, 502, 503, 504, 505,
		995, 996, 997, 998, 999, 1000, 1001, 1002, 1003, 1004, 1005}
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(300, 10, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("target 300 in gap: err=%v, want KeyNotFoundError", err)
	}
	_, err = FuzzyBinarySearch(750, 10, int64(len(ts)), get)
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
	get := sliceGetter(ts)
	target := int64(n/2 + 100)
	idx, err := FuzzyBinarySearch(target, 0, n, get)
	if err != nil {
		t.Fatalf("large n: %v", err)
	}
	if idx != target {
		t.Errorf("large n: idx=%d want %d", idx, target)
	}
}

func TestFuzzyBinarySearch_NegativeLowerBound(t *testing.T) {
	ts := []int64{0, 1, 2, 3, 4}
	get := sliceGetter(ts)
	idx, err := FuzzyBinarySearch(2, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("negative lower: %v", err)
	}
	if idx != 2 {
		t.Errorf("negative lower: idx=%d want 2", idx)
	}
}

func TestFuzzyBinarySearch_ValidationSkewZeroAllowed(t *testing.T) {
	get := func(i int64) (int64, error) {
		if i != 0 {
			return 0, NewKeyNotFoundError("oob", nil)
		}
		return 0, nil
	}
	_, err := FuzzyBinarySearch(0, 0, 1, get)
	if err != nil {
		t.Errorf("skew 0 allowed: %v", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkew86400000Allowed(t *testing.T) {
	get := func(i int64) (int64, error) {
		if i != 0 {
			return 0, NewKeyNotFoundError("oob", nil)
		}
		return 1, nil
	}
	_, err := FuzzyBinarySearch(1, 86400000, 1, get)
	if err != nil {
		t.Errorf("skew 86400000 allowed: %v", err)
	}
}

func TestFuzzyBinarySearch_NumKeysZeroVsNegative(t *testing.T) {
	get := func(int64) (int64, error) { return 0, nil }
	_, err0 := FuzzyBinarySearch(1, 0, 0, get)
	var keyErr *KeyNotFoundError
	if !errors.As(err0, &keyErr) {
		t.Errorf("numKeys 0: %v, want KeyNotFoundError", err0)
	}
	_, errNeg := FuzzyBinarySearch(1, 0, -1, get)
	var inv *InvalidInputError
	if !errors.As(errNeg, &inv) {
		t.Errorf("numKeys -1: %v, want InvalidInputError", errNeg)
	}
}

func TestFuzzyBinarySearch_ValidationTargetNegative(t *testing.T) {
	get := func(int64) (int64, error) { return 0, nil }
	_, err := FuzzyBinarySearch(-1, 0, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("target < 0: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkewNegative(t *testing.T) {
	get := func(int64) (int64, error) { return 0, nil }
	_, err := FuzzyBinarySearch(0, -1, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("skewMs < 0: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationSkewOver86400000(t *testing.T) {
	get := func(int64) (int64, error) { return 0, nil }
	_, err := FuzzyBinarySearch(0, 86400001, 1, get)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("skewMs > 86400000: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_ValidationGetNil(t *testing.T) {
	_, err := FuzzyBinarySearch(0, 0, 1, nil)
	var inv *InvalidInputError
	if !errors.As(err, &inv) {
		t.Errorf("get == nil: %v, want InvalidInputError", err)
	}
}

func TestFuzzyBinarySearch_GetErrorDuringLeftLinearScan(t *testing.T) {
	// Mid in [lower,upper] but v!=target; get fails on first left-scan call.
	ts := []int64{98, 99, 101, 102}
	calls := 0
	get := func(i int64) (int64, error) {
		calls++
		if calls == 2 {
			return 0, NewKeyNotFoundError("missing", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(100, 10, int64(len(ts)), get)
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
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("left scan break: %v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_RightScanBreakWhenAboveUpperPlusSkew(t *testing.T) {
	// Mid in [lower,upper], v!=target; right scan sees v > upper+skewMs and breaks.
	// [98, 99, 101, 102, 200], target 100, skew 5 → upper+skewMs=110.
	ts := []int64{98, 99, 101, 102, 200}
	get := sliceGetter(ts)
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
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
	get := sliceGetter(ts)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_, _ = FuzzyBinarySearch(500, 0, int64(len(ts)), get)
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
	get := sliceGetter(ts)
	target := int64(50_000 * 1000)
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
	get := sliceGetter(ts)
	target := int64(100_500)
	skew := int64(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FuzzyBinarySearch(target, skew, n, get)
	}
}

func TestFuzzyBinarySearch_SkipIndexDuringBinarySearch(t *testing.T) {
	// Skip index during binary search phase
	// Skip index 2, target 400 at index 3 (not skipped)
	ts := []int64{100, 200, 300, 400, 500}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(400, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip during binary search: %v", err)
	}
	if idx != 3 || ts[idx] != 400 {
		t.Errorf("skip during binary search: idx=%d value=%d, want 3 and 400", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexDuringLinearScanLeft(t *testing.T) {
	// Skip index during left linear scan
	// Skip index 2, target 101 at index 3 (not skipped)
	ts := []int64{98, 99, 100, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(101, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip during left scan: %v", err)
	}
	if idx != 3 || ts[idx] != 101 {
		t.Errorf("skip during left scan: idx=%d value=%d, want 3 and 101", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexDuringLinearScanRight(t *testing.T) {
	// Skip index during right linear scan
	// Skip index 2, target 99 at index 1 (not skipped)
	ts := []int64{98, 99, 100, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(99, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip during right scan: %v", err)
	}
	if idx != 1 || ts[idx] != 99 {
		t.Errorf("skip during right scan: idx=%d value=%d, want 1 and 99", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexRetriesAbove(t *testing.T) {
	// When skipping, retry with index+1
	// Skip index 2, target 400 at index 3 (not skipped)
	ts := []int64{100, 200, 300, 400, 500}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(400, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip retries above: %v", err)
	}
	if idx != 3 {
		t.Errorf("skip retries above: idx=%d, want 3", idx)
	}
}

func TestFuzzyBinarySearch_SkipIndexRetriesBelow(t *testing.T) {
	// When skipping at boundary, retry with index-1
	// Skip index 1, target 300 at index 2 (not skipped)
	ts := []int64{100, 200, 300}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 1 {
			return 0, NewSkipIndexError("skip index 1", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(300, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip retries below: %v", err)
	}
	if idx != 2 {
		t.Errorf("skip retries below: idx=%d, want 2", idx)
	}
}

func TestFuzzyBinarySearch_SkipIndexAtFirstIndex(t *testing.T) {
	// Skip first index (0), must retry with index+1
	// Skip index 0, target 200 at index 1 (not skipped)
	ts := []int64{100, 200, 300, 400, 500}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 0 {
			return 0, NewSkipIndexError("skip index 0", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(200, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip at first index: %v", err)
	}
	if idx != 1 {
		t.Errorf("skip at first index: idx=%d, want 1", idx)
	}
}

func TestFuzzyBinarySearch_SkipIndexAtLastIndex(t *testing.T) {
	// Skip last index, must retry with index-1
	// Skip index 4, target 400 at index 3 (not skipped)
	ts := []int64{100, 200, 300, 400, 500}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 4 {
			return 0, NewSkipIndexError("skip index 4", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(400, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip at last index: %v", err)
	}
	if idx != 3 {
		t.Errorf("skip at last index: idx=%d, want 3", idx)
	}
}

func TestFuzzyBinarySearch_SkipIndexMultipleTimes(t *testing.T) {
	// Skip multiple different indices during search
	ts := []int64{100, 200, 300, 400, 500, 600, 700}
	skipped := make(map[int64]bool)
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip indices 1 and 3 once each
		if (i == 1 || i == 3) && !skipped[i] {
			skipped[i] = true
			return 0, NewSkipIndexError("skip index", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(400, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip multiple times: %v", err)
	}
	if idx != 3 || ts[idx] != 400 {
		t.Errorf("skip multiple times: idx=%d value=%d, want 3 and 400", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexWithSkewWindow(t *testing.T) {
	// Skip index when searching with skew window
	// Skip index 2, target 98 at index 1 (not skipped)
	ts := []int64{95, 98, 100, 102, 105}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(98, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip with skew: %v", err)
	}
	if idx != 1 || ts[idx] != 98 {
		t.Errorf("skip with skew: idx=%d value=%d, want 1 and 98", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexPropagatesOtherErrors(t *testing.T) {
	// SkipIndexError should not mask other errors like KeyNotFoundError
	ts := []int64{100, 200, 300}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 1 {
			return 0, NewSkipIndexError("skip index 1", nil)
		}
		if i == 2 {
			return 0, NewKeyNotFoundError("key not found", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(300, 0, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("skip propagates other errors: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_SkipIndexSingleElementSkipped(t *testing.T) {
	// Single element case where the only element is skipped (mid == lo == hi)
	// Should return KeyNotFoundError
	ts := []int64{100}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 0 {
			return 0, NewSkipIndexError("skip index 0", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(100, 0, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("skip single element: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_SkipIndexMidEqualsLo(t *testing.T) {
	// Skip when mid == lo (boundary case), should retry with mid + 1
	// Array: [100, 200, 300], target 200 at index 1
	// When mid == lo == 0, skip it, retry with index 1
	ts := []int64{100, 200, 300}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 0 {
			return 0, NewSkipIndexError("skip index 0", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(200, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip mid equals lo: %v", err)
	}
	if idx != 1 || ts[idx] != 200 {
		t.Errorf("skip mid equals lo: idx=%d value=%d, want 1 and 200", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexMidEqualsHi(t *testing.T) {
	// Skip when mid == hi (boundary case), should retry with mid - 1
	// Array: [100, 200, 300], target 200 at index 1
	// When mid == hi == 2, skip it, retry with index 1
	ts := []int64{100, 200, 300}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(200, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip mid equals hi: %v", err)
	}
	if idx != 1 || ts[idx] != 200 {
		t.Errorf("skip mid equals hi: idx=%d value=%d, want 1 and 200", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexTargetAtSkippedIndex(t *testing.T) {
	// Target is at the skipped index, should return KeyNotFoundError
	// Array: [100, 200, 300], target 200 at index 1 (skipped)
	ts := []int64{100, 200, 300}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 1 {
			return 0, NewSkipIndexError("skip index 1", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(200, 0, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("skip target at skipped index: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_SkipIndexAllIndeterminateRangeSkipped(t *testing.T) {
	// Most indices in indeterminate range are skipped, but target is at mid (not skipped)
	// Array: [98, 99, 100, 101, 102], target 100, skew 5
	// Mid hits index 2 (value 100), skip indices 0, 1, 3, 4 (but not consecutive pairs per guarantee)
	// Since mid value == target, it should return index 2
	ts := []int64{98, 99, 100, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip indices 0, 1, 3, 4 but not 2 (mid) - note: 0 and 1 are consecutive, but
		// per guarantee, if we skip 0, then 1 must work, so this test respects the guarantee
		// Actually, to respect guarantee properly: skip 0 and 3 (non-consecutive)
		if i == 0 || i == 3 {
			return 0, NewSkipIndexError("skip index", nil)
		}
		return ts[i], nil
	}
	// Target 100 is at index 2, mid will hit index 2 with value 100
	// Since v == target, it should return index 2
	idx, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip all indeterminate: %v", err)
	}
	if idx != 2 || ts[idx] != 100 {
		t.Errorf("skip all indeterminate: idx=%d value=%d, want 2 and 100", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexAllIndeterminateRangeSkippedNoMatch(t *testing.T) {
	// Some indices in indeterminate range are skipped (non-consecutive), target not present, should return KeyNotFoundError
	// Array: [98, 99, 101, 102], target 100, skew 5
	// Mid hits index 1 or 2 (value 99 or 101), skip non-consecutive indices
	ts := []int64{98, 99, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip indices 0 and 3 (non-consecutive, respects guarantee)
		// Target 100 is not in array, so should return KeyNotFoundError
		if i == 0 || i == 3 {
			return 0, NewSkipIndexError("skip index", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(100, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("skip all indeterminate no match: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_SkipIndexMultipleInIndeterminateRange(t *testing.T) {
	// Multiple non-consecutive indices in indeterminate range are skipped, but target is not skipped
	// Array: [95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105], target 98, skew 10
	// Skip indices 4 and 6 (non-consecutive, respects guarantee), target 98 at index 3 (not skipped)
	ts := []int64{95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip indices 4 and 6 (non-consecutive, respects guarantee: 4 and 5 won't both skip, 5 and 6 won't both skip)
		if i == 4 || i == 6 {
			return 0, NewSkipIndexError("skip index", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(98, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip multiple in indeterminate: %v", err)
	}
	if idx != 3 || ts[idx] != 98 {
		t.Errorf("skip multiple in indeterminate: idx=%d value=%d, want 3 and 98", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexTargetAtSkippedIndexInLinearScan(t *testing.T) {
	// Target is at a skipped index during linear scan, should return KeyNotFoundError
	// Array: [98, 99, 101, 102], target 100 (not present), but if we had 100 at index 2 (skipped)
	// Actually, let's test: target 99 at index 1, but index 1 is skipped during linear scan
	ts := []int64{98, 99, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip index 1 where target 99 is
		if i == 1 {
			return 0, NewSkipIndexError("skip index 1", nil)
		}
		return ts[i], nil
	}
	_, err := FuzzyBinarySearch(99, 5, int64(len(ts)), get)
	var keyErr *KeyNotFoundError
	if !errors.As(err, &keyErr) {
		t.Errorf("skip target in linear scan: err=%v, want KeyNotFoundError", err)
	}
}

func TestFuzzyBinarySearch_SkipIndexTwoElementsOneSkipped(t *testing.T) {
	// Two elements, one skipped (respects guarantee: consecutive indices won't both skip)
	// Array: [100, 200], skip index 0, target 200 at index 1
	ts := []int64{100, 200}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip only index 0 (index 1 must work per guarantee)
		if i == 0 {
			return 0, NewSkipIndexError("skip index 0", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(200, 0, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip one of two elements: %v", err)
	}
	if idx != 1 || ts[idx] != 200 {
		t.Errorf("skip one of two elements: idx=%d value=%d, want 1 and 200", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexDuringBinarySearchThenLinearScan(t *testing.T) {
	// Skip during binary search, then skip again during linear scan (non-consecutive indices)
	// Array: [98, 99, 100, 101, 102], target 101, skew 5
	// Skip index 2 during binary search, skip index 4 during linear scan (non-consecutive, respects guarantee)
	// Target 101 is at index 3 (not skipped), should find it
	ts := []int64{98, 99, 100, 101, 102}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		// Skip index 2 during binary search, skip index 4 during linear scan (non-consecutive)
		if i == 2 || i == 4 {
			return 0, NewSkipIndexError("skip index", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(101, 5, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip binary then linear: %v", err)
	}
	if idx != 3 || ts[idx] != 101 {
		t.Errorf("skip binary then linear: idx=%d value=%d, want 3 and 101", idx, ts[idx])
	}
}

func TestFuzzyBinarySearch_SkipIndexAtBoundaryWithSkew(t *testing.T) {
	// Skip at boundary when searching with skew window
	// Array: [90, 95, 100, 105, 110], target 95, skew 10
	// Skip index 2 (mid), should retry and find target at index 1 (not skipped)
	ts := []int64{90, 95, 100, 105, 110}
	get := func(i int64) (int64, error) {
		if i < 0 || i >= int64(len(ts)) {
			return 0, NewKeyNotFoundError("index out of range", nil)
		}
		if i == 2 {
			return 0, NewSkipIndexError("skip index 2", nil)
		}
		return ts[i], nil
	}
	idx, err := FuzzyBinarySearch(95, 10, int64(len(ts)), get)
	if err != nil {
		t.Fatalf("skip at boundary with skew: %v", err)
	}
	if idx != 1 || ts[idx] != 95 {
		t.Errorf("skip at boundary with skew: idx=%d value=%d, want 1 and 95", idx, ts[idx])
	}
}
