package httpapi

import (
	"net/http"
	"strconv"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
)

// ItemHandler handles HTTP requests for items and categories.
type ItemHandler struct {
	Service *service.ItemService
}

// List returns the requested collection.
func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := model.Filter{Search: q.Get("search"), Category: q.Get("category"), Page: 1, Limit: 10}
	for _, v := range []struct {
		name   string
		target *int
	}{{"page", &f.Page}, {"limit", &f.Limit}} {
		if q.Has(v.name) {
			n, err := strconv.Atoi(q.Get(v.name))
			if err != nil {
				return model.Invalid(v.name, "Value is outside the supported range")
			}
			*v.target = n
		}
	}
	items, total, err := h.Service.List(r.Context(), f)
	if err != nil {
		return err
	}
	respond(w, 200, "Items retrieved successfully", items, map[string]int{"page": f.Page, "limit": f.Limit, "total": total})
	return nil
}

// Create creates a resource from a JSON request.
func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) error {
	var in model.ItemInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	x, err := h.Service.Create(r.Context(), in)
	if err != nil {
		return err
	}
	respond(w, 201, "Item created successfully", x, nil)
	return nil
}

// Get returns the requested resource.
func (h *ItemHandler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	x, err := h.Service.Get(r.Context(), id)
	if err != nil {
		return err
	}
	respond(w, 200, "Item retrieved successfully", x, nil)
	return nil
}

// Update replaces editable resource metadata.
func (h *ItemHandler) Update(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	var in model.ItemInput
	if err = decode(w, r, &in); err != nil {
		return err
	}
	x, err := h.Service.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	respond(w, 200, "Item updated successfully", x, nil)
	return nil
}

// Delete soft-deletes an item while retaining stock and history.
func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	if err = h.Service.Delete(r.Context(), id); err != nil {
		return err
	}
	respond(w, 200, "Item deleted successfully", map[string]string{"id": id}, nil)
	return nil
}

// Categories returns all active item categories.
func (h *ItemHandler) Categories(w http.ResponseWriter, r *http.Request) error {
	categories, err := h.Service.Categories(r.Context())
	if err != nil {
		return err
	}
	respond(w, 200, "Categories retrieved successfully", categories, nil)
	return nil
}
