package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/ent/enttest"
	"github.com/betallsoph/shiftz/internal/reminder"
	"github.com/betallsoph/shiftz/internal/store"
	_ "github.com/mattn/go-sqlite3"
)

func testConfig() *config.Config {
	return &config.Config{
		DatabaseURL:           "postgres://unused",
		SessionSecret:         "test-session-secret",
		TelegramToken:         "test-token",
		TelegramWebhookSecret: "webhook-secret",
		LLMProvider:           "",
	}
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:cmdapp?mode=memory&cache=shared&_fk=1")
	return store.NewWithClient(client)
}

func testHandler(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()
	st := testStore(t)
	handler, err := wire(context.Background(), cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestWireLivez(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWireDevAPIDisabledByDefault(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/generate-schedule", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestWireWebhookRejectsWrongSecret(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestWireDashboardUnauthenticatedRedirectsLogin(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestWireWebhookAcceptsMatchingSecret(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/telegram/webhook", strings.NewReader(`{"update_id":1}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "webhook-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestWireReminderEndpointDisabledMode(t *testing.T) {
	handler := testHandler(t, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestWireReminderEndpointHTTPMode(t *testing.T) {
	cfg := testConfig()
	cfg.ReminderMode = config.ReminderModeHTTP
	cfg.ReminderTriggerSecret = "tick-secret"
	st := testStore(t)
	rem := reminder.New(st.Shops, st.Shops, st.Employees, st.Availability, st.Reminders, noopMessenger{}, slog.New(slog.NewTextHandler(io.Discard, nil)), reminder.Config{})
	handler, err := wire(context.Background(), cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), rem)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	req.Header.Set(reminder.ReminderSecretHeader, "tick-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestWireReminderEndpointLoopModeNotRegistered(t *testing.T) {
	cfg := testConfig()
	cfg.ReminderMode = config.ReminderModeLoop
	st := testStore(t)
	rem := reminder.New(st.Shops, st.Shops, st.Employees, st.Availability, st.Reminders, noopMessenger{}, slog.New(slog.NewTextHandler(io.Discard, nil)), reminder.Config{})
	handler, err := wire(context.Background(), cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), rem)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

type noopMessenger struct{}

func (noopMessenger) SendMessage(context.Context, int64, string) error { return nil }
