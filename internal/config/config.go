// Package config loads shiftbot configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config carries every setting for both binaries; each cmd validates the
// subset it needs via the Require* helpers.
type Config struct {
	// DatabaseURL is the Postgres DSN, e.g.
	// postgres://shiftbot:shiftbot@localhost:5432/shiftbot?sslmode=disable
	DatabaseURL string

	// ServerAddr is the listen address of the REST API + dashboard binary.
	ServerAddr string
	// BotAddr is the listen address of the Telegram webhook binary.
	BotAddr string

	// TelegramToken is the bot token from @BotFather.
	TelegramToken string
	// TelegramWebhookSecret must match the secret_token passed to
	// setWebhook; empty disables the check (local development only).
	TelegramWebhookSecret string

	// LLMProvider selects the model backend (e.g. "anthropic"); empty means
	// LLM features are disabled.
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string

	// EntDebug logs every SQL statement ent generates. Dev only: verbose
	// and includes query parameters.
	EntDebug bool

	// RemindersEnabled starts the availability reminder/nag background loop.
	RemindersEnabled bool
	// ReminderTickInterval is how often the reminder worker checks due jobs.
	ReminderTickInterval time.Duration
}

// Load reads all settings from the environment, applying defaults for
// optional values.
func Load() *Config {
	return &Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		ServerAddr:            envOr("SERVER_ADDR", ":8080"),
		BotAddr:               envOr("BOT_ADDR", ":8081"),
		TelegramToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		LLMProvider:           os.Getenv("LLM_PROVIDER"),
		LLMAPIKey:             os.Getenv("LLM_API_KEY"),
		LLMModel:              os.Getenv("LLM_MODEL"),
		EntDebug:              os.Getenv("ENT_DEBUG") == "1" || os.Getenv("ENT_DEBUG") == "true",
		RemindersEnabled:      os.Getenv("REMINDERS_ENABLED") == "1" || os.Getenv("REMINDERS_ENABLED") == "true",
		ReminderTickInterval:  envDurationOr("REMINDER_TICK_INTERVAL", time.Minute),
	}
}

// RequireDatabase fails unless DATABASE_URL is set.
func (c *Config) RequireDatabase() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: DATABASE_URL is required")
	}
	return nil
}

// RequireTelegram fails unless TELEGRAM_BOT_TOKEN is set.
func (c *Config) RequireTelegram() error {
	if c.TelegramToken == "" {
		return fmt.Errorf("config: TELEGRAM_BOT_TOKEN is required")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
