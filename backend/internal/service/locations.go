package service

import (
	"context"
	"log/slog"

	"github.com/enrico/warehouse/internal/model"
)

// LocationRepository is the persistence boundary for storage locations.
type LocationRepository interface {
	List(context.Context) ([]model.Location, error)
	Create(context.Context, model.LocationInput) (model.Location, error)
	Update(context.Context, string, model.LocationInput) (model.Location, error)
}

// LocationService owns location validation and lookup caching.
type LocationService struct {
	Repository LocationRepository
	Logger     *slog.Logger
	Cache      LookupCache
}

// List lists warehouse storage positions.
func (s *LocationService) List(ctx context.Context) ([]model.Location, error) {
	return cachedLookup(ctx, s.Cache, s.Logger, "locations", s.Repository.List)
}

// Create validates a new storage position.
func (s *LocationService) Create(ctx context.Context, in model.LocationInput) (model.Location, error) {
	in = model.NormalizeLocation(in)
	if err := model.ValidateLocation(in); err != nil {
		return model.Location{}, err
	}
	out, err := s.Repository.Create(ctx, in)
	if err == nil {
		invalidateLookup(ctx, s.Cache, s.Logger, "locations")
	}
	return out, err
}

// Update keeps the stable location identity and existing stock references.
func (s *LocationService) Update(ctx context.Context, id string, in model.LocationInput) (model.Location, error) {
	in = model.NormalizeLocation(in)
	if err := model.ValidateLocation(in); err != nil {
		return model.Location{}, err
	}
	out, err := s.Repository.Update(ctx, id, in)
	if err == nil {
		invalidateLookup(ctx, s.Cache, s.Logger, "locations")
	}
	return out, err
}
