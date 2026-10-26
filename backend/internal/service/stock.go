package service

import (
	"context"
	"log/slog"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/requestlog"
)

// StockRepository performs stock reads and atomic stock mutations.
type StockRepository interface {
	Get(context.Context, string) (model.StockSummary, error)
	Receive(context.Context, model.ReceiveInput, string) (model.Receipt, bool, error)
	Transfer(context.Context, model.TransferInput, string) (model.Transfer, bool, error)
}

// StockService owns stock receipt and transfer rules, independent of lookup caches.
type StockService struct {
	Repository StockRepository
	Logger     *slog.Logger
}

// Get retrieves an active item's balances.
func (s *StockService) Get(ctx context.Context, id string) (model.StockSummary, error) {
	return s.Repository.Get(ctx, id)
}

// Receive checks positive quantities even when called outside the HTTP handler.
func (s *StockService) Receive(ctx context.Context, in model.ReceiveInput, key string) (model.Receipt, bool, error) {
	var err error
	in.Qty, err = positiveQuantity(in.Qty)
	if err != nil {
		return model.Receipt{}, false, err
	}
	in.ItemID, err = stockIdentifier(in.ItemID, "item_id")
	if err != nil {
		return model.Receipt{}, false, err
	}
	in.LocationID, err = stockIdentifier(in.LocationID, "location_id")
	if err != nil {
		return model.Receipt{}, false, err
	}
	key, err = idempotencyKey(key)
	if err != nil {
		return model.Receipt{}, false, err
	}
	receipt, replay, err := s.Repository.Receive(ctx, in, key)
	if err == nil && !replay {
		requestlog.Logger(ctx, s.Logger).InfoContext(ctx, "stock.received", "receipt_id", receipt.ID, "item_id", receipt.ItemID, "location_id", receipt.LocationID, "qty", receipt.Qty, "balance", receipt.Balance)
	}
	return receipt, replay, err
}

// Transfer validates the request before the repository enforces balances under lock.
func (s *StockService) Transfer(ctx context.Context, in model.TransferInput, key string) (model.Transfer, bool, error) {
	var err error
	in.Qty, err = positiveQuantity(in.Qty)
	if err != nil {
		return model.Transfer{}, false, err
	}
	for _, f := range []struct {
		name  string
		value *string
	}{{"item_id", &in.ItemID}, {"from_location_id", &in.FromLocationID}, {"to_location_id", &in.ToLocationID}} {
		id, e := stockIdentifier(*f.value, f.name)
		if e != nil {
			return model.Transfer{}, false, e
		}
		*f.value = id
	}
	if in.FromLocationID == in.ToLocationID {
		return model.Transfer{}, false, model.Invalid("to_location_id", "Choose a different destination location")
	}
	key, err = idempotencyKey(key)
	if err != nil {
		return model.Transfer{}, false, err
	}
	result, replay, err := s.Repository.Transfer(ctx, in, key)
	if err == nil && !replay {
		requestlog.Logger(ctx, s.Logger).InfoContext(ctx, "stock.transferred", "transfer_id", result.ID, "item_id", result.ItemID, "from_location_id", result.FromLocationID, "to_location_id", result.ToLocationID, "qty", result.Qty)
	}
	return result, replay, err
}
