package dashboard

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/onboarding"
	"github.com/betallsoph/shiftz/internal/store"
)

func TestSignupDisabledReturns404(t *testing.T) {
	_, mux := newSignupTestServer(t, false, &fakeOnboarder{})

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/signup", nil)
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Body = io.NopCloser(strings.NewReader("shop_name=Cafe&timezone=UTC"))
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /signup: status = %d, want 404", method, rec.Code)
		}
	}
}

func TestSignupGETRendersForm(t *testing.T) {
	_, mux := newSignupTestServer(t, true, &fakeOnboarder{})

	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, want := range []string{"Tên quán", "Múi giờ", "Tạo ca mẫu", "Asia/Ho_Chi_Minh"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in body", want)
		}
	}
}

func TestSignupPOSTMissingName(t *testing.T) {
	_, mux := newSignupTestServer(t, true, &fakeOnboarder{})

	form := url.Values{
		"shop_name":             {""},
		"timezone":              {"UTC"},
		"create_default_shifts": {"on"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "nhập tên quán") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSignupPOSTBadTimezone(t *testing.T) {
	fake := &fakeOnboarder{err: errors.New("onboarding: invalid timezone \"Bad/Zone\": unknown time zone Bad/Zone")}
	_, mux := newSignupTestServer(t, true, fake)

	form := url.Values{
		"shop_name": {"Cafe"},
		"timezone":  {"Bad/Zone"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "múi giờ không hợp lệ") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSignupPOSTSuccessShowsCredentials(t *testing.T) {
	shopID := uuid.New()
	fake := &fakeOnboarder{
		result: &onboarding.Result{
			Shop: &store.Shop{
				ID:         shopID,
				InviteCode: "abc123",
			},
			OwnerToken: "sz_owner_testtoken",
		},
	}
	_, mux := newSignupTestServer(t, true, fake)

	form := url.Values{
		"shop_name":             {"Cafe Mới"},
		"timezone":              {"Asia/Ho_Chi_Minh"},
		"create_default_shifts": {"on"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, shopID.String()) {
		t.Fatalf("missing shop id, body = %q", body)
	}
	if !strings.Contains(body, "sz_owner_testtoken") {
		t.Fatalf("missing owner token, body = %q", body)
	}
	if !strings.Contains(body, "abc123") {
		t.Fatalf("missing invite code, body = %q", body)
	}
	if !strings.Contains(body, "/login/legacy") {
		t.Fatalf("missing legacy login link, body = %q", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if !fake.createDefaultShifts {
		t.Fatal("expected default shifts requested")
	}
}

func TestSignupPOSTSkipsDefaultShiftsWhenUnchecked(t *testing.T) {
	fake := &fakeOnboarder{
		result: &onboarding.Result{
			Shop:       &store.Shop{ID: uuid.New(), InviteCode: "xyz"},
			OwnerToken: "sz_owner_x",
		},
	}
	_, mux := newSignupTestServer(t, true, fake)

	form := url.Values{
		"shop_name": {"Cafe"},
		"timezone":  {"UTC"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fake.createDefaultShifts {
		t.Fatal("expected default shifts skipped")
	}
}

func TestLoginShowsSignupLinkWhenEnabled(t *testing.T) {
	_, mux := newSignupTestServer(t, true, &fakeOnboarder{})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "/signup") {
		t.Fatalf("expected signup link, body = %q", rec.Body.String())
	}
}

func TestLoginHidesSignupLinkWhenDisabled(t *testing.T) {
	_, mux := newSignupTestServer(t, false, &fakeOnboarder{})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "/signup") {
		t.Fatalf("unexpected signup link, body = %q", rec.Body.String())
	}
}

func newSignupTestServer(t *testing.T, signupEnabled bool, onboard shopOnboarder) (*Server, *http.ServeMux) {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionManager("signup-test-secret", false)
	srv := &Server{
		shops:         &fakeShops{shop: &store.Shop{ID: uuid.New(), Name: "Cafe", Timezone: "UTC"}},
		shopAuth:      &noopShopAuth{},
		shopTelegram:  &fakeShopTelegram{},
		shifts:        &fakeShifts{},
		schedules:     &fakeSchedules{},
		employees:     &fakeEmployees{},
		employeeMgmt:  &fakeEmployeeMgmt{},
		availability:  &fakeAvailabilityRepo{},
		planner:       &fakePlanner{},
		onboarding:    onboard,
		signupEnabled: signupEnabled,
		sessions:      sessions,
		log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		tmpl:          &templateSet{tmpl},
	}
	mux := http.NewServeMux()
	srv.Register(mux)
	return srv, mux
}

type fakeOnboarder struct {
	result              *onboarding.Result
	err                 error
	createDefaultShifts bool
}

func (f *fakeOnboarder) CreateShop(ctx context.Context, name, timezone string, createDefaultShifts bool) (*onboarding.Result, error) {
	f.createDefaultShifts = createDefaultShifts
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

type noopOnboarder struct{}

func (noopOnboarder) CreateShop(context.Context, string, string, bool) (*onboarding.Result, error) {
	return nil, errors.New("onboarding not configured")
}
