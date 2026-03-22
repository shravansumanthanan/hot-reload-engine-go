package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherFileChange(t *testing.T) {
	dir := t.TempDir()

	// Create an initial .go file
	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	time.Sleep(50 * time.Millisecond) // let watcher settle
	if err := os.WriteFile(testFile, []byte("package main\n// changed"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		if event != testFile {
			t.Errorf("Expected event for %s, got %s", testFile, event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for file change event")
	}
}

func TestWatcherIgnoresExtensions(t *testing.T) {
	dir := t.TempDir()

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Create a .txt file — should NOT trigger an event
	txtFile := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		t.Fatalf("Did not expect event for non-.go file, got: %s", event)
	case <-time.After(300 * time.Millisecond):
		// Good — no event received
	}
}

func TestWatcherNewDirectory(t *testing.T) {
	dir := t.TempDir()

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Create a new subdirectory
	subDir := filepath.Join(dir, "pkg")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Give the watcher time to pick up the new directory
	time.Sleep(200 * time.Millisecond)

	// Create a .go file inside the new subdirectory
	newFile := filepath.Join(subDir, "lib.go")
	if err := os.WriteFile(newFile, []byte("package pkg"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		if event != newFile {
			t.Errorf("Expected event for %s, got %s", newFile, event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for event in new subdirectory")
	}
}

func TestWatcherDeletedDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory with a file
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	subFile := filepath.Join(subDir, "main.go")
	if err := os.WriteFile(subFile, []byte("package sub"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Delete the subdirectory — should not crash the watcher
	if err := os.RemoveAll(subDir); err != nil {
		t.Fatal(err)
	}

	// Drain any events from the removal
	drainTimeout := time.After(500 * time.Millisecond)
drain:
	for {
		select {
		case <-w.Events:
			// Expected — removal may generate events
		case <-drainTimeout:
			break drain
		}
	}

	// Watcher should still work: create a new file in the root
	newFile := filepath.Join(dir, "new.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		if event != newFile {
			t.Errorf("Expected event for %s, got %s", newFile, event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watcher stopped working after directory deletion")
	}
}

func TestWatcherIgnoresDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a .git directory
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Create a file inside .git — should NOT trigger
	gitFile := filepath.Join(gitDir, "HEAD.go")
	if err := os.WriteFile(gitFile, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		t.Fatalf("Did not expect event for ignored dir file, got: %s", event)
	case <-time.After(300 * time.Millisecond):
		// Good
	}
}

func TestWatcherIgnoresTempFiles(t *testing.T) {
	dir := t.TempDir()

	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Create editor temp files
	tempFiles := []string{
		filepath.Join(dir, "main.go~"),
		filepath.Join(dir, ".main.go.swp"),
		filepath.Join(dir, "#main.go#"),
		filepath.Join(dir, ".#main.go"),
	}

	for _, f := range tempFiles {
		if err := os.WriteFile(f, []byte("temp"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case event := <-w.Events:
		t.Fatalf("Did not expect event for temp file, got: %s", event)
	case <-time.After(300 * time.Millisecond):
		// Good — all temp files were ignored
	}
}

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", false},
		{"main.go~", true},
		{".main.go.swp", true},
		{".main.go.swo", true},
		{"#main.go#", true},
		{".#main.go", true},
		{"main.go.tmp", true},
		{"main.go.bak", true},
		{"server.go", false},
		{"/path/to/main.go~", true},
		{"/path/to/server.go", false},
	}

	for _, tt := range tests {
		result := isTempFile(tt.path)
		if result != tt.expected {
			t.Errorf("isTempFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestWatcherCustomIgnores(t *testing.T) {
	dir := t.TempDir()

	// Create a custom directory to ignore
	vendorDir := filepath.Join(dir, "vendor")
	if err := os.Mkdir(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}

	w, err := New(dir, []string{".go"}, []string{"vendor"})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Create a file inside vendor — should NOT trigger
	vendorFile := filepath.Join(vendorDir, "dep.go")
	if err := os.WriteFile(vendorFile, []byte("package vendor"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-w.Events:
		t.Fatalf("Did not expect event for custom-ignored dir, got: %s", event)
	case <-time.After(300 * time.Millisecond):
		// Good
	}
}

func TestHotreloadIgnoreFile(t *testing.T) {
	dir := t.TempDir()

	// Create a .hotreloadignore file
	ignoreContent := `# Comment line
custom_dir
another_dir

# Another comment
third_dir
`
	ignoreFile := filepath.Join(dir, ".hotreloadignore")
	if err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create watcher - should load .hotreloadignore
	w, err := New(dir, []string{".go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Verify custom directories are ignored
	if !w.ignores["custom_dir"] {
		t.Error("Expected custom_dir to be ignored")
	}
	if !w.ignores["another_dir"] {
		t.Error("Expected another_dir to be ignored")
	}
	if !w.ignores["third_dir"] {
		t.Error("Expected third_dir to be ignored")
	}

	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	// Create a custom ignored directory
	customDir := filepath.Join(dir, "custom_dir")
	if err := os.Mkdir(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create a file inside the ignored directory
	ignoredFile := filepath.Join(customDir, "test.go")
	if err := os.WriteFile(ignoredFile, []byte("package test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should NOT receive an event
	select {
	case event := <-w.Events:
		t.Fatalf("Did not expect event for .hotreloadignore'd directory, got: %s", event)
	case <-time.After(300 * time.Millisecond):
		// Good - no event received
	}
}
