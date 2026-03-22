package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"hotreload/process"
	"hotreload/proxy"
)

// Manager coordinates the build and run cycles. It serializes rebuild
// requests through a single-element channel so that rapid file changes
// are coalesced and only the latest state is built.
type Manager struct {
	buildCmd  string
	execCmd   string
	liveProxy *proxy.Proxy

	buildCancel context.CancelFunc
	runner      *process.Runner

	mu        sync.Mutex
	triggerCh chan struct{}
	stopCh    chan struct{}

	crashCount int
	lastStart  time.Time
}

// NewManager creates a Manager and starts its background loop.
func NewManager(buildCmd, execCmd string, liveProxy *proxy.Proxy) *Manager {
	m := &Manager{
		buildCmd:  buildCmd,
		execCmd:   execCmd,
		liveProxy: liveProxy,
		triggerCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
	go m.loop()
	return m
}

// TriggerBuild cancels any in-progress build and schedules a new
// build+run cycle. Multiple rapid calls are coalesced.
func (m *Manager) TriggerBuild() {
	m.mu.Lock()
	if m.buildCancel != nil {
		m.buildCancel()
	}
	m.mu.Unlock()

	select {
	case m.triggerCh <- struct{}{}:
	default:
	}
}

// Stop terminates any in-progress build and running process.
func (m *Manager) Stop() {
	close(m.stopCh)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buildCancel != nil {
		m.buildCancel()
		m.buildCancel = nil
	}
	if m.runner != nil {
		m.runner.Stop()
		m.runner = nil
	}
}

func (m *Manager) loop() {
	for {
		select {
		case <-m.triggerCh:
			m.runCycle()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) runCycle() {
	m.mu.Lock()

	// Create context for this build, cancelling any previous build.
	if m.buildCancel != nil {
		m.buildCancel()
	}
	buildCtx, cancelBuild := context.WithCancel(context.Background())
	m.buildCancel = cancelBuild

	// Stop the running process before rebuilding.
	if m.runner != nil {
		slog.Info("Stopping running process for rebuild")
		m.runner.Stop()
		m.runner = nil
	}
	m.mu.Unlock()

	// Run build — blocks but is cancellable via context.
	err := process.Build(buildCtx, m.buildCmd)
	
	m.mu.Lock()
	// Clear the build cancel function after build completes or fails
	m.buildCancel = nil
	m.mu.Unlock()

	if err != nil {
		if buildCtx.Err() != nil {
			return // Build was cancelled by a newer file change.
		}
		return // Build failed legitimately; wait for next trigger.
	}

	m.mu.Lock()
	if buildCtx.Err() != nil {
		m.mu.Unlock()
		return // Cancelled right after build finished.
	}

	// Start the new server process.
	m.runner = process.NewRunner(m.execCmd)
	err = m.runner.Run()
	if err != nil {
		slog.Error("Failed to start server", "err", err)
		m.runner = nil
		m.mu.Unlock()
		return
	}

	// Notify live-reload proxy clients after a short delay to let the
	// server finish binding its port.
	if m.liveProxy != nil {
		go func() {
			time.Sleep(defaultReloadBroadcastDelay)
			m.liveProxy.BroadcastReload()
		}()
	}

	runnerRef := m.runner
	lastStart := time.Now()
	m.mu.Unlock()

	// Monitor the process for unexpected exits (crashes).
	go func(runner *process.Runner, startTime time.Time) {
		_ = runner.Wait()

		m.mu.Lock()
		defer m.mu.Unlock()

		// If this is still the active runner, it crashed or exited on its own.
		// If m.runner != runner, it was stopped intentionally by runCycle.
		if m.runner == runner {
			m.runner = nil
			slog.Warn("Process exited unexpectedly")

			// Crash loop protection: if the process dies very quickly,
			// back off before retrying to avoid a tight restart loop.
			duration := time.Since(startTime)
			if duration < defaultCrashThreshold {
				m.crashCount++
				backoff := time.Duration(m.crashCount) * time.Second
				if backoff > defaultMaxBackoff {
					backoff = defaultMaxBackoff
				}
				slog.Warn("Rapid crash detected, backing off", "wait", backoff, "crashes", m.crashCount)

				go func(wait time.Duration) {
					time.Sleep(wait)
					select {
					case m.triggerCh <- struct{}{}:
					default:
					}
				}(backoff)
			} else {
				m.crashCount = 0
			}
		}
	}(runnerRef, lastStart)
}
