package dashboard

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

type shopAuthenticator interface {
	VerifyDashboardToken(ctx context.Context, shopID uuid.UUID, token string) (*store.Shop, error)
}

type loginPageData struct {
	Error         string
	SignupEnabled bool
}

func (s *Server) handleLoginGET(w http.ResponseWriter, r *http.Request) {
	if sess, err := s.sessions.FromRequest(r, time.Now()); err == nil && sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := s.tmpl.render(w, "login.html", loginPageData{SignupEnabled: s.signupEnabled}); err != nil {
		s.log.Error("render login", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLoginError(w, "dữ liệu form không hợp lệ")
		return
	}
	rawShop := r.FormValue("shop_id")
	rawToken := r.FormValue("owner_token")
	if rawShop == "" || rawToken == "" {
		s.renderLoginError(w, "nhập mã cửa hàng và token chủ quán")
		return
	}
	shopID, err := uuid.Parse(rawShop)
	if err != nil {
		s.renderLoginError(w, "mã cửa hàng hoặc token không đúng")
		return
	}
	shop, err := s.shopAuth.VerifyDashboardToken(r.Context(), shopID, rawToken)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrInvalidCredentials) {
			s.renderLoginError(w, "mã cửa hàng hoặc token không đúng")
			return
		}
		s.log.Error("verify dashboard token", "err", err)
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}

	now := time.Now()
	sess := s.sessions.NewSession(shop.ID, now)
	if err := s.sessions.SetCookie(w, sess); err != nil {
		s.log.Error("set session cookie", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	_ = shop
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) renderLoginError(w http.ResponseWriter, msg string) {
	if err := s.tmpl.render(w, "login.html", loginPageData{Error: msg, SignupEnabled: s.signupEnabled}); err != nil {
		s.log.Error("render login error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) sessionFromRequest(r *http.Request) (*Session, error) {
	return s.sessions.FromRequest(r, time.Now())
}

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (*Session, bool) {
	sess, err := s.sessionFromRequest(r)
	if err != nil || sess == nil {
		s.unauthorized(w, r)
		return nil, false
	}
	return sess, true
}

func (s *Server) unauthorized(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
