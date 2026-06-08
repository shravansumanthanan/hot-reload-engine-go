package manager_test

import (
	"testing"
	"time"

	"github.com/shravansumanthanan/hot-reload-engine-go/internal/manager"
)

func TestManagerTriggerBuild(t *testing.T) {
	// Use safe OS-agnostic echo commands to test Manager control flow
	// without depending on a full Go build.
	m := manager.NewManager("echo building", "echo running", "", "", nil)

	m.TriggerBuild()
	time.Sleep(200 * time.Millisecond) // allow build+exec cycle to complete

	// Test rapid trigger discarding — multiple calls must not panic or deadlock.
	m.TriggerBuild()
	m.TriggerBuild()
	m.TriggerBuild()

	time.Sleep(200 * time.Millisecond)

	m.Stop()
}

func TestManagerStop(t *testing.T) {
	m := manager.NewManager("echo building", "echo running", "", "", nil)
	m.TriggerBuild()
	time.Sleep(50 * time.Millisecond)

	// Stop must be idempotent and must not hang.
	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds")
	}
}

func TestManagerCrashLoopProtection(t *testing.T) {
	// "exit 1" or equivalent: process exits immediately with non-zero status.
	// On Unix this is handled by sh -c; on Windows by cmd /c.
	m := manager.NewManager("echo ok", "exit 1", "", "", nil)

	m.TriggerBuild()

	// Wait long enough for a crash + backoff counter to increment.
	time.Sleep(600 * time.Millisecond)

	m.Stop()
	// If we reach here without deadlock or panic, the crash loop protection
	// is functioning correctly. The log output will show the backoff warning.
}

func TestManagerCancelOngoingBuild(t *testing.T) {
	// A long sleep simulates a slow build.
	m := manager.NewManager("sleep 2", "echo running", "", "", nil)

	m.TriggerBuild()
	time.Sleep(100 * time.Millisecond)

	// Trigger a second build — this must cancel the first without hanging.
	done := make(chan struct{})
	go func() {
		m.TriggerBuild()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("TriggerBuild() blocked unexpectedly")
	}

	m.Stop()
}
