// Command api serves the YouTube Automator HTTP API. It only wires dependencies together;
// all behaviour lives in internal packages, which is why this file is excluded from coverage.
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/sriharsha2/Project-YT/backend/internal/config"
	"github.com/sriharsha2/Project-YT/backend/internal/httpapi"
	"github.com/sriharsha2/Project-YT/backend/internal/logging"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 20 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	logger := logging.New(os.Stdout, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("configure postgres pool: invalid DATABASE_URL")
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = redisClient.Close() }() // Close errors at exit have no one left to act on them.

	gin.SetMode(gin.ReleaseMode)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		ReadHeaderTimeout: readHeaderTimeout,
		Handler: httpapi.NewRouter(httpapi.Deps{
			Logger: logger,
			ReadyChecks: []httpapi.ReadyCheck{
				{Name: "postgres", Probe: pool.Ping},
				{Name: "redis", Probe: func(ctx context.Context) error { return redisClient.Ping(ctx).Err() }},
			},
			NewRequestID: uuid.NewString,
			Now:          time.Now,
		}),
	}
	return serve(ctx, server, logger)
}

// serve runs server until ctx is cancelled, then drains in-flight requests before returning.
func serve(ctx context.Context, server *http.Server, logger *slog.Logger) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	logger.Info("api listening", "addr", server.Addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server stopped: %w", err)
	case <-ctx.Done():
	}

	logger.Info("api shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
