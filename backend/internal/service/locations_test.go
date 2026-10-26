package service

import (
	"context"
	"strings"
	"testing"

	"github.com/enrico/warehouse/internal/model"
)

type locationRepositoryStub struct {
	LocationRepository
	called   bool
	location model.LocationInput
}

func (s *locationRepositoryStub) Create(_ context.Context, in model.LocationInput) (model.Location, error) {
	s.called = true
	s.location = in
	return model.Location{}, nil
}
func (s *locationRepositoryStub) Update(_ context.Context, _ string, in model.LocationInput) (model.Location, error) {
	s.called = true
	s.location = in
	return model.Location{}, nil
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
				store := &locationRepositoryStub{}
				svc := LocationService{Repository: store}
				var err error
				if update {
					_, err = svc.Update(context.Background(), "id", tc.input)
				} else {
					_, err = svc.Create(context.Background(), tc.input)
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
