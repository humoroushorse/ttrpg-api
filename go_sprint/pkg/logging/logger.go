package logging

import (
	"context"
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

// Handle handles the Record
func (h *ColoredConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add color codes based on level
	levelColor := getLevelColor(r.Level)
	resetColor := "\033[0m"

	// Create a new record with colored level
	newRecord := slog.NewRecord(r.Time, r.Level, levelColor+r.Message+resetColor, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(a)
		return true
	})

	return h.handler.Handle(ctx, newRecord)
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
