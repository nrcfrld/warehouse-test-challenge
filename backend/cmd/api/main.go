// Command api serves the warehouse HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enrico/warehouse/internal/cache"
	"github.com/enrico/warehouse/internal/httpapi"
	"github.com/enrico/warehouse/internal/repository"
	"github.com/enrico/warehouse/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	var level slog.Level
	if value := os.Getenv("LOG_LEVEL"); value != "" {
		if err := level.UnmarshalText([]byte(value)); err != nil {
			return fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("database configuration: %w", err)
	}
	config.MaxConns = 10
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		err = pool.Ping(pingCtx)
		if err == nil {
			break
		}
		select {
		case <-pingCtx.Done():
			return fmt.Errorf("database readiness: %w", err)
		case <-time.After(500 * time.Millisecond):
		}
	}
	itemRepository := &repository.ItemRepository{Pool: pool}
	stockRepository := &repository.StockRepository{Pool: pool}
	locationRepository := &repository.LocationRepository{Pool: pool}
	itemService := &service.ItemService{Repository: itemRepository, Logger: logger}
	locationService := &service.LocationService{Repository: locationRepository, Logger: logger}
	stockService := &service.StockService{Repository: stockRepository, Logger: logger}
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		ttl := time.Minute
		if value := os.Getenv("REDIS_CACHE_TTL"); value != "" {
			ttl, err = time.ParseDuration(value)
			if err != nil {
				return errors.New("REDIS_CACHE_TTL must be a duration such as 60s")
			}
		}
		lookupCache, cacheErr := cache.New(redisURL, ttl)
		if cacheErr != nil {
			return cacheErr
		}
		defer func() {
			if closeErr := lookupCache.Close(); closeErr != nil {
				logger.Warn("cache.close_failed", "error", closeErr)
			}
		}()
		itemService.Cache = lookupCache
		locationService.Cache = lookupCache
		if cacheErr = lookupCache.Ping(ctx); cacheErr != nil {
			logger.Warn("cache.unavailable", "message", "Lookup requests will fall back to PostgreSQL", "error", cacheErr)
		} else {
			logger.Info("cache.ready", "ttl", ttl.String())
		}
	}
	api := &httpapi.API{
		Items:     &httpapi.ItemHandler{Service: itemService},
		Locations: &httpapi.LocationHandler{Service: locationService},
		Stock:     &httpapi.StockHandler{Service: stockService},
		Logger:    logger,
		Ping:      pool.Ping,
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	failed := make(chan error, 1)
	go func() { logger.Info("api.listening", "port", port); failed <- server.ListenAndServe() }()
	select {
	case err = <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
