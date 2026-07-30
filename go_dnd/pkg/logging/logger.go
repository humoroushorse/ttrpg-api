package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/humoroushorse/go_dnd/pkg/config"
)

type contextKey string

const loggerKey contextKey = "logger"

func NewLogger(cfg config.LoggingConfig) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
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
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

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

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext returns the logger stored in ctx, or slog.Default() if none.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

func WithTraceID(logger *slog.Logger, traceID string) *slog.Logger {
	return logger.With(slog.String("trace_id", traceID))
}

func WithUserID(logger *slog.Logger, userID string) *slog.Logger {
	return logger.With(slog.String("user_id", userID))
}

func WithRequestInfo(logger *slog.Logger, method, path string) *slog.Logger {
	return logger.With(slog.String("method", method), slog.String("path", path))
}

type ColoredConsoleHandler struct {
	handler slog.Handler
	writer  io.Writer
	opts    *slog.HandlerOptions
}

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

func (h *ColoredConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *ColoredConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)

	buf = append(buf, "time="...)
	buf = r.Time.AppendFormat(buf, "2006-01-02T15:04:05.000-07:00")
	buf = append(buf, ' ')

	color := getLevelColor(r.Level)
	reset := "\033[0m"
	buf = append(buf, "level="...)
	buf = append(buf, color...)
	buf = append(buf, r.Level.String()...)
	buf = append(buf, reset...)
	buf = append(buf, ' ')

	buf = append(buf, "msg="...)
	buf = append(buf, color...)
	buf = append(buf, '"')
	buf = append(buf, r.Message...)
	buf = append(buf, '"')
	buf = append(buf, reset...)

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

func appendValue(buf []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return append(buf, v.String()...)
	case slog.KindInt64:
		return append(buf, fmt.Sprintf("%d", v.Int64())...)
	case slog.KindUint64:
		return append(buf, fmt.Sprintf("%d", v.Uint64())...)
	case slog.KindFloat64:
		return append(buf, fmt.Sprintf("%g", v.Float64())...)
	case slog.KindBool:
		return append(buf, fmt.Sprintf("%t", v.Bool())...)
	case slog.KindDuration:
		return append(buf, v.Duration().String()...)
	case slog.KindTime:
		return v.Time().AppendFormat(buf, "2006-01-02T15:04:05.000-07:00")
	default:
		return append(buf, fmt.Sprintf("%v", v.Any())...)
	}
}

func (h *ColoredConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColoredConsoleHandler{handler: h.handler.WithAttrs(attrs), writer: h.writer, opts: h.opts}
}

func (h *ColoredConsoleHandler) WithGroup(name string) slog.Handler {
	return &ColoredConsoleHandler{handler: h.handler.WithGroup(name), writer: h.writer, opts: h.opts}
}

func getLevelColor(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "\033[36m" // cyan
	case slog.LevelInfo:
		return "\033[32m" // green
	case slog.LevelWarn:
		return "\033[33m" // yellow
	case slog.LevelError:
		return "\033[31m" // red
	default:
		return "\033[0m"
	}
}
