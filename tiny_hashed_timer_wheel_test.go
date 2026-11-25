package tinyhashedtimerwheel

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestStartAndAdvance(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	w.Start(time.Unix(10, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Advance(time.Unix(9, 0))
	if atomic.LoadInt32(&fired) != 0 {
		t.Fatal("timer fired before deadline")
	}
	w.Advance(time.Unix(10, 0))
	if atomic.LoadInt32(&fired) != 1 {
		t.Fatal("timer did not fire at deadline")
	}
}

func TestCancel(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	id := w.Start(time.Unix(10, 0), func() { atomic.AddInt32(&fired, 1) })
	if !w.Cancel(id) {
		t.Fatal("Cancel returned false for existing timer")
	}
	w.Advance(time.Unix(11, 0))
	if atomic.LoadInt32(&fired) != 0 {
		t.Fatal("cancelled timer fired")
	}
	if w.Cancel(id) {
		t.Fatal("Cancel returned true for already cancelled timer")
	}
}

func TestCancelNonExistent(t *testing.T) {
	w := New(8, time.Second)
	if w.Cancel(999) {
		t.Fatal("Cancel returned true for non-existent timer")
	}
}

func TestMultipleTimersDifferentSlots(t *testing.T) {
	w := New(4, time.Second)
	var fired int32
	w.Start(time.Unix(1, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Start(time.Unix(2, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Start(time.Unix(3, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Advance(time.Unix(2, 0))
	if atomic.LoadInt32(&fired) != 2 {
		t.Fatalf("expected 2 timers fired, got %d", atomic.LoadInt32(&fired))
	}
	w.Advance(time.Unix(3, 0))
	if atomic.LoadInt32(&fired) != 3 {
		t.Fatalf("expected 3 timers fired, got %d", atomic.LoadInt32(&fired))
	}
}

func TestAdvanceBackwardsDoesNothing(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	w.Start(time.Unix(10, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Advance(time.Unix(11, 0))
	if atomic.LoadInt32(&fired) != 1 {
		t.Fatal("timer did not fire")
	}
	// Advance backwards should not fire anything again.
	w.Advance(time.Unix(5, 0))
	if atomic.LoadInt32(&fired) != 1 {
		t.Fatal("timer fired again after backwards advance")
	}
}

func TestCallbackCanStartNewTimer(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	w.Start(time.Unix(1, 0), func() {
		atomic.AddInt32(&fired, 1)
		w.Start(time.Unix(2, 0), func() { atomic.AddInt32(&fired, 1) })
	})
	w.Advance(time.Unix(1, 0))
	if atomic.LoadInt32(&fired) != 1 {
		t.Fatal("first timer did not fire")
	}
	w.Advance(time.Unix(2, 0))
	if atomic.LoadInt32(&fired) != 2 {
		t.Fatal("second timer did not fire")
	}
}

func TestCallbackCanCancel(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	var id TimerID
	id = w.Start(time.Unix(1, 0), func() {
		atomic.AddInt32(&fired, 1)
		w.Cancel(id) // cancel self, should be no-op since already fired
	})
	w.Advance(time.Unix(1, 0))
	if atomic.LoadInt32(&fired) != 1 {
		t.Fatal("timer did not fire")
	}
}

func TestSameDeadline(t *testing.T) {
	w := New(8, time.Second)
	var fired int32
	w.Start(time.Unix(5, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Start(time.Unix(5, 0), func() { atomic.AddInt32(&fired, 1) })
	w.Advance(time.Unix(5, 0))
	if atomic.LoadInt32(&fired) != 2 {
		t.Fatalf("expected 2, got %d", atomic.LoadInt32(&fired))
	}
}

func TestZeroSlotsPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero slots")
		}
	}()
	New(0, time.Second)
}

func TestZeroTickPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for zero tick")
		}
	}()
	New(8, 0)
}
