// Package model defines API data and domain invariants.
package model

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Item is an active warehouse master record.
type Item struct {
	ID        string    `json:"id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Unit      string    `json:"unit"`
	CreatedAt time.Time `json:"created_at"`
}

// ItemInput contains editable item metadata.
type ItemInput struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Unit     string `json:"unit"`
}

// Location identifies a storage position.
type Location struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Zone string `json:"zone"`
	Type string `json:"type"`
}

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

// Filter specifies stable server-side pagination.
type Filter struct {
	Search, Category string
	Page, Limit      int
}

// FieldError associates a validation error with an input.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Error is a safe, public domain error.
type Error struct {
	Status  int
	Message string
	Fields  []FieldError
}

func (e *Error) Error() string { return e.Message }

// Invalid builds a client-visible field validation error.
func Invalid(field, reason string) error {
	return &Error{Status: 400, Message: "Validation failed", Fields: []FieldError{{field, reason}}}
}

// NotFound identifies a missing or soft-deleted resource.
func NotFound(resource string) error { return &Error{Status: 404, Message: resource + " not found"} }

// Conflict identifies a domain conflict.
func Conflict(field, message string) error {
	return &Error{Status: 409, Message: message, Fields: []FieldError{{field, message}}}
}

// Public extracts a safe error through wrapping.
func Public(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Status: 500, Message: "An unexpected error occurred"}
}

var qtyPattern = regexp.MustCompile(`^[0-9]{1,15}(\.[0-9]{1,3})?$`)

// ParseQuantity parses a NUMERIC(18,3) value into exact thousandths.
func ParseQuantity(value string) (int64, error) {
	if !qtyPattern.MatchString(value) {
		return 0, Invalid("qty", "Use a positive decimal with up to 15 integer and 3 fractional digits")
	}
	parts := strings.SplitN(value, ".", 2)
	fraction := "000"
	if len(parts) == 2 {
		fraction = (parts[1] + "000")[:3]
	}
	n, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil {
		return 0, Invalid("qty", "Quantity is out of range")
	}
	return n, nil
}

// FormatQuantity formats exact thousandths without floating-point arithmetic.
func FormatQuantity(n int64) string { return fmt.Sprintf("%d.%03d", n/1000, n%1000) }

// NormalizeItem applies canonical whitespace and SKU casing.
func NormalizeItem(in ItemInput) ItemInput {
	return ItemInput{strings.ToUpper(strings.TrimSpace(in.SKU)), strings.TrimSpace(in.Name), strings.TrimSpace(in.Category), strings.TrimSpace(in.Unit)}
}

// ValidateItem enforces required fields and storage limits.
func ValidateItem(in ItemInput) error {
	fields := []struct {
		name, value string
		max         int
	}{{"sku", in.SKU, 64}, {"name", in.Name, 200}, {"category", in.Category, 100}, {"unit", in.Unit, 32}}
	issues := []FieldError{}
	for _, f := range fields {
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
