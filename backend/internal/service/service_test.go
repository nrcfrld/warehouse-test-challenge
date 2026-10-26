package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type stubStore struct {
	Store
	input    model.ItemInput
	received model.ReceiveInput
	called   bool
	err      error
}

func (s *stubStore) Create(_ context.Context, in model.ItemInput) (model.Item, error) {
	s.called = true
	s.input = in
	return model.Item{}, s.err
}
func (s *stubStore) Update(_ context.Context, _ string, in model.ItemInput) (model.Item, error) {
	s.called = true
	s.input = in
	return model.Item{}, s.err
}
func (s *stubStore) Delete(_ context.Context, _ string) error { s.called = true; return s.err }
func (s *stubStore) Receive(_ context.Context, in model.ReceiveInput, _ string) (model.Receipt, bool, error) {
	s.called = true
	s.received = in
	return model.Receipt{}, false, s.err
}
func TestReceiveRules(t *testing.T) {
	for _, qty := range []string{"0", "-1", "0.0000", "1000000000000000", "NaN"} {
		t.Run(qty, func(t *testing.T) {
			store := &stubStore{}
			svc := Service{Store: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			_, _, err := svc.Receive(context.Background(), model.ReceiveInput{Qty: qty}, "")
			if err == nil || store.called {
				t.Fatalf("service allowed invalid stock: %s", qty)
			}
		})
	}
	store := &stubStore{}
	svc := Service{Store: store, Logger: slog.Default()}
	_, _, err := svc.Receive(context.Background(), model.ReceiveInput{ItemID: "10000000-0000-4000-8000-000000000001", LocationID: "10000000-0000-4000-8000-000000000002", Qty: "1.2"}, "key")
	if err != nil || store.received.Qty != "1.200" {
		t.Fatalf("normalization failed: %v", err)
	}
}
func TestItemRules(t *testing.T) {
	for _, update := range []bool{false, true} {
		store := &stubStore{err: model.Conflict("sku", "SKU already exists")}
		svc := Service{Store: store}
		in := model.ItemInput{SKU: " sku ", Name: " item ", Category: "Parts", Unit: "pcs"}
		var err error
		if update {
			_, err = svc.Update(context.Background(), "id", in)
		} else {
			_, err = svc.Create(context.Background(), in)
		}
		if !errors.Is(err, store.err) || store.input.SKU != "SKU" {
			t.Fatalf("conflict or normalization lost: %v", err)
		}
	}
	store := &stubStore{}
	svc := Service{Store: store}
	if _, err := svc.Create(context.Background(), model.ItemInput{}); err == nil || store.called {
		t.Fatal("invalid item reached repository")
	}
	store.err = model.NotFound("Item")
	if err := svc.Delete(context.Background(), "deleted"); !errors.Is(err, store.err) {
		t.Fatal("soft delete failure lost")
	}
}
