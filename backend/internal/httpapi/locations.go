package httpapi

import (
	"net/http"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
)

// LocationHandler handles HTTP requests for storage locations.
type LocationHandler struct {
	Service *service.LocationService
}

// List returns the requested collection.
func (h *LocationHandler) List(w http.ResponseWriter, r *http.Request) error {
	locations, err := h.Service.List(r.Context())
	if err != nil {
		return err
	}
	respond(w, 200, "Locations retrieved successfully", locations, nil)
	return nil
}

// Create creates a resource from a JSON request.
func (h *LocationHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var in model.LocationInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	out, err := h.Service.Create(r.Context(), in)
	if err != nil {
		return err
	}
	respond(w, 201, "Location created successfully", out, nil)
	return nil
}

// Update replaces editable resource metadata.
func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	var in model.LocationInput
	if err = decode(w, r, &in); err != nil {
		return err
	}
	out, err := h.Service.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	respond(w, 200, "Location updated successfully", out, nil)
	return nil
}
