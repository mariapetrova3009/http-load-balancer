package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-http-load-balancer/internal/balancer"
	"go-http-load-balancer/internal/config"
)

func main() {
	// logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.FromFlags()
	if err != nil {
		slog.Error("invalid config", "err", err)
		os.Exit(2)
	}

	mux := http.NewServeMux()

	// Backends are configured via flags (see internal/config).
	backendURLs := mustParseBackends(cfg.Backends)
	lb := balancer.New(backendURLs)
	lb.SetMaxRetries(cfg.Retry.MaxRetries)
	lb.SetMaxBodyBytes(cfg.Retry.MaxBodyBytes)

	// Active health checks
	hc := balancer.NewHealthChecker(cfg.HealthCheck.Interval, cfg.HealthCheck.Timeout)
	hcCtx, hcCancel := context.WithCancel(context.Background())
	defer hcCancel()
	go hc.Run(hcCtx, lb.Backends())

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(lb.Stats())
	})

	mux.Handle("/", lb)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("balancer started", "addr", srv.Addr, "pid", os.Getpid())
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped with error", "err", err)
		}
	}

	// shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("balancer stopped")
}

func mustParseBackends(raw []config.BackendConfig) []*url.URL {
	out := make([]*url.URL, 0, len(raw))
	for _, b := range raw {
		u, err := url.Parse(b.URL)
		if err != nil {
			panic(err)
		}
		out = append(out, u)
	}
	return out
}

