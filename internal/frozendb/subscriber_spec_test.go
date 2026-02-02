package frozendb

import (
	"testing"
)

// Test_S_040_FR_001_OrderedDataStructure verifies that Subscriber maintains
// callbacks in registration order internally. This is tested implicitly by
// verifying that Snapshot() returns callbacks in the same order they were registered.
func Test_S_040_FR_001_OrderedDataStructure(t *testing.T) {
	sub := NewSubscriber[func(int) int]()

	// Register 5 callbacks that return their callback number
	callback1 := func(x int) int { return 1 }
	callback2 := func(x int) int { return 2 }
	callback3 := func(x int) int { return 3 }
	callback4 := func(x int) int { return 4 }
	callback5 := func(x int) int { return 5 }

	sub.Subscribe(callback1)
	sub.Subscribe(callback2)
	sub.Subscribe(callback3)
	sub.Subscribe(callback4)
	sub.Subscribe(callback5)

	// Get snapshot - should be in registration order
	snapshot := sub.Snapshot()

	if len(snapshot) != 5 {
		t.Fatalf("Expected 5 callbacks in snapshot, got %d", len(snapshot))
	}

	// Verify callbacks execute in registration order by checking their return values
	expectedOrder := []int{1, 2, 3, 4, 5}
	for i, cb := range snapshot {
		result := cb(0)
		if result != expectedOrder[i] {
			t.Errorf("Callback at position %d returned %d, expected %d (registration order violated)",
				i, result, expectedOrder[i])
		}
	}
}

// Test_S_040_FR_002_SnapshotReturnsOrderedCallbacks verifies that Snapshot()
// returns callbacks in exact registration order.
func Test_S_040_FR_002_SnapshotReturnsOrderedCallbacks(t *testing.T) {
	sub := NewSubscriber[func() int]()

	// Register 10 callbacks
	for i := 1; i <= 10; i++ {
		num := i // Capture loop variable
		sub.Subscribe(func() int { return num })
	}

	// Take multiple snapshots and verify order consistency
	for attempt := 0; attempt < 5; attempt++ {
		snapshot := sub.Snapshot()

		if len(snapshot) != 10 {
			t.Fatalf("Attempt %d: Expected 10 callbacks, got %d", attempt, len(snapshot))
		}

		// Verify exact order: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
		for i, cb := range snapshot {
			expectedValue := i + 1
			actualValue := cb()
			if actualValue != expectedValue {
				t.Errorf("Attempt %d: Callback at position %d returned %d, expected %d",
					attempt, i, actualValue, expectedValue)
			}
		}
	}
}

// Test_S_040_FR_003_UnsubscribePreservesOrder verifies that unsubscribe
// doesn't affect the relative order of remaining callbacks.
func Test_S_040_FR_003_UnsubscribePreservesOrder(t *testing.T) {
	sub := NewSubscriber[func() int]()

	// Register 5 callbacks (1, 2, 3, 4, 5)
	var unsubscribers []func() error
	for i := 1; i <= 5; i++ {
		num := i // Capture loop variable
		unsub := sub.Subscribe(func() int { return num })
		unsubscribers = append(unsubscribers, unsub)
	}

	// Initial snapshot should be [1, 2, 3, 4, 5]
	snapshot := sub.Snapshot()
	if len(snapshot) != 5 {
		t.Fatalf("Expected 5 callbacks initially, got %d", len(snapshot))
	}

	// Verify initial order
	for i := 0; i < 5; i++ {
		if snapshot[i]() != i+1 {
			t.Errorf("Initial order incorrect at position %d", i)
		}
	}

	// Unsubscribe callback 3 (middle element)
	err := unsubscribers[2]()
	if err != nil {
		t.Fatalf("Unsubscribe returned error: %v", err)
	}

	// After unsubscribe, order should be [1, 2, 4, 5]
	snapshot = sub.Snapshot()
	if len(snapshot) != 4 {
		t.Fatalf("Expected 4 callbacks after unsubscribe, got %d", len(snapshot))
	}

	expectedAfterUnsubscribe := []int{1, 2, 4, 5}
	for i, cb := range snapshot {
		result := cb()
		if result != expectedAfterUnsubscribe[i] {
			t.Errorf("After unsubscribe: callback at position %d returned %d, expected %d",
				i, result, expectedAfterUnsubscribe[i])
		}
	}

	// Unsubscribe first element (callback 1)
	err = unsubscribers[0]()
	if err != nil {
		t.Fatalf("Second unsubscribe returned error: %v", err)
	}

	// Order should be [2, 4, 5]
	snapshot = sub.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("Expected 3 callbacks after second unsubscribe, got %d", len(snapshot))
	}

	expectedAfterSecondUnsubscribe := []int{2, 4, 5}
	for i, cb := range snapshot {
		result := cb()
		if result != expectedAfterSecondUnsubscribe[i] {
			t.Errorf("After second unsubscribe: callback at position %d returned %d, expected %d",
				i, result, expectedAfterSecondUnsubscribe[i])
		}
	}

	// Unsubscribe last element (callback 5)
	err = unsubscribers[4]()
	if err != nil {
		t.Fatalf("Third unsubscribe returned error: %v", err)
	}

	// Order should be [2, 4]
	snapshot = sub.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("Expected 2 callbacks after third unsubscribe, got %d", len(snapshot))
	}

	expectedAfterThirdUnsubscribe := []int{2, 4}
	for i, cb := range snapshot {
		result := cb()
		if result != expectedAfterThirdUnsubscribe[i] {
			t.Errorf("After third unsubscribe: callback at position %d returned %d, expected %d",
				i, result, expectedAfterThirdUnsubscribe[i])
		}
	}
}
