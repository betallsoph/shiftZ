package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

type fakeShops struct {
	shop *store.Shop
	err  error
}

func (f *fakeShops) ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.shop, nil
}

type fakeSchedules struct {
	approveFn func(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error)
}

func (f *fakeSchedules) ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*store.Schedule, error) {
	return nil, nil
}

func (f *fakeSchedules) Approve(ctx context.Context, shopID, scheduleID uuid.UUID) (*store.Schedule, error) {
	return f.approveFn(ctx, shopID, scheduleID)
}

type fakePlanner struct {
	generateFn func(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error)
}

func (f *fakePlanner) GenerateWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error) {
	return f.generateFn(ctx, shopID, weekStart)
}

func testServer(shops shopReader, schedules scheduleRepo, gen weekGenerator) *Server {
	return &Server{
		shops:     shops,
		schedules: schedules,
		planner:   gen,
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestParseShopID(t *testing.T) {
	if _, err := parseShopID(url.Values{}); err == nil {
		t.Fatal("expected error for missing shop_id")
	}
	if _, err := parseShopID(url.Values{"shop_id": {"not-a-uuid"}}); err == nil {
		t.Fatal("expected error for invalid shop_id")
	}
}

func TestParseWeekStart(t *testing.T) {
	loc := time.UTC
	if _, err := parseWeekStart(url.Values{}, loc); err == nil {
		t.Fatal("expected error for missing week_start")
	}
	if _, err := parseWeekStart(url.Values{"week_start": {"07-13-2026"}}, loc); err == nil {
		t.Fatal("expected error for invalid week_start")
	}
	got, err := parseWeekStart(url.Values{"week_start": {"2026-07-13"}}, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 7, 13, 0, 0, 0, 0, loc)
	// 2026-07-13 is a Monday.
	if !got.Equal(want) {
		t.Fatalf("week start = %v, want %v", got, want)
	}
}

func TestGenerateScheduleBadRequest(t *testing.T) {
	shopID := uuid.New()
	srv := testServer(
		&fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}},
		&fakeSchedules{},
		&fakePlanner{},
	)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/generate-schedule?shop_id=bad", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/dev/generate-schedule?shop_id="+shopID.String()+"&week_start=bad", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGenerateScheduleConflict(t *testing.T) {
	shopID := uuid.New()
	srv := testServer(
		&fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}},
		&fakeSchedules{},
		&fakePlanner{generateFn: func(ctx context.Context, id uuid.UUID, weekStart time.Time) (*planner.GenerateResult, error) {
			return nil, planner.ErrSchedulesExist
		}},
	)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/generate-schedule?shop_id="+shopID.String()+"&week_start=2026-07-13", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestApproveScheduleNotFound(t *testing.T) {
	shopID := uuid.New()
	scheduleID := uuid.New()
	srv := testServer(
		&fakeShops{},
		&fakeSchedules{approveFn: func(ctx context.Context, sid, schedID uuid.UUID) (*store.Schedule, error) {
			return nil, store.ErrNotFound
		}},
		&fakePlanner{},
	)
	mux := http.NewServeMux()
	srv.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules/"+scheduleID.String()+"/approve?shop_id="+shopID.String(), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestErrorResponseJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "invalid shop_id")
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "invalid shop_id" {
		t.Fatalf("error = %q", body.Error)
	}
}
