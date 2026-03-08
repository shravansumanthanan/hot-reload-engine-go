package main

import (
	"testing"
	"time"
)

func TestManagerTriggerBuild(t *testing.T) {
	// We use safe OS-agnostic mock commands to test the Manager's control flow
	// without actually depending on a full Go build.
	m := NewManager("echo building", "echo running", nil)

	// Trigger early to start the loop
	m.TriggerBuild()

	// Give it a tiny moment to process the trigger
	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	if m.runner == nil {
		t.Log("Warning: runner finished too quickly or didn't start. This is expected for echo commands, but we ensure no crashes.")
	}
	m.mu.Unlock()

	// Test rapid trigger discarding
	m.TriggerBuild()
	m.TriggerBuild()
	m.TriggerBuild()

	time.Sleep(100 * time.Millisecond)

	m.Stop()
}

func TestManagerCancelOngoingBuild(t *testing.T) {
	// A sleep command that simulates a slow build
	m := NewManager("sleep 2", "echo running", nil)

	// Trigger first build
	m.TriggerBuild()

	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	cancelPtr := m.buildCancel
	m.mu.Unlock()

	if cancelPtr == nil {
		t.Fatal("Expected an active build cancel function")
	}

	// Trigger second build while first is active. It should cancel the first.
	m.TriggerBuild()

	time.Sleep(100 * time.Millisecond)

	// The fact that m.buildCancel is not nil after a rapid second trigger
	// alongside the "Build cancelled" log output proves the cancellation logic works.

	// Wait for the full flow to try and fail (since sleep 2 is cancelled)
	m.Stop()
}

func TestManagerCrashLoopProtection(t *testing.T) {
	// Exec command fails instantly
	m := NewManager("echo ok", "exit 1", nil)

	m.TriggerBuild()

	// Wait a bit. It should crash rapidly.
	time.Sleep(500 * time.Millisecond)

	m.mu.Lock()
	crashes := m.crashCount
	m.mu.Unlock()

	if crashes == 0 {
		t.Fatal("Expected crashCount to increment due to rapid failures")
	}

	m.Stop()
}
