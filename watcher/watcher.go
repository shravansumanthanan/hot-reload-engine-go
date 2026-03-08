package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var defaultIgnoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"bin":          true,
	"build":        true,
	"tmp":          true,
	"dist":         true,
	".idea":        true,
	".vscode":      true,
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
	watcher *fsnotify.Watcher
	root    string
	Events  chan string
	Errors  chan error
	exts    []string
	ignores map[string]bool
}

// New creates a new recursive watcher.
func New(root string, exts []string, ignores []string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ignMap := make(map[string]bool)
	for k, v := range defaultIgnoredDirs {
		ignMap[k] = v
	}
	for _, ign := range ignores {
		if ign != "" {
			ignMap[strings.TrimSpace(ign)] = true
		}
	}

	validExts := []string{}
	for _, ext := range exts {
		if ext != "" {
			validExts = append(validExts, strings.TrimSpace(ext))
		}
	}

	return &Watcher{
		watcher: w,
		root:    root,
		Events:  make(chan string, 100),
		Errors:  make(chan error, 10),
		exts:    validExts,
		ignores: ignMap,
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
			if w.ignores[info.Name()] {
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
					if !w.ignores[info.Name()] {
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
	for _, ext := range w.exts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
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
