package process

import (
	"context"
	"fmt"
	"sync"
)

// Scheduler manages global worker concurrency.
// It enforces max_total_workers across all executions.
type Scheduler struct {
	mu       sync.Mutex
	maxSlots int
	active   int
	waiters  []chan struct{} // FIFO queue of waiting requests
}

// NewScheduler creates a scheduler with the given max concurrent workers.
func NewScheduler(maxWorkers int) *Scheduler {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	return &Scheduler{
		maxSlots: maxWorkers,
	}
}

// Acquire blocks until a worker slot is available, or ctx is cancelled.
// Returns nil on success, ctx.Err() if cancelled.
func (s *Scheduler) Acquire(ctx context.Context) error {
	s.mu.Lock()

	if s.active < s.maxSlots {
		s.active++
		s.mu.Unlock()
		return nil
	}

	// No slot available — enqueue a waiter.
	ch := make(chan struct{}, 1)
	s.waiters = append(s.waiters, ch)
	s.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		// Remove ourselves from the waiters list to avoid a leak.
		s.mu.Lock()
		found := false
		for i, w := range s.waiters {
			if w == ch {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				found = true
				break
			}
		}
		s.mu.Unlock()
		if !found {
			// Release() already removed us and sent a token. Return it.
			select {
			case <-ch:
				s.Release()
			default:
			}
		}
		return fmt.Errorf("process: acquire: %w", ctx.Err())
	}
}

// Release frees a worker slot. Must be called when a worker exits.
func (s *Scheduler) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.waiters) > 0 {
		next := s.waiters[0]
		s.waiters = s.waiters[1:]
		next <- struct{}{}
		return
	}

	if s.active <= 0 {
		return // programming error: Release without matching Acquire
	}
	s.active--
}

// Active returns the current number of active workers.
func (s *Scheduler) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Available returns the number of available slots.
func (s *Scheduler) Available() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSlots - s.active
}
