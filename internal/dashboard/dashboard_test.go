package dashboard

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

func TestGroupAssignments(t *testing.T) {
	loc := time.UTC
	date := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)
	shiftID := uuid.New()

	assignments := []*store.ScheduleAssignment{
		{
			Date:           date,
			ShiftID:        shiftID,
			ShiftName:      "evening",
			ShiftStartTime: "14:00",
			ShiftEndTime:   "20:00",
			EmployeeName:   "Chi",
		},
		{
			Date:           date,
			ShiftID:        shiftID,
			ShiftName:      "morning",
			ShiftStartTime: "08:00",
			ShiftEndTime:   "14:00",
			EmployeeName:   "Bob",
		},
		{
			Date:           date,
			ShiftID:        shiftID,
			ShiftName:      "morning",
			ShiftStartTime: "08:00",
			ShiftEndTime:   "14:00",
			EmployeeName:   "Anna",
		},
	}

	days := groupAssignments(assignments, loc)
	if len(days) != 1 {
		t.Fatalf("days = %d, want 1", len(days))
	}
	if days[0].Label != "Thứ hai 13/07/2026" {
		t.Fatalf("label = %q", days[0].Label)
	}
	if len(days[0].Shifts) != 2 {
		t.Fatalf("shifts = %d, want 2", len(days[0].Shifts))
	}
	if days[0].Shifts[0].Name != "morning" || days[0].Shifts[0].TimeRange != "08:00-14:00" {
		t.Fatalf("first shift = %+v", days[0].Shifts[0])
	}
	if strings.Join(days[0].Shifts[0].Employees, ",") != "Anna,Bob" {
		t.Fatalf("employees = %v", days[0].Shifts[0].Employees)
	}
}

func TestHandleApproveNotFound(t *testing.T) {
	shopID := uuid.New()
	srv, mux := testDashboard(t,
		&fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		&fakeSchedules{approveFn: func(ctx context.Context, sid, schedID uuid.UUID) (*store.Schedule, error) {
			return nil, store.ErrNotFound
		}},
		&fakeEmployees{},
		&fakeAvailabilityRepo{},
		&fakePlanner{},
	)

	form := url.Values{
		"week_start": {"2026-07-13"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/schedules/"+uuid.New().String()+"/approve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "không tìm thấy lịch") {
		t.Fatalf("body = %q", body)
	}
}

func TestBuildAvailabilityEmployeeViews(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	weekStart := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)

	annaID := uuid.New()
	bobID := uuid.New()
	employees := []*store.Employee{
		{ID: annaID, DisplayName: "Anna", Role: "barista"},
		{ID: bobID, DisplayName: "Bob", Role: "floor"},
	}
	availabilities := []*store.Availability{{
		EmployeeID: annaID,
		RawMessage: `Mon mornings, prefer Wed evening`,
		Slots: []store.AvailabilitySlot{
			{
				Start:      weekStart.Add(18 * time.Hour),
				End:        weekStart.Add(22 * time.Hour),
				Preference: 2,
			},
			{
				Start:      weekStart.Add(8 * time.Hour),
				End:        weekStart.Add(14 * time.Hour),
				Preference: 1,
			},
		},
	}}

	views, submitted, total := buildAvailabilityEmployeeViews(employees, availabilities, loc)
	if submitted != 1 || total != 2 {
		t.Fatalf("counts = %d/%d, want 1/2", submitted, total)
	}
	if !views[0].Submitted || views[0].RawMessage == "" {
		t.Fatalf("anna = %+v", views[0])
	}
	if views[1].Submitted {
		t.Fatalf("bob should be missing: %+v", views[1])
	}
	if views[1].Status != "chưa gửi" {
		t.Fatalf("bob status = %q", views[1].Status)
	}
	if len(views[0].Slots) != 2 {
		t.Fatalf("slots = %d", len(views[0].Slots))
	}
	if views[0].Slots[0].TimeRange != "08:00-14:00" || views[0].Slots[0].Preference != "có thể" {
		t.Fatalf("first slot = %+v", views[0].Slots[0])
	}
	if views[0].Slots[1].Preference != "ưu tiên" {
		t.Fatalf("second slot = %+v", views[0].Slots[1])
	}
}

func TestHandleWeekRendersAvailability(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	weekStart := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	srv, mux := testDashboard(t,
		&fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		&fakeSchedules{},
		&fakeEmployees{employees: []*store.Employee{
			{ID: empID, DisplayName: "Anna", Role: "barista"},
			{ID: uuid.New(), DisplayName: "Bob", Role: "floor"},
		}},
		&fakeAvailabilityRepo{rows: []*store.Availability{{
			EmployeeID: empID,
			RawMessage: "Mon mornings",
			Slots: []store.AvailabilitySlot{{
				Start: weekStart.Add(8 * time.Hour), End: weekStart.Add(14 * time.Hour), Preference: 1,
			}},
		}}},
		&fakePlanner{},
	)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/week?week_start=2026-07-13", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "1/2 đã gửi") {
		t.Fatalf("missing count, body = %q", body)
	}
	if !strings.Contains(body, "đã gửi") || !strings.Contains(body, "chưa gửi") {
		t.Fatalf("missing status labels, body = %q", body)
	}
	if !strings.Contains(body, "Mon mornings") {
		t.Fatalf("missing raw message, body = %q", body)
	}
}

type fakeShops struct {
	shop *store.Shop
}

func (f *fakeShops) ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error) {
	return f.shop, nil
}

type fakeSchedules struct {
	listFn    func(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Schedule, error)
	approveFn func(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error)
}

func (f *fakeSchedules) ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Schedule, error) {
	if f.listFn != nil {
		return f.listFn(ctx, shopID, weekStart)
	}
	return nil, nil
}

func (f *fakeSchedules) Approve(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error) {
	return f.approveFn(ctx, shopID, scheduleID)
}

type fakeEmployees struct {
	employees []*store.Employee
}

func (f *fakeEmployees) ListActiveByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Employee, error) {
	return f.employees, nil
}

type fakeAvailabilityRepo struct {
	rows []*store.Availability
}

func (f *fakeAvailabilityRepo) ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Availability, error) {
	return f.rows, nil
}

type fakePlanner struct{}

func (f *fakePlanner) GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error) {
	return nil, nil
}

func testDashboard(t *testing.T, shops shopReader, schedules scheduleRepo, employees employeeLister, availability availabilityLister, gen weekGenerator) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	shopID := uuid.Nil
	if fs, ok := shops.(*fakeShops); ok && fs.shop != nil {
		shopID = fs.shop.ID
	}
	sessions := NewSessionManager("test-dashboard-secret", false)
	srv := &Server{
		shops:         shops,
		shopAuth:      &noopShopAuth{},
		shopTelegram:  &fakeShopTelegram{},
		schedules:     schedules,
		employees:     employees,
		availability:  availability,
		planner:       gen,
		onboarding:    &noopOnboarder{},
		signupEnabled: false,
		sessions:      sessions,
		log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		tmpl:         &templateSet{tmpl},
	}
	_ = shopID
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

type noopShopAuth struct{}

func (noopShopAuth) VerifyDashboardToken(ctx context.Context, shopID uuid.UUID, token string) (*store.Shop, error) {
	return nil, store.ErrInvalidCredentials
}
