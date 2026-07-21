package dashboard

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

// TelegramEmployeeRowView is one employee row in the Telegram linking section.
type TelegramEmployeeRowView struct {
	ID             string
	DisplayName    string
	RoleLabel      string
	IsActive       bool
	StatusLabel    string
	TelegramLinked bool
	TelegramUserID int64
	LinkLabel      string
}

// TelegramEmployeesView is the HTMX-swapped employee Telegram linking block.
// Include via: {{template "telegram_employees.html" .Employees}}
type TelegramEmployeesView struct {
	Error                  string
	Notice                 string
	Employees              []TelegramEmployeeRowView
	EmployeeInviteURL      string
	EmployeeInviteShareURL string
}

func buildTelegramEmployeesView(
	employees []*store.Employee,
	panelErr string,
	inviteURL string,
	inviteShareURL string,
) TelegramEmployeesView {
	rows := make([]TelegramEmployeeRowView, len(employees))
	for i, emp := range employees {
		status := "đã tạm ngưng"
		if emp.IsActive {
			status = "đang làm"
		}
		linked := emp.TelegramUserID != 0
		linkLabel := "Chưa liên kết"
		if linked {
			linkLabel = "Đã liên kết"
		}
		rows[i] = TelegramEmployeeRowView{
			ID:             emp.ID.String(),
			DisplayName:    emp.DisplayName,
			RoleLabel:      roleDisplayLabel(emp.Role),
			IsActive:       emp.IsActive,
			StatusLabel:    status,
			TelegramLinked: linked,
			TelegramUserID: emp.TelegramUserID,
			LinkLabel:      linkLabel,
		}
	}
	return TelegramEmployeesView{
		Error:                  panelErr,
		Employees:              rows,
		EmployeeInviteURL:      inviteURL,
		EmployeeInviteShareURL: inviteShareURL,
	}
}

func (s *Server) loadTelegramEmployeesView(ctx context.Context, shop *store.Shop) TelegramEmployeesView {
	inviteURL, inviteShareURL := employeeInviteLinks(s.botUsername, shop.InviteCode)
	employees, err := s.employeeMgmt.ListAllByShop(ctx, shop.ID)
	if err != nil {
		s.log.Error("list employees for telegram link panel", "err", err)
		return buildTelegramEmployeesView(nil, "không tải được danh sách nhân viên", inviteURL, inviteShareURL)
	}
	return buildTelegramEmployeesView(employees, "", inviteURL, inviteShareURL)
}

func (s *Server) renderTelegramEmployees(ctx context.Context, shopID uuid.UUID, notice, errMsg string, w http.ResponseWriter) {
	shop, err := s.shops.ByID(ctx, shopID)
	if err != nil {
		s.log.Error("load shop for telegram employees", "err", err)
		s.renderTelegramEmployeesView(w, TelegramEmployeesView{Error: "không tải được thông tin quán"})
		return
	}
	view := s.loadTelegramEmployeesView(ctx, shop)
	if errMsg != "" {
		view.Error = errMsg
	}
	view.Notice = notice
	s.renderTelegramEmployeesView(w, view)
}

func (s *Server) renderTelegramEmployeesView(w http.ResponseWriter, view TelegramEmployeesView) {
	if err := s.tmpl.render(w, "telegram_employees.html", view); err != nil {
		s.log.Error("render telegram employees", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleTelegramEmployeesRefresh(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	s.renderTelegramEmployees(r.Context(), sess.ShopID, "đã làm mới trạng thái liên kết", "", w)
}
