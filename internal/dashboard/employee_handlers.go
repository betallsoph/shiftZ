package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

type employeeAdmin interface {
	ListAllByShop(ctx context.Context, shopID uuid.UUID) ([]*store.Employee, error)
	Update(ctx context.Context, shopID, employeeID uuid.UUID, input store.UpdateEmployeeInput) (*store.Employee, error)
	SetActive(ctx context.Context, shopID, employeeID uuid.UUID, active bool) (*store.Employee, error)
}

// EmployeeFormView is one employee's inline edit form state.
type EmployeeFormView struct {
	DisplayName     string
	Role            string
	MaxHoursPerWeek string
}

// EmployeeRowView is one employee row in the owner panel.
type EmployeeRowView struct {
	ID             string
	DisplayName    string
	RoleLabel      string
	MaxHoursLabel  string
	TelegramLinked bool
	IsActive       bool
	StatusLabel    string
	FieldError     string
	Form           EmployeeFormView
}

// EmployeesPanelView is the HTMX-swapped employees panel.
type EmployeesPanelView struct {
	Error                  string
	Employees              []EmployeeRowView
	EmployeeInviteURL      string
	EmployeeInviteShareURL string
	Telegram               TelegramSetupView
}

type employeePendingEdit struct {
	employeeID uuid.UUID
	form       EmployeeFormView
	errMsg     string
}

func employeeFormFromDB(emp *store.Employee) EmployeeFormView {
	return EmployeeFormView{
		DisplayName:     emp.DisplayName,
		Role:            emp.Role,
		MaxHoursPerWeek: formatMaxHours(emp.MaxHoursPerWeek),
	}
}

