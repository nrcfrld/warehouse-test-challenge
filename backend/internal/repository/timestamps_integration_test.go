//go:build integration

package repository_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/enrico/warehouse/internal/model"
)

func TestRepositoryTimestampsAreUTC(t *testing.T) {
	r, svc, item, destination := transferFixture(t)
	ctx := context.Background()
	assertUTC := func(name string, value time.Time) {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if value.Location() != time.UTC || !strings.HasSuffix(string(encoded), `Z"`) {
			t.Fatalf("%s is not serialized in UTC: %s (%s)", name, encoded, value.Location())
		}
	}
	assertUTC("created item", item.CreatedAt)
	updated, err := svc.Items.Update(ctx, item.ID, model.ItemInput{SKU: item.SKU, Name: "Updated", Category: item.Category, Unit: item.Unit})
	if err != nil {
		t.Fatal(err)
	}
	assertUTC("updated item", updated.CreatedAt)
	loaded, err := svc.Items.Get(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertUTC("loaded item", loaded.CreatedAt)
	listed, _, err := svc.Items.List(ctx, model.Filter{Page: 1, Limit: 10})
	if err != nil || len(listed) != 1 {
		t.Fatal(listed, err)
	}
	assertUTC("listed item", listed[0].CreatedAt)
	input := model.ReceiveInput{ItemID: item.ID, LocationID: locationID, Qty: "2"}
	for range 2 {
		receipt, _, e := svc.Stock.Receive(ctx, input, "utc-receipt")
		if e != nil {
			t.Fatal(e)
		}
		assertUTC("receipt or replay", receipt.CreatedAt)
		legacy := receipt.CreatedAt.In(time.FixedZone("Legacy", 7*3600)).Format(time.RFC3339Nano)
		if _, e = r.Pool.Exec(ctx, "UPDATE idempotency_records SET response=jsonb_set(response,'{created_at}',to_jsonb($1::text)) WHERE key='utc-receipt'", legacy); e != nil {
			t.Fatal(e)
		}
	}
	transfer := model.TransferInput{ItemID: item.ID, FromLocationID: locationID, ToLocationID: destination.ID, Qty: "1"}
	for range 2 {
		result, _, e := svc.Stock.Transfer(ctx, transfer, "utc-transfer")
		if e != nil {
			t.Fatal(e)
		}
		assertUTC("transfer or replay", result.CreatedAt)
		legacy := result.CreatedAt.In(time.FixedZone("Legacy", 7*3600)).Format(time.RFC3339Nano)
		if _, e = r.Pool.Exec(ctx, "UPDATE idempotency_records SET response=jsonb_set(response,'{created_at}',to_jsonb($1::text)) WHERE key='utc-transfer'", legacy); e != nil {
			t.Fatal(e)
		}
	}
	stock, err := r.Stock.Get(ctx, item.ID)
	if err != nil || len(stock.Locations) != 2 {
		t.Fatal(stock, err)
	}
	for _, balance := range stock.Locations {
		assertUTC("balance", balance.UpdatedAt)
	}
}
