package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var defaultIgnoredDirs = []string{
	".git",
	"node_modules",
	"bin",
	"build",
	"tmp",
	"dist",
	".idea",
	".vscode",
}

// tempFileSuffixes are file suffixes created by editors during saves.
// These should be ignored to prevent spurious rebuilds.
var tempFileSuffixes = []string{
	"~",    // Vim/Emacs backup files
	".swp", // Vim swap files
	".swo", // Vim swap files
	".swx", // Vim swap files
	".tmp", // Generic temp files
	".bak", // Backup files
}

// tempFilePrefixes are file prefixes created by editors during saves.
var tempFilePrefixes = []string{
	"#",  // Emacs auto-save files
	".#", // Emacs lock files
}

type Watcher struct {
	watcher        *fsnotify.Watcher
	root           string
	Events         chan string
	Errors         chan error
	exts           map[string]struct{}
	ignorePatterns []string // glob patterns, matched via filepath.Match
}

// New creates a new recursive watcher.
// ignores is a list of additional glob patterns to ignore (e.g. "vendor", "*.gen").
func New(root string, exts []string, ignores []string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Start with the built-in defaults.
	patterns := make([]string, len(defaultIgnoredDirs))
	copy(patterns, defaultIgnoredDirs)

	// Add CLI/config-supplied patterns.
	for _, ign := range ignores {
		if ign = strings.TrimSpace(ign); ign != "" {
			patterns = append(patterns, ign)
		}
	}

	// Load .hotreloadignore file if it exists (supports glob patterns).
	hotreloadIgnores, err := loadHotreloadIgnore(root)
	if err != nil {
		slog.Warn("Failed to load .hotreloadignore", "err", err)
	} else {
		patterns = append(patterns, hotreloadIgnores...)
	}

	extMap := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		if ext = strings.TrimSpace(ext); ext != "" {
			extMap[ext] = struct{}{}
		}
	}

	return &Watcher{
		watcher:        w,
		root:           root,
		Events:         make(chan string, 100),
		Errors:         make(chan error, 10),
		exts:           extMap,
		ignorePatterns: patterns,
	}, nil
}

// Start begins watching the directory tree.
func (w *Watcher) Start() error {
	err := w.watchRecursive(w.root)
	if err != nil {
		return err
	}

	go w.readEvents()
	return nil
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	return w.watcher.Close()
}

func (w *Watcher) watchRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if info.IsDir() {
			if w.shouldIgnoreDir(path, info.Name()) {
				return filepath.SkipDir
			}

			err = w.watcher.Add(path)
			if err != nil {
				slog.Error("Failed to watch directory", "path", path, "err", err)
			} else {
				slog.Debug("Watching directory", "path", path)
			}
		}
		return nil
	})
}

// shouldIgnoreDir returns true if a directory should be skipped.
//
// Each pattern in ignorePatterns is tested against:
//  1. The directory's base name  (e.g. "node_modules")
//  2. The path relative to the watch root (e.g. "internal/generated")
//
// Patterns are evaluated using filepath.Match, so standard shell globs
// (*, ?, [abc]) work correctly. Patterns ending with "/" are also matched
// by stripping the trailing slash before comparison.
func (w *Watcher) shouldIgnoreDir(absPath, baseName string) bool {
	relPath, relErr := filepath.Rel(w.root, absPath)

	for _, pattern := range w.ignorePatterns {
		// Normalise: remove trailing slash so "bin/" matches base "bin".
		p := strings.TrimSuffix(pattern, "/")
		if p == "" {
			continue
		}

		// Match against base name (most common: "vendor", "node_modules").
		if matched, _ := filepath.Match(p, baseName); matched {
			return true
		}

		// Match against relative path (e.g. "internal/generated").
		if relErr == nil {
			if matched, _ := filepath.Match(p, relPath); matched {
				return true
			}
		}
	}
	return false
}

func (w *Watcher) readEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// If a new directory is created, watch it recursively.
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					if !w.shouldIgnoreDir(event.Name, info.Name()) {
						if err := w.watchRecursive(event.Name); err != nil {
							slog.Error("Failed to watch new directory", "path", event.Name, "err", err)
						}
					}
				}
			}

			// If a directory is removed or renamed, remove it from the watch list.
			// fsnotify may return errors for removed watches; we handle this gracefully.
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				// Attempt to remove the watch. This is a no-op if the path
				// wasn't being watched (e.g., it was a file, not a directory).
				_ = w.watcher.Remove(event.Name)
			}

			// Skip temporary editor files
			if isTempFile(event.Name) {
				continue
			}

			// Emit events for interesting file modifications
			if w.isInterestingFile(event.Name) {
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					w.Events <- event.Name
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.Errors <- err
		}
	}
}

func (w *Watcher) isInterestingFile(path string) bool {
	if len(w.exts) == 0 {
		return true
	}
	_, ok := w.exts[filepath.Ext(path)]
	return ok
}

// isTempFile returns true if the file path looks like a temporary editor file.
func isTempFile(path string) bool {
	base := filepath.Base(path)
	for _, suffix := range tempFileSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	for _, prefix := range tempFilePrefixes {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

// loadHotreloadIgnore reads the .hotreloadignore file and returns a list of
// glob patterns to ignore. Empty lines and lines beginning with '#' are
// treated as comments and skipped.
func loadHotreloadIgnore(root string) ([]string, error) {
	ignorePath := filepath.Join(root, ".hotreloadignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, not an error
		}
		return nil, err
	}

	var patterns []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if len(patterns) > 0 {
		slog.Info("Loaded custom ignore patterns from .hotreloadignore", "count", len(patterns))
	}

	return patterns, nil
}
