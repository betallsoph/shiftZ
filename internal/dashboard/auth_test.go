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
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Đặt mật khẩu") {
		t.Fatalf("expected set-password modal, body = %q", rec.Body.String())
	}
}

func TestLoginSetsPasswordAndSucceeds(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	_, mux := testDashboardWithAuth(t, shopID, "", &fakeShops{shop: shop})

	form := url.Values{
		"dashboard_username":          {"demo.cafe"},
		"login_step":                  {"password"},
		"dashboard_email":             {"owner@example.com"},
		"dashboard_password":        {"secret123"},
		"dashboard_password_confirm": {"secret123"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestLoginRequiresEmailOnFirstSetup(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	_, mux := testDashboardWithAuth(t, shopID, "", &fakeShops{shop: shop})

	form := url.Values{
		"dashboard_username":          {"demo.cafe"},
		"login_step":                  {"password"},
		"dashboard_password":          {"secret123"},
		"dashboard_password_confirm":  {"secret123"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "nhập email") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestForgotPasswordShowsConfirmation(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	auth := &fakeShopAuth{shop: shop, hasPassword: true, password: "secret123", email: "owner@example.com"}
	srv, mux := testDashboardWithShopAuth(t, shopID, auth, &fakeShops{shop: shop})
	srv.SetPasswordResetMail(&fakeMailSender{}, "https://shiftz.test")

	form := url.Values{
		"dashboard_username": {"demo.cafe"},
		"login_step":           {"forgot_password"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Quên mật khẩu?") {
		t.Fatalf("expected forgot link, body = %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "đã gửi link đặt lại mật khẩu") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if auth.resetIssued != 1 {
		t.Fatalf("resetIssued = %d", auth.resetIssued)
	}
}

func TestPasswordResetPageSetsNewPassword(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	auth := &fakeShopAuth{shop: shop, hasPassword: true, password: "oldpass1", email: "owner@example.com", resetToken: "sz_pwreset_test"}
	_, mux := testDashboardWithShopAuth(t, shopID, auth, &fakeShops{shop: shop})

	form := url.Values{
		"token":                        {"sz_pwreset_test"},
		"dashboard_password":           {"newpass1"},
		"dashboard_password_confirm":   {"newpass1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login/reset", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if auth.password != "newpass1" {
		t.Fatalf("password = %q", auth.password)
	}
}

func TestLoginFailsWithWrongPassword(t *testing.T) {
	shopID := uuid.New()
	shop := &store.Shop{ID: shopID, Name: "Cafe", Timezone: "UTC", DashboardUsername: "demo.cafe"}
	auth := &fakeShopAuth{shop: shop, hasPassword: true}
	_, mux := testDashboardWithAuthWithShopAuth(t, shopID, auth)

	form := url.Values{
		"dashboard_username": {"demo.cafe"},
		"login_step":           {"password"},
		"dashboard_password":   {"wrongpass"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "mật khẩu không đúng") {
		t.Fatalf("body = %q", rec.Body.String())
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
	auth := &fakeShopAuth{shop: shops.shop}
	return testDashboardWithShopAuth(t, shopID, auth, shops)
}

func testDashboardWithAuthWithShopAuth(t *testing.T, shopID uuid.UUID, auth *fakeShopAuth) (*Server, *http.ServeMux) {
	t.Helper()
	return testDashboardWithShopAuth(t, shopID, auth, &fakeShops{shop: auth.shop})
}

func testDashboardWithShopAuth(t *testing.T, shopID uuid.UUID, auth *fakeShopAuth, shops *fakeShops) (*Server, *http.ServeMux) {
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
		shopAuth:      auth,
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
	shop         *store.Shop
	hasPassword  bool
	password     string
	email        string
	hint         string
	resetToken   string
	resetIssued  int
}

type fakeMailSender struct {
	sent int
}

func (f *fakeMailSender) Send(ctx context.Context, to, subject, body string) error {
	f.sent++
	return nil
}

func (f *fakeShopAuth) ByDashboardUsername(ctx context.Context, username string) (*store.Shop, error) {
	if f.shop == nil || store.NormalizeDashboardUsername(username) != f.shop.DashboardUsername {
		return nil, store.ErrNotFound
	}
	return f.shop, nil
}

func (f *fakeShopAuth) HasDashboardPassword(ctx context.Context, shopID uuid.UUID) (bool, error) {
	if f.shop == nil || f.shop.ID != shopID {
		return false, store.ErrNotFound
	}
	return f.hasPassword, nil
}

func (f *fakeShopAuth) SetDashboardCredentials(ctx context.Context, shopID uuid.UUID, password, email, hint string) error {
	if f.shop == nil || f.shop.ID != shopID {
		return store.ErrNotFound
	}
	if f.hasPassword {
		return store.ErrAlreadyExists
	}
	if err := store.ValidateDashboardPassword(password); err != nil {
		return err
	}
	if err := store.ValidateDashboardEmail(email); err != nil {
		return err
	}
	if err := store.ValidateDashboardPasswordHint(hint); err != nil {
		return err
	}
	f.hasPassword = true
	f.password = password
	f.email = store.NormalizeDashboardEmail(email)
	f.hint = strings.TrimSpace(hint)
	return nil
}

func (f *fakeShopAuth) DashboardEmail(ctx context.Context, shopID uuid.UUID) (string, error) {
	if f.shop == nil || f.shop.ID != shopID {
		return "", store.ErrNotFound
	}
	return f.email, nil
}

func (f *fakeShopAuth) IssueDashboardPasswordReset(ctx context.Context, shopID uuid.UUID) (string, error) {
	if f.shop == nil || f.shop.ID != shopID {
		return "", store.ErrNotFound
	}
	if f.email == "" {
		return "", store.ErrNotFound
	}
	f.resetIssued++
	if f.resetToken == "" {
		f.resetToken = "sz_pwreset_test"
	}
	return f.resetToken, nil
}

func (f *fakeShopAuth) ResetDashboardPasswordWithToken(ctx context.Context, token, password string) (*store.Shop, error) {
	if token != f.resetToken {
		return nil, store.ErrInvalidCredentials
	}
	if err := store.ValidateDashboardPassword(password); err != nil {
		return nil, err
	}
	f.password = password
	f.hasPassword = true
	return f.shop, nil
}

func (f *fakeShopAuth) VerifyDashboardPassword(ctx context.Context, shopID uuid.UUID, password string) error {
	if f.shop == nil || f.shop.ID != shopID {
		return store.ErrNotFound
	}
	if !f.hasPassword || f.password != password {
		return store.ErrInvalidCredentials
	}
	return nil
}
