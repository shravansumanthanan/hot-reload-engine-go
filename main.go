package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/shravansumanthanan/hot-reload-engine-go/debouncer"
	"github.com/shravansumanthanan/hot-reload-engine-go/internal/logger"
	"github.com/shravansumanthanan/hot-reload-engine-go/internal/manager"
	"github.com/shravansumanthanan/hot-reload-engine-go/proxy"
	"github.com/shravansumanthanan/hot-reload-engine-go/watcher"
)

// version is injected at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

type configFlags struct {
	root         *string
	buildCommand *string
	execCommand  *string
	extFlag      *string
	ignoreFlag   *string
	proxyFlag    *string
	logLevel     *string
	configPath   *string
	initConfig   *bool
	showVersion  *bool
}

func parseFlags() (configFlags, map[string]bool) {
	flags := configFlags{
		root:         flag.String("root", defaultRootPath, "Project root directory to watch"),
		buildCommand: flag.String("build", "", "Command to build the project"),
		execCommand:  flag.String("exec", "", "Command to execute the built binary"),
		extFlag:      flag.String("ext", defaultWatchExtensions, "Comma-separated list of file extensions to watch"),
		ignoreFlag:   flag.String("ignore", "", "Comma-separated list of directories to ignore"),
		proxyFlag:    flag.String("proxy", "", "Enable live-reload proxy. Format: <listen_port>:<target_port> (e.g. 8080:8081)"),
		logLevel:     flag.String("log-level", "debug", "Log level: debug, info, warn, error"),
		configPath:   flag.String("config", ".hotreload.yaml", "Path to configuration file"),
		initConfig:   flag.Bool("init", false, "Generate example .hotreload.yaml configuration file"),
		showVersion:  flag.Bool("version", false, "Print version and exit"),
	}

	flag.Parse()

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	return flags, explicitFlags
}

func setupConfigAndFlags() (configFlags, *Config) {
	flags, explicitFlags := parseFlags()

	// Load .env file automatically (silently ignore if not found)
	_ = godotenv.Load()

	// Handle --version flag
	if *flags.showVersion {
		fmt.Printf("hotreload version %s\n", version)
		os.Exit(0)
	}

	// Handle --init flag to generate example config
	if *flags.initConfig {
		if err := WriteExampleConfig(*flags.configPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created example configuration file: %s\n", *flags.configPath)
		fmt.Println("Edit this file and run hotreload again.")
		os.Exit(0)
	}

	cfg, err := LoadConfig(*flags.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
		os.Exit(1)
	}

	if cfg != nil {
		cfg.MergeWithFlags(flags.root, flags.buildCommand, flags.execCommand, flags.extFlag, flags.ignoreFlag, flags.proxyFlag, flags.logLevel, explicitFlags)

		if runtime.GOOS == "windows" && cfg.BuildWindows != "" && !explicitFlags["build"] {
			*flags.buildCommand = cfg.BuildWindows
		}

		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			os.Exit(1)
		}
	}

	if *flags.buildCommand == "" || *flags.execCommand == "" {
		fmt.Fprintln(os.Stderr, "Usage: hotreload --root <path> --build <build_cmd> --exec <exec_cmd>")
		fmt.Fprintln(os.Stderr, "   or: hotreload --init  (to generate example config file)")
		flag.PrintDefaults()
		os.Exit(1)
	}

	return flags, cfg
}

func main() {
	flags, cfg := setupConfigAndFlags()

	slog.SetDefault(slog.New(logger.NewDefault(parseLogLevel(*flags.logLevel))))
	slog.Info("Starting hotreload", "root", *flags.root, "build", *flags.buildCommand, "exec", *flags.execCommand)

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
	exts := strings.Split(*flags.extFlag, ",")
	ignores := strings.Split(*flags.ignoreFlag, ",")
	w, err := watcher.New(*flags.root, exts, ignores)
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
	liveProxy, err := setupProxy(*flags.proxyFlag, cfg)
	if err != nil {
		slog.Error("Failed to initialize proxy", "err", err, "value", *flags.proxyFlag)
		os.Exit(1)
	}
	defer runProxy(liveProxy)()

	// Manager handles build/exec coordination.
	preBuild := ""
	postBuild := ""
	if cfg != nil {
		preBuild = cfg.PreBuild
		postBuild = cfg.PostBuild
	}
	// Guard against the Go typed-nil pitfall: a (*proxy.Proxy)(nil) assigned to
	// a LiveReloader interface is NOT a nil interface — the nil check inside
	// manager would pass but the method call would still panic. Pass an explicit
	// untyped nil when no proxy is configured so the interface value is truly nil.
	var liveReloader manager.LiveReloader
	if liveProxy != nil {
		liveReloader = liveProxy
	}
	m := manager.NewManager(*flags.buildCommand, *flags.execCommand, preBuild, postBuild, liveReloader)
	defer m.Stop()

	// Setup Debouncer for file events before triggering the initial build.
	// This ensures no file-change events are missed during the first build.
	db := debouncer.New(ctx, defaultDebounceDelay, func() {
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

func parseLogLevel(logLevel string) slog.Level {
	switch strings.ToLower(logLevel) {
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func setupProxy(proxyFlag string, cfg *Config) (*proxy.Proxy, error) {
	if proxyFlag == "" {
		return nil, nil
	}
	parts := strings.SplitN(proxyFlag, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid proxy format, expected <listen_port>:<target_port>")
	}
	address := ":" + parts[0]
	targetAddr := "http://127.0.0.1:" + parts[1]

	healthCheckURL := ""
	if cfg != nil {
		healthCheckURL = cfg.HealthCheck
	}

	return proxy.New(address, targetAddr, healthCheckURL)
}

func runProxy(liveProxy *proxy.Proxy) func() {
	if liveProxy == nil {
		return func() {}
	}
	go func() {
		if err := liveProxy.Start(); err != nil {
			slog.Error("Proxy server stopped", "err", err)
		}
	}()
	return func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		if err := liveProxy.Shutdown(shutCtx); err != nil {
			slog.Warn("Proxy shutdown error", "err", err)
		}
	}
}
