// Package requestlog carries a request-scoped logger across application layers.
package requestlog

import (
	"context"
	"log/slog"
)

type loggerKey struct{}

// WithLogger attaches a logger whose attributes include the request ID.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Logger returns the request logger, a supplied fallback, or the default logger.
func Logger(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}
