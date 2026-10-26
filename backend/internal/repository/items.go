package repository

import (
	"context"
	"strings"

	"github.com/enrico/warehouse/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ItemRepository persists item metadata and active categories in PostgreSQL.
type ItemRepository struct{ Pool *pgxpool.Pool }

const itemColumns = "id::text, sku, name, category, unit, created_at"

func scanItem(row pgx.Row) (model.Item, error) {
	var x model.Item
	err := row.Scan(&x.ID, &x.SKU, &x.Name, &x.Category, &x.Unit, &x.CreatedAt)
	x.CreatedAt = x.CreatedAt.UTC()
	return x, translate(err)
}

// Create inserts an item with database-enforced SKU uniqueness.
func (p *ItemRepository) Create(ctx context.Context, in model.ItemInput) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "INSERT INTO items(sku,name,category,unit) VALUES($1,$2,$3,$4) RETURNING "+itemColumns, in.SKU, in.Name, in.Category, in.Unit))
}

// Get finds an active item.
func (p *ItemRepository) Get(ctx context.Context, id string) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "SELECT "+itemColumns+" FROM items WHERE id=$1 AND deleted_at IS NULL", id))
}

// Update replaces active item metadata.
func (p *ItemRepository) Update(ctx context.Context, id string, in model.ItemInput) (model.Item, error) {
	return scanItem(p.Pool.QueryRow(ctx, "UPDATE items SET sku=$2,name=$3,category=$4,unit=$5 WHERE id=$1 AND deleted_at IS NULL RETURNING "+itemColumns, id, in.SKU, in.Name, in.Category, in.Unit))
}

// Delete takes the same row lock as Receive and preserves dependent records.
func (p *ItemRepository) Delete(ctx context.Context, id string) error {
	tag, err := p.Pool.Exec(ctx, "UPDATE items SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.NotFound("Item")
	}
	return nil
}

// List returns a count and page from the same database snapshot.
func (p *ItemRepository) List(ctx context.Context, f model.Filter) ([]model.Item, int, error) {
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
	rows, err := tx.Query(ctx, "SELECT "+itemColumns+where+" ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5", f.Search, literalSearch(f.Search), f.Category, f.Limit, (f.Page-1)*f.Limit)
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
func (p *ItemRepository) Categories(ctx context.Context) ([]string, error) {
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

func literalSearch(s string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(s) + "%"
}
