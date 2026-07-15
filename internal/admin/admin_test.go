package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/ent/enttest"
	"github.com/betallsoph/shiftz/internal/store"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func TestAdminDisabledReturns404(t *testing.T) {
	cfg := &config.Config{AdminPortalEnabled: false}
	srv, err := New(cfg, noopProvisioner{}, &fakeShopAdmin{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)

	for _, path := range []string{"/admin", "/admin/login"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d", path, rec.Code)
		}
	}
}

func TestAdminLoginSuccess(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "test-pass-123")
	form := url.Values{"username": {"adminuser"}, "password": {"test-pass-123"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/admin" {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("cookies = %+v", cookies)
	}
	_ = srv
}

func TestAdminLoginWrongPassword(t *testing.T) {
	_, mux := testAdminServer(t, "adminuser", "right-pass")
	form := url.Values{"username": {"adminuser"}, "password": {"wrong-pass"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "đăng nhập thất bại") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAdminTamperedCookieRejected(t *testing.T) {
	mgr := NewSessionManager("admin-secret-key-32bytes-min!!", false)
	sess := mgr.NewSession("admin", time.Now())
	value, err := mgr.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	value = value[:len(value)-2] + "xx"
	if _, err := mgr.Verify(value, time.Now()); err == nil {
		t.Fatal("expected tampered cookie to fail")
	}
}

func TestAdminExpiredCookieRejected(t *testing.T) {
	mgr := NewSessionManager("admin-secret-key-32bytes-min!!", false)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	sess := &Session{Subject: "admin", ExpiresAt: now.Add(-time.Minute)}
	value, err := mgr.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Verify(value, now); err == nil {
		t.Fatal("expected expired cookie to fail")
	}
}

func TestAdminRateLimit(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "right-pass")
	ip := "203.0.113.10"
	for i := 0; i < 5; i++ {
		form := url.Values{"username": {"adminuser"}, "password": {"wrong"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
	}
	form := url.Values{"username": {"adminuser"}, "password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "quá nhiều lần") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	_ = srv
}

func TestAdminUnauthenticatedRedirect(t *testing.T) {
	_, mux := testAdminServer(t, "adminuser", "pass")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAdminListShops(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "pass")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Demo Shop") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestAdminCreateShopSuccess(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "pass")
	csrf := adminCSRF(t, srv)
	form := url.Values{
		"csrf_token":            {csrf},
		"shop_name":             {"New Cafe"},
		"timezone":              {"UTC"},
		"dashboard_username":    {"new.cafe"},
		"plan":                  {"starter"},
		"create_default_shifts": {"1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sz_owner_") {
		t.Fatalf("expected owner token in body")
	}
	if !strings.Contains(rec.Body.String(), "new.cafe") {
		t.Fatalf("expected username in body")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestAdminCreateShopDuplicateUsernameNoOrphan(t *testing.T) {
	st := newAdminTestClient(t)
	provision := NewProvisionService(st)
	shops := NewShopService(st)
	ctx := context.Background()
	if _, err := provision.CreateShopWithAccount(ctx, "First", "UTC", "dup.user", "free", false); err != nil {
		t.Fatal(err)
	}
	cfg := testAdminConfig(t, "adminuser", "pass")
	srv, err := New(cfg, provision, shops, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	csrf := adminCSRF(t, srv)
	form := url.Values{
		"csrf_token":         {csrf},
		"shop_name":          {"Second"},
		"timezone":           {"UTC"},
		"dashboard_username": {"dup.user"},
		"plan":               {"free"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "username đã được sử dụng") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	count, err := st.Client.Shop.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("shop count = %d, want 1", count)
	}
}

func TestAdminCSRFReject(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "pass")
	form := url.Values{"shop_name": {"X"}, "dashboard_username": {"x"}, "plan": {"free"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestOwnerCookieDoesNotAccessAdmin(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "pass")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: "shiftz_session", Value: "fake-owner-session"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("status=%d", rec.Code)
	}
	_ = srv
}

func TestAdminProvisionExistingShop(t *testing.T) {
	st := newAdminTestClient(t)
	ctx := context.Background()
	shop, err := st.Shops.Create(ctx, "Existing", "UTC", 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := testAdminConfig(t, "adminuser", "pass")
	srv, err := New(cfg, NewProvisionService(st), NewShopService(st), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	csrf := adminCSRF(t, srv)
	form := url.Values{
		"csrf_token":         {csrf},
		"dashboard_username": {"existing.shop"},
		"plan":               {"pro"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops/"+shop.ID.String()+"/provision", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sz_owner_") {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdatePlan(t *testing.T) {
	st := newAdminTestClient(t)
	ctx := context.Background()
	shop, err := st.Shops.Create(ctx, "Plan Shop", "UTC", 0)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := st.Shops.ProvisionDashboardAccount(ctx, shop.ID, "plan.shop", "free")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testAdminConfig(t, "adminuser", "pass")
	srv, err := New(cfg, NewProvisionService(st), NewShopService(st), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	csrf := adminCSRF(t, srv)
	form := url.Values{"csrf_token": {csrf}, "plan": {"starter"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops/"+creds.Shop.ID.String()+"/plan", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d", rec.Code)
	}
	updated, err := st.Shops.ByID(ctx, creds.Shop.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Plan != "starter" {
		t.Fatalf("plan=%q", updated.Plan)
	}
}

func TestAdminRotateToken(t *testing.T) {
	st := newAdminTestClient(t)
	ctx := context.Background()
	shop, err := st.Shops.Create(ctx, "Rotate", "UTC", 0)
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.Shops.ProvisionDashboardAccount(ctx, shop.ID, "rotate.shop", "free")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testAdminConfig(t, "adminuser", "pass")
	srv, err := New(cfg, NewProvisionService(st), NewShopService(st), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	csrf := adminCSRF(t, srv)
	form := url.Values{"csrf_token": {csrf}, "confirm": {"yes"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/shops/"+shop.ID.String()+"/rotate-owner-token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "sz_owner_") {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), first.OwnerToken) {
		t.Fatal("old token should not appear")
	}
}

func TestAdminLogout(t *testing.T) {
	srv, mux := testAdminServer(t, "adminuser", "pass")
	csrf := adminCSRF(t, srv)
	form := url.Values{"csrf_token": {csrf}}
	req := httptest.NewRequest(http.MethodPost, "/admin/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAdminCookie(t, srv, req)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/login" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func testAdminConfig(t *testing.T, username, password string) *config.Config {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	return &config.Config{
		AdminPortalEnabled: true,
		AdminUsername:      username,
		AdminPasswordHash:  string(hash),
		AdminSessionSecret: base64.RawURLEncoding.EncodeToString(secret[:]),
		CookieSecure:       false,
	}
}

func testAdminServer(t *testing.T, username, password string) (*Server, *http.ServeMux) {
	t.Helper()
	cfg := testAdminConfig(t, username, password)
	srv, err := New(cfg, &fakeProvisioner{}, &fakeShopAdmin{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

func addAdminCookie(t *testing.T, srv *Server, req *http.Request) {
	t.Helper()
	sess := srv.sessions.NewSession(srv.username, time.Now())
	value, err := srv.sessions.Sign(sess)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
}

func adminCSRF(t *testing.T, srv *Server) string {
	t.Helper()
	sess := srv.sessions.NewSession(srv.username, time.Now())
	token, err := srv.sessions.CSRFToken(sess)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

type fakeProvisioner struct{}

func (f *fakeProvisioner) CreateShopWithAccount(ctx context.Context, name, timezone, username, plan string, createDefaultShifts bool) (*store.ProvisionedCredentials, error) {
	_ = ctx
	_ = timezone
	_ = createDefaultShifts
	if username == "dup.user" {
		return nil, store.ErrAlreadyExists
	}
	return &store.ProvisionedCredentials{
		Shop: &store.Shop{
			ID:                uuid.New(),
			Name:              name,
			Timezone:          "UTC",
			InviteCode:        "invite01",
			Plan:              plan,
			DashboardUsername: username,
		},
		OwnerToken: "sz_owner_" + strings.Repeat("c", 48),
	}, nil
}

type fakeShopAdmin struct{}

func (f *fakeShopAdmin) ListAll(context.Context) ([]*store.Shop, error) {
	return []*store.Shop{{
		ID:       uuid.New(),
		Name:     "Demo Shop",
		Timezone: "UTC",
		Plan:     "free",
	}}, nil
}

func (f *fakeShopAdmin) ProvisionDashboardAccount(context.Context, string, string, string) (*store.ProvisionedCredentials, error) {
	return nil, store.ErrNotFound
}

func (f *fakeShopAdmin) UpdatePlan(context.Context, string, string) (*store.Shop, error) {
	return nil, store.ErrNotFound
}

func (f *fakeShopAdmin) RotateDashboardToken(context.Context, string) (*store.ProvisionedCredentials, error) {
	return nil, store.ErrNotFound
}

func newAdminTestClient(t *testing.T) *store.Store {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	return store.NewWithClient(client)
}
