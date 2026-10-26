package httpapi

import (
	"net/http"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
)

// StockHandler handles HTTP requests for stock balances, receipts, and transfers.
type StockHandler struct {
	Service *service.StockService
}

// Get returns the requested resource.
func (h *StockHandler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "item_id")
	if err != nil {
		return err
	}
	x, err := h.Service.Get(r.Context(), id)
	if err != nil {
		return err
	}
	respond(w, 200, "Stock retrieved successfully", x, nil)
	return nil
}

// Receive records a stock receipt with optional idempotency.
func (h *StockHandler) Receive(w http.ResponseWriter, r *http.Request) error {
	var in model.ReceiveInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	x, replay, err := h.Service.Receive(r.Context(), in, r.Header.Get("Idempotency-Key"))
	if err != nil {
		return err
	}
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	respond(w, 200, "Stock received successfully", x, nil)
	return nil
}

// Transfer moves stock between locations with optional idempotency.
func (h *StockHandler) Transfer(w http.ResponseWriter, r *http.Request) error {
	var in model.TransferInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	out, replay, err := h.Service.Transfer(r.Context(), in, r.Header.Get("Idempotency-Key"))
	if err != nil {
		return err
	}
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	respond(w, 200, "Stock transferred successfully", out, nil)
	return nil
}
