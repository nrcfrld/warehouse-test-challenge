// Package repository implements PostgreSQL persistence and transactions.
package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enrico/warehouse/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres owns the application's database connection pool.
type Postgres struct{ Pool *pgxpool.Pool }

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

const columns = "id::text, sku, name, category, unit, created_at"

func scanItem(row pgx.Row) (model.Item, error) {
	var x model.Item
	err := row.Scan(&x.ID, &x.SKU, &x.Name, &x.Category, &x.Unit, &x.CreatedAt)
	return x, translate(err)
}

// Create inserts an item with database-enforced SKU uniqueness.
func (p *Postgres) Create(ctx context.Context, in model.ItemInput) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "INSERT INTO items(sku,name,category,unit) VALUES($1,$2,$3,$4) RETURNING "+columns, in.SKU, in.Name, in.Category, in.Unit))
}

// Get finds an active item.
func (p *Postgres) Get(ctx context.Context, id string) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "SELECT "+columns+" FROM items WHERE id=$1 AND deleted_at IS NULL", id))
}

// Update replaces active item metadata.
func (p *Postgres) Update(ctx context.Context, id string, in model.ItemInput) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "UPDATE items SET sku=$2,name=$3,category=$4,unit=$5 WHERE id=$1 AND deleted_at IS NULL RETURNING "+columns, id, in.SKU, in.Name, in.Category, in.Unit))
}

// Delete takes the same row lock as Receive and preserves dependent records.
func (p *Postgres) Delete(ctx context.Context, id string) error {
	tag, err := p.Pool.Exec(ctx, "UPDATE items SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.NotFound("Item")
	}
	return nil
}
func literalSearch(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s) + "%"
}

// List returns a count and page from the same database snapshot.
func (p *Postgres) List(ctx context.Context, f model.Filter) ([]model.Item, int, error) {
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer rollback(ctx, tx)
	where := ` FROM items WHERE deleted_at IS NULL AND ($1='' OR name ILIKE $2 OR sku ILIKE $2) AND ($3='' OR category=$3)`
	var total int
	if err = tx.QueryRow(ctx, "SELECT count(*)"+where, f.Search, literalSearch(f.Search), f.Category).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.Query(ctx, "SELECT "+columns+where+" ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5", f.Search, literalSearch(f.Search), f.Category, f.Limit, (f.Page-1)*f.Limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []model.Item{}
	for rows.Next() {
		x, e := scanItem(rows)
		if e != nil {
			return nil, 0, e
		}
		items = append(items, x)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit(ctx)
}

// Categories returns all active categories independent of pagination.
func (p *Postgres) Categories(ctx context.Context) ([]string, error) {
	rows, err := p.Pool.Query(ctx, "SELECT DISTINCT category FROM items WHERE deleted_at IS NULL ORDER BY category")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var x string
		if err = rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Locations returns all storage locations.
func (p *Postgres) Locations(ctx context.Context) ([]model.Location, error) {
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

// Stock reads item metadata and balances from one consistent snapshot.
func (p *Postgres) Stock(ctx context.Context, id string) (model.StockSummary, error) {
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
		out.Locations = append(out.Locations, s)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// Receive atomically records an exact stock increment and an optional replay result.
func (p *Postgres) Receive(ctx context.Context, in model.ReceiveInput, key string) (model.Receipt, bool, error) {
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
func rollback(ctx context.Context, tx pgx.Tx) { // Rollback after commit is intentionally harmless.
	_ = tx.Rollback(ctx)
}
