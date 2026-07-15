package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

func TestSessionSignVerify(t *testing.T) {
	mgr := NewSessionManager("test-secret", false)
	shopID := uuid.New()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	sess := mgr.NewSession(shopID, now)

	value, err := mgr.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mgr.Verify(value, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.ShopID != shopID {
		t.Fatalf("shop = %s, want %s", got.ShopID, shopID)
	}
}

func TestSessionTamperedCookieRejected(t *testing.T) {
	mgr := NewSessionManager("test-secret", false)
	sess := mgr.NewSession(uuid.New(), time.Now())
	value, err := mgr.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	value = value[:len(value)-2] + "xx"
	if _, err := mgr.Verify(value, time.Now()); err == nil {
		t.Fatal("expected tampered cookie to fail")
	}
}

func TestSessionExpiredCookieRejected(t *testing.T) {
	mgr := NewSessionManager("test-secret", false)
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	sess := &Session{
		ShopID:    uuid.New(),
		ExpiresAt: now.Add(-time.Minute),
	}
	value, err := mgr.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Verify(value, now); err == nil {
		t.Fatal("expected expired cookie to fail")
	}
}

func TestLoginSucceedsWithUsername(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	_, mux := testDashboardWithAuth(t, shopID, "", &fakeShops{shop: shop})

	form := url.Values{
		"dashboard_username": {"demo.cafe"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestLoginFailsWithWrongUsername(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	_, mux := testDashboardWithAuth(t, shopID, "", &fakeShops{shop: shop})

	form := url.Values{
		"dashboard_username": {"wrong.cafe"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tên đăng nhập không đúng") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestLegacyLoginAndSignupRoutesRemoved(t *testing.T) {
	shopID := uuid.New()
	_, mux := testDashboardWithAuth(t, shopID, "", &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})
	for _, path := range []string{"/login/legacy", "/signup"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			req := httptest.NewRequest(method, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: status = %d, want 404", method, path, rec.Code)
			}
		}
	}
}

func TestAdminCookieDoesNotAccessOwnerDashboard(t *testing.T) {
	shopID := uuid.New()
	srv, mux := testDashboardWithAuth(t, shopID, "sz_owner_test", &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "shiftz_admin_session", Value: "fake-admin"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	_ = srv
}

func TestAuthenticatedDashboardUsesSessionShopID(t *testing.T) {
	shopID := uuid.New()
	otherID := uuid.New()
	token := "sz_owner_" + strings.Repeat("b", 48)
	srv, mux := testDashboardWithAuth(t, shopID, token, &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/week?week_start=2026-07-13&shop_id="+otherID.String(), nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Cafe") {
		t.Fatalf("expected session shop name, body = %q", rec.Body.String())
	}
}

func TestUnauthenticatedDashboardBlocked(t *testing.T) {
	shopID := uuid.New()
	_, mux := testDashboardWithAuth(t, shopID, "sz_owner_test", &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/dashboard/week?week_start=2026-07-13"},
		{http.MethodPost, "/dashboard/generate"},
		{http.MethodPost, "/dashboard/schedules/" + uuid.New().String() + "/approve"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Body = io.NopCloser(strings.NewReader("week_start=2026-07-13"))
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s %s: status = %d, want redirect", tc.method, tc.path, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Fatalf("%s %s: location = %q", tc.method, tc.path, loc)
		}
	}
}

func TestUnauthenticatedHTMXReturns401(t *testing.T) {
	shopID := uuid.New()
	_, mux := testDashboardWithAuth(t, shopID, "sz_owner_test", &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/week?week_start=2026-07-13", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/login" {
		t.Fatalf("HX-Redirect = %q", rec.Header().Get("HX-Redirect"))
	}
}

func TestHandleWeekMissingWeekStart(t *testing.T) {
	shopID := uuid.New()
	srv, mux := testDashboardWithAuth(t, shopID, "sz_owner_test", &fakeShops{shop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC"}})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/week", nil)
	addSessionCookie(t, srv, shopID, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "thiếu ngày bắt đầu tuần") {
		t.Fatalf("body = %q", body)
	}
}

func testDashboardWithAuth(t *testing.T, shopID uuid.UUID, validToken string, shops *fakeShops) (*Server, *http.ServeMux) {
	t.Helper()
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager(base64.RawURLEncoding.EncodeToString(secret[:]), false)
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		shops:         shops,
		shopAuth:      &fakeShopAuth{shop: shops.shop},
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
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

func addSessionCookie(t *testing.T, srv *Server, shopID uuid.UUID, req *http.Request) {
	t.Helper()
	sess := srv.sessions.NewSession(shopID, time.Now())
	value, err := srv.sessions.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
}

type fakeShopAuth struct {
	shop *store.Shop
}

func (f *fakeShopAuth) ByDashboardUsername(ctx context.Context, username string) (*store.Shop, error) {
	if f.shop == nil || store.NormalizeDashboardUsername(username) != f.shop.DashboardUsername {
		return nil, store.ErrNotFound
	}
	return f.shop, nil
}
