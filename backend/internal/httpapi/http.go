// Package httpapi exposes the warehouse REST contract and middleware.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/enrico/warehouse/docs"
	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/service"
	"github.com/google/uuid"
)

// API binds transport concerns to domain services.
type API struct {
	Service *service.Service
	Logger  *slog.Logger
	Ping    func(context.Context) error
}
type endpoint func(http.ResponseWriter, *http.Request) error
type envelope struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    any                `json:"data,omitempty"`
	Meta    any                `json:"meta,omitempty"`
	Errors  []model.FieldError `json:"errors,omitempty"`
}

func respond(w http.ResponseWriter, status int, message string, data, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: true, Message: message, Data: data, Meta: meta})
}
func failure(w http.ResponseWriter, err error) {
	e := model.Public(err)
	fields := e.Fields
	if fields == nil {
		fields = []model.FieldError{{Field: "request", Reason: e.Message}}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(envelope{Success: false, Message: e.Message, Errors: fields})
}
func (a *API) wrap(fn endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			if model.Public(err).Status >= 500 {
				a.Logger.ErrorContext(r.Context(), "request.failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
			}
			failure(w, err)
		}
	}
}
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return model.Invalid("body", "Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return model.Invalid("body", "Provide valid JSON with supported fields (maximum 1 MiB)")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return model.Invalid("body", "Provide one JSON object")
	}
	return nil
}
func identifier(r *http.Request, name string) (string, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return "", model.Invalid(name, "Use a valid UUID")
	}
	return id.String(), nil
}

// Handler builds routes with request logging, recovery and bounded request duration.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/items", a.wrap(a.list))
	mux.HandleFunc("POST /api/v1/items", a.wrap(a.create))
	mux.HandleFunc("GET /api/v1/items/{id}", a.wrap(a.get))
	mux.HandleFunc("PUT /api/v1/items/{id}", a.wrap(a.update))
	mux.HandleFunc("DELETE /api/v1/items/{id}", a.wrap(a.remove))
	mux.HandleFunc("GET /api/v1/stock/{item_id}", a.wrap(a.stock))
	mux.HandleFunc("POST /api/v1/stock/receive", a.wrap(a.receive))
	mux.HandleFunc("POST /api/v1/stock/transfer", a.wrap(a.transfer))
	mux.HandleFunc("POST /api/v1/locations", a.wrap(a.createLocation))
	mux.HandleFunc("PUT /api/v1/locations/{id}", a.wrap(a.updateLocation))
	mux.HandleFunc("GET /api/v1/locations", a.wrap(func(w http.ResponseWriter, r *http.Request) error {
		x, e := a.Service.Locations(r.Context())
		if e == nil {
			respond(w, 200, "Locations retrieved successfully", x, nil)
		}
		return e
	}))
	mux.HandleFunc("GET /api/v1/categories", a.wrap(func(w http.ResponseWriter, r *http.Request) error {
		x, e := a.Service.Categories(r.Context())
		if e == nil {
			respond(w, 200, "Categories retrieved successfully", x, nil)
		}
		return e
	}))
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, "Service is alive", map[string]string{"status": "ok"}, nil)
	})
	mux.HandleFunc("GET /health/ready", a.wrap(func(w http.ResponseWriter, r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.Ping(ctx); err != nil {
			return &model.Error{Status: 503, Message: "Database is unavailable"}
		}
		respond(w, 200, "Service is ready", map[string]string{"status": "ok"}, nil)
		return nil
	}))
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(docs.OpenAPI)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, docs.SwaggerHTML)
	})
	mux.HandleFunc("/", a.wrap(func(w http.ResponseWriter, r *http.Request) error { return model.NotFound("Endpoint") }))
	return a.middleware(http.TimeoutHandler(mux, 15*time.Second, `{"success":false,"message":"Request timed out","errors":[{"field":"request","reason":"Request timed out"}]}`))
}
func (a *API) list(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	f := model.Filter{Search: strings.TrimSpace(q.Get("search")), Category: strings.TrimSpace(q.Get("category")), Page: 1, Limit: 10}
	for _, v := range []struct {
		name   string
		target *int
		max    int
	}{{"page", &f.Page, 1000000}, {"limit", &f.Limit, 100}} {
		if q.Has(v.name) {
			n, err := strconv.Atoi(q.Get(v.name))
			if err != nil || n < 1 || n > v.max {
				return model.Invalid(v.name, "Value is outside the supported range")
			}
			*v.target = n
		}
	}
	if len([]rune(f.Search)) > 200 || len([]rune(f.Category)) > 100 {
		return model.Invalid("filter", "Filter is too long")
	}
	items, total, err := a.Service.List(r.Context(), f)
	if err != nil {
		return err
	}
	respond(w, 200, "Items retrieved successfully", items, map[string]int{"page": f.Page, "limit": f.Limit, "total": total})
	return nil
}
func (a *API) create(w http.ResponseWriter, r *http.Request) error {
	var in model.ItemInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in = model.NormalizeItem(in)
	if err := model.ValidateItem(in); err != nil {
		return err
	}
	x, err := a.Service.Create(r.Context(), in)
	if err != nil {
		return err
	}
	respond(w, 201, "Item created successfully", x, nil)
	return nil
}
func (a *API) get(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	x, err := a.Service.Get(r.Context(), id)
	if err != nil {
		return err
	}
	respond(w, 200, "Item retrieved successfully", x, nil)
	return nil
}
func (a *API) update(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	var in model.ItemInput
	if err = decode(w, r, &in); err != nil {
		return err
	}
	in = model.NormalizeItem(in)
	if err = model.ValidateItem(in); err != nil {
		return err
	}
	x, err := a.Service.Update(r.Context(), id, in)
	if err != nil {
		return err
	}
	respond(w, 200, "Item updated successfully", x, nil)
	return nil
}
func (a *API) remove(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "id")
	if err != nil {
		return err
	}
	if err = a.Service.Delete(r.Context(), id); err != nil {
		return err
	}
	respond(w, 200, "Item deleted successfully", map[string]string{"id": id}, nil)
	return nil
}
func (a *API) stock(w http.ResponseWriter, r *http.Request) error {
	id, err := identifier(r, "item_id")
	if err != nil {
		return err
	}
	x, err := a.Service.Stock(r.Context(), id)
	if err != nil {
		return err
	}
	respond(w, 200, "Stock retrieved successfully", x, nil)
	return nil
}
func (a *API) receive(w http.ResponseWriter, r *http.Request) error {
	var in model.ReceiveInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if _, err := model.ParseQuantity(in.Qty); err != nil {
		return err
	}
	if _, err := uuid.Parse(in.ItemID); err != nil {
		return model.Invalid("item_id", "Use a valid UUID")
	}
	if _, err := uuid.Parse(in.LocationID); err != nil {
		return model.Invalid("location_id", "Use a valid UUID")
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 128 {
		return model.Invalid("idempotency_key", "Use at most 128 characters")
	}
	x, replay, err := a.Service.Receive(r.Context(), in, key)
	if err != nil {
		return err
	}
	if replay {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	respond(w, 200, "Stock received successfully", x, nil)
	return nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(n int) { w.status = n; w.ResponseWriter.WriteHeader(n) }
func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := uuid.NewString()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		sw := &statusWriter{w, 200}
		defer func() {
			if recovered := recover(); recovered != nil {
				a.Logger.Error("request.panic", "request_id", id, "panic", recovered)
				failure(sw, errors.New("internal error"))
			}
			a.Logger.Info("http.request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(sw, r)
	})
}
