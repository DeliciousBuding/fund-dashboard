package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/app"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fund-dashboard stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Set up slog with LOG_LEVEL support ("debug", "info", "warn", "error").
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)
	var levelErr error
	if lv := os.Getenv("LOG_LEVEL"); lv != "" {
		var parsed slog.Level
		levelErr = parsed.UnmarshalText([]byte(lv))
		if levelErr == nil {
			level.Set(parsed)
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if levelErr != nil {
		slog.Warn("invalid LOG_LEVEL ignored; using info", "error", levelErr)
	}

	cfg, err := config.Parse(environMap(os.Environ()))
	if err != nil {
		return err
	}

	runtime, err := app.Build(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer runtime.Close()

	// WriteTimeout applies to ordinary request/response handlers.
	// Long-lived SSE (/api/market/stream) clears the write deadline via
	// http.ResponseController and self-caps lifetime (~20m); SPA useSSE reconnects.
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           runtime.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		// Cap header abuse (default is 1 MiB on many platforms; pin explicitly).
		MaxHeaderBytes: 1 << 20, // 1 MiB
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting fund-dashboard Go backend", "addr", cfg.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func environMap(pairs []string) map[string]string {
	env := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		for i := 0; i < len(pair); i++ {
			if pair[i] == '=' {
				// Windows env vars are case-insensitive and may arrive lowercase
				// (e.g. fund_*); config.Parse only looks up uppercase constants.
				// Normalize every key to uppercase so those entries are not
				// silently ignored. Uppercase-after-duplicates mirrors Windows
				// getenv semantics (last entry in environ order wins).
				env[strings.ToUpper(pair[:i])] = pair[i+1:]
				break
			}
		}
	}
	return env
}
