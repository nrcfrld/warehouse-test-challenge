// Package repository implements PostgreSQL persistence and transactions.
package repository

import (
	"context"
	"errors"

	"github.com/enrico/warehouse/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func translate(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return model.NotFound("Item")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return model.Conflict("sku", "SKU already exists")
		case "22003":
			return model.Conflict("qty", "Resulting stock exceeds the supported range")
		}
	}
	return err
}

func rollback(ctx context.Context, tx pgx.Tx) { // Rollback after commit is intentionally harmless.
	_ = tx.Rollback(ctx)
}
