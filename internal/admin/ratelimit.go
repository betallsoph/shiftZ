package admin

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginMaxFailures = 5
	loginWindow      = 15 * time.Minute
)

type loginLimiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: make(map[string][]time.Time)}
}

func (l *loginLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-loginWindow)
	attempts := pruneAttempts(l.failures[ip], cutoff)
	l.failures[ip] = attempts
	return len(attempts) < loginMaxFailures
}

func (l *loginLimiter) recordFailure(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-loginWindow)
	l.failures[ip] = append(pruneAttempts(l.failures[ip], cutoff), now)
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
}

func pruneAttempts(attempts []time.Time, cutoff time.Time) []time.Time {
	out := attempts[:0]
	for _, at := range attempts {
		if at.After(cutoff) {
			out = append(out, at)
		}
	}
	return out
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(ip)
		}
		return strings.TrimSpace(xff)
	}
	host, _, ok := strings.Cut(r.RemoteAddr, ":")
	if ok {
		return host
	}
	return r.RemoteAddr
}

func validateOriginHost(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	// Accept same-scheme/host origins only.
	reqHost := r.Host
	if reqHost == "" {
		return false
	}
	origin = strings.TrimSuffix(origin, "/")
	for _, prefix := range []string{"https://" + reqHost, "http://" + reqHost} {
		if origin == prefix {
			return true
		}
	}
	return false
}
