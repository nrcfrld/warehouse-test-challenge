package service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type stockRepositoryStub struct {
	StockRepository
	received model.ReceiveInput
	transfer model.TransferInput
	called   bool
	err      error
	key      string
}

func (s *stockRepositoryStub) Receive(_ context.Context, in model.ReceiveInput, key string) (model.Receipt, bool, error) {
	s.called = true
	s.received = in
	s.key = key
	return model.Receipt{}, false, s.err
}
func (s *stockRepositoryStub) Transfer(_ context.Context, in model.TransferInput, key string) (model.Transfer, bool, error) {
	s.called = true
	s.transfer = in
	s.key = key
	return model.Transfer{}, false, s.err
}

func TestReceiveRules(t *testing.T) {
	for _, qty := range []string{"0", "-1", "0.0000", "1000000000000000", "NaN"} {
		t.Run(qty, func(t *testing.T) {
			store := &stockRepositoryStub{}
			svc := StockService{Repository: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			_, _, err := svc.Receive(context.Background(), model.ReceiveInput{Qty: qty}, "")
			if err == nil || store.called {
				t.Fatalf("service allowed invalid stock: %s", qty)
			}
		})
	}
	store := &stockRepositoryStub{}
	svc := StockService{Repository: store, Logger: slog.Default()}
	_, _, err := svc.Receive(context.Background(), model.ReceiveInput{ItemID: "10000000-0000-4000-8000-000000000001", LocationID: "10000000-0000-4000-8000-000000000002", Qty: "1.2"}, " key ")
	if err != nil || store.received.Qty != "1.200" || store.key != "key" {
		t.Fatalf("normalization failed: %v", err)
	}
}

func TestTransferRules(t *testing.T) {
	valid := model.TransferInput{ItemID: "20000000-0000-4000-8000-000000000001", FromLocationID: "10000000-0000-4000-8000-000000000001", ToLocationID: "10000000-0000-4000-8000-000000000002", Qty: "0.1"}
	for _, tc := range []struct {
		name   string
		change func(*model.TransferInput)
		key    string
		ok     bool
	}{
		{"valid", func(_ *model.TransferInput) {}, " key ", true},
		{"zero", func(in *model.TransferInput) { in.Qty = "0" }, "", false},
		{"negative", func(in *model.TransferInput) { in.Qty = "-1" }, "", false},
		{"precision", func(in *model.TransferInput) { in.Qty = "0.0001" }, "", false},
		{"overflow", func(in *model.TransferInput) { in.Qty = "1000000000000000" }, "", false},
		{"same location", func(in *model.TransferInput) { in.ToLocationID = in.FromLocationID }, "", false},
		{"invalid item", func(in *model.TransferInput) { in.ItemID = "bad" }, "", false},
		{"invalid source", func(in *model.TransferInput) { in.FromLocationID = "bad" }, "", false},
		{"invalid destination", func(in *model.TransferInput) { in.ToLocationID = "bad" }, "", false},
		{"long key", func(_ *model.TransferInput) {}, strings.Repeat("x", 129), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &stockRepositoryStub{}
			svc := StockService{Repository: store, Logger: slog.Default()}
			in := valid
			tc.change(&in)
			_, _, err := svc.Transfer(context.Background(), in, tc.key)
			if (err == nil) != tc.ok || store.called != tc.ok {
				t.Fatalf("called=%v err=%v", store.called, err)
			}
			if tc.ok && (store.transfer.Qty != "0.100" || store.key != "key") {
				t.Fatal(store.transfer)
			}
		})
	}
}
