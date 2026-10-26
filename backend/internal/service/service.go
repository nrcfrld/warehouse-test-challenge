// Package service enforces warehouse business rules independently of HTTP.
package service

import (
	"context"
	"log/slog"

	"github.com/enrico/warehouse/internal/model"
	"github.com/google/uuid"
)

// Store is the persistence boundary used by the application.
type Store interface {
	Create(context.Context, model.ItemInput) (model.Item, error)
	Get(context.Context, string) (model.Item, error)
	Update(context.Context, string, model.ItemInput) (model.Item, error)
	Delete(context.Context, string) error
	List(context.Context, model.Filter) ([]model.Item, int, error)
	Categories(context.Context) ([]string, error)
	Locations(context.Context) ([]model.Location, error)
	CreateLocation(context.Context, model.LocationInput) (model.Location, error)
	UpdateLocation(context.Context, string, model.LocationInput) (model.Location, error)
	Transfer(context.Context, model.TransferInput, string) (model.Transfer, bool, error)
	Stock(context.Context, string) (model.StockSummary, error)
	Receive(context.Context, model.ReceiveInput, string) (model.Receipt, bool, error)
}

// Service coordinates validated domain operations.
type Service struct {
	Store  Store
	Logger *slog.Logger
	Cache  LookupCache
}

// Create normalizes and validates a new item.
func (s *Service) Create(ctx context.Context, in model.ItemInput) (model.Item, error) {
	in = model.NormalizeItem(in)
	if err := model.ValidateItem(in); err != nil {
		return model.Item{}, err
	}
	out, err := s.Store.Create(ctx, in)
	if err == nil {
		s.invalidateLookup(ctx, "categories")
	}
	return out, err
}

// Update validates replacement metadata.
func (s *Service) Update(ctx context.Context, id string, in model.ItemInput) (model.Item, error) {
	in = model.NormalizeItem(in)
	if err := model.ValidateItem(in); err != nil {
		return model.Item{}, err
	}
	out, err := s.Store.Update(ctx, id, in)
	if err == nil {
		s.invalidateLookup(ctx, "categories")
	}
	return out, err
}

// Receive checks positive quantities even when called outside the HTTP handler.
func (s *Service) Receive(ctx context.Context, in model.ReceiveInput, key string) (model.Receipt, bool, error) {
	n, err := model.ParseQuantity(in.Qty)
	if err != nil {
		return model.Receipt{}, false, err
	}
	if n <= 0 {
		return model.Receipt{}, false, model.Invalid("qty", "Quantity must be greater than zero")
	}
	in.Qty = model.FormatQuantity(n)
	itemID, err := uuid.Parse(in.ItemID)
	if err != nil {
		return model.Receipt{}, false, model.Invalid("item_id", "Use a valid UUID")
	}
	in.ItemID = itemID.String()
	locationID, err := uuid.Parse(in.LocationID)
	if err != nil {
		return model.Receipt{}, false, model.Invalid("location_id", "Use a valid UUID")
	}
	in.LocationID = locationID.String()
	if len(key) > 128 {
		return model.Receipt{}, false, model.Invalid("idempotency_key", "Use at most 128 characters")
	}
	receipt, replay, err := s.Store.Receive(ctx, in, key)
	if err == nil && !replay {
		s.Logger.InfoContext(ctx, "stock.received", "receipt_id", receipt.ID, "item_id", receipt.ItemID, "location_id", receipt.LocationID, "qty", receipt.Qty, "balance", receipt.Balance)
	}
	return receipt, replay, err
}

// Get retrieves an active item.
func (s *Service) Get(ctx context.Context, id string) (model.Item, error) {
	return s.Store.Get(ctx, id)
}

// List retrieves a consistent page of active items.
func (s *Service) List(ctx context.Context, filter model.Filter) ([]model.Item, int, error) {
	return s.Store.List(ctx, filter)
}

// Delete soft-deletes an item without removing its stock or history.
func (s *Service) Delete(ctx context.Context, id string) error {
	err := s.Store.Delete(ctx, id)
	if err == nil {
		s.invalidateLookup(ctx, "categories")
	}
	return err
}

// Categories lists categories belonging to active items.
func (s *Service) Categories(ctx context.Context) ([]string, error) {
	return cachedLookup(ctx, s, "categories", s.Store.Categories)
}

// Locations lists warehouse storage positions.
func (s *Service) Locations(ctx context.Context) ([]model.Location, error) {
	return cachedLookup(ctx, s, "locations", s.Store.Locations)
}

// Stock retrieves an active item's balances.
func (s *Service) Stock(ctx context.Context, id string) (model.StockSummary, error) {
	return s.Store.Stock(ctx, id)
}
