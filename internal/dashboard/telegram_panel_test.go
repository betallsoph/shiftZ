package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestShopNameRendersInTopbar(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{ID: shopID, Name: "Boom Box Lê Văn Lương", Timezone: "UTC"}}

	body := dashboardHTML(t, srv, mux, shopID)

	if !strings.Contains(body, `class="dashboard-shop-name"`) {
		t.Fatalf("shop name missing from topbar, body = %q", body)
	}
	if !strings.Contains(body, "Boom Box Lê Văn Lương") {
		t.Fatalf("expected shop name text, body = %q", body)
	}
	if strings.Contains(body, "dashboard-shop-title") || strings.Contains(body, "dashboard-intro") {
		t.Fatalf("large shop title should be removed from page body, body = %q", body)
	}
	nameIdx := strings.Index(body, `class="dashboard-shop-name"`)
	logoutIdx := strings.Index(body, "Đăng xuất")
	if nameIdx < 0 || logoutIdx < 0 || nameIdx > logoutIdx {
		t.Fatalf("shop name should appear before logout in header markup")
	}
	if !strings.Contains(body, `<h2 class="dashboard-section-title">Xếp lịch</h2>`) {
		t.Fatalf("missing schedule section title, body = %q", body)
	}
}

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

func TestTelegramPanelContainsTwoCardsWithOutsideTitles(t *testing.T) {
	shopID := uuid.New()
	srv, mux := newTelegramTestServer(t)
	srv.shops = &fakeShops{shop: &store.Shop{
		ID: shopID, Name: "Cafe", Timezone: "UTC",
		OwnerTelegramID: 4242,
	}}

	body := dashboardHTML(t, srv, mux, shopID)
	panel := sectionBetween(t, body, `id="telegram-panel"`, `</main>`)

	ownerIdx := strings.Index(panel, `class="telegram-panel-owner"`)
	employeesIdx := strings.Index(panel, `class="telegram-panel-employees"`)
	setupIdx := strings.Index(panel, `id="telegram-setup"`)
	employeeStubIdx := strings.Index(panel, `id="telegram-employees"`)
	ownerTitleIdx := strings.Index(panel, `<h3 class="telegram-card-title">Chủ quán</h3>`)
	employeesTitleIdx := strings.Index(panel, `<h3 class="telegram-card-title">Nhân viên</h3>`)

	if ownerIdx < 0 || employeesIdx < 0 {
		t.Fatalf("missing owner/employees structure in panel = %q", panel)
	}
	if ownerIdx >= employeesIdx {
		t.Fatalf("expected owner section before employees; got owner=%d employees=%d", ownerIdx, employeesIdx)
	}
	if ownerTitleIdx < 0 || employeesTitleIdx < 0 {
		t.Fatalf("missing outside card titles in panel = %q", panel)
	}
	if !(ownerIdx < ownerTitleIdx && ownerTitleIdx < setupIdx && setupIdx < employeesIdx) {
		t.Fatalf("owner title should sit outside #telegram-setup; owner=%d title=%d setup=%d employees=%d", ownerIdx, ownerTitleIdx, setupIdx, employeesIdx)
	}
	if !(employeesIdx < employeesTitleIdx && employeesTitleIdx < employeeStubIdx) {
		t.Fatalf("employees title should sit outside #telegram-employees; employees=%d title=%d stub=%d", employeesIdx, employeesTitleIdx, employeeStubIdx)
	}
	if strings.Contains(panel, "telegram-panel-divider") {
		t.Fatalf("in-card divider should be removed now that sections are separate cards, panel = %q", panel)
	}
	cardCount := strings.Count(panel, `class="dashboard-tool telegram-panel-card"`)
	if cardCount != 2 {
		t.Fatalf("expected two dashboard-tool cards, got %d in panel = %q", cardCount, panel)
	}
	if !strings.Contains(panel, "Đã liên kết") {
		t.Fatalf("owner section missing linked status, panel = %q", panel)
	}

	// Titles must not live inside the HTMX-swapped roots.
	setupStart := strings.Index(panel, `<div id="telegram-setup"`)
	if setupStart < 0 {
		t.Fatalf("missing #telegram-setup open tag")
	}
	ownerSectionEnd := strings.Index(panel[setupStart:], `</section>`)
	if ownerSectionEnd < 0 {
		t.Fatalf("missing owner section close after setup")
	}
	setupFragment := panel[setupStart : setupStart+ownerSectionEnd]
	if strings.Contains(setupFragment, `telegram-card-title`) || strings.Contains(setupFragment, "Chủ quán</h3>") {
		t.Fatalf("owner title must stay outside #telegram-setup swap root, fragment = %q", setupFragment)
	}
	employeesOpen := strings.Index(panel, `<div id="telegram-employees"`)
	if employeesOpen < 0 {
		t.Fatalf("missing #telegram-employees open tag")
	}
	employeesFragment := panel[employeesOpen:]
	if strings.Contains(employeesFragment, `telegram-card-title`) || strings.Contains(employeesFragment, "Nhân viên</h3>") {
		t.Fatalf("employees title must stay outside #telegram-employees swap root, fragment = %q", employeesFragment)
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
