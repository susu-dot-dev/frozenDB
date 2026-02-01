package frozendb

import (
	"sync"
)

// Subscriber manages thread-safe subscription/notification with snapshot pattern.
// T is the callback function type (e.g., func() error, func(int64, *RowUnion) error).
//
// Thread-safety is achieved through:
// - Mutex protection for all map operations
// - Snapshot pattern prevents deadlocks during callback execution
//
// The snapshot pattern works as follows:
// 1. Lock is acquired only to copy callbacks to a slice
// 2. Lock is released before returning the slice
// 3. Caller executes callbacks WITHOUT holding the lock
// 4. This allows callbacks to safely call Subscribe/Unsubscribe
type Subscriber[T any] struct {
	mu        sync.Mutex
	callbacks map[int64]T
	nextID    int64
}

// NewSubscriber creates a new Subscriber instance with empty callback map.
// The nextID counter starts at 1 to avoid confusion with zero values.
func NewSubscriber[T any]() *Subscriber[T] {
	return &Subscriber[T]{
		callbacks: make(map[int64]T),
		nextID:    1,
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
	s.callbacks[id] = callback
	s.mu.Unlock()

	// Return closure that captures the subscription ID
	return func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Delete is idempotent - deleting a non-existent key is a no-op
		delete(s.callbacks, id)
		return nil
	}
}

// Snapshot returns a thread-safe copy of current callbacks as a slice.
//
// Key properties:
// - Creates a copy of the callbacks map to a slice
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

	// Copy callbacks to slice
	result := make([]T, 0, len(s.callbacks))
	for _, callback := range s.callbacks {
		result = append(result, callback)
	}
	return result
}
