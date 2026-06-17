// Package logger provides a colorized slog.Handler for human-readable
// terminal output during development. It uses raw ANSI escape codes so
// it requires zero external dependencies.
//
// Output format:
//
//	12:34:56 INFO  Starting hotreload root=. build="go build ."
//	12:34:56 WARN  Process exited unexpectedly
//	12:34:56 ERROR Build failed err="exit status 1"
package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"
	"unicode"
)

// ANSI escape codes.
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	fgRed     = "\033[31m"
	fgGreen   = "\033[32m"
	fgYellow  = "\033[33m"
	fgCyan    = "\033[36m"
	fgWhite   = "\033[97m"
	fgGray    = "\033[90m"
	bgRed     = "\033[41m"
)

// ColorHandler is a slog.Handler that writes colorised, human-readable log
// lines to w. It is safe for concurrent use.
type ColorHandler struct {
	w       io.Writer
	opts    slog.HandlerOptions
	mu      sync.Mutex
	preAttrs []slog.Attr // attrs added via WithAttrs
	groups   []string    // active group names
}

// New returns a ColorHandler that writes to w. If w is not a terminal (e.g.
// a file or pipe), the caller should use slog.NewTextHandler instead.
func New(w io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ColorHandler{w: w, opts: *opts}
}

// NewDefault creates a ColorHandler writing to os.Stdout with the given level.
func NewDefault(level slog.Level) *ColorHandler {
	return New(os.Stdout, &slog.HandlerOptions{Level: level})
}

func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelDebug
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer

	// Timestamp — dim gray
	buf.WriteString(dim + fgGray)
	buf.WriteString(r.Time.Format("15:04:05"))
	buf.WriteString(reset)
	buf.WriteByte(' ')

	// Level badge
	buf.WriteString(levelColor(r.Level))
	buf.WriteString(levelLabel(r.Level))
	buf.WriteString(reset)
	buf.WriteByte(' ')

	// Message — white
	buf.WriteString(fgWhite)
	buf.WriteString(r.Message)
	buf.WriteString(reset)

	// Pre-attrs added via WithAttrs
	for _, a := range h.preAttrs {
		buf.WriteByte(' ')
		writeAttr(&buf, a, h.groups)
	}

	// Inline attrs from the record
	r.Attrs(func(a slog.Attr) bool {
		buf.WriteByte(' ')
		writeAttr(&buf, a, h.groups)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := h.clone()
	h2.preAttrs = append(h2.preAttrs, attrs...)
	return h2
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	h2 := h.clone()
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *ColorHandler) clone() *ColorHandler {
	return &ColorHandler{
		w:        h.w,
		opts:     h.opts,
		preAttrs: slices.Clone(h.preAttrs),
		groups:   slices.Clone(h.groups),
	}
}

// levelColor returns the ANSI sequence for a given log level.
func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return bold + fgRed
	case level >= slog.LevelWarn:
		return bold + fgYellow
	case level >= slog.LevelInfo:
		return bold + fgGreen
	default:
		return fgCyan
	}
}

// levelLabel returns a fixed-width, padded label for the log level.
func levelLabel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN "
	case level >= slog.LevelInfo:
		return "INFO "
	default:
		return "DEBUG"
	}
}

// writeAttr renders a single slog.Attr as "key=value" with dim coloring.
// Group names are prepended with a dot separator.
func writeAttr(buf *bytes.Buffer, a slog.Attr, groups []string) {
	// Resolve the value first so we can skip empty/zero attrs.
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	// Key — dim gray
	buf.WriteString(dim + fgGray)
	for _, g := range groups {
		buf.WriteString(g)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	buf.WriteByte('=')
	buf.WriteString(reset)

	// Value
	writeValue(buf, a.Value)
}

// writeValue renders a slog.Value, quoting strings that contain spaces.
func writeValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if needsQuoting(s) {
			buf.WriteString(strconv.Quote(s))
		} else {
			buf.WriteString(s)
		}
	case slog.KindTime:
		buf.WriteString(v.Time().Format(time.RFC3339))
	case slog.KindGroup:
		attrs := v.Group()
		buf.WriteByte('{')
		for i, a := range attrs {
			if i > 0 {
				buf.WriteByte(' ')
			}
			writeAttr(buf, a, nil)
		}
		buf.WriteByte('}')
	default:
		buf.WriteString(fmt.Sprintf("%v", v.Any()))
	}
}

// needsQuoting returns true if s contains whitespace or special characters
// that would make the log line ambiguous without quotes.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '=' {
			return true
		}
	}
	return false
}
