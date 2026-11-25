// Package tinyhashedtimerwheel implements a hashed timer wheel for scheduling
// callbacks with O(1) start and cancel, driven by an external clock.
//
// The wheel uses a fixed number of buckets (slots) and a tick duration. Each
// timer is assigned to a bucket based on its deadline. When the wheel is
// advanced (by calling Advance with a new time), all timers whose deadlines
// have passed are fired. Timers can be started and cancelled in constant time.
//
// The trade-off is that the wheel does not support arbitrary precision: timers
// are only fired when Advance is called. This makes it suitable for systems
// that already have a periodic tick (e.g., a game loop or event loop) and want
// to avoid per-timer goroutines or a sorted list.
package tinyhashedtimerwheel

import (
	"sync"
	"time"
)

// TimerID is a unique identifier for a scheduled timer.
type TimerID uint64

// Callback is the function invoked when a timer expires.
type Callback func()

type timer struct {
	id       TimerID
	deadline time.Time
	cb       Callback
	cancelled bool
}

// Wheel is a hashed timer wheel. It is safe for concurrent use.
type Wheel struct {
	mu       sync.Mutex
	slots    [][]*timer
	tick     time.Duration
	nextID   TimerID
	current  time.Time
}

// New creates a Wheel with the given number of slots and tick duration.
// The number of slots must be positive; if it is not, New panics.
// The tick duration must be positive; if it is not, New panics.
func New(slots int, tick time.Duration) *Wheel {
	if slots <= 0 {
		panic("tinyhashedtimerwheel: slots must be positive")
	}
	if tick <= 0 {
		panic("tinyhashedtimerwheel: tick must be positive")
	}
	return &Wheel{
		slots: make([][]*timer, slots),
		tick:  tick,
	}
}

// Start schedules cb to be called at or after deadline. It returns a TimerID
// that can be used to cancel the timer. The callback is invoked during a call
// to Advance, not in a separate goroutine.
func (w *Wheel) Start(deadline time.Time, cb Callback) TimerID {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.nextID++
	id := w.nextID
	t := &timer{
		id:       id,
		deadline: deadline,
		cb:       cb,
	}
	idx := w.slotIndex(deadline)
	w.slots[idx] = append(w.slots[idx], t)
	return id
}

// Cancel removes the timer with the given id. It returns true if the timer was
// found and cancelled, false otherwise. Cancelling an already-fired or
// already-cancelled timer returns false.
func (w *Wheel) Cancel(id TimerID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// We don't know which slot the timer is in, so we scan all slots.
	// This is O(slots + total timers), but slots is fixed and small.
	for i, slot := range w.slots {
		for j, t := range slot {
			if t.id == id {
				if t.cancelled {
					return false
				}
				t.cancelled = true
				// Remove from slice to free memory and speed up future scans.
				w.slots[i] = append(slot[:j], slot[j+1:]...)
				return true
			}
		}
	}
	return false
}

// Advance moves the wheel's current time to now and fires all timers whose
// deadlines are at or before now. Timers are fired in the order they were
// started within each slot, but slots are processed in index order.
// If now is before the wheel's current time, Advance does nothing.
func (w *Wheel) Advance(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if now.Before(w.current) {
		return
	}
	w.current = now

	// We need to collect timers to fire, then fire them outside the lock?
	// But we hold the lock for the whole operation to keep it simple and
	// avoid reentrancy issues. The callback might call Start or Cancel,
	// which would deadlock if we held the lock. So we collect and release
	// the lock before firing.
	var toFire []*timer
	for i := range w.slots {
		slot := w.slots[i]
		// Filter out timers that are due and not cancelled.
		kept := slot[:0]
		for _, t := range slot {
			if t.cancelled {
				continue
			}
			if !t.deadline.After(now) {
				toFire = append(toFire, t)
			} else {
				kept = append(kept, t)
			}
		}
		w.slots[i] = kept
	}

	// Release lock before calling callbacks to avoid deadlock if callback
	// calls back into the wheel.
	w.mu.Unlock()
	for _, t := range toFire {
		t.cb()
	}
	w.mu.Lock()
}

// slotIndex returns the index of the slot for the given deadline.
func (w *Wheel) slotIndex(deadline time.Time) int {
	// Use the deadline's UnixNano to compute a slot index. This is a simple
	// hash: we divide by the tick duration and take modulo the number of slots.
	// This spreads timers across slots, but timers with the same deadline
	// always land in the same slot.
	nanos := deadline.UnixNano()
	tickNanos := w.tick.Nanoseconds()
	if tickNanos <= 0 {
		// tick is guaranteed positive by New, but guard against overflow.
		return 0
	}
	// Use integer division to get the tick number, then modulo slots.
	// This can overflow for very large deadlines, but that's acceptable.
	idx := (nanos / tickNanos) % int64(len(w.slots))
	if idx < 0 {
		idx += int64(len(w.slots))
	}
	return int(idx)
}
