// Command server runs the SheetHappens print API and serves the web portal.
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

	"sheethappens/internal/api"
	"sheethappens/internal/config"
	"sheethappens/internal/printer"
)

// maxConcurrentJobs bounds simultaneous uploads being spooled to CUPS.
const maxConcurrentJobs = 4

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	client, err := printer.NewClient(cfg.CUPSHost,
		printer.WithDefaultPrinter(cfg.DefaultPrinter),
		printer.WithTempDir(cfg.TempDir),
	)
	if err != nil {
		return err
	}

	h := api.NewHandler(client, cfg.MaxUploadBytes(), maxConcurrentJobs, log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/printers", h.Printers)
	mux.HandleFunc("POST /api/print", h.Print)
	mux.Handle("GET /", api.StaticFiles(cfg.WebDir))

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.Recoverer(log, api.RequestLogger(log, api.SecurityHeaders(mux))),
		ReadHeaderTimeout: 10 * time.Second,
		// Generous read/write windows for large uploads through the tunnel.
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		IdleTimeout:    2 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("SheetHappens print-api listening",
			"addr", srv.Addr, "cups_host", cfg.CUPSHost,
			"default_printer", cfg.DefaultPrinter, "max_upload_mb", cfg.MaxUploadMB)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutdown signal received, draining connections")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
