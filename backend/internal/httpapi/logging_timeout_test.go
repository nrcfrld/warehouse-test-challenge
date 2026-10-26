package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
)

type successfulStockRepository struct{ service.StockRepository }

func (successfulStockRepository) Receive(_ context.Context, in model.ReceiveInput, _ string) (model.Receipt, bool, error) {
	return model.Receipt{ItemID: in.ItemID, LocationID: in.LocationID, Qty: in.Qty}, false, nil
}
func (successfulStockRepository) Transfer(_ context.Context, in model.TransferInput, _ string) (model.Transfer, bool, error) {
	return model.Transfer{ItemID: in.ItemID, Qty: in.Qty}, false, nil
}

func TestRequestIDCorrelatesErrorsAndStockLogs(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body, event string
		panicOnList                     bool
	}{
		{name: "error", method: "GET", path: "/api/v1/items", event: "request.failed"},
		{name: "panic", method: "GET", path: "/api/v1/items", event: "request.panic", panicOnList: true},
		{name: "receipt", method: "POST", path: "/api/v1/stock/receive", body: `{"item_id":"20000000-0000-4000-8000-000000000001","location_id":"10000000-0000-4000-8000-000000000001","qty":"1"}`, event: "stock.received"},
		{name: "transfer", method: "POST", path: "/api/v1/stock/transfer", body: `{"item_id":"20000000-0000-4000-8000-000000000001","from_location_id":"10000000-0000-4000-8000-000000000001","to_location_id":"10000000-0000-4000-8000-000000000002","qty":"1"}`, event: "stock.transferred"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			api := API{
				Items:  &ItemHandler{Service: &service.ItemService{Repository: brokenStore{panicOnList: tc.panicOnList}}},
				Stock:  &StockHandler{Service: &service.StockService{Repository: successfulStockRepository{}}},
				Logger: logger,
			}
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			api.Handler().ServeHTTP(response, request)
			id := response.Header().Get("X-Request-ID")
			if id == "" {
				t.Fatal("missing response request ID")
			}
			found := false
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if event["request_id"] != id {
					t.Fatalf("uncorrelated event: %s", line)
				}
				if event["msg"] == tc.event {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s event: %s", tc.event, logs.String())
			}
		})
	}
}

func TestTimeoutReturnsJSONEnvelope(t *testing.T) {
	api := API{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	slow := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	server := httptest.NewServer(api.middleware(http.TimeoutHandler(slow, time.Millisecond, `{"success":false,"message":"Request timed out","errors":[{"field":"request","reason":"Request timed out"}]}`)))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode != 503 || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("timeout response: %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	var body envelope
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Success || body.Message != "Request timed out" || len(body.Errors) != 1 {
		t.Fatalf("invalid error envelope: %+v", body)
	}
}
