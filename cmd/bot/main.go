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
	"github.com/betallsoph/shiftz/internal/llm"
	"github.com/betallsoph/shiftz/internal/scheduler"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	llmSvc := llm.NewService(newProvider(cfg, log))
	tg := telegram.NewClient(cfg.TelegramToken)
	bot := telegram.NewBot(tg, llmSvc, st.Employees, st.Availability, st.Votes, log)

	// Cron: remind Thursdays 10:00, nag Saturdays 10:00, finalize Sundays 18:00.
	runner := scheduler.NewRunner(log,
		&scheduler.WeeklyReminderJob{
			Weekday: time.Thursday, Hour: 10,
			Targets: st.Employees,
			Notify:  notifierFunc{c: tg},
		},
		&scheduler.NagJob{Weekday: time.Saturday, Hour: 10},
		&scheduler.FinalizeJob{Weekday: time.Sunday, Hour: 18},
	)
	go runner.Start(ctx)

	mux := http.NewServeMux()
	mux.Handle("POST /telegram/webhook", telegram.WebhookHandler(bot, cfg.TelegramWebhookSecret, log))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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
	default:
		log.Warn("unknown LLM provider; availability parsing disabled", "provider", cfg.LLMProvider)
		return llm.Unconfigured()
	}
}

// notifierFunc adapts telegram.Client to the scheduler's Notifier interface
// (drops the inline-keyboard argument).
type notifierFunc struct{ c *telegram.Client }

func (n notifierFunc) SendMessage(ctx context.Context, chatID int64, text string) error {
	return n.c.SendMessage(ctx, chatID, text, nil)
}
