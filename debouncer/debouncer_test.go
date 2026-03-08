package debouncer

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer(t *testing.T) {
	var count int32

	d := New(50*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
	})
	defer d.Stop()

	// Rapidly trigger 5 times
	for i := 0; i < 5; i++ {
		d.Trigger()
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce period
	time.Sleep(100 * time.Millisecond)

	finalCount := atomic.LoadInt32(&count)
	if finalCount != 1 {
		t.Errorf("Expected action to be called 1 time, but was %d", finalCount)
	}
}

func TestDebouncerMultiple(t *testing.T) {
	var count int32

	d := New(50*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
	})
	defer d.Stop()

	// Trigger 1st batch
	d.Trigger()
	time.Sleep(100 * time.Millisecond)

	// Trigger 2nd batch
	d.Trigger()
	time.Sleep(100 * time.Millisecond)

	finalCount := atomic.LoadInt32(&count)
	if finalCount != 2 {
		t.Errorf("Expected action to be called 2 times, but was %d", finalCount)
	}
}
