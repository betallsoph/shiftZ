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

func TestShiftsPanelRendersList(t *testing.T) {
	shopID := uuid.New()
	shiftID := uuid.New()
	srv, mux := newShiftsTestServer(t, shopID, &fakeShifts{shifts: []*store.Shift{
		{ID: shiftID, ShopID: shopID, Name: "morning", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2, IsActive: true},
	}})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "morning") {
		t.Fatalf("body = %q", body)
	}
	if !strings.Contains(body, `role="switch"`) || !strings.Contains(body, `aria-checked="true"`) {
		t.Fatalf("expected active status switch, body = %q", body)
	}
	if !strings.Contains(body, `hx-post="/dashboard/shifts/`+shiftID.String()+`/deactivate"`) {
		t.Fatalf("expected deactivate endpoint, body = %q", body)
	}
}

func TestCreateShiftValid(t *testing.T) {
	shopID := uuid.New()
	fake := &fakeShifts{}
	srv, mux := newShiftsTestServer(t, shopID, fake)

	form := url.Values{
		"name":       {"evening"},
		"weekday":    {"2"},
		"start_time": {"14:00"},
		"end_time":   {"20:00"},
		"min_staff":  {"1"},
		"max_staff":  {"2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/shifts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if len(fake.shifts) != 1 || fake.shifts[0].Name != "evening" {
		t.Fatalf("shifts = %+v", fake.shifts)
	}
	if !strings.Contains(rec.Body.String(), "evening") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCreateShiftInvalidShowsError(t *testing.T) {
	shopID := uuid.New()
	fake := &fakeShifts{}
	srv, mux := newShiftsTestServer(t, shopID, fake)

	form := url.Values{
		"name":       {""},
		"weekday":    {"1"},
		"start_time": {"08:00"},
		"end_time":   {"14:00"},
		"min_staff":  {"1"},
		"max_staff":  {"2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/shifts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if len(fake.shifts) != 0 {
		t.Fatal("should not insert invalid shift")
	}
	if !strings.Contains(rec.Body.String(), "tên ca không được để trống") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestDeactivateShiftUpdatesPanel(t *testing.T) {
	shopID := uuid.New()
	shiftID := uuid.New()
	fake := &fakeShifts{shifts: []*store.Shift{
		{ID: shiftID, ShopID: shopID, Name: "morning", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2, IsActive: true},
	}}
	srv, mux := newShiftsTestServer(t, shopID, fake)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/shifts/"+shiftID.String()+"/deactivate", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if fake.shifts[0].IsActive {
		t.Fatal("expected inactive")
	}
	body := rec.Body.String()
	if !strings.Contains(body, `role="switch"`) || !strings.Contains(body, `aria-checked="false"`) {
		t.Fatalf("expected inactive status switch, body = %q", body)
	}
	if !strings.Contains(body, `hx-post="/dashboard/shifts/`+shiftID.String()+`/activate"`) {
		t.Fatalf("expected activate endpoint, body = %q", body)
	}
}

func TestToggleShiftOtherShopNotFound(t *testing.T) {
	shopID := uuid.New()
	otherShift := uuid.New()
	fake := &fakeShifts{shifts: []*store.Shift{
		{ID: otherShift, ShopID: uuid.New(), Name: "other", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2, IsActive: true},
	}}
	srv, mux := newShiftsTestServer(t, shopID, fake)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/shifts/"+otherShift.String()+"/deactivate", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestShiftsRoutesRequireAuth(t *testing.T) {
	_, mux := newShiftsTestServer(t, uuid.New(), &fakeShifts{})

	req := httptest.NewRequest(http.MethodPost, "/dashboard/shifts", nil)
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

func newShiftsTestServer(t *testing.T, shopID uuid.UUID, shifts *fakeShifts) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager("shifts-test-secret", false)
	srv := &Server{
		shops:         &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		shopAuth:      &noopShopAuth{},
		shifts:        shifts,
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

type fakeShifts struct {
	mu     sync.Mutex
	shifts []*store.Shift
}

func (f *fakeShifts) ListAllByShop(_ context.Context, shopID uuid.UUID) ([]*store.Shift, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*store.Shift, 0)
	for _, sh := range f.shifts {
		if sh.ShopID == shopID {
			out = append(out, sh)
		}
	}
	return out, nil
}

func (f *fakeShifts) Create(_ context.Context, shopID uuid.UUID, input store.CreateShiftInput) (*store.Shift, error) {
	if err := store.ValidateCreateShiftInput(input); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	sh := &store.Shift{
		ID:        uuid.New(),
		ShopID:    shopID,
		Name:      strings.TrimSpace(input.Name),
		Weekday:   input.Weekday,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
		MinStaff:  input.MinStaff,
		MaxStaff:  input.MaxStaff,
		IsActive:  true,
	}
	f.shifts = append(f.shifts, sh)
	return sh, nil
}

func (f *fakeShifts) SetActive(_ context.Context, shopID, shiftID uuid.UUID, active bool) (*store.Shift, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sh := range f.shifts {
		if sh.ID == shiftID && sh.ShopID == shopID {
			sh.IsActive = active
			return sh, nil
		}
	}
	return nil, store.ErrNotFound
}
