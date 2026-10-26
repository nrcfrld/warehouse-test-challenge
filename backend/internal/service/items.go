package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/enrico/warehouse/internal/model"
)

// ItemRepository is the persistence boundary for item metadata and categories.
type ItemRepository interface {
	Create(context.Context, model.ItemInput) (model.Item, error)
	Get(context.Context, string) (model.Item, error)
	Update(context.Context, string, model.ItemInput) (model.Item, error)
	Delete(context.Context, string) error
	List(context.Context, model.Filter) ([]model.Item, int, error)
	Categories(context.Context) ([]string, error)
}

// ItemService owns item validation, metadata, and category cache invalidation.
type ItemService struct {
	Repository ItemRepository
	Logger     *slog.Logger
	Cache      LookupCache
}

// Create normalizes and validates a new item.
func (s *ItemService) Create(ctx context.Context, in model.ItemInput) (model.Item, error) {
	in = model.NormalizeItem(in)
	if err := model.ValidateItem(in); err != nil {
		return model.Item{}, err
	}
	out, err := s.Repository.Create(ctx, in)
	if err == nil {
		invalidateLookup(ctx, s.Cache, s.Logger, "categories")
	}
	return out, err
}

// Get retrieves an active item.
func (s *ItemService) Get(ctx context.Context, id string) (model.Item, error) {
	return s.Repository.Get(ctx, id)
}

// List retrieves a consistent page of active items.
func (s *ItemService) List(ctx context.Context, filter model.Filter) ([]model.Item, int, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Category = strings.TrimSpace(filter.Category)
	for _, field := range []struct {
		name       string
		value, max int
	}{{"page", filter.Page, 1000000}, {"limit", filter.Limit, 100}} {
		if field.value < 1 || field.value > field.max {
			return nil, 0, model.Invalid(field.name, "Value is outside the supported range")
		}
	}
	if len([]rune(filter.Search)) > 200 || len([]rune(filter.Category)) > 100 {
		return nil, 0, model.Invalid("filter", "Filter is too long")
	}
	return s.Repository.List(ctx, filter)
}

// Update validates replacement metadata.
func (s *ItemService) Update(ctx context.Context, id string, in model.ItemInput) (model.Item, error) {
	in = model.NormalizeItem(in)
	if err := model.ValidateItem(in); err != nil {
		return model.Item{}, err
	}
	out, err := s.Repository.Update(ctx, id, in)
	if err == nil {
		invalidateLookup(ctx, s.Cache, s.Logger, "categories")
	}
	return out, err
}

// Delete soft-deletes an item without removing its stock or history.
func (s *ItemService) Delete(ctx context.Context, id string) error {
	err := s.Repository.Delete(ctx, id)
	if err == nil {
		invalidateLookup(ctx, s.Cache, s.Logger, "categories")
	}
	return err
}

// Categories lists categories belonging to active items.
func (s *ItemService) Categories(ctx context.Context) ([]string, error) {
	return cachedLookup(ctx, s.Cache, s.Logger, "categories", s.Repository.Categories)
}
