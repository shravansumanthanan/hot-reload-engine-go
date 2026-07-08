package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewNilOpts(t *testing.T) {
	h := New(&bytes.Buffer{}, nil)
	if h == nil {
		t.Fatal("New returned nil with nil opts")
	}
}

func TestNewWithOpts(t *testing.T) {
	h := New(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	if h == nil {
		t.Fatal("New returned nil")
	}
}

func TestEnabled(t *testing.T) {
	h := New(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("LevelDebug should be disabled when handler level is Warn")
	}
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("LevelInfo should be disabled when handler level is Warn")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("LevelWarn should be enabled when handler level is Warn")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("LevelError should be enabled when handler level is Warn")
	}
}

func TestEnabledDefaultLevel(t *testing.T) {
	h := New(&bytes.Buffer{}, nil) // nil opts → default LevelDebug
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("LevelDebug should be enabled with default handler options")
	}
}

func TestHandleBasic(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	r := slog.NewRecord(time.Date(2024, 1, 15, 12, 34, 56, 0, time.UTC), slog.LevelInfo, "test message", 0)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "12:34:56") {
		t.Error("Expected timestamp in output")
	}
	if !strings.Contains(out, "INFO") {
		t.Error("Expected INFO level label")
	}
	if !strings.Contains(out, "test message") {
		t.Error("Expected message in output")
	}
}

func TestHandleWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("key", "value"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "key=") {
		t.Error("Expected key= in output")
	}
	if !strings.Contains(out, "value") {
		t.Error("Expected value in output")
	}
}

func TestHandleQuotedAttr(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.String("cmd", "go build ."))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	// "go build ." contains a space, so it should be quoted
	if !strings.Contains(out, `"go build ."`) {
		t.Errorf("Expected quoted value for string with spaces, got: %s", out)
	}
}

func TestWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "watcher")})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "event", 0)
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "component=") {
		t.Error("Expected pre-attr 'component=' in output")
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	h2 := h.WithGroup("http")

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)
	r.AddAttrs(slog.String("method", "GET"))
	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "http.method=") {
		t.Errorf("Expected grouped attr 'http.method=', got: %s", out)
	}
}

func TestLevelLabels(t *testing.T) {
	tests := []struct {
		level slog.Level
		label string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		h := New(&buf, nil)
		r := slog.NewRecord(time.Now(), tt.level, "msg", 0)
		if err := h.Handle(context.Background(), r); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), tt.label) {
			t.Errorf("Expected %s label for level %v", tt.label, tt.level)
		}
	}
}

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"simple", false},
		{"has space", true},
		{`has"quote`, true},
		{"has=equals", true},
		{"no-special-chars", false},
		{"tab\there", true},
	}

	for _, tt := range tests {
		got := needsQuoting(tt.input)
		if got != tt.want {
			t.Errorf("needsQuoting(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWriteValueTime(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Time("ts", time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "2024-06-15T10:30:00Z") {
		t.Errorf("Expected RFC3339 time in output, got: %s", buf.String())
	}
}

func TestWriteValueInt(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Int("count", 42))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "42") {
		t.Errorf("Expected integer 42 in output, got: %s", buf.String())
	}
}

func TestConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf, nil)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			r := slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent", 0)
			_ = h.Handle(context.Background(), r)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	lines := strings.Count(buf.String(), "\n")
	if lines != 10 {
		t.Errorf("Expected 10 lines from concurrent writes, got %d", lines)
	}
}
