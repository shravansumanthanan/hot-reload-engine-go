package main

import "context"

// ProcessRunner is the interface for managing a long-running child process.
// Using an interface here decouples the Manager from the concrete *process.Runner
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
