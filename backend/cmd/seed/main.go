// Command seed creates deterministic locations and optional demonstration inventory.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/repository"
	"github.com/enrico/warehouse/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	demo := flag.Bool("demo", false, "include demonstration inventory")
	flag.Parse()
	if err := run(*demo); err != nil {
		log.Fatal(err)
	}
}
func run(demo bool) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `INSERT INTO locations(id,code,zone,type) VALUES ('10000000-0000-4000-8000-000000000001','A-01','Main warehouse','Shelf'),('10000000-0000-4000-8000-000000000002','A-02','Main warehouse','Shelf'),('10000000-0000-4000-8000-000000000003','B-01','Receiving','Bin') ON CONFLICT DO NOTHING`)
	if err != nil || !demo {
		return err
	}
	svc := &service.Service{Store: &repository.Postgres{Pool: pool}, Logger: slog.Default()}
	samples := []model.ItemInput{{SKU: "PKG-001", Name: "Recycled shipping box", Category: "Packaging", Unit: "pcs"}, {SKU: "RAW-002", Name: "Natural cotton canvas", Category: "Raw materials", Unit: "m"}, {SKU: "SUP-003", Name: "Thermal shipping labels", Category: "Supplies", Unit: "roll"}, {SKU: "PKG-004", Name: "Kraft paper tape", Category: "Packaging", Unit: "roll"}, {SKU: "RAW-005", Name: "Aluminum sheet", Category: "Raw materials", Unit: "kg"}, {SKU: "SUP-006", Name: "Protective work gloves", Category: "Supplies", Unit: "pair"}}
	for _, in := range samples {
		item, e := svc.Create(ctx, in)
		if e != nil {
			if model.Public(e).Status == 409 {
				continue
			}
			return e
		}
		if _, _, e = svc.Receive(ctx, model.ReceiveInput{ItemID: item.ID, LocationID: "10000000-0000-4000-8000-000000000001", Qty: "24.500"}, "demo:"+item.SKU); e != nil {
			return e
		}
	}
	return nil
}
