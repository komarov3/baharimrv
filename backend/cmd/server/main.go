// Command server runs the BahariMRV conservation API: identity provisioning,
// offline MRV ingest, provenance anchoring, proof rendering, and the marketplace.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/baharilink/baharimrv/internal/app"
	"github.com/baharilink/baharimrv/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	built, err := app.Build(cfg)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           built.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	// The refresh loop drains queued imagery computations and stops when the server
	// does. A cancelled context abandons a live computation, which the reaper handles
	// on the next start.
	background, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()
	go built.Satellite.RunRefreshLoop(background)
	serveErrors := make(chan error, 1)
	go func() {
		slog.Info("baharimrv api listening", "address", server.Addr, "environment", cfg.Environment,
			"chain_mode", cfg.ChainMode, "satellite_mode", cfg.SatelliteMode,
			"satellite_provider", cfg.SatelliteProvider,
			"registry_mode", cfg.RegistryMode, "payment_mode", cfg.PaymentMode)
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdown:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
