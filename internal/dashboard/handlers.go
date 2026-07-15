package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shop, err := s.shops.ByID(r.Context(), sess.ShopID)
	if err != nil {
		s.log.Error("load shop for page", "err", err)
		s.sessions.ClearCookie(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	employeeInviteURL, employeeInviteShareURL := employeeInviteLinks(s.botUsername, shop.InviteCode)
	telegram := buildTelegramSetupView(shop, "", time.Time{}, time.Now())
	if err := s.tmpl.render(w, "page.html", PageData{
		Today:     time.Now().Format(dateLayout),
		ShopName:  shop.Name,
		Shifts:    s.loadShiftsPanelView(r.Context(), sess.ShopID),
		Employees: s.loadEmployeesPanelView(r.Context(), shop, telegram, employeeInviteURL, employeeInviteShareURL),
	}); err != nil {
		s.log.Error("render page", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleWeek(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireSession(w, r); !ok {
		return
	}
	s.renderWeek(w, r, "", nil)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shop, shopID, weekStart, errMsg := s.parseWeekForm(r, sess.ShopID)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}

	result, err := s.planner.GenerateWeek(r.Context(), shopID, weekStart)
	if err != nil {
		if errors.Is(err, planner.ErrSchedulesExist) {
			data, loadErr := s.loadWeekData(r.Context(), shopID, weekStart)
			if loadErr != nil {
				s.log.Error("load week after exist", "err", loadErr)
				s.renderWeekView(w, WeekView{Error: "không tải được lịch hiện có"})
				return
			}
			s.renderWeekView(w, buildWeekView(shop, weekStart, data.schedules, data.employees, data.availabilities, nil,
				"Tuần này đã có lịch.", nil))
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			s.renderWeekView(w, WeekView{Error: "không tìm thấy cửa hàng"})
			return
		}
		s.log.Error("generate week", "err", err)
		s.renderWeekView(w, WeekView{Error: "tạo lịch thất bại"})
		return
	}

	data, err := s.loadWeekData(r.Context(), shopID, weekStart)
	if err != nil {
		s.log.Error("load week after generate", "err", err)
		s.renderWeekView(w, WeekView{Error: "không tải được lịch vừa tạo"})
		return
	}
	s.renderWeekView(w, buildWeekView(shop, weekStart, data.schedules, data.employees, data.availabilities, result.Warnings, "", result))
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shop, shopID, weekStart, errMsg := s.parseWeekForm(r, sess.ShopID)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}

	scheduleID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.renderWeekView(w, WeekView{Error: "mã lịch không hợp lệ"})
		return
	}

	if _, err := s.schedules.Approve(r.Context(), shopID, scheduleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.renderWeekView(w, WeekView{
				ShopName:  shop.Name,
				WeekStart: weekStart.Format(dateLayout),
				Error:     "không tìm thấy lịch",
			})
			return
		}
		s.log.Error("approve schedule", "err", err)
		s.renderWeekView(w, WeekView{Error: "duyệt lịch thất bại"})
		return
	}

	s.renderWeek(w, r, "", nil)
}

type weekPanelData struct {
	schedules      []*store.Schedule
	employees      []*store.Employee
	availabilities []*store.Availability
}

func (s *Server) loadWeekData(ctx context.Context, shopID uuid.UUID, weekStart time.Time) (*weekPanelData, error) {
	schedules, err := s.schedules.ListByShopWeek(ctx, shopID, weekStart)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	employees, err := s.employees.ListActiveByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	availabilities, err := s.availability.ListByShopWeek(ctx, shopID, weekStart)
	if err != nil {
		return nil, fmt.Errorf("list availability: %w", err)
	}
	return &weekPanelData{
		schedules:      schedules,
		employees:      employees,
		availabilities: availabilities,
	}, nil
}

func (s *Server) renderWeek(w http.ResponseWriter, r *http.Request, notice string, generated *planner.GenerateResult) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shop, shopID, weekStart, errMsg := s.parseWeekForm(r, sess.ShopID)
	if errMsg != "" {
		s.renderWeekView(w, WeekView{Error: errMsg})
		return
	}
	_ = shopID

	data, err := s.loadWeekData(r.Context(), sess.ShopID, weekStart)
	if err != nil {
		s.log.Error("load week panel", "err", err)
		s.renderWeekView(w, WeekView{Error: "không tải được dữ liệu tuần"})
		return
	}

	view := buildWeekView(shop, weekStart, data.schedules, data.employees, data.availabilities, nil, notice, generated)
	s.renderWeekView(w, view)
}

func (s *Server) renderWeekView(w http.ResponseWriter, view WeekView) {
	if err := s.tmpl.render(w, "week_panel.html", view); err != nil {
		s.log.Error("render week panel", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) parseWeekForm(r *http.Request, sessionShopID uuid.UUID) (*store.Shop, uuid.UUID, time.Time, string) {
	if err := r.ParseForm(); err != nil {
		return nil, uuid.Nil, time.Time{}, "dữ liệu form không hợp lệ"
	}
	shop, err := s.shops.ByID(r.Context(), sessionShopID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, uuid.Nil, time.Time{}, "không tìm thấy cửa hàng"
		}
		s.log.Error("load shop", "err", err)
		return nil, uuid.Nil, time.Time{}, "không tải được cửa hàng"
	}
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		s.log.Error("load timezone", "timezone", shop.Timezone, "err", err)
		return nil, uuid.Nil, time.Time{}, "múi giờ cửa hàng không hợp lệ"
	}
	rawWeek := r.FormValue("week_start")
	if rawWeek == "" {
		return nil, uuid.Nil, time.Time{}, "thiếu ngày bắt đầu tuần"
	}
	parsed, err := time.ParseInLocation(dateLayout, rawWeek, loc)
	if err != nil {
		return nil, uuid.Nil, time.Time{}, "ngày không hợp lệ (YYYY-MM-DD)"
	}
	return shop, sessionShopID, store.WeekStart(parsed, loc), ""
}
