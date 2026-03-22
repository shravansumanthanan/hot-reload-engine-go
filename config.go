package main

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the hotreload configuration file structure
type Config struct {
	Root       string   `yaml:"root"`
	Build      string   `yaml:"build"`
	Exec       string   `yaml:"exec"`
	Extensions []string `yaml:"extensions"`
	Ignore     []string `yaml:"ignore"`
	Proxy      string   `yaml:"proxy"`
	LogLevel   string   `yaml:"log_level"`
}

// LoadConfig attempts to load configuration from a .hotreload.yaml file
// Returns nil if the file doesn't exist (not an error)
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Config file is optional
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// MergeWithFlags merges config file values with CLI flags
// CLI flags take precedence over config file values
func (c *Config) MergeWithFlags(
	rootFlag, buildFlag, execFlag, extFlag, ignoreFlag, proxyFlag, logLevelFlag *string,
) {
	// CLI flags override config file
	if *rootFlag != defaultRootPath {
		c.Root = *rootFlag
	}
	if *buildFlag != "" {
		c.Build = *buildFlag
	}
	if *execFlag != "" {
		c.Exec = *execFlag
	}
	if *extFlag != defaultWatchExtensions {
		// CLI provided extensions
	} else if len(c.Extensions) > 0 {
		// Use config file extensions
		*extFlag = ""
		for i, ext := range c.Extensions {
			if i > 0 {
				*extFlag += ","
			}
			*extFlag += ext
		}
	}
	if *ignoreFlag != "" {
		// CLI provided ignore patterns
	} else if len(c.Ignore) > 0 {
		// Use config file ignore patterns
		*ignoreFlag = ""
		for i, ign := range c.Ignore {
			if i > 0 {
				*ignoreFlag += ","
			}
			*ignoreFlag += ign
		}
	}
	if *proxyFlag != "" {
		c.Proxy = *proxyFlag
	}
	if *logLevelFlag != "debug" {
		c.LogLevel = *logLevelFlag
	}

	// Apply config values back to flags
	if c.Root != "" {
		*rootFlag = c.Root
	}
	if c.Build != "" {
		*buildFlag = c.Build
	}
	if c.Exec != "" {
		*execFlag = c.Exec
	}
	if c.Proxy != "" {
		*proxyFlag = c.Proxy
	}
	if c.LogLevel != "" {
		*logLevelFlag = c.LogLevel
	}
}

// Example .hotreload.yaml file content
const exampleConfig = `# Hotreload Configuration File
# CLI flags take precedence over these values

# Project root directory to watch
root: .

# Command to build the project
build: "go build -o ./bin/server ."

# Command to execute the built binary
exec: "./bin/server"

# File extensions to watch (optional, defaults to .go)
extensions:
  - .go
  - .mod

# Directories to ignore (optional, adds to default ignores)
ignore:
  - vendor
  - tmp

# Live-reload proxy configuration (optional)
# Format: <listen_port>:<target_port>
proxy: "8080:8081"

# Log level: debug, info, warn, error (optional, defaults to debug)
log_level: info
`

// WriteExampleConfig writes an example configuration file
func WriteExampleConfig(path string) error {
	return os.WriteFile(path, []byte(exampleConfig), 0644)
}

// Default values
const (
	defaultDebounceDelay        = 100 * time.Millisecond
	defaultReloadBroadcastDelay = 300 * time.Millisecond
	defaultCrashThreshold       = 1 * time.Second
	defaultMaxBackoff           = 10 * time.Second
	defaultWatchExtensions      = ".go"
	defaultRootPath             = "."
)
