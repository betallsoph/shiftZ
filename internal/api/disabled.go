package api

import "net/http"

// RegisterDisabled mounts a catch-all handler that returns 404 for /api/ paths.
func RegisterDisabled(mux *http.ServeMux) {
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
