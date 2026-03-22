package main

import (
	"os"
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

	cfg.MergeWithFlags(&rootFlag, &buildFlag, &execFlag, &extFlag, &ignoreFlag, &proxyFlag, &logLevelFlag)

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

	cfg2.MergeWithFlags(&rootFlag2, &buildFlag2, &execFlag2, &extFlag2, &ignoreFlag2, &proxyFlag2, &logLevelFlag2)

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
	if !contains(content, "root:") {
		t.Error("Example config missing 'root' field")
	}
	if !contains(content, "build:") {
		t.Error("Example config missing 'build' field")
	}
	if !contains(content, "exec:") {
		t.Error("Example config missing 'exec' field")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
