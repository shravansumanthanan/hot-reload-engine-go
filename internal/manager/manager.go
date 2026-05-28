// Package manager coordinates the build and run lifecycle of a watched project.
// It serialises rebuild requests through a single-element channel so that
// rapid file changes are coalesced — only the latest state is ever built.
package manager

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/shravansumanthanan/hot-reload-engine-go/process"
)

// ProcessRunner is the interface for managing a long-running child process.
// Using an interface here decouples Manager from the concrete *process.Runner
// and allows mock implementations in tests.
type ProcessRunner interface {
	Run() error
	Stop()
	Wait() error
}

// LiveReloader is the interface for notifying browser clients to reload.
// Decouples Manager from the concrete *proxy.Proxy.
type LiveReloader interface {
	BroadcastReload()
	WaitForTarget(ctx context.Context) bool
	Shutdown(ctx context.Context) error
}

const (
	// defaultCrashThreshold is the minimum uptime required before a process
	// exit is NOT considered a crash.
	defaultCrashThreshold = 1 * time.Second

	// defaultMaxBackoff is the maximum delay between crash-loop retries.
	defaultMaxBackoff = 10 * time.Second
)

// Manager coordinates the build and run cycles. It serialises rebuild
// requests through a single-element channel so that rapid file changes
// are coalesced and only the latest state is built.
type Manager struct {
	buildCmd  string
	execCmd   string
	liveProxy LiveReloader

	buildCancel context.CancelFunc
	runner      ProcessRunner

	mu        sync.Mutex
	triggerCh chan struct{}
	stopCh    chan struct{}

	crashCount int
}

// NewManager creates a Manager and starts its background loop.
func NewManager(buildCmd, execCmd string, liveProxy LiveReloader) *Manager {
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
	// Clear the build cancel function after build completes or fails.
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
	runner := process.NewRunner(m.execCmd)
	err = runner.Run()
	if err != nil {
		slog.Error("Failed to start server", "err", err)
		m.mu.Unlock()
		return
	}
	m.runner = runner

	// Reset crash count on successful start.
	m.crashCount = 0

	// Notify live-reload proxy clients once the target port is ready.
	// We wait up to 10s for the server to bind its port rather than
	// using a fixed sleep, which prevents premature browser refreshes.
	if m.liveProxy != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if m.liveProxy.WaitForTarget(ctx) {
				m.liveProxy.BroadcastReload()
			} else {
				slog.Warn("Target did not become ready within 10s, skipping reload broadcast")
			}
		}()
	}

	runnerRef := runner
	lastStart := time.Now()
	m.mu.Unlock()

	// Monitor the process for unexpected exits (crashes).
	go func(r ProcessRunner, startTime time.Time) {
		_ = r.Wait()

		// Check if the manager has been stopped before doing anything.
		select {
		case <-m.stopCh:
			return
		default:
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// If this is still the active runner, it crashed or exited on its own.
		// If m.runner != r, it was stopped intentionally by runCycle.
		if m.runner == runnerRef {
			m.runner = nil
			slog.Warn("Process exited unexpectedly")

			// Crash loop protection: if the process dies very quickly,
			// apply exponential backoff with jitter before retrying.
			duration := time.Since(startTime)
			if duration < defaultCrashThreshold {
				m.crashCount++

				// Exponential backoff: 2^(crashCount-1) * 500ms, capped at max.
				base := time.Duration(1<<uint(m.crashCount-1)) * 500 * time.Millisecond
				if base > defaultMaxBackoff {
					base = defaultMaxBackoff
				}
				// Add jitter: up to 500ms of random delay.
				jitter := time.Duration(rand.Int63n(int64(500 * time.Millisecond)))
				backoff := base + jitter

				slog.Warn("Rapid crash detected, backing off",
					"wait", backoff.Round(time.Millisecond),
					"crashes", m.crashCount,
				)

				go func(wait time.Duration) {
					timer := time.NewTimer(wait)
					defer timer.Stop()
					select {
					case <-timer.C:
						select {
						case m.triggerCh <- struct{}{}:
						default:
						}
					case <-m.stopCh:
						// Manager stopped — do not re-trigger.
					}
				}(backoff)
			} else {
				m.crashCount = 0
			}
		}
	}(runnerRef, lastStart)
}
