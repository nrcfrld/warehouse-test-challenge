//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/enrico/warehouse/internal/httpapi"
	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/repository"
	"github.com/enrico/warehouse/internal/service"
	"github.com/enrico/warehouse/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func database(t *testing.T) *repository.Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL must point to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, e := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error(e)
		}
		admin.Close()
	})
	files, err := fs.Glob(migrations.Files, "*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		sql, e := migrations.Files.ReadFile(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(sql)); e != nil {
			t.Fatalf("migration %s: %v", name, e)
		}
	}
	if _, err = pool.Exec(ctx, "INSERT INTO locations(id,code,zone,type) VALUES($1,'A-01','Main','Shelf')", locationID); err != nil {
		t.Fatal(err)
	}
	return &repository.Postgres{Pool: pool}
}

const locationID = "10000000-0000-4000-8000-000000000001"

func item(t *testing.T, r *repository.Postgres) model.Item {
	t.Helper()
	x, err := r.Create(context.Background(), model.ItemInput{SKU: strings.ToUpper(uuid.NewString()), Name: "Cotton", Category: "Materials", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func TestCRUDAndRetention(t *testing.T) {
	ctx := context.Background()
	r := database(t)
	x := item(t, r)
	if _, err := r.Create(ctx, model.ItemInput{SKU: x.SKU, Name: "Duplicate", Category: "Other", Unit: "kg"}); model.Public(err).Status != 409 {
		t.Fatalf("duplicate: %v", err)
	}
	items, total, err := r.List(ctx, model.Filter{Search: "cott", Category: "Materials", Page: 1, Limit: 1})
	if err != nil || len(items) != 1 || total != 1 {
		t.Fatalf("list %v %d %v", items, total, err)
	}
	items, total, err = r.List(ctx, model.Filter{Page: 2, Limit: 1})
	if err != nil || len(items) != 0 || total != 1 {
		t.Fatal("empty page lost total", err)
	}
	items, _, err = r.List(ctx, model.Filter{Search: "%", Page: 1, Limit: 10})
	if err != nil || len(items) != 0 {
		t.Fatal("search must treat wildcard literally", err)
	}
	categories, err := r.Categories(ctx)
	if err != nil || len(categories) != 1 {
		t.Fatal(categories, err)
	}
	if _, _, err = r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1.250"}, "receipt"); err != nil {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Get(ctx, x.ID); model.Public(err).Status != 404 {
		t.Fatal("deleted item visible")
	}
	if _, err = r.Stock(ctx, x.ID); model.Public(err).Status != 404 {
		t.Fatal("deleted stock visible")
	}
	if _, _, err = r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1.000"}, "new"); model.Public(err).Status != 404 {
		t.Fatal("deleted item received stock", err)
	}
	if _, err = r.Create(ctx, model.ItemInput{SKU: x.SKU, Name: "Reused", Category: "Materials", Unit: "kg"}); model.Public(err).Status != 409 {
		t.Fatal("deleted SKU reused", err)
	}
	var count int
	if err = r.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE item_id=$1", x.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("lost audit", err)
	}
	var qty string
	if err = r.Pool.QueryRow(ctx, "SELECT qty::text FROM stocks WHERE item_id=$1", x.ID).Scan(&qty); err != nil || qty != "1.250" {
		t.Fatal("lost stock", err)
	}
}
func TestConcurrentReceiptsAndReplay(t *testing.T) {
	ctx := context.Background()
	r := database(t)
	x := item(t, r)
	in := model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "0.100"}
	var wg sync.WaitGroup
	errs := make(chan error, 30)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := r.Receive(ctx, in, uuid.NewString()); errs <- err }()
	}
	wg.Wait()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := r.Receive(ctx, in, "same-key"); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	stock, err := r.Stock(ctx, x.ID)
	if err != nil || stock.Total != "2.100" {
		t.Fatalf("lost or duplicate update: %+v %v", stock, err)
	}
	original, replay, err := r.Receive(ctx, in, "same-key")
	if err != nil || !replay {
		t.Fatal("missing replay", err)
	}
	again, _, err := r.Receive(ctx, in, "same-key")
	if err != nil || original.ID != again.ID || original.Balance != again.Balance {
		t.Fatal("replay changed", err)
	}
	in.Qty = "0.200"
	if _, _, err = r.Receive(ctx, in, "same-key"); model.Public(err).Status != 409 {
		t.Fatal("key mismatch allowed", err)
	}
	var count int
	if err = r.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE item_id=$1", x.ID).Scan(&count); err != nil || count != 21 {
		t.Fatal("audit count", count, err)
	}
}
func TestRollbackAndOverflow(t *testing.T) {
	ctx := context.Background()
	r := database(t)
	x := item(t, r)
	if _, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: uuid.NewString(), Qty: "1.000"}, "retry"); model.Public(err).Status != 404 {
		t.Fatal(err)
	}
	if _, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "999999999999999.999"}, "retry"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "0.001"}, "overflow"); model.Public(err).Status != 409 {
		t.Fatal("overflow allowed", err)
	}
	var count int
	if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements").Scan(&count); err != nil || count != 1 {
		t.Fatal("failed receipt leaked mutation", err)
	}
	if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM idempotency_records WHERE key='overflow'").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed key persisted", err)
	}
}
func TestReceiveVersusDelete(t *testing.T) {
	ctx := context.Background()
	r := database(t)
	x := item(t, r)
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1.000"}, "race")
		errs <- err
	}()
	go func() { <-start; errs <- r.Delete(ctx, x.ID) }()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil && model.Public(err).Status != 404 {
			t.Fatal(err)
		}
	}
	if _, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1.000"}, "after-delete"); model.Public(err).Status != 404 {
		t.Fatal("receipt after delete", err)
	}
	var stockCount, movementCount int
	if err := r.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM stocks),(SELECT count(*) FROM stock_movements)").Scan(&stockCount, &movementCount); err != nil || stockCount != movementCount {
		t.Fatal("inconsistent transaction", err)
	}
}
func TestHTTPContract(t *testing.T) {
	r := database(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &service.Service{Store: r, Logger: logger}
	api := &httpapi.API{Service: svc, Logger: logger, Ping: r.Pool.Ping}
	handler := api.Handler()
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{{"POST", "/api/v1/items", `{"sku":" test-1 ","name":"Test","category":"General","unit":"kg"}`, 201}, {"POST", "/api/v1/items", `{"sku":"TEST-1","name":"Test","category":"General","unit":"kg"}`, 409}, {"GET", "/api/v1/items?limit=0", "", 400}, {"GET", "/api/v1/items/invalid", "", 400}, {"POST", "/api/v1/items", `{"unknown":true}`, 400}, {"POST", "/api/v1/items", `{} {}`, 400}, {"GET", "/missing", "", 404}, {"GET", "/health/ready", "", 200}} {
		t.Run(tt.method+tt.path+tt.body, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["success"] != (tt.status < 400) {
				t.Fatal("invalid envelope", body)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("missing request ID")
			}
		})
	}
}
func TestMigrationDown(t *testing.T) {
	r := database(t)
	files, err := fs.Glob(migrations.Files, "*.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := len(files) - 1; i >= 0; i-- {
		sql, e := migrations.Files.ReadFile(files[i])
		if e != nil {
			t.Fatal(e)
		}
		if _, e = r.Pool.Exec(context.Background(), string(sql)); e != nil {
			t.Fatal(e)
		}
	}

}

