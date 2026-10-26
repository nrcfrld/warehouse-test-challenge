package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// LookupCache is an optional, best-effort cache. Revisions protect concurrent cache fills.
type LookupCache interface {
	Read(context.Context, string) ([]byte, string, error)
	Write(context.Context, string, string, []byte) error
	Invalidate(context.Context, string) error
}

func (s *Service) cacheWarning(ctx context.Context, operation, key string, err error) {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(ctx, "cache.failed", "operation", operation, "key", key, "error", err)
}

func cachedLookup[T any](ctx context.Context, s *Service, key string, load func(context.Context) (T, error)) (T, error) {
	if s.Cache == nil {
		return load(ctx)
	}
	raw, revision, err := s.Cache.Read(ctx, key)
	if err != nil {
		s.cacheWarning(ctx, "read", key, err)
	} else if len(raw) > 0 {
		var value T
		if decodeErr := json.Unmarshal(raw, &value); decodeErr == nil && strings.TrimSpace(string(raw)) != "null" {
			return value, nil
		} else if decodeErr != nil {
			s.cacheWarning(ctx, "decode", key, decodeErr)
		}
	}
	value, err := load(ctx)
	if err != nil {
		return value, err
	}
	if revision != "" {
		raw, err = json.Marshal(value)
		if err != nil {
			s.cacheWarning(ctx, "encode", key, err)
		} else if err = s.Cache.Write(ctx, key, revision, raw); err != nil {
			s.cacheWarning(ctx, "write", key, err)
		}
	}
	return value, nil
}

func (s *Service) invalidateLookup(ctx context.Context, key string) {
	if s.Cache == nil {
		return
	}
	// A client disconnect after commit must not cancel the invalidation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 250*time.Millisecond)
	defer cancel()
	if err := s.Cache.Invalidate(ctx, key); err != nil {
		s.cacheWarning(ctx, "invalidate", key, err)
	}
}
