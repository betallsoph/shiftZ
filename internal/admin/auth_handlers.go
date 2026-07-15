package admin

import (
	"crypto/subtle"
	"net/http"
)

type loginPageData struct {
	Error string
}

func (s *Server) handleLoginGET(w http.ResponseWriter, r *http.Request) {
	if sess, err := s.sessions.FromRequest(r, s.now()); err == nil && sess != nil && sess.Subject == s.username {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := s.tmpl.render(w, "login.html", loginPageData{}); err != nil {
		s.log.Error("admin render login", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	ip := clientIP(r)
	if !s.limiter.allow(ip, now) {
		s.renderLoginError(w, "quá nhiều lần đăng nhập thất bại, thử lại sau")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		s.limiter.recordFailure(ip, now)
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}
	if !constantTimeEqual(username, s.username) {
		s.limiter.recordFailure(ip, now)
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}
	if !constantTimeEqual(password, s.password) {
		s.limiter.recordFailure(ip, now)
		s.renderLoginError(w, "đăng nhập thất bại")
		return
	}
	s.limiter.reset(ip)
	sess := s.sessions.NewSession(s.username, now)
	if err := s.sessions.SetCookie(w, sess); err != nil {
		s.log.Error("admin set session", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleLogoutPOST(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireMutation(w, r, sess) {
		return
	}
	s.sessions.ClearCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (s *Server) renderLoginError(w http.ResponseWriter, msg string) {
	if err := s.tmpl.render(w, "login.html", loginPageData{Error: msg}); err != nil {
		s.log.Error("admin render login error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
