package reminder

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// ReminderSecretHeader is the HTTP header carrying the reminder trigger secret.
const ReminderSecretHeader = "X-ShiftZ-Reminder-Secret"

// Ticker runs one reminder processing pass.
type Ticker interface {
	Tick(ctx context.Context, now time.Time) error
}

// HTTPHandler returns an authenticated endpoint for scheduler-triggered ticks.
func HTTPHandler(ticker Ticker, secret string, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	h := &httpTickHandler{ticker: ticker, secret: secret, log: log}
	return http.HandlerFunc(h.serve)
}

type httpTickHandler struct {
	ticker  Ticker
	secret  string
	log     *slog.Logger
	running atomic.Bool
}

func (h *httpTickHandler) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(ReminderSecretHeader)), []byte(h.secret)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !h.running.CompareAndSwap(false, true) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	defer h.running.Store(false)

	if err := h.ticker.Tick(r.Context(), time.Now()); err != nil {
		h.log.Error("reminder tick failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
