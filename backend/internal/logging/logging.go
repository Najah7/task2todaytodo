package logging

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// Logger is the structured logging contract shared by application and transport code.
type Logger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
}

type Config struct {
	Service     string
	Environment string
	Version     string
	Level       slog.Level
}

type slogLogger struct{ logger *slog.Logger }

func New(writer io.Writer, cfg Config) Logger {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: cfg.Level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) != 0 {
				return attr
			}
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "timestamp"
			case slog.MessageKey:
				attr.Key = "message"
			}
			return attr
		},
	})
	logger := slog.New(handler)
	if cfg.Service != "" {
		logger = logger.With("service", cfg.Service)
	}
	if cfg.Environment != "" {
		logger = logger.With("environment", cfg.Environment)
	}
	if cfg.Version != "" {
		logger = logger.With("version", cfg.Version)
	}
	return &slogLogger{logger: logger}
}

type nopLogger struct{}

func Nop() Logger { return nopLogger{} }

func (nopLogger) Debug(context.Context, string, ...any) {}
func (nopLogger) Info(context.Context, string, ...any)  {}
func (nopLogger) Warn(context.Context, string, ...any)  {}
func (nopLogger) Error(context.Context, string, ...any) {}

func OrNop(logger Logger) Logger {
	if logger == nil {
		return Nop()
	}
	return logger
}

func (l slogLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, append(contextAttrs(ctx), args...)...)
}
func (l slogLogger) Info(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, append(contextAttrs(ctx), args...)...)
}
func (l slogLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, append(contextAttrs(ctx), args...)...)
}
func (l slogLogger) Error(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, append(contextAttrs(ctx), args...)...)
}

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func contextAttrs(ctx context.Context) []any {
	attrs := make([]any, 0, 4)
	if id := RequestID(ctx); id != "" {
		attrs = append(attrs, "request_id", id)
	}
	span := trace.SpanFromContext(ctx).SpanContext()
	if span.IsValid() {
		attrs = append(attrs,
			"trace_id", span.TraceID().String(),
			"span_id", span.SpanID().String(),
			"trace_flags", strings.ToLower(span.TraceFlags().String()),
		)
	}
	return attrs
}

// ErrorFields returns diagnostic error metadata without exposing error text or SQL details.
func ErrorFields(err error) []any {
	if err == nil {
		return nil
	}
	unwrapped := err
	for errors.Unwrap(unwrapped) != nil {
		unwrapped = errors.Unwrap(unwrapped)
	}
	fields := []any{"error_type", reflect.TypeOf(unwrapped).String()}
	var sqlState interface{ SQLState() string }
	if errors.As(err, &sqlState) {
		if state := sqlState.SQLState(); state != "" {
			fields = append(fields, "sqlstate", state)
		}
	}
	return fields
}

func IsRoutineError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sql.ErrNoRows) {
		return true
	}
	var sqlState interface{ SQLState() string }
	if errors.As(err, &sqlState) {
		switch sqlState.SQLState() {
		case "23505", "23503":
			return true
		}
	}
	return false
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
