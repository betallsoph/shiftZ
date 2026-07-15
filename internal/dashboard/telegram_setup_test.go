package dashboard

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestDashboardShowsConnectedTelegramGroup(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
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

func TestDashboardShowsDisconnectedTelegramGroup(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "Mã setup") || strings.Contains(body, "Tạo mã setup") {
		t.Fatalf("setup code UI should be removed, body = %q", body)
	}
	if !strings.Contains(body, "Chưa kết nối") {
		t.Fatalf("missing disconnected status, body = %q", body)
	}
}

func newTelegramTestServer(t *testing.T) (*Server, *http.ServeMux) {
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
		shifts:        &fakeShifts{},
		schedules:     &fakeSchedules{},
		employees:     &fakeEmployees{},
		employeeMgmt:  &fakeEmployeeMgmt{},
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
