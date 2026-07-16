package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/betallsoph/shiftz/internal/store"
)

const maxIncidentDescriptionLen = 2000

type incidentMessenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// IncidentReportView is the HTMX response after submitting an incident report.
type IncidentReportView struct {
	Success bool
	Message string
}

// SetIncidentReporter configures Telegram delivery for owner incident reports.
func (s *Server) SetIncidentReporter(messenger incidentMessenger, chatID int64) {
	s.incidentMessenger = messenger
	s.incidentChatID = chatID
}

func (s *Server) incidentReportEnabled() bool {
	return s.incidentMessenger != nil && s.incidentChatID != 0
}

func (s *Server) handleIncidentReport(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if !s.incidentReportEnabled() {
		s.renderIncidentReportView(w, IncidentReportView{
			Message: "tính năng báo cáo sự cố chưa được cấu hình",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderIncidentReportView(w, IncidentReportView{Message: "dữ liệu form không hợp lệ"})
		return
	}

	shop, err := s.shops.ByID(r.Context(), sess.ShopID)
	if err != nil {
		s.log.Error("load shop for incident report", "err", err)
		s.renderIncidentReportView(w, IncidentReportView{Message: "không tải được thông tin quán"})
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > maxIncidentDescriptionLen {
		s.renderIncidentReportView(w, IncidentReportView{
			Message: fmt.Sprintf("mô tả tối đa %d ký tự", maxIncidentDescriptionLen),
		})
		return
	}

	text := formatIncidentReport(shop, description, time.Now())
	if err := s.incidentMessenger.SendMessage(r.Context(), s.incidentChatID, text); err != nil {
		s.log.Error("send incident report", "shop_id", shop.ID, "err", err)
		s.renderIncidentReportView(w, IncidentReportView{Message: "gửi báo cáo thất bại, thử lại sau"})
		return
	}

	s.renderIncidentReportView(w, IncidentReportView{
		Success: true,
		Message: "đã gửi báo cáo sự cố. Đội ngũ shiftZ sẽ phản hồi qua Telegram.",
	})
}

func (s *Server) renderIncidentReportView(w http.ResponseWriter, view IncidentReportView) {
	if err := s.tmpl.render(w, "incident_report_status.html", view); err != nil {
		s.log.Error("render incident report status", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func formatIncidentReport(shop *store.Shop, description string, now time.Time) string {
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		loc = time.UTC
	}
	when := now.In(loc).Format("2006-01-02 15:04 MST")

	if description == "" {
		description = "Không có mô tả"
	}

	username := shop.DashboardUsername
	if username == "" {
		username = "—"
	}

	var b strings.Builder
	b.WriteString("🚨 Báo cáo sự cố — shiftZ\n\n")
	b.WriteString("Quán: ")
	b.WriteString(shop.Name)
	b.WriteString("\nUsername: ")
	b.WriteString(username)
	b.WriteString("\nShop ID: ")
	b.WriteString(shop.ID.String())
	b.WriteString("\nThời gian: ")
	b.WriteString(when)
	b.WriteString("\n\nMô tả:\n")
	b.WriteString(description)
	return b.String()
}
