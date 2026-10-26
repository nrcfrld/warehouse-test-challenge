package service

import (
	"strings"

	"github.com/enrico/warehouse/internal/model"
	"github.com/google/uuid"
)

func positiveQuantity(value string) (string, error) {
	n, err := model.ParseQuantity(value)
	if err != nil {
		return "", err
	}
	if n <= 0 {
		return "", model.Invalid("qty", "Quantity must be greater than zero")
	}
	return model.FormatQuantity(n), nil
}

func stockIdentifier(value, field string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", model.Invalid(field, "Use a valid UUID")
	}
	return id.String(), nil
}

func idempotencyKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if len(key) > 128 {
		return "", model.Invalid("idempotency_key", "Use at most 128 characters")
	}
	return key, nil
}
