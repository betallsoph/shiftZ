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

func TestBuildTelegramEmployeesViewLinkedAndUnlinked(t *testing.T) {
	shopID := uuid.New()
	linkedID := uuid.New()
	unlinkedID := uuid.New()
	view := buildTelegramEmployeesView([]*store.Employee{
		{ID: linkedID, ShopID: shopID, TelegramUserID: 4242, DisplayName: "Anna", Role: "barista", IsActive: true},
		{ID: unlinkedID, ShopID: shopID, TelegramUserID: 0, DisplayName: "Bình", Role: "", IsActive: false},
	}, "", "https://t.me/bot?start=code", "https://t.me/share/url?url=x")

	if len(view.Employees) != 2 {
		t.Fatalf("rows = %d", len(view.Employees))
	}
	if !view.Employees[0].TelegramLinked || view.Employees[0].TelegramUserID != 4242 || view.Employees[0].LinkLabel != "Đã liên kết" {
		t.Fatalf("linked row = %+v", view.Employees[0])
	}
	if view.Employees[1].TelegramLinked || view.Employees[1].LinkLabel != "Chưa liên kết" || view.Employees[1].RoleLabel != "chưa đặt" {
		t.Fatalf("unlinked row = %+v", view.Employees[1])
	}
	if view.EmployeeInviteURL == "" || view.EmployeeInviteShareURL == "" {
		t.Fatal("expected invite URLs")
	}
}

func TestTelegramEmployeesRefreshShowsInviteAndStatus(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	srv, mux := newTelegramEmployeesTestServer(t, shopID, &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 99, DisplayName: "Chi", Role: "pha chế", IsActive: true},
		{ID: uuid.New(), ShopID: shopID, TelegramUserID: 0, DisplayName: "Dũng", IsActive: true},
	}})
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", InviteCode: "invite99"}}
	srv.SetTelegramBotUsername("shiftzz_bot")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/employees/refresh", nil)
	addSessionCookie(t, srv, shopID, req)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="telegram-employees"`) {
		t.Fatalf("missing swap root, body = %q", body)
	}
	if !strings.Contains(body, "https://t.me/shiftzz_bot?start=invite99") {
		t.Fatalf("missing invite link, body = %q", body)
	}
	if !strings.Contains(body, "Sao chép link") || !strings.Contains(body, "Gửi qua Telegram") {
		t.Fatalf("missing invite actions, body = %q", body)
	}
	if !strings.Contains(body, "Chi") || !strings.Contains(body, "Đã liên kết") || !strings.Contains(body, "99") {
		t.Fatalf("missing linked employee, body = %q", body)
	}
	if !strings.Contains(body, "Dũng") || !strings.Contains(body, "Chưa liên kết") {
		t.Fatalf("missing unlinked employee, body = %q", body)
	}
	if !strings.Contains(body, "đã làm mới trạng thái liên kết") {
		t.Fatalf("missing refresh notice, body = %q", body)
	}
	if strings.Contains(body, "Tạo nhân viên") && strings.Contains(body, `name="display_name"`) {
		t.Fatalf("employee create form must not appear here, body = %q", body)
	}
	if strings.Contains(body, `telegram-card-title`) || strings.Contains(body, "Nhân viên</h3>") {
		t.Fatalf("employees title must stay outside #telegram-employees swap root, body = %q", body)
	}
}

func TestTelegramEmployeesRefreshRequiresAuth(t *testing.T) {
	_, mux := newTelegramEmployeesTestServer(t, uuid.New(), &fakeEmployeeMgmt{})
	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/employees/refresh", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTelegramEmployeesEmptyState(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramEmployeesTestServer(t, shopID, &fakeEmployeeMgmt{})
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", InviteCode: "empty01"}}
	srv.SetTelegramBotUsername("shiftzz_bot")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/telegram/employees/refresh", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Chưa có nhân viên để liên kết") {
		t.Fatalf("missing empty state, body = %q", body)
	}
	if !strings.Contains(body, "Tạo nhân viên ở tab Nhân viên") {
		t.Fatalf("missing pointer to Nhân viên tab, body = %q", body)
	}
}

func newTelegramEmployeesTestServer(t *testing.T, shopID uuid.UUID, employees *fakeEmployeeMgmt) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager("telegram-employees-test-secret", false)
	srv := &Server{
		shops:         &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", InviteCode: "testinv"}},
		shopAuth:      &noopShopAuth{},
		ownerLinks:    &fakeOwnerLinks{token: "tok"},
		shifts:        &fakeShifts{},
		schedules:     &fakeSchedules{},
		employees:     &fakeEmployees{},
		employeeMgmt:  employees,
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
