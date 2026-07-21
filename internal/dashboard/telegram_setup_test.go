package dashboard

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestDashboardShowsLinkedOwnerTelegram(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		OwnerTelegramID: 4242, TelegramGroupID: -100555,
	}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Đã liên kết") {
		t.Fatalf("missing owner linked status, body = %q", body)
	}
	if !strings.Contains(body, "4242") {
		t.Fatalf("missing owner telegram id, body = %q", body)
	}
	if !strings.Contains(body, "-100555") {
		t.Fatalf("missing broadcast group id, body = %q", body)
	}
	if !strings.Contains(body, "Tạo group Thông báo") {
		t.Fatalf("missing group checklist, body = %q", body)
	}
}

func TestDashboardShowsUnlinkedOwnerTelegram(t *testing.T) {
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
	if !strings.Contains(body, "Chưa liên kết") {
		t.Fatalf("missing unlinked status, body = %q", body)
	}
	if !strings.Contains(body, "Liên kết Telegram") {
		t.Fatalf("missing owner link button, body = %q", body)
	}
	if !strings.Contains(body, "Chat đội") {
		t.Fatalf("missing optional team chat checklist, body = %q", body)
	}
}

func TestOwnerTelegramLinkGeneratesDeepLink(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}
	srv.shops = &fakeShops{shop: shop}
	srv.ownerLinks = &fakeOwnerLinks{token: "tok123"}
	srv.SetTelegramBotUsername("shiftzz_bot")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/owner-link", nil)
	addSessionCookie(t, srv, shopID, req)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	want := "https://t.me/shiftzz_bot?start=owner_tok123"
	if !strings.Contains(body, want) {
		t.Fatalf("missing deep link %q in body = %q", want, body)
	}
	if !strings.Contains(body, "Mở Telegram") {
		t.Fatalf("missing open button, body = %q", body)
	}
	if !strings.Contains(body, `id="telegram-setup"`) {
		t.Fatalf("expected telegram partial swap target, body = %q", body)
	}
}

func TestTelegramStatusRefreshShowsGroupAndOwner(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		OwnerTelegramID: 99, TelegramGroupID: -1001, TelegramTeamChatID: -1002,
	}}

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/refresh", nil)
	addSessionCookie(t, srv, shopID, req)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Đã liên kết") {
		t.Fatalf("missing linked status, body = %q", body)
	}
	if !strings.Contains(body, "-1001") {
		t.Fatalf("missing broadcast id, body = %q", body)
	}
	if !strings.Contains(body, "-1002") {
		t.Fatalf("missing team chat id, body = %q", body)
	}
	if !strings.Contains(body, "đã làm mới trạng thái") {
		t.Fatalf("missing refresh notice, body = %q", body)
	}
}

func TestOwnerTelegramLinkRequiresAuth(t *testing.T) {
	_, mux := newTelegramTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/owner-link", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
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
		ownerLinks:    &fakeOwnerLinks{token: "test-token"},
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
	srv.SetTelegramBotUsername("shiftzz_bot")
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

type fakeOwnerLinks struct {
	token string
	err   error
}

func (f *fakeOwnerLinks) IssueOwnerLinkToken(_ context.Context, _ uuid.UUID) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}
