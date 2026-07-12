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

func TestHandleWeekBadShopID(t *testing.T) {
	srv := testDashboard(t, &fakeShops{}, &fakeSchedules{}, &fakePlanner{})
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/week?shop_id=bad&week_start=2026-07-13", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "mã cửa hàng không hợp lệ") {
		t.Fatalf("body = %q", body)
	}
	if strings.Contains(body, "Tạo lịch") {
		t.Fatalf("error state should not render generate button, body = %q", body)
	}
	if strings.Contains(body, `name="shop_id"`) {
		t.Fatalf("error state should not render hidden shop_id field, body = %q", body)
	}
}

func TestHandleApproveNotFound(t *testing.T) {
	shopID := uuid.New()
	srv := testDashboard(t,
		&fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		&fakeSchedules{approveFn: func(ctx context.Context, sid, schedID uuid.UUID) (*store.Schedule, error) {
			return nil, store.ErrNotFound
		}},
		&fakePlanner{},
	)
	mux := http.NewServeMux()
	srv.Register(mux)

	form := url.Values{
		"shop_id":    {shopID.String()},
		"week_start": {"2026-07-13"},
	}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/schedules/"+uuid.New().String()+"/approve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "không tìm thấy lịch") {
		t.Fatalf("body = %q", body)
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

type fakePlanner struct{}

func (f *fakePlanner) GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error) {
	return nil, nil
}

func testDashboard(t *testing.T, shops shopReader, schedules scheduleRepo, gen weekGenerator) *Server {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		shops:     shops,
		schedules: schedules,
		planner:   gen,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		tmpl:      &templateSet{tmpl},
	}
}
