package dashboard

import (
	"errors"
	"net/http"
	"time"

	"github.com/betallsoph/shiftz/internal/store"
)

type loginPageData struct {
	Error string
}

func (s *Server) handleLoginGET(w http.ResponseWriter, r *http.Request) {
	if sess, err := s.sessions.FromRequest(r, time.Now()); err == nil && sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := s.tmpl.render(w, "login.html", loginPageData{}); err != nil {
		s.log.Error("render login", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderLoginError(w, "dữ liệu form không hợp lệ")
		return
	}
	rawUsername := r.FormValue("dashboard_username")
	if rawUsername == "" {
		s.renderLoginError(w, "nhập tên đăng nhập")
		return
	}
	shop, err := s.shopAuth.ByDashboardUsername(r.Context(), rawUsername)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderLoginError(w, "tên đăng nhập không đúng")
			return
		}
		s.log.Error("find dashboard account", "err", err)
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}
	if err := s.setOwnerSession(w, shop); err != nil {
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) setOwnerSession(w http.ResponseWriter, shop *store.Shop) error {
	now := time.Now()
	sess := s.sessions.NewSession(shop.ID, now)
	if err := s.sessions.SetCookie(w, sess); err != nil {
		s.log.Error("set session cookie", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return err
	}
	_ = shop
	return nil
}

func (s *Server) renderLoginError(w http.ResponseWriter, msg string) {
	if err := s.tmpl.render(w, "login.html", loginPageData{Error: msg}); err != nil {
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
