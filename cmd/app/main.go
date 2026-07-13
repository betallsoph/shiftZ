// Command app runs shiftbot's unified production runtime: dashboard,
// Telegram webhook, and optional reminder loop in one process.
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

	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/reminder"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/betallsoph/shiftz/internal/telegram"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("app exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.RequireProduction(); err != nil {
		return err
	}
	addr, err := cfg.ResolveAppAddr()
	if err != nil {
		return err
	}
	reminderMode, err := cfg.ResolvedReminderMode()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.NewWithOptions(ctx, cfg.DatabaseURL, cfg.EntDebug, cfg.DBOptions())
	if err != nil {
		return err
	}
	defer st.Close()

	tg := telegram.NewClient(cfg.TelegramToken)
	var rem *reminder.Service
	if reminderMode != config.ReminderModeDisabled {
		rem = reminder.New(st.Shops, st.Shops, st.Employees, st.Availability, st.Reminders, reminderMessenger{c: tg}, log, reminder.Config{
			TickInterval: cfg.ReminderTickInterval,
		})
	}
	switch reminderMode {
	case config.ReminderModeLoop:
		go func() {
			if err := rem.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("reminder loop stopped", "err", err)
			}
		}()
		log.Info("reminder loop enabled", "tick", cfg.ReminderTickInterval)
	case config.ReminderModeHTTP:
		log.Info("reminder http trigger enabled")
	}

	handler, err := wire(ctx, cfg, st, log, rem)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("app listening", "addr", addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// reminderMessenger adapts telegram.Client to reminder.Messenger.
type reminderMessenger struct{ c *telegram.Client }

func (n reminderMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	return n.c.SendMessage(ctx, chatID, text, nil)
}
