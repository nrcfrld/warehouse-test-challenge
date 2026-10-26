package httpapi

import (
	"github.com/enrico/warehouse/internal/model"
	"net/http"
)

func (a *API) createLocation(w http.ResponseWriter, r *http.Request) error {
	var in model.LocationInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	out, err := a.Service.CreateLocation(r.Context(), in)
	if err != nil {
		return err
	}
	respond(w, 201, "Location created successfully", out, nil)
	return nil
}
func (a *API) updateLocation(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	var in model.LocationInput
	if err = decode(w, r, &in); err != nil {
		return err
	}
	out, err := a.Service.UpdateLocation(r.Context(), id, in)
	if err != nil {
		return err
	}
	respond(w, 200, "Location updated successfully", out, nil)
	return nil
}
func (a *API) transfer(w http.ResponseWriter, r *http.Request) error {
	var in model.TransferInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	out, replay, err := a.Service.Transfer(r.Context(), in, r.Header.Get("Idempotency-Key"))
	if err != nil {
		return err
	}
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	respond(w, 200, "Stock transferred successfully", out, nil)
	return nil
}
