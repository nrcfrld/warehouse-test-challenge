package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
)

type brokenStore struct {
	service.Store
	panicOnList bool
}

func (s brokenStore) List(context.Context, model.Filter) ([]model.Item, int, error) {
	if s.panicOnList {
		panic("private panic details")
	}
	return nil, 0, errors.New("private database credentials")
}
func TestSanitizedFailures(t *testing.T) {
	for _, panicOnList := range []bool{false, true} {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		api := API{Service: &service.Service{Store: brokenStore{panicOnList: panicOnList}, Logger: logger}, Logger: logger, Ping: func(context.Context) error { return errors.New("private readiness error") }}
		w := httptest.NewRecorder()
		api.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/items", nil))
		if w.Code != 500 || strings.Contains(w.Body.String(), "private") {
			t.Fatalf("unsafe response: %d %s", w.Code, w.Body)
		}
		w = httptest.NewRecorder()
		api.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
		if w.Code != 503 || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("readiness contract: %d %s", w.Code, w.Body)
		}
	}
}
func TestInvalidRequestsNeverReachService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	api := API{Service: &service.Service{}, Logger: logger}
	handler := api.Handler()
	for _, tt := range []struct{ method, path, body, contentType string }{
		{"POST", "/api/v1/items", `{}`, "text/plain"},
		{"POST", "/api/v1/items", `{"sku":"` + strings.Repeat("x", 1<<20) + `"}`, "application/json"},
		{"POST", "/api/v1/stock/receive", `{"qty":"1.0001"}`, "application/json"},
		{"GET", "/api/v1/items?search=" + strings.Repeat("a", 201), "", ""},
		{"GET", "/api/v1/items?category=" + strings.Repeat("a", 101), "", ""},
		{"GET", "/api/v1/items?page=oops", "", ""},
		{"DELETE", "/api/v1/items/not-uuid", "", ""},
		{"PUT", "/api/v1/items/not-uuid", `{}`, "application/json"},
		{"GET", "/api/v1/stock/not-uuid", "", ""},
	} {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		req.Header.Set("Content-Type", tt.contentType)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body)
		}
	}
}
