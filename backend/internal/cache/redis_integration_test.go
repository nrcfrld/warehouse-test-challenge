//go:build integration

package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
	"github.com/google/uuid"
)

func redisFixture(t *testing.T, ttl time.Duration) *Redis {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Fatal("TEST_REDIS_URL must point to a disposable Redis instance")
	}
	client, err := New(url, ttl)
	if err != nil {
		t.Fatal(err)
	}
	client.prefix = "warehouse:test:" + uuid.NewString() + ":"
	if err = client.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		keys, err := client.client.Keys(ctx, client.prefix+"*").Result()
		if err != nil {
			t.Error(err)
		}
		if len(keys) > 0 {
			if err = client.client.Del(ctx, keys...).Err(); err != nil {
				t.Error(err)
			}
		}
		if err = client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}
func TestRedisRevisionPreventsStaleFill(t *testing.T) {
	c := redisFixture(t, time.Minute)
	ctx := context.Background()
	raw, revision, err := c.Read(ctx, "locations")
	if err != nil || len(raw) != 0 || revision == "" {
		t.Fatal(string(raw), revision, err)
	}
	if err = c.Write(ctx, "locations", revision, []byte(`["fresh"]`)); err != nil {
		t.Fatal(err)
	}
	raw, _, err = c.Read(ctx, "locations")
	if err != nil || string(raw) != `["fresh"]` {
		t.Fatal(string(raw), err)
	}
	if err = c.Invalidate(ctx, "locations"); err != nil {
		t.Fatal(err)
	}
	// A query started before the mutation must not fill an invalidated entry.
	if err = c.Write(ctx, "locations", revision, []byte(`["stale"]`)); err != nil {
		t.Fatal(err)
	}
	raw, next, err := c.Read(ctx, "locations")
	if err != nil || len(raw) != 0 || next == revision || next == "" {
		t.Fatal(string(raw), next, err)
	}
	// A newer reader may already have filled the cache; the old reader still cannot overwrite it.
	if err = c.Write(ctx, "locations", next, []byte(`["newest"]`)); err != nil {
		t.Fatal(err)
	}
	if err = c.Write(ctx, "locations", revision, []byte(`["stale"]`)); err != nil {
		t.Fatal(err)
	}
	raw, _, err = c.Read(ctx, "locations")
	if err != nil || string(raw) != `["newest"]` {
		t.Fatal(string(raw), err)
	}
	ttl, err := c.client.PTTL(ctx, c.prefix+"locations").Result()
	if err != nil || ttl <= 0 || ttl > time.Minute {
		t.Fatal(ttl, err)
	}
}
func TestRedisExpiry(t *testing.T) {
	c := redisFixture(t, 200*time.Millisecond)
	ctx := context.Background()
	_, revision, err := c.Read(ctx, "categories")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Write(ctx, "categories", revision, []byte(`[]`)); err != nil {
		t.Fatal(err)
	}
	raw, _, err := c.Read(ctx, "categories")
	if err != nil || string(raw) != "[]" {
		t.Fatal("value was not cached before expiry", string(raw), err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		raw, next, e := c.Read(ctx, "categories")
		if e != nil {
			t.Fatal(e)
		}
		if len(raw) == 0 {
			if next == revision {
				t.Fatal("expiration reused a revision")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cache did not expire")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type lookupStore struct {
	service.LocationRepository
	locations []model.Location
	loads     int
	err       error
}

func (s *lookupStore) List(context.Context) ([]model.Location, error) {
	s.loads++
	return s.locations, s.err
}
func TestServiceWithRealRedis(t *testing.T) {
	c := redisFixture(t, time.Minute)
	ctx := context.Background()
	store := &lookupStore{locations: []model.Location{{ID: "id", Code: "A-01", Zone: "Main", Type: "Shelf"}}}
	svc := &service.LocationService{Repository: store, Cache: c}
	for range 2 {
		out, err := svc.List(ctx)
		if err != nil || len(out) != 1 || out[0].Code != "A-01" {
			t.Fatal(out, err)
		}
	}
	if store.loads != 1 {
		t.Fatal("cache hit queried the database", store.loads)
	}
	store.locations[0].Code = "B-01"
	if err := c.Invalidate(ctx, "locations"); err != nil {
		t.Fatal(err)
	}
	out, err := svc.List(ctx)
	if err != nil || out[0].Code != "B-01" || store.loads != 2 {
		t.Fatal(out, store.loads, err)
	}
	// Invalid cached JSON is repaired from the source of truth.
	if err = c.client.HSet(ctx, c.prefix+"locations", "value", "broken JSON").Err(); err != nil {
		t.Fatal(err)
	}
	out, err = svc.List(ctx)
	if err != nil || out[0].Code != "B-01" || store.loads != 3 {
		t.Fatal(out, store.loads, err)
	}
	// An unreachable Redis server must not make a database lookup fail.
	unavailable, err := New("redis://127.0.0.1:1/0", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := unavailable.Close(); e != nil {
			t.Error(e)
		}
	}()
	svc.Cache = unavailable
	start := time.Now()
	out, err = svc.List(ctx)
	if err != nil || out[0].Code != "B-01" || time.Since(start) > time.Second {
		t.Fatal("fallback failed or exceeded its bound", out, err)
	}
	store.err = errors.New("database failed")
	if _, err = svc.List(ctx); !errors.Is(err, store.err) {
		t.Fatal("cache hid a database error", err)
	}
}
