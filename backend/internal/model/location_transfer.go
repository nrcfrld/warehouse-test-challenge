package model

import (
	"fmt"
	"strings"
	"time"
)

// LocationInput describes editable storage metadata.
type LocationInput struct {
	Code string `json:"code"`
	Zone string `json:"zone"`
	Type string `json:"type"`
}

// NormalizeLocation canonicalizes location codes and whitespace.
func NormalizeLocation(in LocationInput) LocationInput {
	return LocationInput{strings.ToUpper(strings.TrimSpace(in.Code)), strings.TrimSpace(in.Zone), strings.TrimSpace(in.Type)}
}

// ValidateLocation checks required metadata and database limits.
func ValidateLocation(in LocationInput) error {
	issues := []FieldError{}
	for _, f := range []struct {
		name, value string
		max         int
	}{{"code", in.Code, 32}, {"zone", in.Zone, 100}, {"type", in.Type, 50}} {
		if strings.TrimSpace(f.value) == "" {
			issues = append(issues, FieldError{f.name, "This field is required"})
		} else if len([]rune(f.value)) > f.max {
			issues = append(issues, FieldError{f.name, fmt.Sprintf("Use at most %d characters", f.max)})
		}
	}
	if len(issues) > 0 {
		return &Error{Status: 400, Message: "Validation failed", Fields: issues}
	}
	return nil
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
