// Package cache implements optional, disposable Redis lookup caching.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const operationTimeout = 200 * time.Millisecond

// Redis caches JSON lookups under a versioned namespace.
// Random revisions prevent reads started before invalidation from repopulating stale data.
type Redis struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// New configures bounded Redis calls without automatic retries. No connection is required at startup.
func New(url string, ttl time.Duration) (*Redis, error) {
	if ttl < time.Millisecond || ttl > time.Hour {
		return nil, errors.New("REDIS_CACHE_TTL must be between 1ms and 1h")
	}
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, errors.New("REDIS_URL must be a valid redis:// or rediss:// connection URL")
	}
	options.MaxRetries = -1
	options.DialerRetries = 1
	options.DialTimeout = operationTimeout
	options.ReadTimeout = operationTimeout
	options.WriteTimeout = operationTimeout
	options.PoolTimeout = operationTimeout
	options.ContextTimeoutEnabled = true
	options.PoolSize = 10
	return &Redis{client: redis.NewClient(options), ttl: ttl, prefix: "warehouse:v1:lookups:"}, nil
}

// Close releases connections during shutdown.
func (c *Redis) Close() error { return c.client.Close() }

// Ping probes Redis without making it an API readiness dependency.
func (c *Redis) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return c.client.Ping(ctx).Err()
}

var readScript = redis.NewScript(`
local revision = redis.call('HGET', KEYS[1], 'revision')
if not revision then
 revision = ARGV[1]
 redis.call('HSET', KEYS[1], 'revision', revision)
 redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return {revision, redis.call('HGET', KEYS[1], 'value') or ''}
`)
var writeScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'revision') ~= ARGV[1] then
 return 0
end
redis.call('HSET', KEYS[1], 'value', ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1
`)

// Read returns JSON and a revision used for a conditional fill; nil data means a miss.
func (c *Redis) Read(ctx context.Context, key string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	result, err := readScript.Run(ctx, c.client, []string{c.prefix + key}, uuid.NewString(), c.ttl.Milliseconds()).Slice()
	if err != nil {
		return nil, "", err
	}
	if len(result) != 2 {
		return nil, "", errors.New("unexpected Redis cache response")
	}
	revision, ok := result[0].(string)
	if !ok {
		return nil, "", errors.New("invalid Redis cache revision")
	}
	value, ok := result[1].(string)
	if !ok {
		return nil, "", errors.New("invalid Redis cache value")
	}
	if value == "" {
		return nil, revision, nil
	}
	return []byte(value), revision, nil
}

// Write fills a miss only if no mutation or expiration has invalidated its revision.
func (c *Redis) Write(ctx context.Context, key, revision string, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := writeScript.Run(ctx, c.client, []string{c.prefix + key}, revision, string(value), c.ttl.Milliseconds()).Err(); err != nil {
		return fmt.Errorf("fill lookup cache: %w", err)
	}
	return nil
}

// Invalidate removes a lookup after a successful database commit.
func (c *Redis) Invalidate(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	return c.client.Del(ctx, c.prefix+key).Err()
}
