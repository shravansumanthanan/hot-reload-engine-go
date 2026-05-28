package main

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test loading non-existent file (should return nil, nil)
	cfg, err := LoadConfig("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}
	if cfg != nil {
		t.Fatal("Expected nil config for missing file")
	}

	// Create a temporary config file
	tmpFile := "test_config.yaml"
	defer os.Remove(tmpFile)

	content := `root: ./testdir
build: "make build"
exec: "./bin/app"
extensions:
  - .go
  - .mod
ignore:
  - vendor
  - tmp
proxy: "3000:3001"
log_level: info
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load the config
	cfg, err = LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify values
	if cfg.Root != "./testdir" {
		t.Errorf("Expected root './testdir', got '%s'", cfg.Root)
	}
	if cfg.Build != "make build" {
		t.Errorf("Expected build 'make build', got '%s'", cfg.Build)
	}
	if cfg.Exec != "./bin/app" {
		t.Errorf("Expected exec './bin/app', got '%s'", cfg.Exec)
	}
	if len(cfg.Extensions) != 2 {
		t.Errorf("Expected 2 extensions, got %d", len(cfg.Extensions))
	}
	if len(cfg.Ignore) != 2 {
		t.Errorf("Expected 2 ignore patterns, got %d", len(cfg.Ignore))
	}
	if cfg.Proxy != "3000:3001" {
		t.Errorf("Expected proxy '3000:3001', got '%s'", cfg.Proxy)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected log_level 'info', got '%s'", cfg.LogLevel)
	}
}

func TestMergeWithFlags(t *testing.T) {
	cfg := &Config{
		Root:       "./config-root",
		Build:      "config-build",
		Exec:       "config-exec",
		Extensions: []string{".go", ".mod"},
		Ignore:     []string{"vendor"},
		Proxy:      "8080:8081",
		LogLevel:   "info",
	}

	// Flags with default values (should use config)
	rootFlag := defaultRootPath
	buildFlag := ""
	execFlag := ""
	extFlag := defaultWatchExtensions
	ignoreFlag := ""
	proxyFlag := ""
	logLevelFlag := "debug"

	// No CLI flags were explicitly set — config file values should win.
	explicitFlags := map[string]bool{}
	cfg.MergeWithFlags(&rootFlag, &buildFlag, &execFlag, &extFlag, &ignoreFlag, &proxyFlag, &logLevelFlag, explicitFlags)

	if rootFlag != "./config-root" {
		t.Errorf("Expected root from config, got '%s'", rootFlag)
	}
	if buildFlag != "config-build" {
		t.Errorf("Expected build from config, got '%s'", buildFlag)
	}
	if execFlag != "config-exec" {
		t.Errorf("Expected exec from config, got '%s'", execFlag)
	}
	if extFlag != ".go,.mod" {
		t.Errorf("Expected extensions from config, got '%s'", extFlag)
	}
	if ignoreFlag != "vendor" {
		t.Errorf("Expected ignore from config, got '%s'", ignoreFlag)
	}

	// Test CLI flags override config
	cfg2 := &Config{
		Root:  "./config-root",
		Build: "config-build",
		Exec:  "config-exec",
	}

	rootFlag2 := "./cli-root"
	buildFlag2 := "cli-build"
	execFlag2 := "cli-exec"
	extFlag2 := defaultWatchExtensions
	ignoreFlag2 := ""
	proxyFlag2 := ""
	logLevelFlag2 := "debug"

	// root, build, exec were explicitly set from CLI — they should override config.
	explicitFlags2 := map[string]bool{"root": true, "build": true, "exec": true}
	cfg2.MergeWithFlags(&rootFlag2, &buildFlag2, &execFlag2, &extFlag2, &ignoreFlag2, &proxyFlag2, &logLevelFlag2, explicitFlags2)

	if rootFlag2 != "./cli-root" {
		t.Errorf("Expected CLI root to override config, got '%s'", rootFlag2)
	}
	if buildFlag2 != "cli-build" {
		t.Errorf("Expected CLI build to override config, got '%s'", buildFlag2)
	}
	if execFlag2 != "cli-exec" {
		t.Errorf("Expected CLI exec to override config, got '%s'", execFlag2)
	}
}

func TestWriteExampleConfig(t *testing.T) {
	tmpFile := "test_example.yaml"
	defer os.Remove(tmpFile)

	if err := WriteExampleConfig(tmpFile); err != nil {
		t.Fatalf("Failed to write example config: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read example config: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Fatal("Example config is empty")
	}

	// Should contain key sections
	if !strings.Contains(content, "root:") {
		t.Error("Example config missing 'root' field")
	}
	if !strings.Contains(content, "build:") {
		t.Error("Example config missing 'build' field")
	}
	if !strings.Contains(content, "exec:") {
		t.Error("Example config missing 'exec' field")
	}
}

func TestConfigValidate(t *testing.T) {
	t.Run("valid empty config passes", func(t *testing.T) {
		cfg := &Config{}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Expected no error for empty config, got: %v", err)
		}
	})

	t.Run("valid proxy format passes", func(t *testing.T) {
		cfg := &Config{Proxy: "8080:8081"}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("invalid proxy format fails", func(t *testing.T) {
		for _, bad := range []string{"8080", "abc:def", "8080:8081:extra", ":8081"} {
			cfg := &Config{Proxy: bad}
			if err := cfg.Validate(); err == nil {
				t.Errorf("Expected error for proxy %q, got nil", bad)
			}
		}
	})

	t.Run("valid log levels pass", func(t *testing.T) {
		for _, level := range []string{"debug", "info", "warn", "error"} {
			cfg := &Config{LogLevel: level}
			if err := cfg.Validate(); err != nil {
				t.Errorf("Unexpected error for log level %q: %v", level, err)
			}
		}
	})

	t.Run("invalid log level fails", func(t *testing.T) {
		cfg := &Config{LogLevel: "verbose"}
		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for invalid log level, got nil")
		}
	})

	t.Run("valid health_check URL passes", func(t *testing.T) {
		cfg := &Config{HealthCheck: "http://localhost:8081/healthz"}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("invalid health_check URL fails", func(t *testing.T) {
		cfg := &Config{HealthCheck: "localhost:8081/healthz"}
		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for health_check without scheme, got nil")
		}
	})

	t.Run("non-existent root fails", func(t *testing.T) {
		cfg := &Config{Root: "/tmp/hotreload-nonexistent-dir-xyz"}
		if err := cfg.Validate(); err == nil {
			t.Error("Expected error for non-existent root, got nil")
		}
	})

	t.Run("existing root passes", func(t *testing.T) {
		cfg := &Config{Root: t.TempDir()}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Unexpected error for existing root: %v", err)
		}
	})
}
