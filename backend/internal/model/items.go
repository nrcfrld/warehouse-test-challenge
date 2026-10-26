package model

import (
	"fmt"
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

// Filter specifies stable server-side pagination.
type Filter struct {
	Search, Category string
	Page, Limit      int
}

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
