package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the hotreload configuration file structure.
type Config struct {
	Root        string   `yaml:"root"`
	Build       string   `yaml:"build"`
	Exec        string   `yaml:"exec"`
	Extensions  []string `yaml:"extensions"`
	Ignore      []string `yaml:"ignore"`
	Proxy       string   `yaml:"proxy"`
	LogLevel    string   `yaml:"log_level"`
	HealthCheck string   `yaml:"health_check"` // optional HTTP URL polled before broadcasting reload
	PreBuild    string   `yaml:"pre_build"`    // optional shell command run before each build
	PostBuild   string   `yaml:"post_build"`   // optional shell command run after a successful build
}

// validProxyFormat matches "<listen_port>:<target_port>" e.g. "8080:8081".
var validProxyFormat = regexp.MustCompile(`^\d+:\d+$`)

// Validate checks that the configuration values are semantically correct.
// It returns an error describing the first problem found, or nil if everything
// is valid. Validate should be called after MergeWithFlags so that CLI flags
// have already been applied.
func (c *Config) Validate() error {
	if c.Root != "" {
		if _, err := os.Stat(c.Root); os.IsNotExist(err) {
			return fmt.Errorf("root directory %q does not exist", c.Root)
		}
	}

	if c.Proxy != "" && !validProxyFormat.MatchString(c.Proxy) {
		return fmt.Errorf("invalid proxy value %q: expected format <listen_port>:<target_port> (e.g. 8080:8081)", c.Proxy)
	}

	if c.LogLevel != "" {
		switch strings.ToLower(c.LogLevel) {
		case "debug", "info", "warn", "error":
			// valid
		default:
			return fmt.Errorf("invalid log_level %q: must be one of debug, info, warn, error", c.LogLevel)
		}
	}

	if c.HealthCheck != "" {
		if !strings.HasPrefix(c.HealthCheck, "http://") && !strings.HasPrefix(c.HealthCheck, "https://") {
			return fmt.Errorf("invalid health_check %q: must be an http:// or https:// URL", c.HealthCheck)
		}
	}

	return nil
}

// LoadConfig attempts to load configuration from a .hotreload.yaml file.
// Returns nil if the file doesn't exist (not an error).
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

// MergeWithFlags merges config file values with CLI flags.
// explicitFlags is the set of flag names that the user explicitly provided on
// the command line (obtained via flag.Visit). CLI flags always win over the
// config file; config file values are used only as a fallback.
func (c *Config) MergeWithFlags(
	rootFlag, buildFlag, execFlag, extFlag, ignoreFlag, proxyFlag, logLevelFlag *string,
	explicitFlags map[string]bool,
) {
	// For each field: if the user explicitly passed the flag, keep the flag
	// value (and optionally push it back into the Config struct for consistency).
	// Otherwise, fall back to the config file value.

	if explicitFlags["root"] {
		c.Root = *rootFlag
	} else if c.Root != "" {
		*rootFlag = c.Root
	}

	if explicitFlags["build"] {
		c.Build = *buildFlag
	} else if c.Build != "" {
		*buildFlag = c.Build
	}

	if explicitFlags["exec"] {
		c.Exec = *execFlag
	} else if c.Exec != "" {
		*execFlag = c.Exec
	}

	if explicitFlags["ext"] {
		// CLI extensions provided — leave *extFlag as-is.
	} else if len(c.Extensions) > 0 {
		// Build a comma-separated string from the config file extensions.
		var buf strings.Builder
		for i, ext := range c.Extensions {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(ext)
		}
		*extFlag = buf.String()
	}

	if explicitFlags["ignore"] {
		// CLI ignore patterns provided — leave *ignoreFlag as-is.
	} else if len(c.Ignore) > 0 {
		var buf strings.Builder
		for i, ign := range c.Ignore {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(ign)
		}
		*ignoreFlag = buf.String()
	}

	if explicitFlags["proxy"] {
		c.Proxy = *proxyFlag
	} else if c.Proxy != "" {
		*proxyFlag = c.Proxy
	}

	if explicitFlags["log-level"] {
		c.LogLevel = *logLevelFlag
	} else if c.LogLevel != "" {
		*logLevelFlag = c.LogLevel
	}
}

// Example .hotreload.yaml file content
const ExampleConfig = `# Hotreload Configuration File
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

# Directories or glob patterns to ignore (optional, adds to default ignores).
# Standard shell globs (*, ?, [abc]) are supported.
ignore:
  - vendor
  - tmp
  - "*.gen"

# Live-reload proxy configuration (optional)
# Format: <listen_port>:<target_port>
proxy: "8080:8081"

# Optional HTTP URL polled before broadcasting a browser reload.
# Use this when your app needs time to complete startup tasks after the port
# opens (e.g. database migrations, cache warming).
# Falls back to TCP port polling when not set.
# health_check: "http://localhost:8081/healthz"

# Optional shell command run before each build (e.g. code generation).
# pre_build: "go generate ./..."

# Optional shell command run after each successful build.
# post_build: "echo Build complete"

# Log level: debug, info, warn, error (optional, defaults to debug)
log_level: info
`

// WriteExampleConfig writes an example configuration file.
func WriteExampleConfig(path string) error {
	return os.WriteFile(path, []byte(ExampleConfig), 0644)
}

// Default values
const (
	DefaultDebounceDelay        = 100 * time.Millisecond
	DefaultReloadBroadcastDelay = 300 * time.Millisecond
	DefaultWatchExtensions      = ".go"
	DefaultRootPath             = "."
)
