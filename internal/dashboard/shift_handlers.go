package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

type shiftRepo interface {
	ListAllByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Shift, error)
	Create(ctx context.Context, shopID uuid.UUID, input store.CreateShiftInput) (*store.Shift, error)
	SetActive(ctx context.Context, shopID, shiftID uuid.UUID, active bool) (*store.Shift, error)
}

// ShiftFormView is the create-shift form state in the panel.
type ShiftFormView struct {
	Name      string
	Weekday   int
	StartTime string
	EndTime   string
	MinStaff  int
	MaxStaff  int
}

// ShiftRowView is one shift row in the owner panel.
type ShiftRowView struct {
	ID          string
	Weekday     string
	Name        string
	TimeRange   string
	MinStaff    int
	MaxStaff    int
	IsActive    bool
	StatusLabel string
}

// ShiftsPanelView is the HTMX-swapped shifts panel.
type ShiftsPanelView struct {
	Error  string
	Shifts []ShiftRowView
	Form   ShiftFormView
}

func defaultShiftForm() ShiftFormView {
	return ShiftFormView{
		Weekday:   1,
		StartTime: "08:00",
		EndTime:   "14:00",
		MinStaff:  1,
		MaxStaff:  2,
	}
}

func buildShiftsPanelView(shifts []*store.Shift, form ShiftFormView, errMsg string) ShiftsPanelView {
	rows := make([]ShiftRowView, len(shifts))
	for i, sh := range shifts {
		status := "đã tắt"
		if sh.IsActive {
			status = "đang dùng"
		}
		rows[i] = ShiftRowView{
			ID:          sh.ID.String(),
			Weekday:     weekdayLabel(sh.Weekday),
			Name:        sh.Name,
			TimeRange:   fmt.Sprintf("%s–%s", sh.StartTime, sh.EndTime),
			MinStaff:    sh.MinStaff,
			MaxStaff:    sh.MaxStaff,
			IsActive:    sh.IsActive,
			StatusLabel: status,
		}
	}
	return ShiftsPanelView{
		Error:  errMsg,
		Shifts: rows,
		Form:   form,
	}
}

func weekdayLabel(weekday int) string {
	if weekday < 0 || weekday >= len(weekdayVI) {
		return strconv.Itoa(weekday)
	}
	return weekdayVI[weekday]
}

func (s *Server) renderShiftsPanel(ctx context.Context, shopID uuid.UUID, form ShiftFormView, errMsg string, w http.ResponseWriter) {
	shifts, err := s.shifts.ListAllByShop(ctx, shopID)
	if err != nil {
		s.log.Error("list shifts", "err", err)
		s.renderShiftsPanelView(w, buildShiftsPanelView(nil, form, "không tải được danh sách ca"))
		return
	}
	s.renderShiftsPanelView(w, buildShiftsPanelView(shifts, form, errMsg))
}

func (s *Server) renderShiftsPanelView(w http.ResponseWriter, view ShiftsPanelView) {
	if err := s.tmpl.render(w, "shifts_panel.html", view); err != nil {
		s.log.Error("render shifts panel", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleCreateShift(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderShiftsPanel(r.Context(), sess.ShopID, defaultShiftForm(), "dữ liệu form không hợp lệ", w)
		return
	}
	form := parseShiftForm(r)
	if _, err := s.shifts.Create(r.Context(), sess.ShopID, formToInput(form)); err != nil {
		if errors.Is(err, store.ErrValidation) {
			s.renderShiftsPanel(r.Context(), sess.ShopID, form, store.ValidationMessage(err), w)
			return
		}
		s.log.Error("create shift", "err", err)
		s.renderShiftsPanel(r.Context(), sess.ShopID, form, "tạo ca thất bại", w)
		return
	}
	s.renderShiftsPanel(r.Context(), sess.ShopID, defaultShiftForm(), "", w)
}

func (s *Server) handleActivateShift(w http.ResponseWriter, r *http.Request) {
	s.handleSetShiftActive(w, r, true)
}

func (s *Server) handleDeactivateShift(w http.ResponseWriter, r *http.Request) {
	s.handleSetShiftActive(w, r, false)
}

func (s *Server) handleSetShiftActive(w http.ResponseWriter, r *http.Request, active bool) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shiftID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.shifts.SetActive(r.Context(), sess.ShopID, shiftID, active); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.log.Error("set shift active", "err", err)
		s.renderShiftsPanel(r.Context(), sess.ShopID, defaultShiftForm(), "cập nhật ca thất bại", w)
		return
	}
	s.renderShiftsPanel(r.Context(), sess.ShopID, defaultShiftForm(), "", w)
}

func parseShiftForm(r *http.Request) ShiftFormView {
	weekday, _ := strconv.Atoi(r.FormValue("weekday"))
	minStaff, _ := strconv.Atoi(r.FormValue("min_staff"))
	maxStaff, _ := strconv.Atoi(r.FormValue("max_staff"))
	return ShiftFormView{
		Name:      r.FormValue("name"),
		Weekday:   weekday,
		StartTime: normalizeTimeInput(r.FormValue("start_time")),
		EndTime:   normalizeTimeInput(r.FormValue("end_time")),
		MinStaff:  minStaff,
		MaxStaff:  maxStaff,
	}
}

func normalizeTimeInput(v string) string {
	if len(v) >= 5 {
		return v[:5]
	}
	return v
}

func formToInput(form ShiftFormView) store.CreateShiftInput {
	return store.CreateShiftInput{
		Name:      form.Name,
		Weekday:   form.Weekday,
		StartTime: form.StartTime,
		EndTime:   form.EndTime,
		MinStaff:  form.MinStaff,
		MaxStaff:  form.MaxStaff,
	}
}

func (s *Server) loadShiftsPanelView(ctx context.Context, shopID uuid.UUID) ShiftsPanelView {
	shifts, err := s.shifts.ListAllByShop(ctx, shopID)
	if err != nil {
		s.log.Error("list shifts for page", "err", err)
		return buildShiftsPanelView(nil, defaultShiftForm(), "")
	}
	return buildShiftsPanelView(shifts, defaultShiftForm(), "")
}
