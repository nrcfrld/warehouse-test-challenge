//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/enrico/warehouse/internal/httpapi"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/repository"
	"github.com/enrico/warehouse/internal/service"
	"github.com/enrico/warehouse/migrations"
	"github.com/google/uuid"
)

func transferFixture(t *testing.T) (*repository.Postgres, *service.Service, model.Item, model.Location) {
	t.Helper()
	r := database(t)
	svc := &service.Service{Store: r, Logger: slog.Default()}
	x := item(t, r)
	destination, err := svc.CreateLocation(context.Background(), model.LocationInput{Code: "B-01", Zone: "Other", Type: "Bin"})
	if err != nil {
		t.Fatal(err)
	}
	return r, svc, x, destination
}
func balanceAt(t *testing.T, r *repository.Postgres, itemID, location string) string {
	t.Helper()
	var qty string
	if err := r.Pool.QueryRow(context.Background(), "SELECT qty::text FROM stocks WHERE item_id=$1 AND location_id=$2", itemID, location).Scan(&qty); err != nil {
		t.Fatal(err)
	}
	return qty
}
func TestLocationsCreateEditAndUniqueness(t *testing.T) {
	r, svc, x, destination := transferFixture(t)
	ctx := context.Background()
	if _, err := svc.CreateLocation(ctx, model.LocationInput{Code: " b-01 ", Zone: "Other", Type: "Shelf"}); model.Public(err).Status != 409 {
		t.Fatal(err)
	}
	if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: destination.ID, Qty: "1"}, ""); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateLocation(ctx, destination.ID, model.LocationInput{Code: " c-02 ", Zone: " New zone ", Type: "Rack"})
	if err != nil || updated.Code != "C-02" || updated.ID != destination.ID {
		t.Fatal(updated, err)
	}
	stock, err := svc.Stock(ctx, x.ID)
	if err != nil || stock.Total != "1.000" || stock.Locations[0].Location.Zone != "New zone" {
		t.Fatal(stock, err)
	}
	if _, err = svc.UpdateLocation(ctx, destination.ID, model.LocationInput{Code: " a-01 ", Zone: "Main", Type: "Rack"}); model.Public(err).Status != 409 {
		t.Fatal(err)
	}
	if _, err = svc.UpdateLocation(ctx, uuid.NewString(), model.LocationInput{Code: "C", Zone: "Main", Type: "Rack"}); model.Public(err).Status != 404 {
		t.Fatal(err)
	}
	if balanceAt(t, r, x.ID, destination.ID) != "1.000" {
		t.Fatal("edit changed balance")
	}
}
func TestTransferExactIdempotencyAndRetention(t *testing.T) {
	r, svc, x, destination := transferFixture(t)
	ctx := context.Background()
	for _, qty := range []string{"0.100", "0.200"} {
		if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: qty}, ""); err != nil {
			t.Fatal(err)
		}
	}
	in := model.TransferInput{ItemID: x.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "0.1"}
	first, replay, err := svc.Transfer(ctx, in, "move")
	if err != nil || replay || first.SourceBalance != "0.200" || first.DestinationBalance != "0.100" {
		t.Fatal(first, replay, err)
	}
	in.Qty = "0.100"
	again, replay, err := svc.Transfer(ctx, in, "move")
	if err != nil || !replay || again.ID != first.ID {
		t.Fatal(again, replay, err)
	}
	in.Qty = "0.2"
	if _, _, err = svc.Transfer(ctx, in, "move"); model.Public(err).Status != 409 {
		t.Fatal(err)
	}
	if _, _, err = svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1"}, "move"); model.Public(err).Status != 409 {
		t.Fatal("cross-operation key accepted", err)
	}
	in.Qty = "0.201"
	if _, _, err = svc.Transfer(ctx, in, "insufficient"); model.Public(err).Status != 409 {
		t.Fatal(err)
	}
	if balanceAt(t, r, x.ID, locationID) != "0.200" || balanceAt(t, r, x.ID, destination.ID) != "0.100" {
		t.Fatal("failed transfer mutated balances")
	}
	if err = svc.Delete(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	in.Qty = "0.100"
	if _, _, err = svc.Transfer(ctx, in, "new"); model.Public(err).Status != 404 {
		t.Fatal(err)
	}
	if _, replay, err = svc.Transfer(ctx, in, "move"); err != nil || !replay {
		t.Fatal("replay after delete", err)
	}
	var count int
	if err = r.Pool.QueryRow(ctx, "SELECT count(*) FROM stock_movements WHERE kind='transfer' AND from_location_id=$1 AND location_id=$2", locationID, destination.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	down, err := migrations.Files.ReadFile("000002_transfers.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Pool.Exec(ctx, string(down)); err == nil {
		t.Fatal("rollback must not discard transfer history")
	}
}
func TestConcurrentTransferNoOverdraftOrDuplicate(t *testing.T) {
	for _, sameKey := range []bool{false, true} {
		t.Run(fmt.Sprint(sameKey), func(t *testing.T) {
			r, svc, x, destination := transferFixture(t)
			ctx := context.Background()
			if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1"}, ""); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			results := make(chan error, 20)
			for i := range 20 {
				wg.Go(func() {
					key := "shared"
					if !sameKey {
						key = fmt.Sprintf("key-%d", i)
					}
					_, _, err := svc.Transfer(ctx, model.TransferInput{ItemID: x.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "0.100"}, key)
					results <- err
				})
			}
			wg.Wait()
			close(results)
			success := 0
			for err := range results {
				if err == nil {
					success++
				} else if model.Public(err).Status != 409 {
					t.Fatal(err)
				}
			}
			wantSource, wantDest, wantSuccess := "0.000", "1.000", 10
			if sameKey {
				wantSource, wantDest, wantSuccess = "0.900", "0.100", 20
			}
			if success != wantSuccess || balanceAt(t, r, x.ID, locationID) != wantSource || balanceAt(t, r, x.ID, destination.ID) != wantDest {
				t.Fatal("concurrent transfer violated balances", success)
			}
		})
	}
}
func TestTransferRollback(t *testing.T) {
	for _, failure := range []string{"overflow", "audit", "missing destination"} {
		t.Run(failure, func(t *testing.T) {
			r, svc, x, destination := transferFixture(t)
			ctx := context.Background()
			if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1"}, ""); err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "overflow":
				if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: destination.ID, Qty: "999999999999999.999"}, ""); err != nil {
					t.Fatal(err)
				}
			case "audit":
				if _, err := r.Pool.Exec(ctx, "ALTER TABLE stock_movements ADD CONSTRAINT reject_transfer CHECK (kind <> 'transfer')"); err != nil {
					t.Fatal(err)
				}
			case "missing destination":
				destination.ID = uuid.NewString()
			}
			if _, _, err := svc.Transfer(ctx, model.TransferInput{ItemID: x.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "0.100"}, "failed"); err == nil {
				t.Fatal("expected failure")
			}
			if balanceAt(t, r, x.ID, locationID) != "1.000" {
				t.Fatal("source debit was not rolled back")
			}
			var count int
			if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM idempotency_records WHERE key='failed'").Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
			if failure == "overflow" && balanceAt(t, r, x.ID, destination.ID) != "999999999999999.999" {
				t.Fatal("destination changed")
			}
			if failure == "audit" {
				if err := r.Pool.QueryRow(ctx, "SELECT count(*) FROM stocks WHERE item_id=$1 AND location_id=$2", x.ID, destination.ID).Scan(&count); err != nil || count != 0 {
					t.Fatal("destination insert not rolled back", err)
				}
			}
		})
	}
}
func TestTransferVersusDelete(t *testing.T) {
	for range 8 {
		r, svc, x, destination := transferFixture(t)
		ctx := context.Background()
		if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1"}, ""); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var transferErr, deleteErr error
		wg.Go(func() {
			_, _, transferErr = svc.Transfer(ctx, model.TransferInput{ItemID: x.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "0.100"}, "move")
		})
		wg.Go(func() { deleteErr = svc.Delete(ctx, x.ID) })
		wg.Wait()
		if deleteErr != nil {
			t.Fatal(deleteErr)
		}
		if transferErr != nil && model.Public(transferErr).Status != 404 {
			t.Fatal(transferErr)
		}
		want := "1.000"
		if transferErr == nil {
			want = "0.900"
		}
		if balanceAt(t, r, x.ID, locationID) != want {
			t.Fatal("inconsistent transfer/delete result")
		}
	}
}

