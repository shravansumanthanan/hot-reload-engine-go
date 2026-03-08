package debouncer

import (
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

// New creates a new Debouncer.
func New(duration time.Duration, action func()) *Debouncer {
	d := &Debouncer{
		duration: duration,
		events:   make(chan struct{}, 1),
		action:   action,
		stop:     make(chan struct{}),
	}
	go d.run()
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

func (d *Debouncer) run() {
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-d.events:
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(d.duration)
			timerC = timer.C

		case <-timerC:
			timer = nil
			timerC = nil
			d.action()

		case <-d.stop:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
