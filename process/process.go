package process

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

	// buildKillTimeout is how long to wait for a cancelled build process
	// to exit after receiving SIGKILL.
	buildKillTimeout = 1 * time.Second
)

// ANSI escape codes used to colorize build and app output prefixes.
// Using raw codes keeps the process package dependency-free.
const (
	ansiReset    = "\033[0m"
	ansiBold     = "\033[1m"
	ansiBoldCyan = "\033[1;36m" // [build] prefix
	ansiBoldGreen = "\033[1;32m" // [app]   prefix
	ansiDim      = "\033[2m"
)

// buildPrefix / appPrefix are the colorized line prefixes prepended to
// child-process stdout/stderr so they are visually distinct from hotreload's
// own structured log lines.
const (
	buildPrefix = ansiBoldCyan + "[build]" + ansiReset + ansiDim + " "
	appPrefix   = ansiBoldGreen + "[app]" + ansiReset + ansiDim + "   "
)

// prefixWriter wraps an io.Writer and prepends a fixed prefix to every line.
// This keeps build and app output visually distinct from hotreload's own logs.
type prefixWriter struct {
	prefix []byte
	w      io.Writer
	buf    bytes.Buffer
	mu     sync.Mutex
}

func newPrefixWriter(prefix string, w io.Writer) *prefixWriter {
	return &prefixWriter{prefix: []byte(prefix), w: w}
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.buf.Write(p)
	for {
		line, after, found := bytes.Cut(pw.buf.Bytes(), []byte("\n"))
		if !found {
			break
		}
		_, err = fmt.Fprintf(pw.w, "%s%s\n", pw.prefix, line)
		if err != nil {
			return 0, err
		}
		pw.buf.Reset()
		pw.buf.Write(after)
	}
	return len(p), nil
}

// Flush writes any remaining buffered bytes (no trailing newline) to the
// underlying writer so output is never silently dropped.
func (pw *prefixWriter) Flush() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.buf.Len() > 0 {
		_, _ = fmt.Fprintf(pw.w, "%s%s\n", pw.prefix, pw.buf.Bytes())
		pw.buf.Reset()
	}
}

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

	pw := newPrefixWriter(appPrefix+ansiReset, os.Stdout)
	r.cmd.Stdout = pw
	r.cmd.Stderr = pw

	setSysProcAttr(r.cmd)

	slog.Info("Starting process", "cmd", r.cmdStr)
	err := r.cmd.Start()
	if err != nil {
		slog.Error("Failed to start process", "err", err)
		return err
	}

	// Structured audit entry — useful for correlating logs when multiple
	// processes are started across the lifecycle of hotreload.
	slog.Info("Process started", "cmd", r.cmdStr, "pid", r.cmd.Process.Pid)

	go func() {
		r.err = r.cmd.Wait()
		pw.Flush()
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

	pw := newPrefixWriter(buildPrefix+ansiReset, os.Stdout)
	cmd.Stdout = pw
	cmd.Stderr = pw

	setSysProcAttr(cmd)

	err := cmd.Start()
	if err != nil {
		slog.Error("Failed to start build", "err", err)
		return err
	}

	done := make(chan error, 1) // Buffered so goroutine won't block forever
	go func() {
		done <- cmd.Wait()
		pw.Flush()
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
		case <-time.After(buildKillTimeout):
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