func TestLocationAndTransferHTTPContract(t *testing.T) {
	r, svc, x, destination := transferFixture(t)
	handler := (&httpapi.API{Service: svc, Logger: slog.Default(), Ping: r.Pool.Ping}).Handler()
	ctx := context.Background()
	if _, _, err := svc.Receive(ctx, model.ReceiveInput{ItemID: x.ID, LocationID: locationID, Qty: "1"}, ""); err != nil {
		t.Fatal(err)
	}
	send := func(method, path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/v1/locations", `{"code":" c-03 ","zone":"Main","type":"Rack"}`, 201},
		{"POST", "/api/v1/locations", `{"code":"C-03","zone":"Main","type":"Rack"}`, 409},
		{"POST", "/api/v1/locations", `{}`, 400},
		{"POST", "/api/v1/locations", `{"unknown":true}`, 400},
		{"PUT", "/api/v1/locations/invalid", `{}`, 400},
		{"PUT", "/api/v1/locations/" + destination.ID, `{"code":"D-01","zone":"Main","type":"Rack"}`, 200},
		{"PUT", "/api/v1/locations/" + destination.ID, `{}`, 400},
		{"PUT", "/api/v1/locations/" + destination.ID, `{"unknown":true}`, 400},
		{"POST", "/api/v1/stock/transfer", `{}`, 400},
		{"POST", "/api/v1/stock/transfer", `{"qty":1}`, 400},
	} {
		w := send(tc.method, tc.path, tc.body, "")
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}
	raw, err := json.Marshal(model.TransferInput{ItemID: x.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "0.100"})
	if err != nil {
		t.Fatal(err)
	}
	first := send("POST", "/api/v1/stock/transfer", string(raw), "http-transfer")
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body)
	}
	replay := send("POST", "/api/v1/stock/transfer", string(raw), "http-transfer")
	if replay.Code != 200 || replay.Header().Get("Idempotency-Replayed") != "true" || replay.Body.String() != first.Body.String() {
		t.Fatal(replay.Code, replay.Body)
	}
}