func TestAuditFailureRollsBackBalance(t *testing.T) {
	ctx := context.Background()
	r := database(t)
	x := item(t, r)
	// Force a failure after the balance upsert, to verify the entire transaction rolls back.
	if _, err := r.Pool.Exec(ctx, "ALTER TABLE stock_movements ADD CONSTRAINT test_reject CHECK (qty < 10)"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "11.000"}, "audit-failure"); err == nil {
		t.Fatal("expected audit insert failure")
	}
	var balances, records, keys int
	if err := r.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM stocks),(SELECT count(*) FROM stock_movements),(SELECT count(*) FROM idempotency_records)").Scan(&balances, &records, &keys); err != nil {
		t.Fatal(err)
	}
	if balances+records+keys != 0 {
		t.Fatalf("partial transaction: %d balances, %d records, %d keys", balances, records, keys)
	}
}

func TestHTTPWarehouseFlow(t *testing.T) {
	r := database(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := &httpapi.API{Service: &service.Service{Store: r, Logger: logger}, Logger: logger, Ping: r.Pool.Ping}
	handler := api.Handler()
	x := item(t, r)
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/v1/items/" + x.ID, "", 200},
		{"PUT", "/api/v1/items/" + x.ID, `{"sku":" updated ","name":"Updated item","category":"Updated","unit":"kg"}`, 200},
		{"PUT", "/api/v1/items/" + x.ID, `{"sku":""}`, 400},
		{"GET", "/api/v1/locations", "", 200},
		{"GET", "/api/v1/categories", "", 200},
		{"POST", "/api/v1/stock/receive", `{"item_id":"` + x.ID + `","location_id":"` + locationID + `","qty":"0.100"}`, 200},
		{"POST", "/api/v1/stock/receive", `{"item_id":"` + x.ID + `","location_id":"` + locationID + `","qty":"0.200"}`, 200},
		{"POST", "/api/v1/stock/receive", `{"item_id":"` + x.ID + `","location_id":"` + locationID + `","qty":"0"}`, 400},
		{"POST", "/api/v1/stock/receive", `{"item_id":"` + x.ID + `","location_id":"` + locationID + `","qty":0.1}`, 400},
		{"GET", "/api/v1/stock/" + x.ID, "", 200},
		{"DELETE", "/api/v1/items/" + x.ID, "", 200},
		{"DELETE", "/api/v1/items/" + x.ID, "", 404},
		{"GET", "/api/v1/items/" + x.ID, "", 404},
		{"GET", "/health/live", "", 200},
		{"GET", "/docs", "", 200},
		{"GET", "/docs/openapi.yaml", "", 200},
	} {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tt.status {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body)
		}
	}
}
