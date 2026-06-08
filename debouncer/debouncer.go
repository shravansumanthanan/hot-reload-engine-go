package debouncer

import (
	"context"
	"time"
)

// Debouncer groups rapid events together and triggers an action
// after a period of silence.
type Debouncer struct {
	duration time.Duration
	events   chan struct{}
	action   func()
	stop     chan struct{}
}

// New creates a new Debouncer. The provided context controls the debouncer's
// lifetime: when ctx is cancelled the background goroutine exits cleanly
// without waiting for Stop() to be called.
func New(ctx context.Context, duration time.Duration, action func()) *Debouncer {
	d := &Debouncer{
		duration: duration,
		events:   make(chan struct{}, 1),
		action:   action,
		stop:     make(chan struct{}),
	}
	go d.run(ctx)
	return d
}

// Trigger notifies the debouncer that an event has occurred.
func (d *Debouncer) Trigger() {
	select {
	case d.events <- struct{}{}:
	default:
		// Channel already has a pending event
	}
}

// Stop stops the debouncer background routine.
func (d *Debouncer) Stop() {
	close(d.stop)
}

func (d *Debouncer) run(ctx context.Context) {
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-d.events:
			if timer != nil {
				if !timer.Stop() {
					// Timer already fired, drain the channel
					select {
					case <-timer.C:
					default:
					}
				}
			}
			timer = time.NewTimer(d.duration)
			timerC = timer.C

		case <-timerC:
			timer = nil
			timerC = nil
			d.action()

		case <-ctx.Done():
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			return

		case <-d.stop:
			if timer != nil {
				if !timer.Stop() {
					// Timer already fired, drain the channel
					select {
					case <-timer.C:
					default:
					}
				}
			}
			return
		}
	}
}
