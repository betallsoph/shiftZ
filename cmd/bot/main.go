// Command bot runs shiftbot's Telegram bot in webhook mode, plus the cron
// jobs (weekly reminders, nagging, vote finalization).
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
	"github.com/betallsoph/shiftz/internal/health"
	"github.com/betallsoph/shiftz/internal/llm"
	"github.com/betallsoph/shiftz/internal/reminder"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/betallsoph/shiftz/internal/telegram"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("bot exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	if err := cfg.RequireTelegram(); err != nil {
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

	llmSvc := llm.NewService(newProvider(cfg, log))
	tg := telegram.NewClient(cfg.TelegramToken)
	drafts := telegram.NewStoreAvailabilityDraftStore(st.AvailabilityDrafts)
	bot := telegram.NewBot(tg, llmSvc, st.Shops, st.Employees, st.Availability, st.Votes, drafts, log)

	if reminderMode == config.ReminderModeLoop {
		rem := reminder.New(st.Shops, st.Shops, st.Employees, st.Availability, st.Reminders, reminderMessenger{c: tg}, log, reminder.Config{
			TickInterval: cfg.ReminderTickInterval,
		})
		go func() {
			if err := rem.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("reminder loop stopped", "err", err)
			}
		}()
		log.Info("reminder loop enabled", "tick", cfg.ReminderTickInterval)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /telegram/webhook", telegram.WebhookHandler(bot, cfg.TelegramWebhookSecret, log))
	health.Register(mux, st)

	srv := &http.Server{
		Addr:              cfg.BotAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("bot webhook listening", "addr", cfg.BotAddr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newProvider picks the LLM backend from config. Concrete providers plug in
// here behind llm.Provider; until one is configured the bot answers LLM
// features with a friendly "not configured" message.
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

// reminderMessenger adapts telegram.Client to reminder.Messenger.
type reminderMessenger struct{ c *telegram.Client }

func (n reminderMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	return n.c.SendMessage(ctx, chatID, text, nil)
}
