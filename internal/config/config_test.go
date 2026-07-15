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

func TestLoadTelegramBotUsername(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_USERNAME", "shiftzz_bot")
	if got := Load().TelegramBotUsername; got != "shiftzz_bot" {
		t.Fatalf("TelegramBotUsername = %q", got)
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
	cfg := &Config{DatabaseURL: "postgres://x", SessionSecret: "s", TelegramToken: "t", TelegramBotUsername: "bot"}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing TELEGRAM_WEBHOOK_SECRET")
	}
}

func TestRequireProductionMissingTelegramBotUsername(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramWebhookSecret: "w",
	}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing TELEGRAM_BOT_USERNAME")
	}
}

func TestRequireProductionGeminiMissingAPIKey(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramBotUsername:   "bot",
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
		TelegramBotUsername:   "bot",
		TelegramWebhookSecret: "w",
		LLMProvider:           "gemini",
		LLMAPIKey:             "key",
	}
	if err := cfg.RequireProduction(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireAdminPortalMissingUsername(t *testing.T) {
	cfg := &Config{AdminPortalEnabled: true, AdminPassword: "secret", AdminSessionSecret: "01234567890123456789012345678901"}
	if err := cfg.RequireAdminPortal(); err == nil {
		t.Fatal("want error")
	}
}

func TestLoadAdminPassword(t *testing.T) {
	t.Setenv("ADMIN_PORTAL_ENABLED", "true")
	t.Setenv("ADMIN_USERNAME", "antt")
	t.Setenv("ADMIN_PASSWORD", "ann123")
	t.Setenv("ADMIN_SESSION_SECRET", "01234567890123456789012345678901")
	cfg := Load()
	if cfg.AdminUsername != "antt" || cfg.AdminPassword != "ann123" {
		t.Fatalf("admin credentials not loaded")
	}
}

func TestRequireAdminPortalOKWhenDisabled(t *testing.T) {
	cfg := &Config{AdminPortalEnabled: false}
	if err := cfg.RequireAdminPortal(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireAdminPortalRejectsMissingPassword(t *testing.T) {
	cfg := &Config{
		AdminPortalEnabled: true,
		AdminUsername:      "admin",
		AdminSessionSecret: "01234567890123456789012345678901",
	}
	if err := cfg.RequireAdminPortal(); err == nil {
		t.Fatal("want error for missing ADMIN_PASSWORD")
	}
}

func TestRequireAdminPortalRejectsShortSessionSecret(t *testing.T) {
	cfg := &Config{
		AdminPortalEnabled: true,
		AdminUsername:      "admin",
		AdminPassword:      "test-password",
		AdminSessionSecret: "too-short",
	}
	if err := cfg.RequireAdminPortal(); err == nil {
		t.Fatal("want error for short ADMIN_SESSION_SECRET")
	}
}

func TestRequireAdminPortalOK(t *testing.T) {
	cfg := &Config{
		AdminPortalEnabled: true,
		AdminUsername:      "admin",
		AdminPassword:      "test-password",
		AdminSessionSecret: "01234567890123456789012345678901",
	}
	if err := cfg.RequireAdminPortal(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireProductionAdminPortalEnabledMissingSecret(t *testing.T) {
	cfg := &Config{
		DatabaseURL:           "postgres://x",
		SessionSecret:         "s",
		TelegramToken:         "t",
		TelegramWebhookSecret: "w",
		AdminPortalEnabled:    true,
		AdminUsername:         "admin",
		AdminPassword:         "password",
	}
	if err := cfg.RequireProduction(); err == nil {
		t.Fatal("want error for missing ADMIN_SESSION_SECRET")
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
