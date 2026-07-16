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

	"github.com/betallsoph/shiftz/internal/store"
)

type fakeIncidentMessenger struct {
	lastChatID int64
	lastText   string
	err        error
}

func (f *fakeIncidentMessenger) SendMessage(_ context.Context, chatID int64, text string) error {
	f.lastChatID = chatID
	f.lastText = text
	return f.err
}

func TestFormatIncidentReport(t *testing.T) {
	shopID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shop := &store.Shop{
		ID:                shopID,
		Name:              "Demo Cafe",
		Timezone:          "Asia/Ho_Chi_Minh",
		DashboardUsername: "demo.cafe",
	}
	when := time.Date(2026, 7, 16, 14, 30, 0, 0, time.UTC)

	got := formatIncidentReport(shop, "Bot không phản hồi", when)
	for _, want := range []string{
		"Demo Cafe",
		"demo.cafe",
		shopID.String(),
		"Bot không phản hồi",
		"21:30 +07", // ICT
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestHandleIncidentReportSuccess(t *testing.T) {
	shopID := uuid.New()
	messenger := &fakeIncidentMessenger{}
	srv, mux := testDashboardWithIncident(t, shopID, messenger, 424242)

	form := url.Values{"description": {"Lịch tuần không tải được"}}
	req := httptest.NewRequest(http.MethodPost, "/dashboard/incident-report", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "đã gửi báo cáo") {
		t.Fatalf("body = %q", body)
	}
	if messenger.lastChatID != 424242 {
		t.Fatalf("chatID = %d", messenger.lastChatID)
	}
	if !strings.Contains(messenger.lastText, "Lịch tuần không tải được") {
		t.Fatalf("text = %q", messenger.lastText)
	}
}

func TestHandleIncidentReportNotConfigured(t *testing.T) {
	shopID := uuid.New()
	srv, mux := testDashboard(t,
		&fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}},
		&fakeSchedules{},
		&fakeEmployees{},
		&fakeAvailabilityRepo{},
		&fakePlanner{},
	)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/incident-report", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "chưa được cấu hình") {
		t.Fatalf("body = %q", body)
	}
}

func testDashboardWithIncident(t *testing.T, shopID uuid.UUID, messenger incidentMessenger, chatID int64) (*Server, *http.ServeMux) {
	t.Helper()
	srv, mux := testDashboard(t,
		&fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "cafe.demo"}},
		&fakeSchedules{},
		&fakeEmployees{},
		&fakeAvailabilityRepo{},
		&fakePlanner{},
	)
	srv.SetIncidentReporter(messenger, chatID)
	return srv, mux
}

func TestIncidentReportEnabled(t *testing.T) {
	srv := &Server{}
	if srv.incidentReportEnabled() {
		t.Fatal("want disabled by default")
	}
	srv.SetIncidentReporter(&fakeIncidentMessenger{}, 1)
	if !srv.incidentReportEnabled() {
		t.Fatal("want enabled with messenger and chat id")
	}
	srv.SetIncidentReporter(&fakeIncidentMessenger{}, 0)
	if srv.incidentReportEnabled() {
		t.Fatal("want disabled without chat id")
	}
	_ = slog.New(slog.NewTextHandler(io.Discard, nil))
}
