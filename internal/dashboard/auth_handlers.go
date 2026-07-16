package dashboard

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/betallsoph/shiftz/internal/store"
)

type loginPageData struct {
	Username          string
	Error             string
	Info              string
	ShowPasswordModal bool
	SetPassword       bool
	ForgotSent        bool
}

type passwordResetPageData struct {
	Token string
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
		s.renderLoginPage(w, loginPageData{Error: "dữ liệu form không hợp lệ"})
		return
	}
	rawUsername := r.FormValue("dashboard_username")
	if rawUsername == "" {
		s.renderLoginPage(w, loginPageData{Error: "nhập tên đăng nhập"})
		return
	}
	shop, err := s.shopAuth.ByDashboardUsername(r.Context(), rawUsername)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderLoginPage(w, loginPageData{Username: rawUsername, Error: "tên đăng nhập không đúng"})
			return
		}
		s.log.Error("find dashboard account", "err", err)
		s.renderLoginPage(w, loginPageData{Username: rawUsername, Error: "đăng nhập thất bại"})
		return
	}

	switch r.FormValue("login_step") {
	case "forgot_password":
		s.handleForgotPassword(w, r, shop)
		return
	case "password":
		password := r.FormValue("dashboard_password")
		if password == "" {
			s.showPasswordModal(w, r, shop)
			return
		}
		if err := s.completePasswordLogin(w, r, shop, password, r.FormValue("dashboard_password_confirm"), r.FormValue("dashboard_email"), r.FormValue("dashboard_password_hint")); err != nil {
			return
		}
		return
	default:
		s.showPasswordModal(w, r, shop)
	}
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request, shop *store.Shop) {
	email, err := s.shopAuth.DashboardEmail(r.Context(), shop.ID)
	if err != nil {
		s.log.Error("load dashboard email", "err", err)
	}
	if email != "" && s.mail != nil {
		token, err := s.shopAuth.IssueDashboardPasswordReset(r.Context(), shop.ID)
		if err != nil {
			s.log.Error("issue dashboard password reset", "err", err)
		} else {
			resetURL := s.passwordResetURL(r, token)
			subject := "Đặt lại mật khẩu ShiftBot"
			body := fmt.Sprintf(
				"Xin chào,\n\nNhấn vào link sau để đặt lại mật khẩu dashboard (hết hạn sau 1 giờ):\n\n%s\n\nNếu bạn không yêu cầu, hãy bỏ qua email này.\n",
				resetURL,
			)
			if err := s.mail.Send(r.Context(), email, subject, body); err != nil {
				s.log.Error("send password reset email", "err", err)
			}
		}
	}
	s.renderLoginPage(w, loginPageData{
		Username:          shop.DashboardUsername,
		ShowPasswordModal: true,
		ForgotSent:        true,
		Info:              "Nếu tài khoản có email, chúng tôi đã gửi link đặt lại mật khẩu.",
	})
}

func (s *Server) handlePasswordResetGET(w http.ResponseWriter, r *http.Request) {
	if sess, err := s.sessions.FromRequest(r, time.Now()); err == nil && sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		s.renderPasswordResetPage(w, passwordResetPageData{Error: "link không hợp lệ"})
		return
	}
	s.renderPasswordResetPage(w, passwordResetPageData{Token: token})
}

func (s *Server) handlePasswordResetPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderPasswordResetPage(w, passwordResetPageData{Error: "dữ liệu form không hợp lệ"})
		return
	}
	token := strings.TrimSpace(r.FormValue("token"))
	password := r.FormValue("dashboard_password")
	confirm := r.FormValue("dashboard_password_confirm")
	if token == "" {
		s.renderPasswordResetPage(w, passwordResetPageData{Error: "link không hợp lệ"})
		return
	}
	if password == "" {
		s.renderPasswordResetPage(w, passwordResetPageData{Token: token, Error: "nhập mật khẩu mới"})
		return
	}
	if confirm != password {
		s.renderPasswordResetPage(w, passwordResetPageData{Token: token, Error: "mật khẩu xác nhận không khớp"})
		return
	}
	shop, err := s.shopAuth.ResetDashboardPasswordWithToken(r.Context(), token, password)
	if err != nil {
		if errors.Is(err, store.ErrValidation) {
			s.renderPasswordResetPage(w, passwordResetPageData{Token: token, Error: "mật khẩu phải có ít nhất 6 ký tự"})
			return
		}
		if errors.Is(err, store.ErrInvalidCredentials) {
			s.renderPasswordResetPage(w, passwordResetPageData{Error: "link đặt lại mật khẩu không hợp lệ hoặc đã hết hạn"})
			return
		}
		s.log.Error("reset dashboard password", "err", err)
		s.renderPasswordResetPage(w, passwordResetPageData{Token: token, Error: "không thể đặt lại mật khẩu"})
		return
	}
	if err := s.setOwnerSession(w, shop); err != nil {
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) showPasswordModal(w http.ResponseWriter, r *http.Request, shop *store.Shop) {
	hasPassword, err := s.shopAuth.HasDashboardPassword(r.Context(), shop.ID)
	if err != nil {
		s.log.Error("check dashboard password", "err", err)
		s.renderLoginPage(w, loginPageData{Username: shop.DashboardUsername, Error: "đăng nhập thất bại"})
		return
	}
	s.renderLoginPage(w, loginPageData{
		Username:          shop.DashboardUsername,
		ShowPasswordModal: true,
		SetPassword:       !hasPassword,
	})
}

