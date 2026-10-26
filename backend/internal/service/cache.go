package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/enrico/warehouse/internal/requestlog"
)

// LookupCache is an optional, best-effort cache. Revisions protect concurrent cache fills.
type LookupCache interface {
	Read(context.Context, string) ([]byte, string, error)
	Write(context.Context, string, string, []byte) error
	Invalidate(context.Context, string) error
}

func cacheWarning(ctx context.Context, logger *slog.Logger, operation, key string, err error) {
	requestlog.Logger(ctx, logger).WarnContext(ctx, "cache.failed", "operation", operation, "key", key, "error", err)
}

func cachedLookup[T any](ctx context.Context, cache LookupCache, logger *slog.Logger, key string, load func(context.Context) (T, error)) (T, error) {
	if cache == nil {
		return load(ctx)
	}
	raw, revision, err := cache.Read(ctx, key)
	if err != nil {
		cacheWarning(ctx, logger, "read", key, err)
	} else if len(raw) > 0 {
		var value T
		if decodeErr := json.Unmarshal(raw, &value); decodeErr == nil && strings.TrimSpace(string(raw)) != "null" {
			return value, nil
		} else if decodeErr != nil {
			cacheWarning(ctx, logger, "decode", key, decodeErr)
		}
	}
	value, err := load(ctx)
	if err != nil {
		return value, err
	}
	if revision != "" {
		raw, err = json.Marshal(value)
		if err != nil {
			cacheWarning(ctx, logger, "encode", key, err)
		} else if err = cache.Write(ctx, key, revision, raw); err != nil {
			cacheWarning(ctx, logger, "write", key, err)
		}
	}
	return value, nil
}

func invalidateLookup(ctx context.Context, cache LookupCache, logger *slog.Logger, key string) {
	if cache == nil {
		return
	}
	// A client disconnect after commit must not cancel the invalidation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 250*time.Millisecond)
	defer cancel()
	if err := cache.Invalidate(ctx, key); err != nil {
		cacheWarning(ctx, logger, "invalidate", key, err)
	}
}
