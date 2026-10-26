package service

import (
	"context"
	"github.com/enrico/warehouse/internal/model"
	"github.com/google/uuid"
)

// CreateLocation validates a new storage position.
func (s *Service) CreateLocation(ctx context.Context, in model.LocationInput) (model.Location, error) {
	in = model.NormalizeLocation(in)
	if err := model.ValidateLocation(in); err != nil {
		return model.Location{}, err
	}
	out, err := s.Store.CreateLocation(ctx, in)
	if err == nil {
		s.invalidateLookup(ctx, "locations")
	}
	return out, err
}

// UpdateLocation keeps the stable location identity and existing stock references.
func (s *Service) UpdateLocation(ctx context.Context, id string, in model.LocationInput) (model.Location, error) {
	in = model.NormalizeLocation(in)
	if err := model.ValidateLocation(in); err != nil {
		return model.Location{}, err
	}
	out, err := s.Store.UpdateLocation(ctx, id, in)
	if err == nil {
		s.invalidateLookup(ctx, "locations")
	}
	return out, err
}

// Transfer validates the request before the store enforces balances under lock.
func (s *Service) Transfer(ctx context.Context, in model.TransferInput, key string) (model.Transfer, bool, error) {
	n, err := model.ParseQuantity(in.Qty)
	if err != nil {
		return model.Transfer{}, false, err
	}
	if n <= 0 {
		return model.Transfer{}, false, model.Invalid("qty", "Quantity must be greater than zero")
	}
	in.Qty = model.FormatQuantity(n)
	for _, f := range []struct {
		name  string
		value *string
	}{{"item_id", &in.ItemID}, {"from_location_id", &in.FromLocationID}, {"to_location_id", &in.ToLocationID}} {
		id, e := uuid.Parse(*f.value)
		if e != nil {
			return model.Transfer{}, false, model.Invalid(f.name, "Use a valid UUID")
		}
		*f.value = id.String()
	}
	if in.FromLocationID == in.ToLocationID {
		return model.Transfer{}, false, model.Invalid("to_location_id", "Choose a different destination location")
	}
	if len(key) > 128 {
		return model.Transfer{}, false, model.Invalid("idempotency_key", "Use at most 128 characters")
	}
	result, replay, err := s.Store.Transfer(ctx, in, key)
	if err == nil && !replay {
		s.Logger.InfoContext(ctx, "stock.transferred", "transfer_id", result.ID, "item_id", result.ItemID, "from_location_id", result.FromLocationID, "to_location_id", result.ToLocationID, "qty", result.Qty)
	}
	return result, replay, err
}
