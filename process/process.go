package process

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	// gracefulShutdownTimeout is how long to wait for a process to exit
	// before sending SIGKILL.
	gracefulShutdownTimeout = 500 * time.Millisecond
)

// Runner manages a long-running process like a server.
type Runner struct {
	cmdStr string
	cmd    *exec.Cmd
	exited chan struct{}
	err    error
	mu     sync.Mutex
}

// NewRunner creates a new Runner.
func NewRunner(cmdStr string) *Runner {
	return &Runner{
		cmdStr: cmdStr,
		exited: make(chan struct{}),
	}
}

// Run executes the command using the OS-specific shell.
func (r *Runner) Run() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cmd = getShellCmd(r.cmdStr)
	r.cmd.Stdout = os.Stdout
	r.cmd.Stderr = os.Stderr

	setSysProcAttr(r.cmd)

	slog.Info("Starting process", "cmd", r.cmdStr)
	err := r.cmd.Start()
	if err != nil {
		slog.Error("Failed to start process", "err", err)
		return err
	}

	go func() {
		r.err = r.cmd.Wait()
		close(r.exited)
	}()

	return nil
}

// Wait waits for the process to exit.
func (r *Runner) Wait() error {
	r.mu.Lock()
	cmd := r.cmd
	exited := r.exited
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	<-exited
	return r.err
}

// Stop gracefully shuts down the process, falling back to forceful kill.
func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	exited := r.exited
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	err := terminateProcess(cmd)
	if err != nil {
		slog.Error("Failed to terminate cleanly", "err", err)
		_ = killProcess(cmd)
		return
	}

	// Wait for process to exit gracefully
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	select {
	case <-ctx.Done():
		// Timeout reached, forceful kill
		slog.Warn("Process did not exit gracefully, sending KILL")
		_ = killProcess(cmd)
	case <-exited:
		// Exited gracefully
		slog.Debug("Process exited gracefully")
	}

	r.mu.Lock()
	r.cmd = nil
	r.mu.Unlock()
}

// Build executes a short-lived build command and waits for it to finish.
func Build(ctx context.Context, cmdStr string) error {
	slog.Info("Running build", "cmd", cmdStr)
	cmd := getShellCmdContext(ctx, cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	setSysProcAttr(cmd)

	err := cmd.Start()
	if err != nil {
		slog.Error("Failed to start build", "err", err)
		return err
	}

	done := make(chan error, 1) // Buffered so goroutine won't block forever
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Context cancelled, kill the build process group
		if cmd.Process != nil {
			_ = killProcess(cmd)
		}
		// Wait for the process to actually exit with a timeout
		select {
		case <-done:
			// Process exited
		case <-time.After(1 * time.Second):
			// Timeout waiting for process to die
		}
		err = ctx.Err()
	case err = <-done:
		// Process finished
	}

	if err != nil {
		if ctx.Err() != nil {
			slog.Warn("Build cancelled")
			return ctx.Err()
		}
		slog.Error("Build failed", "err", err)
	} else {
		slog.Info("Build succeeded")
	}
	return err
}