func (s *Server) completePasswordLogin(w http.ResponseWriter, r *http.Request, shop *store.Shop, password, confirm, email, hint string) error {
	hasPassword, err := s.shopAuth.HasDashboardPassword(r.Context(), shop.ID)
	if err != nil {
		s.log.Error("check dashboard password", "err", err)
		s.renderLoginPage(w, loginPageData{Username: shop.DashboardUsername, Error: "đăng nhập thất bại"})
		return err
	}

	if !hasPassword {
		if confirm != password {
			s.renderLoginPage(w, loginPageData{
				Username:          shop.DashboardUsername,
				ShowPasswordModal: true,
				SetPassword:       true,
				Error:             "mật khẩu xác nhận không khớp",
			})
			return errors.New("password confirm mismatch")
		}
		if strings.TrimSpace(email) == "" {
			s.renderLoginPage(w, loginPageData{
				Username:          shop.DashboardUsername,
				ShowPasswordModal: true,
				SetPassword:       true,
				Error:             "nhập email",
			})
			return errors.New("email required")
		}
		if err := s.shopAuth.SetDashboardCredentials(r.Context(), shop.ID, password, email, hint); err != nil {
			if errors.Is(err, store.ErrValidation) {
				msg := "dữ liệu không hợp lệ"
				switch {
				case strings.Contains(err.Error(), "email"):
					msg = "email không hợp lệ"
				case strings.Contains(err.Error(), "password"):
					msg = "mật khẩu phải có ít nhất 6 ký tự"
				case strings.Contains(err.Error(), "hint"):
					msg = "gợi ý quá dài"
				}
				s.renderLoginPage(w, loginPageData{
					Username:          shop.DashboardUsername,
					ShowPasswordModal: true,
					SetPassword:       true,
					Error:             msg,
				})
				return err
			}
			if errors.Is(err, store.ErrAlreadyExists) {
				s.showPasswordModal(w, r, shop)
				return err
			}
			s.log.Error("set dashboard credentials", "err", err)
			s.renderLoginPage(w, loginPageData{Username: shop.DashboardUsername, Error: "đăng nhập thất bại"})
			return err
		}
	} else if err := s.shopAuth.VerifyDashboardPassword(r.Context(), shop.ID, password); err != nil {
		if errors.Is(err, store.ErrInvalidCredentials) {
			s.renderLoginPage(w, loginPageData{
				Username:          shop.DashboardUsername,
				ShowPasswordModal: true,
				Error:             "mật khẩu không đúng",
			})
			return err
		}
		s.log.Error("verify dashboard password", "err", err)
		s.renderLoginPage(w, loginPageData{Username: shop.DashboardUsername, Error: "đăng nhập thất bại"})
		return err
	}

	if err := s.setOwnerSession(w, shop); err != nil {
		return err
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
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

func (s *Server) renderLoginPage(w http.ResponseWriter, data loginPageData) {
	if data.Username != "" {
		data.Username = store.NormalizeDashboardUsername(data.Username)
	}
	if err := s.tmpl.render(w, "login.html", data); err != nil {
		s.log.Error("render login", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderPasswordResetPage(w http.ResponseWriter, data passwordResetPageData) {
	if err := s.tmpl.render(w, "password_reset.html", data); err != nil {
		s.log.Error("render password reset", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) passwordResetURL(r *http.Request, token string) string {
	base := strings.TrimRight(s.dashboardBaseURL, "/")
	if base == "" {
		base = requestBaseURL(r)
	}
	return base + "/login/reset?token=" + url.QueryEscape(token)
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
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
