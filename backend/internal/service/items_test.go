package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type itemRepositoryStub struct {
	ItemRepository
	input  model.ItemInput
	called bool
	err    error
	filter model.Filter
}

func (s *itemRepositoryStub) Create(_ context.Context, in model.ItemInput) (model.Item, error) {
	s.called = true
	s.input = in
	return model.Item{}, s.err
}
func (s *itemRepositoryStub) Update(_ context.Context, _ string, in model.ItemInput) (model.Item, error) {
	s.called = true
	s.input = in
	return model.Item{}, s.err
}
func (s *itemRepositoryStub) Delete(_ context.Context, _ string) error { s.called = true; return s.err }
func (s *itemRepositoryStub) List(_ context.Context, filter model.Filter) ([]model.Item, int, error) {
	s.called = true
	s.filter = filter
	return nil, 0, s.err
}

func TestListValidatesBeforeRepository(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter model.Filter
		valid  bool
	}{
		{"normalized", model.Filter{Search: " cotton ", Category: " Materials ", Page: 1, Limit: 10}, true},
		{"page zero", model.Filter{Page: 0, Limit: 10}, false},
		{"page too large", model.Filter{Page: 1000001, Limit: 10}, false},
		{"negative limit", model.Filter{Page: 1, Limit: -1}, false},
		{"limit too large", model.Filter{Page: 1, Limit: 101}, false},
		{"long search", model.Filter{Search: strings.Repeat("a", 201), Page: 1, Limit: 10}, false},
		{"long category", model.Filter{Category: strings.Repeat("a", 101), Page: 1, Limit: 10}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &itemRepositoryStub{}
			_, _, err := (&ItemService{Repository: repo}).List(context.Background(), tc.filter)
			if (err == nil) != tc.valid || repo.called != tc.valid {
				t.Fatalf("called=%v err=%v", repo.called, err)
			}
			if tc.valid && (repo.filter.Search != "cotton" || repo.filter.Category != "Materials") {
				t.Fatal(repo.filter)
			}
		})
	}
}

func TestItemRules(t *testing.T) {
	for _, update := range []bool{false, true} {
		store := &itemRepositoryStub{err: model.Conflict("sku", "SKU already exists")}
		svc := ItemService{Repository: store}
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
	store := &itemRepositoryStub{}
	svc := ItemService{Repository: store}
	if _, err := svc.Create(context.Background(), model.ItemInput{}); err == nil || store.called {
		t.Fatal("invalid item reached repository")
	}
	store.err = model.NotFound("Item")
	if err := svc.Delete(context.Background(), "deleted"); !errors.Is(err, store.err) {
		t.Fatal("soft delete failure lost")
	}
}
