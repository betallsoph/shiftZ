package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivezReturnsOKWithoutDB(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakePinger{err: errors.New("db down")})

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHealthzAliasLivez(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakePinger{err: errors.New("db down")})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyzReturnsOKWhenPingSucceeds(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakePinger{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyzReturns503WhenPingFails(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, &fakePinger{err: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

type fakePinger struct {
	err error
}

func (f *fakePinger) Ping(context.Context) error {
	return f.err
}
