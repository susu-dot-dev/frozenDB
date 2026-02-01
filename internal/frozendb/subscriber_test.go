package frozendb

import (
	"sync"
	"testing"
)

// TestSubscriber_BasicSubscription tests basic subscribe/unsubscribe flow
func TestSubscriber_BasicSubscription(t *testing.T) {
	sub := NewSubscriber[func() error]()

	// Subscribe a callback
	called := false
	callback := func() error {
		called = true
		return nil
	}

	unsubscribe := sub.Subscribe(callback)

	// Get snapshot and call callbacks
	snapshot := sub.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Expected 1 callback in snapshot, got %d", len(snapshot))
	}

	// Call the callback from snapshot
	err := snapshot[0]()
	if err != nil {
		t.Fatalf("Callback returned error: %v", err)
	}
	if !called {
		t.Error("Callback was not called")
	}

	// Unsubscribe
	err = unsubscribe()
	if err != nil {
		t.Fatalf("Unsubscribe returned error: %v", err)
	}

	// Verify callback is removed
	snapshot = sub.Snapshot()
	if len(snapshot) != 0 {
		t.Errorf("Expected 0 callbacks after unsubscribe, got %d", len(snapshot))
	}
}

// TestSubscriber_MultipleSubscribers tests multiple independent subscribers
func TestSubscriber_MultipleSubscribers(t *testing.T) {
	sub := NewSubscriber[func() error]()

	// Subscribe 3 callbacks
	var call1, call2, call3 bool
	unsub1 := sub.Subscribe(func() error {
		call1 = true
		return nil
	})
	unsub2 := sub.Subscribe(func() error {
		call2 = true
		return nil
	})
	unsub3 := sub.Subscribe(func() error {
		call3 = true
		return nil
	})

	// Get snapshot
	snapshot := sub.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("Expected 3 callbacks, got %d", len(snapshot))
	}

	// Call all callbacks
	for _, cb := range snapshot {
		if err := cb(); err != nil {
			t.Fatalf("Callback returned error: %v", err)
		}
	}

	if !call1 || !call2 || !call3 {
		t.Error("Not all callbacks were called")
	}

	// Unsubscribe one and verify others remain
	unsub2()
	snapshot = sub.Snapshot()
	if len(snapshot) != 2 {
		t.Errorf("Expected 2 callbacks after one unsubscribe, got %d", len(snapshot))
	}

	// Clean up
	unsub1()
	unsub3()
}

// TestSubscriber_IdempotentUnsubscribe tests that unsubscribe can be called multiple times
func TestSubscriber_IdempotentUnsubscribe(t *testing.T) {
	sub := NewSubscriber[func() error]()

	unsub := sub.Subscribe(func() error { return nil })

	// Call unsubscribe multiple times - should not panic or error
	for i := 0; i < 3; i++ {
		err := unsub()
		if err != nil {
			t.Fatalf("Unsubscribe call %d returned error: %v", i, err)
		}
	}

	// Verify callback was removed
	snapshot := sub.Snapshot()
	if len(snapshot) != 0 {
		t.Errorf("Expected 0 callbacks, got %d", len(snapshot))
	}
}

// TestSubscriber_ThreadSafety tests concurrent subscribe/unsubscribe/snapshot operations
func TestSubscriber_ThreadSafety(t *testing.T) {
	sub := NewSubscriber[func() error]()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrently subscribe, snapshot, and unsubscribe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Subscribe
			unsub := sub.Subscribe(func() error { return nil })

			// Get snapshot multiple times
			for j := 0; j < 5; j++ {
				snapshot := sub.Snapshot()
				_ = snapshot // Use snapshot
			}

			// Unsubscribe
			unsub()
		}()
	}

	wg.Wait()

	// After all operations, should have no subscribers
	finalSnapshot := sub.Snapshot()
	if len(finalSnapshot) != 0 {
		t.Errorf("Expected 0 callbacks after all goroutines complete, got %d", len(finalSnapshot))
	}
}

// TestSubscriber_SnapshotIsolation tests that snapshot is independent of current state
func TestSubscriber_SnapshotIsolation(t *testing.T) {
	sub := NewSubscriber[func() error]()

	// Subscribe callback
	unsub := sub.Subscribe(func() error { return nil })

	// Get snapshot
	snapshot1 := sub.Snapshot()
	if len(snapshot1) != 1 {
		t.Fatalf("Expected 1 callback in snapshot1, got %d", len(snapshot1))
	}

	// Unsubscribe
	unsub()

	// Original snapshot should still have 1 callback (it's a copy)
	if len(snapshot1) != 1 {
		t.Error("Snapshot should be independent of current state")
	}

	// New snapshot should have 0 callbacks
	snapshot2 := sub.Snapshot()
	if len(snapshot2) != 0 {
		t.Errorf("Expected 0 callbacks in snapshot2, got %d", len(snapshot2))
	}
}

// TestSubscriber_GenericType tests Subscriber with different callback types
func TestSubscriber_GenericType(t *testing.T) {
	// Test with func() error
	sub1 := NewSubscriber[func() error]()
	unsub1 := sub1.Subscribe(func() error { return nil })
	if len(sub1.Snapshot()) != 1 {
		t.Error("Subscriber[func() error] failed")
	}
	unsub1()

	// Test with func(int64, *RowUnion) error (RowEmitter callback type)
	sub2 := NewSubscriber[func(int64, *RowUnion) error]()
	unsub2 := sub2.Subscribe(func(index int64, row *RowUnion) error { return nil })
	if len(sub2.Snapshot()) != 1 {
		t.Error("Subscriber[func(int64, *RowUnion) error] failed")
	}
	unsub2()
}
