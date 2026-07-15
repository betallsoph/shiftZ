package admin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/betallsoph/shiftz/internal/store"
)

type homePageData struct {
	CSRFToken string
	Shops     []shopRowData
	Error     string
}

type shopRowData struct {
	ID              string
	Name            string
	Username        string
	UsernameDisplay string
	Plan            string
	Timezone        string
	CreatedAt       string
	TelegramStatus  string
	CSRFToken       string
}

type successPageData struct {
	Title      string
	Username   string
	ShopID     string
	InviteCode string
	BackURL    string
}

func (s *Server) handleHomeGET(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	csrf, err := s.csrfForSession(sess)
	if err != nil {
		s.log.Error("admin csrf", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	shops, err := s.shops.ListAll(r.Context())
	if err != nil {
		s.log.Error("admin list shops", "err", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	rows := make([]shopRowData, len(shops))
	for i, shop := range shops {
		rows[i] = shopRowFromStore(shop, csrf)
	}
	if err := s.tmpl.render(w, "home.html", homePageData{CSRFToken: csrf, Shops: rows}); err != nil {
		s.log.Error("admin render home", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateShopPOST(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireMutation(w, r, sess) {
		return
	}
	name := strings.TrimSpace(r.FormValue("shop_name"))
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}
	username := r.FormValue("dashboard_username")
	plan := strings.TrimSpace(r.FormValue("plan"))
	createDefaultShifts := r.FormValue("create_default_shifts") != ""

	if name == "" {
		s.renderHomeError(w, r, sess, "nhập tên quán")
		return
	}
	if err := store.ValidateDashboardUsername(username); err != nil {
		s.renderHomeError(w, r, sess, "username không hợp lệ")
		return
	}
	if err := store.ValidatePlan(plan); err != nil {
		s.renderHomeError(w, r, sess, "gói dịch vụ không hợp lệ")
		return
	}

	shop, err := s.provision.CreateShopWithAccount(r.Context(), name, timezone, username, plan, createDefaultShifts)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			s.renderHomeError(w, r, sess, "username đã được sử dụng")
			return
		}
		if strings.Contains(err.Error(), "invalid timezone") {
			s.renderHomeError(w, r, sess, "múi giờ không hợp lệ")
			return
		}
		s.log.Error("admin create shop", "err", err)
		s.renderHomeError(w, r, sess, "tạo quán thất bại")
		return
	}
	s.renderSuccess(w, successPageData{
		Title:      "Quán đã tạo",
		Username:   shop.DashboardUsername,
		ShopID:     shop.ID.String(),
		InviteCode: shop.InviteCode,
		BackURL:    "/admin",
	})
}

func (s *Server) handleProvisionPOST(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireMutation(w, r, sess) {
		return
	}
	shopID := r.PathValue("id")
	username := r.FormValue("dashboard_username")
	plan := strings.TrimSpace(r.FormValue("plan"))
	if err := store.ValidateDashboardUsername(username); err != nil {
		s.renderHomeError(w, r, sess, "username không hợp lệ")
		return
	}
	if err := store.ValidatePlan(plan); err != nil {
		s.renderHomeError(w, r, sess, "gói dịch vụ không hợp lệ")
		return
	}
	shop, err := s.shops.ProvisionDashboardAccount(r.Context(), shopID, username, plan)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderHomeError(w, r, sess, "không tìm thấy quán")
			return
		}
		if errors.Is(err, store.ErrAlreadyExists) {
			s.renderHomeError(w, r, sess, "username đã được sử dụng")
			return
		}
		s.log.Error("admin provision", "err", err)
		s.renderHomeError(w, r, sess, "cấp tài khoản thất bại")
		return
	}
	s.renderSuccess(w, successPageData{
		Title:      "Tài khoản đã cấp",
		Username:   shop.DashboardUsername,
		ShopID:     shop.ID.String(),
		InviteCode: shop.InviteCode,
		BackURL:    "/admin",
	})
}

func (s *Server) handleUpdatePlanPOST(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.requireMutation(w, r, sess) {
		return
	}
	shopID := r.PathValue("id")
	plan := strings.TrimSpace(r.FormValue("plan"))
	if err := store.ValidatePlan(plan); err != nil {
		s.renderHomeError(w, r, sess, "gói dịch vụ không hợp lệ")
		return
	}
	if _, err := s.shops.UpdatePlan(r.Context(), shopID, plan); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderHomeError(w, r, sess, "không tìm thấy quán")
			return
		}
		s.log.Error("admin update plan", "err", err)
		s.renderHomeError(w, r, sess, "cập nhật gói thất bại")
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) renderHomeError(w http.ResponseWriter, r *http.Request, sess *Session, msg string) {
	csrf, err := s.csrfForSession(sess)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	shops, err := s.shops.ListAll(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	rows := make([]shopRowData, len(shops))
	for i, shop := range shops {
		rows[i] = shopRowFromStore(shop, csrf)
	}
	if err := s.tmpl.render(w, "home.html", homePageData{CSRFToken: csrf, Shops: rows, Error: msg}); err != nil {
		s.log.Error("admin render home error", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderSuccess(w http.ResponseWriter, data successPageData) {
	if err := s.tmpl.render(w, "success.html", data); err != nil {
		s.log.Error("admin render success", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func shopRowFromStore(shop *store.Shop, csrf string) shopRowData {
	return shopRowData{
		ID:              shop.ID.String(),
		Name:            shop.Name,
		Username:        shop.DashboardUsername,
		UsernameDisplay: formatUsername(shop.DashboardUsername),
		Plan:            shop.Plan,
		Timezone:        shop.Timezone,
		CreatedAt:       shop.CreatedAt.UTC().Format("2006-01-02"),
		TelegramStatus:  formatTelegramStatus(shop.TelegramGroupID),
		CSRFToken:       csrf,
	}
}
