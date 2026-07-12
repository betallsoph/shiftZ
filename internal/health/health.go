package health

import (
	"context"
	"net/http"
)

// Pinger checks database reachability for readiness probes.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Register mounts liveness and readiness endpoints on mux.
//
//   - GET /livez  — process is up, no database access
//   - GET /readyz — database ping succeeds
//   - GET /healthz — alias for /livez (backward compatible)
func Register(mux *http.ServeMux, db Pinger) {
	mux.HandleFunc("GET /livez", handleLive)
	mux.HandleFunc("GET /healthz", handleLive)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		handleReady(w, r, db)
	})
}

func handleLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func handleReady(w http.ResponseWriter, r *http.Request, db Pinger) {
	if db == nil {
		http.Error(w, "db not configured", http.StatusServiceUnavailable)
		return
	}
	if err := db.Ping(r.Context()); err != nil {
		http.Error(w, "db unreachable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
