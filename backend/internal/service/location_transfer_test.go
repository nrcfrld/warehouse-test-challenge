package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type locationTransferStore struct {
	Store
	called   bool
	location model.LocationInput
	transfer model.TransferInput
}

func (s *locationTransferStore) CreateLocation(_ context.Context, in model.LocationInput) (model.Location, error) {
	s.called = true
	s.location = in
	return model.Location{}, nil
}
func (s *locationTransferStore) UpdateLocation(_ context.Context, _ string, in model.LocationInput) (model.Location, error) {
	s.called = true
	s.location = in
	return model.Location{}, nil
}
func (s *locationTransferStore) Transfer(_ context.Context, in model.TransferInput, _ string) (model.Transfer, bool, error) {
	s.called = true
	s.transfer = in
	return model.Transfer{}, false, nil
}

func TestLocationRules(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, tc := range []struct {
			name  string
			input model.LocationInput
			valid bool
		}{
			{"valid", model.LocationInput{Code: " c-01 ", Zone: " Main ", Type: " Shelf "}, true},
			{"empty", model.LocationInput{}, false},
			{"blank zone", model.LocationInput{Code: "A", Zone: " ", Type: "Shelf"}, false},
			{"long code", model.LocationInput{Code: strings.Repeat("a", 33), Zone: "Main", Type: "Shelf"}, false},
			{"long zone", model.LocationInput{Code: "A", Zone: strings.Repeat("a", 101), Type: "Shelf"}, false},
			{"long type", model.LocationInput{Code: "A", Zone: "Main", Type: strings.Repeat("a", 51)}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				store := &locationTransferStore{}
				svc := Service{Store: store}
				var err error
				if update {
					_, err = svc.UpdateLocation(context.Background(), "id", tc.input)
				} else {
					_, err = svc.CreateLocation(context.Background(), tc.input)
				}
				if (err == nil) != tc.valid || store.called != tc.valid {
					t.Fatalf("valid=%v called=%v err=%v", tc.valid, store.called, err)
				}
				if tc.valid && store.location != (model.LocationInput{Code: "C-01", Zone: "Main", Type: "Shelf"}) {
					t.Fatal(store.location)
				}
			})
		}
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
		{"valid", func(_ *model.TransferInput) {}, "key", true},
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
			store := &locationTransferStore{}
			svc := Service{Store: store, Logger: slog.Default()}
			in := valid
			tc.change(&in)
			_, _, err := svc.Transfer(context.Background(), in, tc.key)
			if (err == nil) != tc.ok || store.called != tc.ok {
				t.Fatalf("called=%v err=%v", store.called, err)
			}
			if tc.ok && store.transfer.Qty != "0.100" {
				t.Fatal(store.transfer)
			}
		})
	}
}
