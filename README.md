# tiny-hashed-timer-wheel

A hashed timer wheel that schedules callbacks with O(1) start and cancel, ticked by an external clock.

## Usage

```go
package main

import (
	"fmt"
	"time"

	tinyhashedtimerwheel "github.com/example/tiny-hashed-timer-wheel"
)

func main() {
	w := tinyhashedtimerwheel.New(64, time.Millisecond)
	deadline := time.Now().Add(100 * time.Millisecond)
	id := w.Start(deadline, func() {
		fmt.Println("timer fired")
	})
	// Later, to cancel:
	_ = id
	w.Cancel(id)

	// Advance the wheel to the current time to fire due timers.
	w.Advance(time.Now())
}
```

## Why this library exists

A timer wheel is useful when you have many timers and want to avoid the overhead of a
heap or per-timer goroutines. The trade-off is that timers are not fired automatically;
you must call `Advance` periodically. This is ideal for event loops or game loops that
already have a tick.

The hashed approach means that `Start` and `Cancel` are O(1) on average, but `Advance`
scans all slots. With a fixed number of slots, the scan is bounded and predictable.

## Awkward edge

Timers with deadlines far in the future may be assigned to the same slot as timers with
near deadlines if their tick numbers collide modulo the number of slots. This is inherent
to the hashed design. If you need precise ordering, use a heap.

Also, `Advance` does not fire timers that were started after the last `Advance` call
unless their deadline is at or before the new `now`. This means you should call `Advance`
regularly with the current time.
