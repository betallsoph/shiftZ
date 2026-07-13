package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterDisabledReturns404(t *testing.T) {
	mux := http.NewServeMux()
	RegisterDisabled(mux)

	paths := []string{
		"/api/v1/schedules",
		"/api/v1/dev/generate-schedule",
		"/api/v1/schedules/" + "00000000-0000-0000-0000-000000000001" + "/approve",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", path, rec.Code)
		}
	}
}
