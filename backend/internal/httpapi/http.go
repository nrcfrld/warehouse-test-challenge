// Package httpapi exposes the warehouse REST contract and middleware.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/enrico/warehouse/docs"
	"github.com/enrico/warehouse/internal/model"
	"github.com/enrico/warehouse/internal/requestlog"
	"github.com/google/uuid"
)

// API assembles domain handlers, shared middleware, and operational routes.
type API struct {
	Items     *ItemHandler
	Locations *LocationHandler
	Stock     *StockHandler
	Logger    *slog.Logger
	Ping      func(context.Context) error
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
				requestlog.Logger(r.Context(), a.Logger).ErrorContext(r.Context(), "request.failed", "error", err)
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
	mux.HandleFunc("GET /api/v1/items", a.wrap(a.Items.List))
	mux.HandleFunc("POST /api/v1/items", a.wrap(a.Items.Create))
	mux.HandleFunc("GET /api/v1/items/{id}", a.wrap(a.Items.Get))
	mux.HandleFunc("PUT /api/v1/items/{id}", a.wrap(a.Items.Update))
	mux.HandleFunc("DELETE /api/v1/items/{id}", a.wrap(a.Items.Delete))
	mux.HandleFunc("GET /api/v1/stock/{item_id}", a.wrap(a.Stock.Get))
	mux.HandleFunc("POST /api/v1/stock/receive", a.wrap(a.Stock.Receive))
	mux.HandleFunc("POST /api/v1/stock/transfer", a.wrap(a.Stock.Transfer))
	mux.HandleFunc("POST /api/v1/locations", a.wrap(a.Locations.Create))
	mux.HandleFunc("PUT /api/v1/locations/{id}", a.wrap(a.Locations.Update))
	mux.HandleFunc("GET /api/v1/locations", a.wrap(a.Locations.List))
	mux.HandleFunc("GET /api/v1/categories", a.wrap(a.Items.Categories))
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

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(n int) { w.status = n; w.ResponseWriter.WriteHeader(n) }
func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := uuid.NewString()
		logger := requestlog.Logger(r.Context(), a.Logger).With("request_id", id)
		r = r.WithContext(requestlog.WithLogger(r.Context(), logger))
		w.Header().Set("X-Request-ID", id)
		// TimeoutHandler writes directly to this writer without copying its inner headers.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		sw := &statusWriter{w, 200}
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(r.Context(), "request.panic", "panic", recovered)
				failure(sw, errors.New("internal error"))
			}
			logger.InfoContext(r.Context(), "http.request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration_ms", time.Since(start).Milliseconds())
		}()
		next.ServeHTTP(sw, r)
	})
}
