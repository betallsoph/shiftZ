package dashboard

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestRotateTelegramSetupCodeUnauthenticated(t *testing.T) {
	_, mux := newTelegramTestServer(t, &fakeShopTelegram{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/setup-code", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d location = %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRotateTelegramSetupCodeRendersPlaintextOnce(t *testing.T) {
	shopID := uuid.New()
	fakeTG := &fakeShopTelegram{code: "tg_setup_plaintext"}
	srv, mux := newTelegramTestServer(t, fakeTG)
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/setup-code", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "tg_setup_plaintext") {
		t.Fatalf("missing setup code, body = %q", body)
	}
	if !strings.Contains(body, "/setup tg_setup_plaintext") {
		t.Fatalf("missing setup instructions, body = %q", body)
	}
	if fakeTG.lastShopID != shopID {
		t.Fatalf("rotate shop = %s", fakeTG.lastShopID)
	}
}

func TestDashboardShowsConnectedTelegramGroup(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t, &fakeShopTelegram{})
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", TelegramGroupID: -100555}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Đã kết nối") {
		t.Fatalf("missing connected status, body = %q", body)
	}
	if !strings.Contains(body, "-100555") {
		t.Fatalf("missing group id, body = %q", body)
	}
}

func TestDashboardDoesNotShowSetupCodeOnLoad(t *testing.T) {
	shopID := uuid.New()
	expires := time.Now().Add(20 * time.Minute)
	srv, mux := newTelegramTestServer(t, &fakeShopTelegram{})
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		TelegramSetupCodeExpiresAt: &expires,
	}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "tg_setup_") {
		t.Fatalf("should not show setup code on load, body = %q", body)
	}
	if !strings.Contains(body, "Chưa kết nối") {
		t.Fatalf("missing disconnected status, body = %q", body)
	}
}

func newTelegramTestServer(t *testing.T, fakeTG *fakeShopTelegram) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager("telegram-test-secret", false)
	shopID := uuid.New()
	srv := &Server{
		shops:         &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		shopAuth:      &noopShopAuth{},
		shopTelegram:  fakeTG,
		shifts:        &fakeShifts{},
		schedules:     &fakeSchedules{},
		employees:     &fakeEmployees{},
		availability:  &fakeAvailabilityRepo{},
		planner:       &fakePlanner{},
		onboarding:    &noopOnboarder{},
		signupEnabled: false,
		sessions:      sessions,
		log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		tmpl:          &templateSet{tmpl},
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

type fakeShopTelegram struct {
	code       string
	lastShopID uuid.UUID
}

func (f *fakeShopTelegram) RotateTelegramSetupCode(_ context.Context, shopID uuid.UUID, _ time.Time) (string, error) {
	f.lastShopID = shopID
	if f.code == "" {
		f.code = "tg_setup_testcode"
	}
	return f.code, nil
}
