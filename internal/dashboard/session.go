package dashboard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const sessionCookieName = "shiftz_session"

const defaultSessionMaxAge = 7 * 24 * time.Hour

// Session is the signed owner dashboard session payload.
type Session struct {
	ShopID    uuid.UUID
	ExpiresAt time.Time
}

func (s *Session) Expired(now time.Time) bool {
	return !s.ExpiresAt.After(now)
}

// SessionManager signs and verifies dashboard session cookies.
type SessionManager struct {
	secret       []byte
	cookieSecure bool
	maxAge       time.Duration
}

// NewSessionManager creates a session manager for dashboard auth cookies.
func NewSessionManager(secret string, cookieSecure bool) *SessionManager {
	return &SessionManager{
		secret:       []byte(secret),
		cookieSecure: cookieSecure,
		maxAge:       defaultSessionMaxAge,
	}
}

// NewSession builds a session for shopID expiring after the manager max age.
func (m *SessionManager) NewSession(shopID uuid.UUID, now time.Time) *Session {
	return &Session{
		ShopID:    shopID,
		ExpiresAt: now.Add(m.maxAge),
	}
}

// Sign encodes and signs a session for the session cookie value.
func (m *SessionManager) Sign(sess *Session) (string, error) {
	payload := encodeSessionPayload(sess)
	mac := hmac.New(sha256.New, m.secret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

// Verify parses and validates a signed session cookie value.
func (m *SessionManager) Verify(value string, now time.Time) (*Session, error) {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok || payload == "" || sig == "" {
		return nil, errors.New("dashboard: malformed session")
	}
	mac := hmac.New(sha256.New, m.secret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return nil, err
	}
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, errors.New("dashboard: invalid session signature")
	}
	sess, err := decodeSessionPayload(payload)
	if err != nil {
		return nil, err
	}
	if sess.Expired(now) {
		return nil, errors.New("dashboard: session expired")
	}
	return sess, nil
}

// SetCookie writes the signed session cookie.
func (m *SessionManager) SetCookie(w http.ResponseWriter, sess *Session) error {
	value, err := m.Sign(sess)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(m.maxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.cookieSecure,
	})
	return nil
}

// ClearCookie removes the session cookie.
func (m *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.cookieSecure,
	})
}

// FromRequest reads and verifies the session cookie on r.
func (m *SessionManager) FromRequest(r *http.Request, now time.Time) (*Session, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	return m.Verify(c.Value, now)
}

func encodeSessionPayload(sess *Session) string {
	raw := sess.ShopID.String() + "|" + strconv.FormatInt(sess.ExpiresAt.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSessionPayload(payload string) (*Session, error) {
	rawBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("dashboard: decode session: %w", err)
	}
	parts := strings.Split(string(rawBytes), "|")
	if len(parts) != 2 {
		return nil, errors.New("dashboard: invalid session payload")
	}
	shopID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, errors.New("dashboard: invalid shop id in session")
	}
	expUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, errors.New("dashboard: invalid expiry in session")
	}
	return &Session{
		ShopID:    shopID,
		ExpiresAt: time.Unix(expUnix, 0).UTC(),
	}, nil
}
