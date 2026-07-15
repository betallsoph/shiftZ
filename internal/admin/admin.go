// Package admin provides the platform owner admin portal.
package admin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/store"
)

// ShopProvisioner creates shops with dashboard accounts for the admin portal.
type ShopProvisioner interface {
	CreateShopWithAccount(ctx context.Context, name, timezone, username, plan string, createDefaultShifts bool) (*store.ProvisionedCredentials, error)
}
type ShopAdmin interface {
	ListAll(ctx context.Context) ([]*store.Shop, error)
	ProvisionDashboardAccount(ctx context.Context, shopID string, username, plan string) (*store.ProvisionedCredentials, error)
	UpdatePlan(ctx context.Context, shopID, plan string) (*store.Shop, error)
	RotateDashboardToken(ctx context.Context, shopID string) (*store.ProvisionedCredentials, error)
}

// Server renders the platform admin portal.
type Server struct {
	enabled      bool
	username     string
	passwordHash string
	cookieSecure bool
	sessions     *SessionManager
	provision    ShopProvisioner
	shops        ShopAdmin
	limiter      *loginLimiter
	log          *slog.Logger
	tmpl         *templateSet
}

// New builds an admin server from config. When disabled, Register returns 404 for /admin/*.
func New(cfg *config.Config, provision ShopProvisioner, shops ShopAdmin, log *slog.Logger) (*Server, error) {
	if err := cfg.RequireAdminPortal(); err != nil {
		return nil, err
	}
	tmpl, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		enabled:      cfg.AdminPortalEnabled,
		username:     cfg.AdminUsername,
		passwordHash: cfg.AdminPasswordHash,
		cookieSecure: cfg.CookieSecure,
		provision:    provision,
		shops:        shops,
		limiter:      newLoginLimiter(),
		log:          log,
		tmpl:         &templateSet{tmpl},
	}
	if cfg.AdminPortalEnabled {
		s.sessions = NewSessionManager(cfg.AdminSessionSecret, cfg.CookieSecure)
	}
	return s, nil
}

// Register mounts admin routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	if !s.enabled {
		mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		return
	}
	mux.HandleFunc("GET /admin/login", s.handleLoginGET)
	mux.HandleFunc("POST /admin/login", s.handleLoginPOST)
	mux.HandleFunc("POST /admin/logout", s.handleLogoutPOST)
	mux.HandleFunc("GET /admin", s.handleHomeGET)
	mux.HandleFunc("POST /admin/shops", s.handleCreateShopPOST)
	mux.HandleFunc("POST /admin/shops/{id}/provision", s.handleProvisionPOST)
	mux.HandleFunc("POST /admin/shops/{id}/plan", s.handleUpdatePlanPOST)
	mux.HandleFunc("POST /admin/shops/{id}/rotate-owner-token", s.handleRotateTokenPOST)
}

func (s *Server) now() time.Time { return time.Now().UTC() }

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (*Session, bool) {
	sess, err := s.sessions.FromRequest(r, s.now())
	if err != nil || sess == nil || sess.Subject != s.username {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return nil, false
	}
	return sess, true
}

func (s *Server) requireMutation(w http.ResponseWriter, r *http.Request, sess *Session) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	if !validateOriginHost(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return false
	}
	if !s.sessions.VerifyCSRF(sess, r.FormValue("csrf_token")) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) csrfForSession(sess *Session) (string, error) {
	return s.sessions.CSRFToken(sess)
}

func formatTelegramStatus(groupID int64) string {
	if groupID != 0 {
		return "đã kết nối"
	}
	return "chưa kết nối"
}

func formatUsername(username string) string {
	if username == "" {
		return "chưa cấp"
	}
	return username
}

// noopProvisioner satisfies compile-time checks in tests.
type noopProvisioner struct{}

func (noopProvisioner) CreateShopWithAccount(context.Context, string, string, string, string, bool) (*store.ProvisionedCredentials, error) {
	return nil, fmt.Errorf("admin: not implemented")
}