func formatMaxHours(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func buildEmployeesPanelView(
	employees []*store.Employee,
	pending *employeePendingEdit,
	panelErr string,
	inviteURL string,
	inviteShareURL string,
	telegram TelegramSetupView,
) EmployeesPanelView {
	rows := make([]EmployeeRowView, len(employees))
	for i, emp := range employees {
		form := employeeFormFromDB(emp)
		fieldErr := ""
		if pending != nil && emp.ID == pending.employeeID {
			form = pending.form
			fieldErr = pending.errMsg
		}
		roleLabel := roleDisplayLabel(emp.Role)
		if pending != nil && emp.ID == pending.employeeID {
			roleLabel = roleDisplayLabel(form.Role)
		}
		status := "đã tạm ngưng"
		if emp.IsActive {
			status = "đang làm"
		}
		rows[i] = EmployeeRowView{
			ID:             emp.ID.String(),
			DisplayName:    emp.DisplayName,
			RoleLabel:      roleLabel,
			MaxHoursLabel:  formatMaxHours(emp.MaxHoursPerWeek),
			TelegramLinked: emp.TelegramUserID != 0,
			IsActive:       emp.IsActive,
			StatusLabel:    status,
			FieldError:     fieldErr,
			Form:           form,
		}
		if pending != nil && emp.ID == pending.employeeID {
			if maxHours, err := strconv.ParseFloat(form.MaxHoursPerWeek, 64); err == nil {
				rows[i].MaxHoursLabel = formatMaxHours(maxHours)
			} else {
				rows[i].MaxHoursLabel = form.MaxHoursPerWeek
			}
		}
	}
	return EmployeesPanelView{
		Error:                  panelErr,
		Employees:              rows,
		EmployeeInviteURL:      inviteURL,
		EmployeeInviteShareURL: inviteShareURL,
		Telegram:               telegram,
	}
}

func roleDisplayLabel(role string) string {
	if s := strings.TrimSpace(role); s != "" {
		return s
	}
	return "chưa đặt"
}

func (s *Server) renderEmployeesPanel(ctx context.Context, shopID uuid.UUID, pending *employeePendingEdit, panelErr string, telegram *TelegramSetupView, w http.ResponseWriter) {
	shop, err := s.shops.ByID(ctx, shopID)
	if err != nil {
		s.log.Error("load shop for employees panel", "err", err)
		s.renderEmployeesPanelView(w, buildEmployeesPanelView(nil, pending, "không tải được thông tin quán", "", "", TelegramSetupView{}))
		return
	}
	inviteURL, inviteShareURL := employeeInviteLinks(s.botUsername, shop.InviteCode)
	tg := buildTelegramSetupView(shop, "", time.Time{}, time.Now())
	if telegram != nil {
		tg = *telegram
	}
	employees, err := s.employeeMgmt.ListAllByShop(ctx, shopID)
	if err != nil {
		s.log.Error("list employees", "err", err)
		s.renderEmployeesPanelView(w, buildEmployeesPanelView(nil, pending, "không tải được danh sách nhân viên", inviteURL, inviteShareURL, tg))
		return
	}
	s.renderEmployeesPanelView(w, buildEmployeesPanelView(employees, pending, panelErr, inviteURL, inviteShareURL, tg))
}

func (s *Server) renderEmployeesPanelView(w http.ResponseWriter, view EmployeesPanelView) {
	if err := s.tmpl.render(w, "employees_panel.html", view); err != nil {
		s.log.Error("render employees panel", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleUpdateEmployee(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderEmployeesPanel(r.Context(), sess.ShopID, nil, "dữ liệu form không hợp lệ", nil, w)
		return
	}
	employeeID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	form := parseEmployeeForm(r)
	input, parseErr := formToEmployeeInput(form)
	if parseErr != nil {
		s.renderEmployeesPanel(r.Context(), sess.ShopID, &employeePendingEdit{
			employeeID: employeeID,
			form:       form,
			errMsg:     parseErr.Error(),
		}, "", nil, w)
		return
	}
	if _, err := s.employeeMgmt.Update(r.Context(), sess.ShopID, employeeID, input); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, store.ErrValidation) {
			s.renderEmployeesPanel(r.Context(), sess.ShopID, &employeePendingEdit{
				employeeID: employeeID,
				form:       form,
				errMsg:     store.ValidationMessage(err),
			}, "", nil, w)
			return
		}
		s.log.Error("update employee", "err", err)
		s.renderEmployeesPanel(r.Context(), sess.ShopID, &employeePendingEdit{
			employeeID: employeeID,
			form:       form,
			errMsg:     "cập nhật nhân viên thất bại",
		}, "", nil, w)
		return
	}
	s.renderEmployeesPanel(r.Context(), sess.ShopID, nil, "", nil, w)
}

func (s *Server) handleActivateEmployee(w http.ResponseWriter, r *http.Request) {
	s.handleSetEmployeeActive(w, r, true)
}

func (s *Server) handleDeactivateEmployee(w http.ResponseWriter, r *http.Request) {
	s.handleSetEmployeeActive(w, r, false)
}

func (s *Server) handleSetEmployeeActive(w http.ResponseWriter, r *http.Request, active bool) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	employeeID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.employeeMgmt.SetActive(r.Context(), sess.ShopID, employeeID, active); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.log.Error("set employee active", "err", err)
		s.renderEmployeesPanel(r.Context(), sess.ShopID, nil, "cập nhật trạng thái thất bại", nil, w)
		return
	}
	s.renderEmployeesPanel(r.Context(), sess.ShopID, nil, "", nil, w)
}

func parseEmployeeForm(r *http.Request) EmployeeFormView {
	return EmployeeFormView{
		DisplayName:     r.FormValue("display_name"),
		Role:            r.FormValue("role"),
		MaxHoursPerWeek: r.FormValue("max_hours_per_week"),
	}
}

func formToEmployeeInput(form EmployeeFormView) (store.UpdateEmployeeInput, error) {
	maxHours, err := strconv.ParseFloat(strings.TrimSpace(form.MaxHoursPerWeek), 64)
	if err != nil {
		return store.UpdateEmployeeInput{}, fmt.Errorf("giới hạn giờ/tuần không hợp lệ")
	}
	return store.UpdateEmployeeInput{
		DisplayName:     form.DisplayName,
		Role:            form.Role,
		MaxHoursPerWeek: maxHours,
	}, nil
}

func (s *Server) loadEmployeesPanelView(ctx context.Context, shop *store.Shop, telegram TelegramSetupView, inviteURL, inviteShareURL string) EmployeesPanelView {
	employees, err := s.employeeMgmt.ListAllByShop(ctx, shop.ID)
	if err != nil {
		s.log.Error("list employees for page", "err", err)
		return buildEmployeesPanelView(nil, nil, "", inviteURL, inviteShareURL, telegram)
	}
	return buildEmployeesPanelView(employees, nil, "", inviteURL, inviteShareURL, telegram)
}
