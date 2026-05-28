package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shravansumanthanan/hot-reload-engine-go/debouncer"
	"github.com/shravansumanthanan/hot-reload-engine-go/internal/manager"
	"github.com/shravansumanthanan/hot-reload-engine-go/proxy"
	"github.com/shravansumanthanan/hot-reload-engine-go/watcher"
)

// version is injected at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

func main() {
	rootPath := flag.String("root", defaultRootPath, "Project root directory to watch")
	buildCommand := flag.String("build", "", "Command to build the project")
	execCommand := flag.String("exec", "", "Command to execute the built binary")
	extFlag := flag.String("ext", defaultWatchExtensions, "Comma-separated list of file extensions to watch")
	ignoreFlag := flag.String("ignore", "", "Comma-separated list of directories to ignore")
	proxyFlag := flag.String("proxy", "", "Enable live-reload proxy. Format: <listen_port>:<target_port> (e.g. 8080:8081)")
	logLevel := flag.String("log-level", "debug", "Log level: debug, info, warn, error")
	configPath := flag.String("config", ".hotreload.yaml", "Path to configuration file")
	initConfig := flag.Bool("init", false, "Generate example .hotreload.yaml configuration file")
	showVersion := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	// Collect the set of flags that were explicitly set by the user.
	// This is the correct way to distinguish "user passed --root ." from
	// "user didn't pass --root at all and it stayed at the default".
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// Handle --version flag
	if *showVersion {
		fmt.Printf("hotreload version %s\n", version)
		return
	}

	// Handle --init flag to generate example config
	if *initConfig {
		if err := WriteExampleConfig(*configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created example configuration file: %s\n", *configPath)
		fmt.Println("Edit this file and run hotreload again.")
		return
	}

	// Load configuration file if it exists.
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
		os.Exit(1)
	}

	// Merge config file with CLI flags (CLI flags take precedence).
	if cfg != nil {
		cfg.MergeWithFlags(rootPath, buildCommand, execCommand, extFlag, ignoreFlag, proxyFlag, logLevel, explicitFlags)
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
	}

	if *buildCommand == "" || *execCommand == "" {
		fmt.Fprintln(os.Stderr, "Usage: hotreload --root <path> --build <build_cmd> --exec <exec_cmd>")
		fmt.Fprintln(os.Stderr, "   or: hotreload --init  (to generate example config file)")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var level slog.Level
	switch strings.ToLower(*logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	slog.Info("Starting hotreload", "root", *rootPath, "build", *buildCommand, "exec", *execCommand)

	// Context for graceful shutdown of hotreload itself.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		slog.Info("Received interrupt, shutting down...")
		cancel()
	}()

	// Start File Watcher.
	exts := strings.Split(*extFlag, ",")
	ignores := strings.Split(*ignoreFlag, ",")
	w, err := watcher.New(*rootPath, exts, ignores)
	if err != nil {
		slog.Error("Failed to initialize watcher", "err", err)
		os.Exit(1)
	}
	if err := w.Start(); err != nil {
		slog.Error("Failed to start watcher", "err", err)
		os.Exit(1)
	}
	defer w.Close()

	// Start optional live-reload proxy.
	var liveProxy *proxy.Proxy
	if *proxyFlag != "" {
		parts := strings.SplitN(*proxyFlag, ":", 2)
		if len(parts) != 2 {
			slog.Error("Invalid proxy format, expected <listen_port>:<target_port>", "value", *proxyFlag)
			os.Exit(1)
		}
		address := ":" + parts[0]
		targetAddr := "http://127.0.0.1:" + parts[1]

		// Use health_check URL from config if provided, otherwise TCP polling.
		healthCheckURL := ""
		if cfg != nil {
			healthCheckURL = cfg.HealthCheck
		}

		var proxyErr error
		liveProxy, proxyErr = proxy.New(address, targetAddr, healthCheckURL)
		if proxyErr != nil {
			slog.Error("Failed to initialize proxy", "err", proxyErr)
			os.Exit(1)
		}
		go func() {
			if err := liveProxy.Start(); err != nil {
				slog.Error("Proxy server stopped", "err", err)
			}
		}()
		defer func() {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer shutCancel()
			if err := liveProxy.Shutdown(shutCtx); err != nil {
				slog.Warn("Proxy shutdown error", "err", err)
			}
		}()
	}

	// Manager handles build/exec coordination.
	m := manager.NewManager(*buildCommand, *execCommand, liveProxy)
	defer m.Stop()

	// Setup Debouncer for file events before triggering the initial build.
	// This ensures no file-change events are missed during the first build.
	db := debouncer.New(defaultDebounceDelay, func() {
		slog.Info("Changes detected, scheduling rebuild")
		m.TriggerBuild()
	})
	defer db.Stop()

	// Trigger the initial build. The manager loop runs in a background
	// goroutine, so this returns immediately and the event loop below
	// starts processing file changes right away.
	slog.Info("Triggering initial build")
	m.TriggerBuild()

	// Event loop
	for {
		select {
		case event := <-w.Events:
			slog.Debug("File changed", "event", event)
			db.Trigger()
		case err := <-w.Errors:
			slog.Error("Watcher error", "err", err)
		case <-ctx.Done():
			return
		}
	}
}
