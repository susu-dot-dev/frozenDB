package frozendb

import (
	"sync"
)

// entry pairs a subscription ID with its callback function.
// This is used internally by Subscriber to maintain registration order.
type entry[T any] struct {
	id       int64
	callback T
}

// Subscriber manages thread-safe subscription/notification with snapshot pattern.
// T is the callback function type (e.g., func() error, func(int64, *RowUnion) error).
//
// Callbacks are maintained in registration order - Snapshot() returns callbacks
// in the exact order they were registered via Subscribe().
//
// Thread-safety is achieved through:
// - Mutex protection for all slice operations
// - Snapshot pattern prevents deadlocks during callback execution
//
// The snapshot pattern works as follows:
// 1. Lock is acquired only to copy callbacks to a slice
// 2. Lock is released before returning the slice
// 3. Caller executes callbacks WITHOUT holding the lock
// 4. This allows callbacks to safely call Subscribe/Unsubscribe
type Subscriber[T any] struct {
	mu      sync.Mutex
	entries []entry[T]
	nextID  int64
}

// NewSubscriber creates a new Subscriber instance with empty callback slice.
// The nextID counter starts at 1 to avoid confusion with zero values.
func NewSubscriber[T any]() *Subscriber[T] {
	return &Subscriber[T]{
		entries: make([]entry[T], 0),
		nextID:  1,
	}
}

// Subscribe adds a callback and returns an unsubscribe closure.
//
// The returned unsubscribe function:
// - Is idempotent (safe to call multiple times)
// - Takes effect immediately (callback won't be in future snapshots)
// - Allows self-unsubscription during callback execution (callback completes current execution)
//
// Thread-safe: May be called concurrently with other Subscribe/Unsubscribe/Snapshot calls.
func (s *Subscriber[T]) Subscribe(callback T) func() error {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.entries = append(s.entries, entry[T]{id: id, callback: callback})
	s.mu.Unlock()

	// Return closure that captures the subscription ID
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Find and remove the entry with this ID
		for i, e := range s.entries {
			if e.id == id {
				// Remove entry by slicing around it
				s.entries = append(s.entries[:i], s.entries[i+1:]...)
				break
			}
		}
		return nil
	}
}

// Snapshot returns a thread-safe copy of current callbacks as a slice.
//
// Key properties:
// - Returns callbacks in exact registration order (order Subscribe() was called)
// - Creates a copy of callbacks to a slice
// - Lock is held only during the copy operation
// - Returned slice is independent of the current subscriber state
// - New subscriptions after snapshot creation are NOT included
// - Unsubscriptions during callback execution complete the current execution
//
// This pattern prevents deadlocks:
// - Callbacks can safely call Subscribe/Unsubscribe on the same Subscriber
// - No lock is held during callback execution
//
// Thread-safe: May be called concurrently with other Subscribe/Unsubscribe/Snapshot calls.
func (s *Subscriber[T]) Snapshot() []T {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy callbacks to slice in registration order
	result := make([]T, 0, len(s.entries))
	for _, entry := range s.entries {
		result = append(result, entry.callback)
	}
	return result
}
