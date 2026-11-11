// Command natscope serves the Natscope web UI and API.
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

	"github.com/codewithnithish/natscope/internal/api"
	"github.com/codewithnithish/natscope/internal/config"
	"github.com/codewithnithish/natscope/internal/conn"
	"github.com/codewithnithish/natscope/internal/webui"
)

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "natscope:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.FromEnv()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	mgr := conn.NewManager(log, cfg.StoragePath)
	defer mgr.Close()

	if err := mgr.Load(); err != nil {
		log.Warn("failed to load saved connections", "error", err)
	}

	if cfg.NATSURL != "" {
		_, err := mgr.Add(conn.Profile{
			ID:       "default",
			Name:     cfg.DefaultConnectionName,
			URL:      cfg.NATSURL,
			ReadOnly: cfg.ReadOnly,
		})
		if err != nil && !errors.Is(err, conn.ErrExists) {
			// Not fatal: the UI can still start and a connection can be added later.
			log.Warn("default connection failed", "url", cfg.NATSURL, "error", err)
		}
	}

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: api.New(mgr, webui.FS(), version, log, api.AuthConfig{
			Enabled:  cfg.AuthEnabled,
			Username: cfg.AuthUser,
			Password: cfg.AuthPass,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("natscope listening", "addr", cfg.Addr, "version", version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
