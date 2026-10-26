package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/enrico/warehouse/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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

// CreateLocation inserts a location with case-insensitive code uniqueness.
func (p *Postgres) CreateLocation(ctx context.Context, in model.LocationInput) (model.Location, error) {
	return scanLocation(p.Pool.QueryRow(ctx, `INSERT INTO locations(code,zone,type) VALUES($1,$2,$3) RETURNING id::text,code,zone,type`, in.Code, in.Zone, in.Type))
}

// UpdateLocation edits metadata without altering its identity or stock history.
func (p *Postgres) UpdateLocation(ctx context.Context, id string, in model.LocationInput) (model.Location, error) {
	return scanLocation(p.Pool.QueryRow(ctx, `UPDATE locations SET code=$2,zone=$3,type=$4 WHERE id=$1 RETURNING id::text,code,zone,type`, id, in.Code, in.Zone, in.Type))
}

// Transfer serializes all stock operations on the item, preserving totals and audit history.
func (p *Postgres) Transfer(ctx context.Context, in model.TransferInput, key string) (model.Transfer, bool, error) {
	var result model.Transfer
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, false, err
	}
	defer rollback(ctx, tx)
	raw, err := json.Marshal(in)
	if err != nil {
		return result, false, err
	}
	sum := sha256.Sum256(append([]byte("transfer:"), raw...))
	hash := hex.EncodeToString(sum[:])
	if key != "" {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return result, false, err
		}
		var storedHash string
		var stored []byte
		err = tx.QueryRow(ctx, "SELECT payload_hash,response FROM idempotency_records WHERE key=$1", key).Scan(&storedHash, &stored)
		if err == nil {
			if storedHash != hash {
				return result, false, model.Conflict("idempotency_key", "Idempotency key was already used for another stock operation")
			}
			if err = json.Unmarshal(stored, &result); err != nil {
				return result, false, err
			}
			return result, true, tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return result, false, err
		}
	}
	var id string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM items WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", in.ItemID).Scan(&id); err != nil {
		return result, false, translate(err)
	}
	for _, locationID := range []string{in.FromLocationID, in.ToLocationID} {
		if err = tx.QueryRow(ctx, "SELECT id::text FROM locations WHERE id=$1", locationID).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
			return result, false, model.NotFound("Location")
		} else if err != nil {
			return result, false, err
		}
	}
	err = tx.QueryRow(ctx, `UPDATE stocks SET qty=qty-$3::numeric,updated_at=now() WHERE item_id=$1 AND location_id=$2 AND qty >= $3::numeric RETURNING qty::text`, in.ItemID, in.FromLocationID, in.Qty).Scan(&result.SourceBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, false, model.Conflict("qty", "Insufficient stock at the source location")
	}
	if err != nil {
		return result, false, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO stocks(item_id,location_id,qty) VALUES($1,$2,$3::numeric) ON CONFLICT(item_id,location_id) DO UPDATE SET qty=stocks.qty+EXCLUDED.qty,updated_at=now() RETURNING qty::text`, in.ItemID, in.ToLocationID, in.Qty).Scan(&result.DestinationBalance)
	if err != nil {
		return result, false, translate(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO stock_movements(item_id,location_id,from_location_id,qty,kind) VALUES($1,$2,$3,$4::numeric,'transfer') RETURNING id::text,item_id::text,from_location_id::text,location_id::text,qty::text,created_at`, in.ItemID, in.ToLocationID, in.FromLocationID, in.Qty).Scan(&result.ID, &result.ItemID, &result.FromLocationID, &result.ToLocationID, &result.Qty, &result.CreatedAt)
	if err != nil {
		return result, false, err
	}
	if key != "" {
		body, e := json.Marshal(result)
		if e != nil {
			return result, false, e
		}
		if _, err = tx.Exec(ctx, "INSERT INTO idempotency_records(key,payload_hash,response) VALUES($1,$2,$3)", key, hash, body); err != nil {
			return result, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, false, fmt.Errorf("commit transfer: %w", err)
	}
	return result, false, nil
}
