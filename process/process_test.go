package process

import (
	"context"
	"testing"
	"time"
)

func TestBuildCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Start a build that takes 5 seconds
	go func() {
		// Cancel it almost immediately
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := Build(ctx, "sleep 5")
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected an error from a cancelled build, got nil")
	}

	if duration > 2*time.Second {
		t.Fatalf("Build took %v, meaning it was not cancelled promptly", duration)
	}
}

func TestRunnerStartStop(t *testing.T) {
	runner := NewRunner("sleep 10")

	err := runner.Run()
	if err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give it time to actually spawn
	time.Sleep(100 * time.Millisecond)

	// Stop it
	runner.Stop()

	// If Wait doesn't hang, it means it died
	done := make(chan struct{})
	go func() {
		runner.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Runner failed to stop promptly")
	}
}
