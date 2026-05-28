# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `LICENSE` file (MIT).
- `CONTRIBUTING.md`, `SECURITY.md`, and PR template.
- CI pipeline: GitHub Actions matrix (Linux/macOS/Windows × Go 1.22–1.24) with race detector.
- Release pipeline: GoReleaser cross-platform binary publishing on git tag.
- `.goreleaser.yaml` for automated cross-platform releases.
- `--version` flag with build-time version injection via `ldflags`.
- `Makefile` targets: `test`, `test-race`, `lint`, `vet`, `fmt`, `version`, `help`.
- `interfaces.go`: `ProcessRunner` and `LiveReloader` interfaces for testable Manager design.

### Changed
- **go.mod**: Module path updated from `hotreload` to `github.com/shravansumanthanan/hot-reload-engine-go` (enables `go install` from source).
- **proxy**: Added `Shutdown(ctx)` method for graceful HTTP server teardown on SIGTERM.
- **proxy**: SSE `Access-Control-Allow-Origin` restricted from `*` to the proxy's own localhost origin.
- **proxy**: `http.Server` now stores `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` fields.
- **proxy**: `chan bool` client map replaced with idiomatic `chan struct{}`.
- **proxy**: `WaitForTarget(ctx)` port-readiness check replaces hardcoded 300ms sleep before `BroadcastReload`.
- **manager**: `NewManager` accepts `LiveReloader` interface instead of concrete `*proxy.Proxy`.
- **manager**: Crash-monitor goroutine now checks `stopCh` before re-triggering to prevent spurious rebuilds after shutdown.
- **manager**: Crash backoff changed from linear (`n * 1s`) to exponential with jitter (`2^(n-1) * 500ms ± 500ms`), capped at `defaultMaxBackoff`.
- **process**: Added `buildKillTimeout` named constant (was inline `1 * time.Second`).
- **process**: Added structured audit log with PID on every process start.
- **main**: Proxy error is now fatal (exit 1) instead of silently continuing without a proxy.
- **main**: Proxy `Shutdown` is wired into main's defer chain.

### Fixed
- Proxy goroutine was never stopped on SIGTERM (leaked HTTP server + SSE connections).
- Crash-monitor goroutine could fire after `Manager.Stop()` causing a spurious rebuild.

---

## [0.1.0] — 2026-05-27 *(initial tagged release)*

### Added
- Initial implementation of hotreload CLI.
- File watcher using `fsnotify` with recursive directory support.
- Debouncer to coalesce rapid file-change events (100ms window).
- Build and exec command coordination with context cancellation.
- Reverse proxy with SSE-based live-reload injection.
- Crash-loop detection with backoff.
- Cross-platform support: Unix (Setpgid + SIGKILL) and Windows (taskkill).
- YAML configuration via `.hotreload.yaml`.
- `--init` flag to scaffold example config.
