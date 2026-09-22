// Package logging builds the shared JSON logger and carries request-scoped loggers through contexts.
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// RedactedValue replaces the value of any attribute whose key looks sensitive.
const RedactedValue = "[REDACTED]"

// New returns a JSON logger writing to w that redacts secrets by attribute key.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactSensitive,
	}))
}

func redactSensitive(_ []string, attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, RedactedValue)
	}
	return attr
}

// isSensitiveKey matches by substring so variants like "refresh_token" or "db_password" are caught too.
func isSensitiveKey(key string) bool {
	lowered := strings.ToLower(key)
	for _, fragment := range []string{"token", "secret", "password", "authorization", "cookie", "database_url"} {
		if strings.Contains(lowered, fragment) {
			return true
		}
	}
	return false
}

type contextKey struct{}

// WithLogger returns a copy of ctx that carries logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger stored by WithLogger, or fallback when ctx carries none.
func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return logger
	}
	return fallback
}
