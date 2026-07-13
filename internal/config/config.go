// Package config loads shiftbot configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/betallsoph/shiftz/internal/store"
)

const (
	// ReminderModeDisabled turns off reminder processing.
	ReminderModeDisabled = "disabled"
	// ReminderModeLoop runs the background ticker in-process.
	ReminderModeLoop = "loop"
	// ReminderModeHTTP exposes an authenticated HTTP trigger for schedulers.
	ReminderModeHTTP = "http"
)

// Config carries every setting for both binaries; each cmd validates the
// subset it needs via the Require* helpers.
type Config struct {
	// DatabaseURL is the Postgres DSN for app runtime (Neon: use pooled URL).
	DatabaseURL string
	// MigrationDatabaseURL is the direct Postgres DSN for Atlas migrations only.
	MigrationDatabaseURL string

	// ServerAddr is the listen address of the REST API + dashboard binary.
	ServerAddr string
	// BotAddr is the listen address of the Telegram webhook binary.
	BotAddr string
	// AppAddr is the listen address of the unified production binary (APP_ADDR).
	AppAddr string

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

	// RemindersEnabled is the legacy flag for the background reminder loop.
	RemindersEnabled bool
	// ReminderMode selects disabled, loop, or http (REMINDER_MODE).
	ReminderMode string
	// ReminderTriggerSecret authenticates POST /internal/reminders/tick.
	ReminderTriggerSecret string
	// ReminderTickInterval is how often the reminder worker checks due jobs.
	ReminderTickInterval time.Duration

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration

	// SessionSecret signs owner dashboard session cookies (required in production).
	SessionSecret string
	// CookieSecure sets the Secure flag on dashboard session cookies.
	CookieSecure bool
	// DevAPIEnabled exposes unauthenticated JSON API routes (local dev only).
	DevAPIEnabled bool
	// OwnerSignupEnabled exposes the /signup owner onboarding flow.
	OwnerSignupEnabled bool
}

// Load reads all settings from the environment, applying defaults for
// optional values.
func Load() *Config {
	pool := store.DefaultOptions()
	return &Config{
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		MigrationDatabaseURL:  os.Getenv("MIGRATION_DATABASE_URL"),
		ServerAddr:            envOr("SERVER_ADDR", ":8080"),
		BotAddr:               envOr("BOT_ADDR", ":8081"),
		AppAddr:               os.Getenv("APP_ADDR"),
		TelegramToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		LLMProvider:           os.Getenv("LLM_PROVIDER"),
		LLMAPIKey:             os.Getenv("LLM_API_KEY"),
		LLMModel:              os.Getenv("LLM_MODEL"),
		EntDebug:              envBool("ENT_DEBUG"),
		RemindersEnabled:      envBool("REMINDERS_ENABLED"),
		ReminderMode:          os.Getenv("REMINDER_MODE"),
		ReminderTriggerSecret: os.Getenv("REMINDER_TRIGGER_SECRET"),
		ReminderTickInterval:  envDurationOr("REMINDER_TICK_INTERVAL", time.Minute),
		DBMaxOpenConns:        envIntOr("DB_MAX_OPEN_CONNS", pool.MaxOpenConns),
		DBMaxIdleConns:        envIntOr("DB_MAX_IDLE_CONNS", pool.MaxIdleConns),
		DBConnMaxLifetime:     envDurationOr("DB_CONN_MAX_LIFETIME", pool.ConnMaxLifetime),
		DBConnMaxIdleTime:     envDurationOr("DB_CONN_MAX_IDLE_TIME", pool.ConnMaxIdleTime),
		SessionSecret:         os.Getenv("SESSION_SECRET"),
		CookieSecure:          envBool("COOKIE_SECURE"),
		DevAPIEnabled:         envBool("DEV_API_ENABLED"),
		OwnerSignupEnabled:    envBool("OWNER_SIGNUP_ENABLED"),
	}
}

// DBOptions returns normalized database pool settings for store.NewWithOptions.
func (c *Config) DBOptions() store.Options {
	return store.Options{
		MaxOpenConns:    c.DBMaxOpenConns,
		MaxIdleConns:    c.DBMaxIdleConns,
		ConnMaxLifetime: c.DBConnMaxLifetime,
		ConnMaxIdleTime: c.DBConnMaxIdleTime,
	}.Normalize()
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

// ResolveAppAddr returns the listen address for cmd/app.
// Precedence: APP_ADDR, then PORT (as :PORT), then :8080.
func (c *Config) ResolveAppAddr() (string, error) {
	if c.AppAddr != "" {
		return c.AppAddr, nil
	}
	port := os.Getenv("PORT")
	if port == "" {
		return ":8080", nil
	}
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 || n > 65535 {
		return "", fmt.Errorf("config: invalid PORT %q", port)
	}
	return ":" + port, nil
}

// ResolvedReminderMode returns the effective reminder mode.
// Precedence: explicit REMINDER_MODE, then legacy REMINDERS_ENABLED=true → loop.
func (c *Config) ResolvedReminderMode() (string, error) {
	if c.ReminderMode != "" {
		switch c.ReminderMode {
		case ReminderModeDisabled, ReminderModeLoop, ReminderModeHTTP:
			return c.ReminderMode, nil
		default:
			return "", fmt.Errorf("config: invalid REMINDER_MODE %q", c.ReminderMode)
		}
	}
	if c.RemindersEnabled {
		return ReminderModeLoop, nil
	}
	return ReminderModeDisabled, nil
}

// RequireProduction fails unless production-required settings are set.
func (c *Config) RequireProduction() error {
	if err := c.RequireDatabase(); err != nil {
		return err
	}
	if c.SessionSecret == "" {
		return fmt.Errorf("config: SESSION_SECRET is required")
	}
	if err := c.RequireTelegram(); err != nil {
		return err
	}
	if c.TelegramWebhookSecret == "" {
		return fmt.Errorf("config: TELEGRAM_WEBHOOK_SECRET is required")
	}
	if c.LLMProvider == "gemini" && c.LLMAPIKey == "" {
		return fmt.Errorf("config: LLM_API_KEY is required when LLM_PROVIDER=gemini")
	}
	mode, err := c.ResolvedReminderMode()
	if err != nil {
		return err
	}
	if mode == ReminderModeHTTP && c.ReminderTriggerSecret == "" {
		return fmt.Errorf("config: REMINDER_TRIGGER_SECRET is required when REMINDER_MODE=http")
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

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string) bool {
	v := os.Getenv(key)
	return v == "1" || v == "true"
}
