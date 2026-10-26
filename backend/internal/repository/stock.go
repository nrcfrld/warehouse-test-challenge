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
	"github.com/jackc/pgx/v5/pgxpool"
)

// StockRepository owns atomic stock transactions, movements, and idempotency records.
type StockRepository struct{ Pool *pgxpool.Pool }

// Get reads item metadata and balances from one consistent snapshot.
func (p *StockRepository) Get(ctx context.Context, id string) (model.StockSummary, error) {
	out := model.StockSummary{ItemID: id, Locations: []model.Stock{}}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer rollback(ctx, tx)
	if err = tx.QueryRow(ctx, "SELECT unit FROM items WHERE id=$1 AND deleted_at IS NULL", id).Scan(&out.Unit); err != nil {
		return out, translate(err)
	}
	if err = tx.QueryRow(ctx, "SELECT coalesce(sum(qty),0.000)::text FROM stocks WHERE item_id=$1", id).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,l.id::text,l.code,l.zone,l.type,s.qty::text,s.updated_at FROM stocks s JOIN locations l ON l.id=s.location_id WHERE s.item_id=$1 ORDER BY l.code`, id)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var s model.Stock
		if err = rows.Scan(&s.ID, &s.Location.ID, &s.Location.Code, &s.Location.Zone, &s.Location.Type, &s.Qty, &s.UpdatedAt); err != nil {
			return out, err
		}
		s.UpdatedAt = s.UpdatedAt.UTC()
		out.Locations = append(out.Locations, s)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// Receive atomically records an exact stock increment and an optional replay result.
func (p *StockRepository) Receive(ctx context.Context, in model.ReceiveInput, key string) (model.Receipt, bool, error) {
	var result model.Receipt
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return result, false, err
	}
	defer rollback(ctx, tx)
	raw, err := json.Marshal(in)
	if err != nil {
		return result, false, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if key != "" {
		// A transaction-scoped lock also serializes requests for keys not yet inserted.
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
			result.CreatedAt = result.CreatedAt.UTC()
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
	if err = tx.QueryRow(ctx, "SELECT id::text FROM locations WHERE id=$1", in.LocationID).Scan(&id); errors.Is(err, pgx.ErrNoRows) {
		return result, false, model.NotFound("Location")
	} else if err != nil {
		return result, false, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO stocks(item_id,location_id,qty) VALUES($1,$2,$3::numeric) ON CONFLICT(item_id,location_id) DO UPDATE SET qty=stocks.qty+EXCLUDED.qty,updated_at=now() RETURNING qty::text`, in.ItemID, in.LocationID, in.Qty).Scan(&result.Balance)
	if err != nil {
		return result, false, translate(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO stock_movements(item_id,location_id,qty) VALUES($1,$2,$3::numeric) RETURNING id::text,item_id::text,location_id::text,qty::text,created_at`, in.ItemID, in.LocationID, in.Qty).Scan(&result.ID, &result.ItemID, &result.LocationID, &result.Qty, &result.CreatedAt)
	if err != nil {
		return result, false, err
	}
	result.CreatedAt = result.CreatedAt.UTC()
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
		return result, false, fmt.Errorf("commit receipt: %w", err)
	}
	return result, false, nil
}

// Transfer serializes all stock operations on the item, preserving totals and audit history.
func (p *StockRepository) Transfer(ctx context.Context, in model.TransferInput, key string) (model.Transfer, bool, error) {
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
			result.CreatedAt = result.CreatedAt.UTC()
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
	result.CreatedAt = result.CreatedAt.UTC()
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
