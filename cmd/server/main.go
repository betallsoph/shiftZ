// Command server runs shiftbot's REST API and serves the embedded dashboard.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/betallsoph/shiftz/internal/api"
	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/dashboard"
	"github.com/betallsoph/shiftz/internal/health"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/betallsoph/shiftz/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.NewWithOptions(ctx, cfg.DatabaseURL, cfg.EntDebug, cfg.DBOptions())
	if err != nil {
		return err
	}
	defer st.Close()

	mux := http.NewServeMux()
	health.Register(mux, st)
	api.New(st, log).Register(mux)

	dash, err := dashboard.New(st, log)
	if err != nil {
		return err
	}
	dash.Register(mux)

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServerFS(dist))

	srv := &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("server listening", "addr", cfg.ServerAddr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
