package reminder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTicker struct {
	calls atomic.Int32
	err   error
	block chan struct{}
}

func (f *fakeTicker) Tick(context.Context, time.Time) error {
	f.calls.Add(1)
	if f.block != nil {
		<-f.block
	}
	return f.err
}

func (f *fakeTicker) count() int {
	return int(f.calls.Load())
}

func testHTTPHandler(t *testing.T, ticker Ticker) http.Handler {
	t.Helper()
	return HTTPHandler(ticker, "secret", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHTTPHandlerRejectsMissingSecret(t *testing.T) {
	ticker := &fakeTicker{}
	handler := testHTTPHandler(t, ticker)

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if ticker.count() != 0 {
		t.Fatal("tick should not run")
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response leaked secret: %q", rec.Body.String())
	}
}

func TestHTTPHandlerRejectsWrongSecret(t *testing.T) {
	ticker := &fakeTicker{}
	handler := testHTTPHandler(t, ticker)

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	req.Header.Set(ReminderSecretHeader, "wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if ticker.count() != 0 {
		t.Fatal("tick should not run")
	}
}

func TestHTTPHandlerRunsTickOnValidSecret(t *testing.T) {
	ticker := &fakeTicker{}
	handler := testHTTPHandler(t, ticker)

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	req.Header.Set(ReminderSecretHeader, "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if ticker.count() != 1 {
		t.Fatalf("tick calls = %d, want 1", ticker.count())
	}
}

func TestHTTPHandlerTickErrorReturns500(t *testing.T) {
	ticker := &fakeTicker{err: errors.New("boom")}
	handler := testHTTPHandler(t, ticker)

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	req.Header.Set(ReminderSecretHeader, "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response leaked secret: %q", rec.Body.String())
	}
}

func TestHTTPHandlerRejectsNonPost(t *testing.T) {
	ticker := &fakeTicker{}
	handler := testHTTPHandler(t, ticker)

	req := httptest.NewRequest(http.MethodGet, "/internal/reminders/tick", nil)
	req.Header.Set(ReminderSecretHeader, "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if ticker.count() != 0 {
		t.Fatal("tick should not run")
	}
}

func TestHTTPHandlerSkipsOverlappingTick(t *testing.T) {
	block := make(chan struct{})
	ticker := &fakeTicker{block: block}
	handler := testHTTPHandler(t, ticker)

	started := make(chan struct{})
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
		req.Header.Set(ReminderSecretHeader, "secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("first status = %d, want 204", rec.Code)
		}
		close(started)
	}()

	time.Sleep(20 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/internal/reminders/tick", nil)
	req.Header.Set(ReminderSecretHeader, "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("overlap status = %d, want 204", rec.Code)
	}
	if ticker.count() != 1 {
		t.Fatalf("tick calls = %d, want 1", ticker.count())
	}

	close(block)
	<-started
}
