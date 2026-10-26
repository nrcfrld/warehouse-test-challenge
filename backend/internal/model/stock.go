package model

import "time"

// Stock describes a location balance.
type Stock struct {
	ID        string    `json:"id"`
	Location  Location  `json:"location"`
	Qty       string    `json:"qty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StockSummary contains balances in the item's unit.
type StockSummary struct {
	ItemID    string  `json:"item_id"`
	Unit      string  `json:"unit"`
	Total     string  `json:"total"`
	Locations []Stock `json:"locations"`
}

// ReceiveInput is a positive stock receipt.
type ReceiveInput struct {
	ItemID     string `json:"item_id"`
	LocationID string `json:"location_id"`
	Qty        string `json:"qty"`
}

// Receipt is the persisted result of a stock mutation.
type Receipt struct {
	ID         string    `json:"id"`
	ItemID     string    `json:"item_id"`
	LocationID string    `json:"location_id"`
	Qty        string    `json:"qty"`
	Balance    string    `json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
}

// TransferInput moves an exact quantity between distinct locations.
type TransferInput struct {
	ItemID         string `json:"item_id"`
	FromLocationID string `json:"from_location_id"`
	ToLocationID   string `json:"to_location_id"`
	Qty            string `json:"qty"`
}

// Transfer is the committed movement and balances at the time of transfer.
type Transfer struct {
	ID                 string    `json:"id"`
	ItemID             string    `json:"item_id"`
	FromLocationID     string    `json:"from_location_id"`
	ToLocationID       string    `json:"to_location_id"`
	Qty                string    `json:"qty"`
	SourceBalance      string    `json:"source_balance"`
	DestinationBalance string    `json:"destination_balance"`
	CreatedAt          time.Time `json:"created_at"`
}
