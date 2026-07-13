package config

import "testing"

func TestDevAPIEnabledDefault(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "")
	cfg := Load()
	if cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled false by default")
	}
}

func TestDevAPIEnabledTrue(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "true")
	cfg := Load()
	if !cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled true for DEV_API_ENABLED=true")
	}
}

func TestDevAPIEnabledOne(t *testing.T) {
	t.Setenv("DEV_API_ENABLED", "1")
	cfg := Load()
	if !cfg.DevAPIEnabled {
		t.Fatal("want DevAPIEnabled true for DEV_API_ENABLED=1")
	}
}

func TestOwnerSignupEnabledDefault(t *testing.T) {
	t.Setenv("OWNER_SIGNUP_ENABLED", "")
	cfg := Load()
	if cfg.OwnerSignupEnabled {
		t.Fatal("want OwnerSignupEnabled false by default")
	}
}

func TestOwnerSignupEnabledTrue(t *testing.T) {
	t.Setenv("OWNER_SIGNUP_ENABLED", "true")
	cfg := Load()
	if !cfg.OwnerSignupEnabled {
		t.Fatal("want OwnerSignupEnabled true")
	}
}

func TestResolveAppAddrPrefersAPPAddr(t *testing.T) {
	t.Setenv("APP_ADDR", ":9000")
	t.Setenv("PORT", "10000")
	cfg := Load()
	addr, err := cfg.ResolveAppAddr()
	if err != nil {
		t.Fatal(err)
	}
	if addr != ":9000" {
		t.Fatalf("addr = %q, want :9000", addr)
	}
}

func TestResolveAppAddrFromPORT(t *testing.T) {
	t.Setenv("APP_ADDR", "")
	t.Setenv("PORT", "10000")
	cfg := Load()
	addr, err := cfg.ResolveAppAddr()
	if err != nil {
		t.Fatal(err)
	}
	if addr != ":10000" {
		t.Fatalf("addr = %q, want :10000", addr)
	}
}

func TestResolveAppAddrDefault(t *testing.T) {
	t.Setenv("APP_ADDR", "")
	t.Setenv("PORT", "")
	cfg := Load()
	addr, err := cfg.ResolveAppAddr()
	if err != nil {
		t.Fatal(err)
	}
	if addr != ":8080" {
		t.Fatalf("addr = %q, want :8080", addr)
	}
}

func TestResolveAppAddrInvalidPORT(t *testing.T) {
	cases := []string{"abc", "0", "70000", "-1"}
	for _, port := range cases {
		t.Run(port, func(t *testing.T) {
			t.Setenv("APP_ADDR", "")
			t.Setenv("PORT", port)
			cfg := Load()
			if _, err := cfg.ResolveAppAddr(); err == nil {
				t.Fatalf("PORT=%q: want error", port)
			}
		})
	}
}

func TestRequireProductionMissingDatabase(t *testing.T) {
	cfg := &Config{SessionSecret: "s", TelegramToken: "t", TelegramWebhookSecret: "w"}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing DATABASE_URL")
	}
}

func TestRequireProductionMissingSessionSecret(t *testing.T) {
	cfg := &Config{DatabaseURL: "postgres://x", TelegramToken: "t", TelegramWebhookSecret: "w"}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing SESSION_SECRET")
	}
}

func TestRequireProductionMissingTelegramToken(t *testing.T) {
	cfg := &Config{DatabaseURL: "postgres://x", SessionSecret: "s", TelegramWebhookSecret: "w"}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing TELEGRAM_BOT_TOKEN")
	}
}

func TestRequireProductionMissingWebhookSecret(t *testing.T) {
	cfg := &Config{DatabaseURL: "postgres://x", SessionSecret: "s", TelegramToken: "t"}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing TELEGRAM_WEBHOOK_SECRET")
	}
}

func TestRequireProductionGeminiMissingAPIKey(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramWebhookSecret: "w",
		LLMProvider:           "gemini",
	}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing LLM_API_KEY with gemini")
	}
}

func TestRequireProductionOK(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramWebhookSecret: "w",
		LLMProvider:           "gemini",
		LLMAPIKey:             "key",
	}
	if err := cfg.RequireProduction(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvedReminderModeDefaultDisabled(t *testing.T) {
	t.Setenv("REMINDER_MODE", "")
	t.Setenv("REMINDERS_ENABLED", "")
	cfg := Load()
	mode, err := cfg.ResolvedReminderMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode != ReminderModeDisabled {
		t.Fatalf("mode = %q, want disabled", mode)
	}
}

func TestResolvedReminderModeLegacyLoop(t *testing.T) {
	t.Setenv("REMINDER_MODE", "")
	t.Setenv("REMINDERS_ENABLED", "true")
	cfg := Load()
	mode, err := cfg.ResolvedReminderMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode != ReminderModeLoop {
		t.Fatalf("mode = %q, want loop", mode)
	}
}

func TestResolvedReminderModeExplicitPrecedence(t *testing.T) {
	t.Setenv("REMINDER_MODE", "disabled")
	t.Setenv("REMINDERS_ENABLED", "true")
	cfg := Load()
	mode, err := cfg.ResolvedReminderMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode != ReminderModeDisabled {
		t.Fatalf("mode = %q, want disabled", mode)
	}
}

func TestResolvedReminderModeInvalid(t *testing.T) {
	cfg := &Config{ReminderMode: "cron"}
	if _, err := cfg.ResolvedReminderMode(); err == nil {
		t.Fatal("want error for invalid mode")
	}
}

func TestRequireProductionHTTPModeMissingTriggerSecret(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramWebhookSecret: "w",
		ReminderMode:          ReminderModeHTTP,
	}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing REMINDER_TRIGGER_SECRET")
	}
}
