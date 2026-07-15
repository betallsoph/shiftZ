package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/betallsoph/shiftz/internal/admin"
	"github.com/betallsoph/shiftz/internal/api"
	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/dashboard"
	"github.com/betallsoph/shiftz/internal/health"
	"github.com/betallsoph/shiftz/internal/llm"
	"github.com/betallsoph/shiftz/internal/onboarding"
	"github.com/betallsoph/shiftz/internal/reminder"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/betallsoph/shiftz/internal/telegram"
	"github.com/betallsoph/shiftz/web"
)

func wire(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger, rem *reminder.Service) (http.Handler, error) {
	_ = ctx
	reminderMode, err := cfg.ResolvedReminderMode()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	health.Register(mux, st)
	if cfg.DevAPIEnabled {
		api.New(st, log).Register(mux)
		log.Info("dev API enabled")
	} else {
		api.RegisterDisabled(mux)
	}

	if reminderMode == config.ReminderModeHTTP && rem != nil {
		mux.Handle("POST /internal/reminders/tick", reminder.HTTPHandler(rem, cfg.ReminderTriggerSecret, log))
	}

	llmSvc := llm.NewService(newProvider(cfg, log))
	tg := telegram.NewClient(cfg.TelegramToken)
	drafts := telegram.NewStoreAvailabilityDraftStore(st.AvailabilityDrafts)
	bot := telegram.NewBot(tg, llmSvc, st.Shops, st.Shops, st.Employees, st.Availability, st.Votes, drafts, log)
	mux.Handle("POST /telegram/webhook", telegram.WebhookHandler(bot, cfg.TelegramWebhookSecret, log))

	sessions := dashboard.NewSessionManager(cfg.SessionSecret, cfg.CookieSecure)
	onboard := onboarding.New(st)
	dash, err := dashboard.New(st, sessions, onboard, cfg.OwnerSignupEnabled, log)
	if err != nil {
		return nil, err
	}
	if cfg.OwnerSignupEnabled {
		log.Info("owner signup enabled")
	}
	dash.Register(mux)

	adminPortal, err := admin.New(cfg, admin.NewProvisionService(st), admin.NewShopService(st), log)
	if err != nil {
		return nil, err
	}
	if cfg.AdminPortalEnabled {
		log.Info("admin portal enabled")
	}
	adminPortal.Register(mux)

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServerFS(dist))

	return mux, nil
}

// newProvider picks the LLM backend from config.
func newProvider(cfg *config.Config, log *slog.Logger) llm.Provider {
	switch cfg.LLMProvider {
	case "":
		log.Warn("LLM_PROVIDER not set; availability parsing disabled")
		return llm.Unconfigured()
	case "gemini":
		if cfg.LLMAPIKey == "" {
			log.Warn("LLM_API_KEY not set; availability parsing disabled")
			return llm.Unconfigured()
		}
		return llm.NewGeminiProvider(cfg.LLMAPIKey, cfg.LLMModel)
	default:
		log.Warn("unknown LLM provider; availability parsing disabled", "provider", cfg.LLMProvider)
		return llm.Unconfigured()
	}
}
