package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/humoroushorse/go_sprint/pkg/config"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const loggerKey contextKey = "logger"

// NewLogger creates a new slog.Logger based on the provided configuration
func NewLogger(cfg config.LoggingConfig) *slog.Logger {
	var handler slog.Handler

	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "console":
		if cfg.EnableColors {
			handler = NewColoredConsoleHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}
	default:
		// Default to JSON for production safety
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// parseLevel converts a string log level to slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from the context
// If no logger is found, it returns the default logger
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithTraceID adds a trace ID to the logger
func WithTraceID(logger *slog.Logger, traceID string) *slog.Logger {
	return logger.With(slog.String("trace_id", traceID))
}

// WithUserID adds a user ID to the logger
func WithUserID(logger *slog.Logger, userID string) *slog.Logger {
	return logger.With(slog.String("user_id", userID))
}

// WithRequestInfo adds request information to the logger
func WithRequestInfo(logger *slog.Logger, method, path string) *slog.Logger {
	return logger.With(
		slog.String("method", method),
		slog.String("path", path),
	)
}

// ColoredConsoleHandler is a custom handler that adds colors to console output
type ColoredConsoleHandler struct {
	handler slog.Handler
	writer  io.Writer
	opts    *slog.HandlerOptions
}

// NewColoredConsoleHandler creates a new colored console handler
func NewColoredConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *ColoredConsoleHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// Create a custom handler that wraps text handler with color formatting
	return &ColoredConsoleHandler{
		handler: slog.NewTextHandler(w, opts),
		writer:  w,
		opts:    opts,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *ColoredConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle handles the Record with colored output
func (h *ColoredConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format: time=... level=LEVEL msg="message" key=value ...
	buf := make([]byte, 0, 1024)

	// Time
	buf = append(buf, "time="...)
	buf = r.Time.AppendFormat(buf, "2006-01-02T15:04:05.000-07:00")
	buf = append(buf, ' ')

	// Level with color
	levelColor := getLevelColor(r.Level)
	resetColor := "\033[0m"
	buf = append(buf, "level="...)
	buf = append(buf, levelColor...)
	buf = append(buf, r.Level.String()...)
	buf = append(buf, resetColor...)
	buf = append(buf, ' ')

	// Message with color
	buf = append(buf, "msg="...)
	buf = append(buf, levelColor...)
	buf = append(buf, '"')
	buf = append(buf, r.Message...)
	buf = append(buf, '"')
	buf = append(buf, resetColor...)

	// Attributes
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		buf = appendValue(buf, a.Value)
		return true
	})

	buf = append(buf, '\n')
	_, err := h.writer.Write(buf)
	return err
}

// appendValue appends a slog.Value to the buffer
func appendValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return append(buf, v.String()...)
	case slog.KindInt64:
		return append(buf, []byte(fmt.Sprintf("%d", v.Int64()))...)
	case slog.KindUint64:
		return append(buf, []byte(fmt.Sprintf("%d", v.Uint64()))...)
	case slog.KindFloat64:
		return append(buf, []byte(fmt.Sprintf("%g", v.Float64()))...)
	case slog.KindBool:
		return append(buf, []byte(fmt.Sprintf("%t", v.Bool()))...)
	case slog.KindDuration:
		return append(buf, []byte(v.Duration().String())...)
	case slog.KindTime:
		return v.Time().AppendFormat(buf, "2006-01-02T15:04:05.000-07:00")
	default:
		return append(buf, []byte(fmt.Sprintf("%v", v.Any()))...)
	}
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments
func (h *ColoredConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColoredConsoleHandler{
		handler: h.handler.WithAttrs(attrs),
		writer:  h.writer,
		opts:    h.opts,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups
func (h *ColoredConsoleHandler) WithGroup(name string) slog.Handler {
	return &ColoredConsoleHandler{
		handler: h.handler.WithGroup(name),
		writer:  h.writer,
		opts:    h.opts,
	}
}

// getLevelColor returns the ANSI color code for the given log level
func getLevelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "\033[36m" // Cyan
	case slog.LevelInfo:
		return "\033[32m" // Green
	case slog.LevelWarn:
		return "\033[33m" // Yellow
	case slog.LevelError:
		return "\033[31m" // Red
	default:
		return "\033[0m" // Reset
	}
}
