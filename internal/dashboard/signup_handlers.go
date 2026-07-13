package dashboard

import (
	"context"
	"net/http"
	"strings"

	"github.com/betallsoph/shiftz/internal/onboarding"
)

type shopOnboarder interface {
	CreateShop(ctx context.Context, name, timezone string, createDefaultShifts bool) (*onboarding.Result, error)
}

type signupPageData struct {
	Error               string
	ShopName            string
	Timezone            string
	CreateDefaultShifts bool
}

type signupSuccessData struct {
	ShopID     string
	OwnerToken string
	InviteCode string
}

func (s *Server) handleSignupGET(w http.ResponseWriter, r *http.Request) {
	if !s.signupEnabled {
		http.NotFound(w, r)
		return
	}
	if err := s.tmpl.render(w, "signup.html", signupPageData{
		Timezone:            "Asia/Ho_Chi_Minh",
		CreateDefaultShifts: true,
	}); err != nil {
		s.log.Error("render signup", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleSignupPOST(w http.ResponseWriter, r *http.Request) {
	if !s.signupEnabled {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderSignupError(w, signupPageData{Timezone: "Asia/Ho_Chi_Minh", CreateDefaultShifts: true}, "dữ liệu form không hợp lệ")
		return
	}

	name := strings.TrimSpace(r.FormValue("shop_name"))
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	createDefaultShifts := r.FormValue("create_default_shifts") != ""

	form := signupPageData{
		ShopName:            name,
		Timezone:            timezone,
		CreateDefaultShifts: createDefaultShifts,
	}
	if name == "" {
		s.renderSignupError(w, form, "nhập tên quán")
		return
	}

	result, err := s.onboarding.CreateShop(r.Context(), name, timezone, createDefaultShifts)
	if err != nil {
		if strings.Contains(err.Error(), "invalid timezone") {
			s.renderSignupError(w, form, "múi giờ không hợp lệ")
			return
		}
		s.log.Error("create shop", "err", err)
		s.renderSignupError(w, form, "tạo quán thất bại")
		return
	}

	if err := s.tmpl.render(w, "signup_success.html", signupSuccessData{
		ShopID:     result.Shop.ID.String(),
		OwnerToken: result.OwnerToken,
		InviteCode: result.Shop.InviteCode,
	}); err != nil {
		s.log.Error("render signup success", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderSignupError(w http.ResponseWriter, form signupPageData, msg string) {
	form.Error = msg
	if err := s.tmpl.render(w, "signup.html", form); err != nil {
		s.log.Error("render signup error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
