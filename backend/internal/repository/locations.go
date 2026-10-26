package repository

import (
	"context"
	"errors"

	"github.com/enrico/warehouse/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LocationRepository persists storage locations in PostgreSQL.
type LocationRepository struct{ Pool *pgxpool.Pool }

func scanLocation(row pgx.Row) (model.Location, error) {
	var out model.Location
	err := row.Scan(&out.ID, &out.Code, &out.Zone, &out.Type)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, model.NotFound("Location")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return out, model.Conflict("code", "Location code already exists")
	}
	return out, err
}

// List returns all storage locations.
func (p *LocationRepository) List(ctx context.Context) ([]model.Location, error) {
	rows, err := p.Pool.Query(ctx, "SELECT id::text,code,zone,type FROM locations ORDER BY code")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Location{}
	for rows.Next() {
		var x model.Location
		if err = rows.Scan(&x.ID, &x.Code, &x.Zone, &x.Type); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Create inserts a location with case-insensitive code uniqueness.
func (p *LocationRepository) Create(ctx context.Context, in model.LocationInput) (model.Location, error) {
	return scanLocation(p.Pool.QueryRow(ctx, `INSERT INTO locations(code,zone,type) VALUES($1,$2,$3) RETURNING id::text,code,zone,type`, in.Code, in.Zone, in.Type))
}

// Update edits metadata without altering its identity or stock history.
func (p *LocationRepository) Update(ctx context.Context, id string, in model.LocationInput) (model.Location, error) {
	return scanLocation(p.Pool.QueryRow(ctx, `UPDATE locations SET code=$2,zone=$3,type=$4 WHERE id=$1 RETURNING id::text,code,zone,type`, id, in.Code, in.Zone, in.Type))
}
