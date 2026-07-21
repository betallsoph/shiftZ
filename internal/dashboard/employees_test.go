package dashboard

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestEmployeesPanelRendersList(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	srv, mux := newEmployeesTestServer(t, shopID, &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 42, DisplayName: "Anna", Role: "barista", MaxHoursPerWeek: 40, IsActive: true},
	}})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	employeesPanel := sectionBetween(t, body, `id="employees-panel"`, `id="shifts-panel"`)
	if !strings.Contains(employeesPanel, "Anna") || !strings.Contains(employeesPanel, "đang làm") || !strings.Contains(employeesPanel, "đã liên kết") {
		t.Fatalf("employees panel = %q", employeesPanel)
	}
	if !strings.Contains(employeesPanel, `class="status-toggle is-active"`) ||
		!strings.Contains(employeesPanel, `role="switch"`) ||
		!strings.Contains(employeesPanel, `aria-checked="true"`) ||
		!strings.Contains(employeesPanel, `/dashboard/employees/`+empID.String()+`/deactivate`) {
		t.Fatalf("missing active status toggle markup: %q", employeesPanel)
	}
	if strings.Contains(employeesPanel, "status-dot") || strings.Contains(employeesPanel, "Tạm ngưng") || strings.Contains(employeesPanel, "Bật lại") {
		t.Fatalf("expected status badge/text buttons replaced by toggle: %q", employeesPanel)
	}
}

func TestUpdateEmployeeValid(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	fake := &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 42, DisplayName: "Anna", MaxHoursPerWeek: 40, IsActive: true},
	}}
	srv, mux := newEmployeesTestServer(t, shopID, fake)

	form := url.Values{
		"display_name": {"Anna K"},
		"role":         {"kitchen"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+empID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.employees[0].DisplayName != "Anna K" || fake.employees[0].Role != "kitchen" || fake.employees[0].MaxHoursPerWeek != 40 {
		t.Fatalf("employee = %+v", fake.employees[0])
	}
	if !strings.Contains(rec.Body.String(), "Anna K") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestUpdateEmployeeInvalidShowsError(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	fake := &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 42, DisplayName: "Anna", MaxHoursPerWeek: 40, IsActive: true},
	}}
	srv, mux := newEmployeesTestServer(t, shopID, fake)

	form := url.Values{
		"display_name": {""},
		"role":         {"barista"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+empID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.employees[0].DisplayName != "Anna" {
		t.Fatal("should not update invalid employee")
	}
	if !strings.Contains(rec.Body.String(), "tên hiển thị không được để trống") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestDeactivateEmployeeUpdatesPanel(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	fake := &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 42, DisplayName: "Anna", MaxHoursPerWeek: 40, IsActive: true},
	}}
	srv, mux := newEmployeesTestServer(t, shopID, fake)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+empID.String()+"/deactivate", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.employees[0].IsActive {
		t.Fatal("expected inactive")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "đã tạm ngưng") {
		t.Fatalf("body = %q", body)
	}
	if !strings.Contains(body, `id="employees-panel" class="dashboard-view is-active"`) {
		t.Fatalf("expected employees panel to keep is-active after HTMX swap: %q", body)
	}
	if !strings.Contains(body, `class="status-toggle"`) ||
		!strings.Contains(body, `aria-checked="false"`) ||
		!strings.Contains(body, `/dashboard/employees/`+empID.String()+`/activate`) {
		t.Fatalf("missing inactive status toggle markup: %q", body)
	}
}

func TestReactivateEmployeeUpdatesPanel(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	fake := &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: empID, ShopID: shopID, TelegramUserID: 42, DisplayName: "Anna", MaxHoursPerWeek: 40, IsActive: false},
	}}
	srv, mux := newEmployeesTestServer(t, shopID, fake)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+empID.String()+"/activate", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !fake.employees[0].IsActive {
		t.Fatal("expected active")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "đang làm") {
		t.Fatalf("body = %q", body)
	}
	if !strings.Contains(body, `id="employees-panel" class="dashboard-view is-active"`) {
		t.Fatalf("expected employees panel to keep is-active after HTMX swap: %q", body)
	}
	if !strings.Contains(body, `class="status-toggle is-active"`) ||
		!strings.Contains(body, `aria-checked="true"`) ||
		!strings.Contains(body, `/dashboard/employees/`+empID.String()+`/deactivate`) {
		t.Fatalf("missing active status toggle markup: %q", body)
	}
}

func TestToggleEmployeeOtherShopNotFound(t *testing.T) {
	shopID := uuid.New()
	otherEmp := uuid.New()
	fake := &fakeEmployeeMgmt{employees: []*store.Employee{
		{ID: otherEmp, ShopID: uuid.New(), TelegramUserID: 42, DisplayName: "Other", MaxHoursPerWeek: 40, IsActive: true},
	}}
	srv, mux := newEmployeesTestServer(t, shopID, fake)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+otherEmp.String()+"/deactivate", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestEmployeesRoutesRequireAuth(t *testing.T) {
	_, mux := newEmployeesTestServer(t, uuid.New(), &fakeEmployeeMgmt{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/employees/"+uuid.New().String(), nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/login" {
		t.Fatalf("redirect = %q", rec.Header().Get("HX-Redirect"))
	}
}

func newEmployeesTestServer(t *testing.T, shopID uuid.UUID, employees *fakeEmployeeMgmt) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager("employees-test-secret", false)
	srv := &Server{
		shops:         &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		shopAuth:      &noopShopAuth{},
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
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

type fakeEmployeeMgmt struct {
	mu        sync.Mutex
	employees []*store.Employee
}

func (f *fakeEmployeeMgmt) ListAllByShop(_ context.Context, shopID uuid.UUID) ([]*store.Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*store.Employee, 0)
	for _, emp := range f.employees {
		if emp.ShopID == shopID {
			out = append(out, emp)
		}
	}
	return out, nil
}

func (f *fakeEmployeeMgmt) Update(_ context.Context, shopID, employeeID uuid.UUID, input store.UpdateEmployeeInput) (*store.Employee, error) {
	if err := store.ValidateUpdateEmployeeInput(input); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, emp := range f.employees {
		if emp.ID == employeeID && emp.ShopID == shopID {
			emp.DisplayName = strings.TrimSpace(input.DisplayName)
			emp.Role = strings.TrimSpace(input.Role)
			emp.MaxHoursPerWeek = input.MaxHoursPerWeek
			return emp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeEmployeeMgmt) SetActive(_ context.Context, shopID, employeeID uuid.UUID, active bool) (*store.Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, emp := range f.employees {
		if emp.ID == employeeID && emp.ShopID == shopID {
			emp.IsActive = active
			return emp, nil
		}
	}
	return nil, store.ErrNotFound
}
