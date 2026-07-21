package telegram

import (
	"fmt"
	"strings"
	"time"

)

const (
	btnConfirm = "Đúng rồi"
	btnCancel  = "Sửa lại"

	msgStartNoCode = "Chào bạn! Gửi /start <mã-mời> để tham gia lịch quán, rồi nhắn mình lịch rảnh bằng lời thường nha."

	msgBadInviteCode = "Mã mời không khớp với quán nào. Bạn kiểm tra lại với chủ quán nha."

	msgEmployeeInactive = "Tài khoản của bạn đang bị tạm ngưng trong quán này.\nHãy liên hệ chủ quán để được bật lại."

	msgUnknownUser = "Mình chưa biết bạn — tham gia quán trước bằng /start <mã-mời> nha."

	msgShopNotFound = "Không tìm thấy quán của bạn. Hỏi chủ quán giúp mình nha."

	msgNoLLMProvider = "Chưa cấu hình đọc lịch rảnh — nhờ chủ quán thiết lập giúp nha."

	msgParseFailed = `Mình chưa hiểu rõ lắm. Thử nhắn lại kiểu:
Thứ 2–6 rảnh sáng, thứ 4 bận, thứ 6 ưu tiên ca tối nha.`

	msgNoSlotsFound = `Mình chưa thấy lịch rảnh trong tin nhắn đó. Thử lại với ngày và giờ, ví dụ:
Thứ 2–4 rảnh sáng, thứ 5 bận nha.`

	msgInvalidSlots = "Mình chưa chuyển được thành lịch rảnh hợp lệ cho tuần tới. Bạn mô tả lại rõ ngày và giờ giúp mình nha."

	msgConfirmExpired = "Xác nhận này đã hết hạn rồi. Gửi lại lịch rảnh giúp mình nha."

	msgConfirmInvalid = "Xác nhận không hợp lệ."

	msgConfirmNotYours = "Đây không phải xác nhận của bạn."

	msgAvailabilitySaved = "Đã lưu lịch rảnh."

	msgAvailabilitySavedFollowUp = "Mình đã lưu lịch rảnh tuần từ %s nha."

	msgDraftDiscarded = "Đã hủy."

	msgDraftDiscardedFollowUp = "Đã hủy rồi. Khi sẵn sàng thì gửi lại lịch rảnh cho mình nha."
)

func msgWelcomeJoin(displayName string) string {
	return fmt.Sprintf("Chào %s! Bạn đã tham gia rồi. Khi rảnh thì nhắn mình lịch rảnh tuần tới — cứ nói tự nhiên là được nha.", displayName)
}

func formatAvailabilityDraft(d AvailabilityDraft, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	var b strings.Builder
	weekStart := d.WeekStart.In(loc)
	fmt.Fprintf(&b, "Mình hiểu lịch rảnh tuần từ %s như sau:\n\n", weekStart.Format("02/01/2006"))
	for _, slot := range d.Slots {
		start := slot.Start.In(loc)
		end := slot.End.In(loc)
		fmt.Fprintf(&b, "%s %s–%s · %s\n",
			viWeekdayName(start.Weekday()),
			start.Format("15:04"),
			end.Format("15:04"),
			viPreferenceLabel(slot.Preference),
		)
	}
	b.WriteString("\nĐúng chưa?")
	return b.String()
}

func formatClarificationMessage(questions []string) string {
	var b strings.Builder
	b.WriteString("Mình cần hỏi thêm chút trước khi lưu nha:\n")
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", q)
	}
	return strings.TrimSpace(b.String())
}

func viWeekdayName(w time.Weekday) string {
	switch w {
	case time.Monday:
		return "Thứ 2"
	case time.Tuesday:
		return "Thứ 3"
	case time.Wednesday:
		return "Thứ 4"
	case time.Thursday:
		return "Thứ 5"
	case time.Friday:
		return "Thứ 6"
	case time.Saturday:
		return "Thứ 7"
	default:
		return "Chủ nhật"
	}
}

func viPreferenceLabel(pref int) string {
	switch pref {
	case 0:
		return "bận"
	case 2:
		return "ưu tiên"
	default:
		return "rảnh"
	}
}
