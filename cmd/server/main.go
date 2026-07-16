// Command server runs shiftbot's REST API and serves the embedded dashboard.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/betallsoph/shiftz/internal/admin"
	"github.com/betallsoph/shiftz/internal/api"
	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/dashboard"
	"github.com/betallsoph/shiftz/internal/health"
	"github.com/betallsoph/shiftz/internal/mail"
	"github.com/betallsoph/shiftz/internal/onboarding"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/betallsoph/shiftz/internal/telegram"
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
	if cfg.DevAPIEnabled {
		api.New(st, log).Register(mux)
		log.Info("dev API enabled")
	} else {
		api.RegisterDisabled(mux)
		log.Info("dev API disabled")
	}

	sessionSecret := cfg.SessionSecret
	if sessionSecret == "" {
		var b [32]byte
		if _, err := rand.Read(b[:]); err != nil {
			return err
		}
		sessionSecret = base64.RawURLEncoding.EncodeToString(b[:])
		log.Warn("SESSION_SECRET not set; using ephemeral dev secret (sessions reset on restart)")
	}
	sessions := dashboard.NewSessionManager(sessionSecret, cfg.CookieSecure)

	onboard := onboarding.New(st)
	dash, err := dashboard.New(st, sessions, onboard, cfg.OwnerSignupEnabled, log)
	if err != nil {
		return err
	}
	dash.SetTelegramBotUsername(cfg.TelegramBotUsername)
	configureDashboardPasswordReset(dash, cfg, log)
	if cfg.TelegramToken != "" && cfg.TelegramChatID != 0 {
		tg := telegram.NewClient(cfg.TelegramToken)
		dash.SetIncidentReporter(telegramIncidentMessenger{
			send: func(ctx context.Context, chatID int64, text string) error {
				return tg.SendMessage(ctx, chatID, text, nil)
			},
		}, cfg.TelegramChatID)
	}
	if cfg.OwnerSignupEnabled {
		log.Info("owner signup enabled")
	}
	dash.Register(mux)

	adminPortal, err := admin.New(cfg, admin.NewProvisionService(st), admin.NewShopService(st), log)
	if err != nil {
		return err
	}
	if cfg.AdminPortalEnabled {
		log.Info("admin portal enabled")
	}
	adminPortal.Register(mux)

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

func configureDashboardPasswordReset(dash *dashboard.Server, cfg *config.Config, log *slog.Logger) {
	if cfg.SMTPConfigured() {
		dash.SetPasswordResetMail(mail.NewSMTP(mail.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUser,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		}), cfg.DashboardBaseURL)
		return
	}
	log.Info("owner password recovery email disabled (SMTP not configured)")
}
