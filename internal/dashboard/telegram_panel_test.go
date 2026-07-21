package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestTelegramTabRenders(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}}

	body := dashboardHTML(t, srv, mux, shopID)

	if !strings.Contains(body, `href="#telegram-panel"`) {
		t.Fatalf("missing Liên kết Telegram tab link, body = %q", body)
	}
	if !strings.Contains(body, ">Liên kết Telegram</a>") {
		t.Fatalf("missing tab label, body = %q", body)
	}
	if !strings.Contains(body, `id="telegram-panel"`) {
		t.Fatalf("missing telegram panel, body = %q", body)
	}
	if !strings.Contains(body, `data-dashboard-view="telegram"`) {
		t.Fatalf("missing telegram dashboard view marker, body = %q", body)
	}
	if !strings.Contains(body, `<h2 class="dashboard-section-title">Liên kết Telegram</h2>`) {
		t.Fatalf("missing telegram panel title, body = %q", body)
	}
}

func TestEmployeesPanelOmitsOwnerTelegramSetup(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		OwnerTelegramID: 77, TelegramGroupID: -1001,
	}}

	body := dashboardHTML(t, srv, mux, shopID)
	employeesSection := sectionBetween(t, body, `id="employees-panel"`, `id="shifts-panel"`)

	if strings.Contains(employeesSection, `id="telegram-setup"`) {
		t.Fatalf("employees panel still contains owner telegram setup: %q", employeesSection)
	}
	if strings.Contains(employeesSection, "employees-telegram-block") {
		t.Fatalf("employees panel still has telegram block wrapper: %q", employeesSection)
	}
	if strings.Contains(employeesSection, "Tạo group Thông báo") {
		t.Fatalf("employees panel still shows owner group checklist: %q", employeesSection)
	}
}

func TestTelegramPanelContainsOwnerEmployeesAndDivider(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		OwnerTelegramID: 4242,
	}}

	body := dashboardHTML(t, srv, mux, shopID)
	panel := sectionBetween(t, body, `id="telegram-panel"`, `</main>`)

	ownerIdx := strings.Index(panel, `class="telegram-panel-owner"`)
	dividerIdx := strings.Index(panel, `class="telegram-panel-divider"`)
	employeesIdx := strings.Index(panel, `class="telegram-panel-employees"`)
	setupIdx := strings.Index(panel, `id="telegram-setup"`)
	employeeStubIdx := strings.Index(panel, `id="telegram-employees"`)

	if ownerIdx < 0 || dividerIdx < 0 || employeesIdx < 0 {
		t.Fatalf("missing owner/divider/employees structure in panel = %q", panel)
	}
	if !(ownerIdx < dividerIdx && dividerIdx < employeesIdx) {
		t.Fatalf("expected owner → divider → employees order; got owner=%d divider=%d employees=%d", ownerIdx, dividerIdx, employeesIdx)
	}
	if setupIdx < 0 || setupIdx < ownerIdx || setupIdx > dividerIdx {
		t.Fatalf("owner telegram setup should sit in owner section before divider; setup=%d", setupIdx)
	}
	if employeeStubIdx < 0 || employeeStubIdx < employeesIdx {
		t.Fatalf("employee telegram section missing after divider; employees=%d stub=%d", employeesIdx, employeeStubIdx)
	}
	if !strings.Contains(panel, "<hr class=\"telegram-panel-divider\"") {
		t.Fatalf("missing short horizontal divider markup, panel = %q", panel)
	}
	if !strings.Contains(panel, "Đã liên kết") {
		t.Fatalf("owner section missing linked status, panel = %q", panel)
	}
}

func dashboardHTML(t *testing.T, srv *Server, mux *http.ServeMux, shopID uuid.UUID) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func sectionBetween(t *testing.T, body, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(body, startMarker)
	if start < 0 {
		t.Fatalf("missing start marker %q", startMarker)
	}
	rest := body[start:]
	end := strings.Index(rest, endMarker)
	if end < 0 {
		t.Fatalf("missing end marker %q after %q", endMarker, startMarker)
	}
	return rest[:end]
}
