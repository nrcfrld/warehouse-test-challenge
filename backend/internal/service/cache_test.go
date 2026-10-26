package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type fakeLookupCache struct {
	raw                              []byte
	revision                         string
	readErr, writeErr, invalidateErr error
	readKey, writeKey, writeRevision string
	written                          []byte
	invalidated                      []string
	invalidateCanceled               bool
}

func (c *fakeLookupCache) Read(_ context.Context, key string) ([]byte, string, error) {
	c.readKey = key
	return c.raw, c.revision, c.readErr
}
func (c *fakeLookupCache) Write(_ context.Context, key, revision string, raw []byte) error {
	c.writeKey = key
	c.writeRevision = revision
	c.written = raw
	return c.writeErr
}
func (c *fakeLookupCache) Invalidate(ctx context.Context, key string) error {
	c.invalidated = append(c.invalidated, key)
	c.invalidateCanceled = ctx.Err() != nil
	return c.invalidateErr
}

func TestLookupCacheFallback(t *testing.T) {
	unavailable := errors.New("Redis unavailable")
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name          string
		cache         fakeLookupCache
		loadErr       error
		want          []string
		loads, writes int
	}{
		{name: "hit", cache: fakeLookupCache{raw: []byte(`["Cached"]`), revision: "r1"}, want: []string{"Cached"}},
		{name: "miss", cache: fakeLookupCache{revision: "r1"}, want: []string{"Fresh"}, loads: 1, writes: 1},
		{name: "read failure", cache: fakeLookupCache{readErr: unavailable}, want: []string{"Fresh"}, loads: 1},
		{name: "write failure", cache: fakeLookupCache{revision: "r1", writeErr: unavailable}, want: []string{"Fresh"}, loads: 1, writes: 1},
		{name: "corrupt JSON", cache: fakeLookupCache{raw: []byte(`broken`), revision: "r1"}, want: []string{"Fresh"}, loads: 1, writes: 1},
		{name: "null", cache: fakeLookupCache{raw: []byte(`null`), revision: "r1"}, want: []string{"Fresh"}, loads: 1, writes: 1},
		{name: "database failure", cache: fakeLookupCache{revision: "r1"}, loadErr: databaseErr, loads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			loads := 0
			value, err := cachedLookup(context.Background(), &tc.cache, logger, "categories", func(context.Context) ([]string, error) {
				loads++
				if tc.loadErr != nil {
					return nil, tc.loadErr
				}
				return []string{"Fresh"}, nil
			})
			if !errors.Is(err, tc.loadErr) || !reflect.DeepEqual(value, tc.want) || loads != tc.loads {
				t.Fatalf("value=%v loads=%d err=%v", value, loads, err)
			}
			if (tc.cache.written != nil) != (tc.writes == 1) {
				t.Fatal("incorrect fill", string(tc.cache.written))
			}
			if tc.writes == 1 && (tc.cache.writeKey != "categories" || tc.cache.writeRevision != "r1") {
				t.Fatal("wrong revision or namespace")
			}
		})
	}
}

type cacheRepositoryState struct {
	err   error
	loads int
}
type itemCacheRepository struct {
	ItemRepository
	state *cacheRepositoryState
}

func (r *itemCacheRepository) Create(context.Context, model.ItemInput) (model.Item, error) {
	return model.Item{}, r.state.err
}
func (r *itemCacheRepository) Update(context.Context, string, model.ItemInput) (model.Item, error) {
	return model.Item{}, r.state.err
}
func (r *itemCacheRepository) Delete(context.Context, string) error { return r.state.err }
func (r *itemCacheRepository) Categories(context.Context) ([]string, error) {
	r.state.loads++
	return []string{"General"}, r.state.err
}

type locationCacheRepository struct {
	LocationRepository
	state *cacheRepositoryState
}

func (r *locationCacheRepository) Create(context.Context, model.LocationInput) (model.Location, error) {
	return model.Location{}, r.state.err
}
func (r *locationCacheRepository) Update(context.Context, string, model.LocationInput) (model.Location, error) {
	return model.Location{}, r.state.err
}
func (r *locationCacheRepository) List(context.Context) ([]model.Location, error) {
	r.state.loads++
	return []model.Location{{ID: "id", Code: "A-01", Zone: "Main", Type: "Shelf"}}, r.state.err
}

func TestSuccessfulMutationsInvalidateAfterCommit(t *testing.T) {
	item := model.ItemInput{SKU: "SKU", Name: "Item", Category: "General", Unit: "pcs"}
	location := model.LocationInput{Code: "A", Zone: "Main", Type: "Shelf"}
	for _, tc := range []struct {
		name, key string
		run       func(context.Context, *ItemService, *LocationService) error
	}{
		{"create item", "categories", func(ctx context.Context, s *ItemService, _ *LocationService) error {
			_, err := s.Create(ctx, item)
			return err
		}},
		{"update item", "categories", func(ctx context.Context, s *ItemService, _ *LocationService) error {
			_, err := s.Update(ctx, "id", item)
			return err
		}},
		{"delete item", "categories", func(ctx context.Context, s *ItemService, _ *LocationService) error { return s.Delete(ctx, "id") }},
		{"create location", "locations", func(ctx context.Context, _ *ItemService, s *LocationService) error {
			_, err := s.Create(ctx, location)
			return err
		}},
		{"update location", "locations", func(ctx context.Context, _ *ItemService, s *LocationService) error {
			_, err := s.Update(ctx, "id", location)
			return err
		}},
	} {
		for _, fail := range []bool{false, true} {
			t.Run(tc.name+"/failure="+fmtBool(fail), func(t *testing.T) {
				store := &cacheRepositoryState{}
				if fail {
					store.err = errors.New("database rejected write")
				}
				cache := &fakeLookupCache{invalidateErr: errors.New("Redis unavailable")}
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				itemService := &ItemService{Repository: &itemCacheRepository{state: store}, Cache: cache, Logger: logger}
				locationService := &LocationService{Repository: &locationCacheRepository{state: store}, Cache: cache, Logger: logger}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				err := tc.run(ctx, itemService, locationService)
				if !errors.Is(err, store.err) {
					t.Fatal("cache failure changed the mutation result", err)
				}
				if fail && len(cache.invalidated) != 0 {
					t.Fatal("failed mutation invalidated cache")
				}
				if !fail && (!reflect.DeepEqual(cache.invalidated, []string{tc.key}) || cache.invalidateCanceled) {
					t.Fatal(cache.invalidated, cache.invalidateCanceled)
				}
			})
		}
	}
}
func fmtBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func TestLookupCacheOptionalAndTyped(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmtBool(enabled), func(t *testing.T) {
			store := &cacheRepositoryState{}
			itemService := &ItemService{Repository: &itemCacheRepository{state: store}}
			locationService := &LocationService{Repository: &locationCacheRepository{state: store}}
			cache := &fakeLookupCache{revision: "r1"}
			if enabled {
				itemService.Cache = cache
				locationService.Cache = cache
			}
			locations, err := locationService.List(context.Background())
			if err != nil || len(locations) != 1 || locations[0].Code != "A-01" {
				t.Fatal(locations, err)
			}
			if enabled && cache.readKey != "locations" {
				t.Fatal(cache.readKey)
			}
			categories, err := itemService.Categories(context.Background())
			if err != nil || len(categories) != 1 || categories[0] != "General" {
				t.Fatal(categories, err)
			}
			if enabled && cache.readKey != "categories" {
				t.Fatal(cache.readKey)
			}
			if store.loads != 2 {
				t.Fatal(store.loads)
			}
		})
	}
}
