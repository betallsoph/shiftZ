package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

const (
	ownerStartPrefix     = "owner_"
	ownerConfirmPrefix   = "owner_confirm:"
	ownerCancelPrefix    = "owner_cancel:"
	ownerDraftTTL        = 30 * time.Minute
	ownerLeaveRuleWeight = 5.0
)

const (
	msgOwnerLinked = "Đã liên kết tài khoản chủ quán thành công!\n\n" +
		"Tiếp theo:\n" +
		"1. Tạo nhóm Telegram để gửi thông báo lịch\n" +
		"2. Thêm bot vào nhóm và cấp quyền admin\n" +
		"3. Bot sẽ hỏi nhóm đó là Thông báo hay Chat đội"

	msgOwnerLinkInvalid = "Link liên kết chủ quán không hợp lệ hoặc đã hết hạn. Mở lại link từ dashboard nha."

	msgOwnerLinkTaken = "Telegram này đang liên kết với quán khác rồi. Hủy liên kết cũ trước khi dùng link mới."

	msgOwnerHelp = "Mình đang ở chế độ chủ quán.\n" +
		"Thử nhắn kiểu: \"Lan nghỉ thứ 2\" hoặc \"Mai off chiều mai\"."

	msgOwnerLeaveUnparsed = "Mình chưa hiểu yêu cầu nghỉ. Thử: \"<tên NV> nghỉ thứ 3\" hoặc \"<tên> off chiều mai\"."

	msgOwnerNoEmployees = "Quán chưa có nhân viên nào. Mời nhân viên qua /start <mã-mời> trước nha."

	msgOwnerLeaveSaved = "Đã lưu quy tắc nghỉ."

	msgOwnerLeaveDiscarded = "Đã hủy."

	msgOwnerConfirmExpired = "Xác nhận đã hết hạn. Gửi lại yêu cầu nghỉ nha."

	msgOwnerConfirmNotYours = "Đây không phải xác nhận của bạn."

	msgOwnerConfirmInvalid = "Xác nhận không hợp lệ."
)

// OwnerDraft is a pending owner ad-hoc action awaiting confirm/cancel.
type OwnerDraft struct {
	ID             uuid.UUID
	TelegramUserID int64
	ChatID         int64
	ShopID         uuid.UUID
	EmployeeID     uuid.UUID
	EmployeeName   string
	Date           time.Time
	Scope          string // "full" or "afternoon"
	RawMessage     string
	CreatedAt      time.Time
}

// OwnerDraftStore holds pending owner confirmations (TTL).
type OwnerDraftStore interface {
	Create(ctx context.Context, draft OwnerDraft) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*OwnerDraft, bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// MemoryOwnerDraftStore stores owner drafts in memory with TTL expiry.
type MemoryOwnerDraftStore struct {
	ttl    time.Duration
	mu     sync.Mutex
	drafts map[uuid.UUID]OwnerDraft
}

// NewMemoryOwnerDraftStore returns an in-memory owner draft store.
func NewMemoryOwnerDraftStore(ttl time.Duration) *MemoryOwnerDraftStore {
	if ttl <= 0 {
		ttl = ownerDraftTTL
	}
	return &MemoryOwnerDraftStore{
		ttl:    ttl,
		drafts: make(map[uuid.UUID]OwnerDraft),
	}
}

// Create stores a draft and returns its ID.
func (s *MemoryOwnerDraftStore) Create(_ context.Context, draft OwnerDraft) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	if draft.ID == uuid.Nil {
		draft.ID = uuid.New()
	}
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = time.Now()
	}
	s.drafts[draft.ID] = draft
	return draft.ID, nil
}

// Get returns a draft if it exists and has not expired.
func (s *MemoryOwnerDraftStore) Get(_ context.Context, id uuid.UUID) (*OwnerDraft, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.purgeExpiredLocked(now)
	draft, ok := s.drafts[id]
	if !ok {
		return nil, false, nil
	}
	if now.Sub(draft.CreatedAt) > s.ttl {
		delete(s.drafts, id)
		return nil, false, nil
	}
	copy := draft
	return &copy, true, nil
}

// Delete removes a draft.
func (s *MemoryOwnerDraftStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, id)
	return nil
}

func (s *MemoryOwnerDraftStore) purgeExpiredLocked(now time.Time) {
	for id, draft := range s.drafts {
		if now.Sub(draft.CreatedAt) > s.ttl {
			delete(s.drafts, id)
		}
	}
}

func (b *Bot) handleOwnerStart(ctx context.Context, m *Message, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerLinkInvalid, nil)
	}
	shop, err := b.shops.ConsumeOwnerLinkToken(ctx, token)
	if errors.Is(err, store.ErrInvalidCredentials) || errors.Is(err, store.ErrNotFound) {
		return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerLinkInvalid, nil)
	}
	if err != nil {
		return fmt.Errorf("telegram: consume owner link: %w", err)
	}
	if err := b.shops.SetOwnerTelegramID(ctx, shop.ID, m.From.ID); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerLinkTaken, nil)
		}
		return fmt.Errorf("telegram: set owner telegram id: %w", err)
	}
	return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerLinked, nil)
}

func (b *Bot) handleOwnerText(ctx context.Context, m *Message, shop *store.Shop, text string) error {
	if b.ownerDrafts == nil || b.rules == nil {
		return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerHelp, nil)
	}

	emps, err := b.employees.ListActiveByShop(ctx, shop.ID)
	if err != nil {
		return fmt.Errorf("telegram: list employees for owner: %w", err)
	}
	if len(emps) == 0 {
		return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerNoEmployees, nil)
	}

	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		b.log.Warn("invalid shop timezone, falling back to UTC", "timezone", shop.Timezone, "shop", shop.ID)
		loc = time.UTC
	}

	intent, ok := parseOwnerLeaveIntent(text, emps, time.Now().In(loc), loc)
	if !ok {
		return b.api.SendMessage(ctx, m.Chat.ID, msgOwnerLeaveUnparsed, nil)
	}

	draft := OwnerDraft{
		TelegramUserID: m.From.ID,
		ChatID:         m.Chat.ID,
		ShopID:         shop.ID,
		EmployeeID:     intent.EmployeeID,
		EmployeeName:   intent.EmployeeName,
		Date:           intent.Date,
		Scope:          intent.Scope,
		RawMessage:     text,
	}
	draftID, err := b.ownerDrafts.Create(ctx, draft)
	if err != nil {
		return fmt.Errorf("telegram: create owner draft: %w", err)
	}
	return b.api.SendMessage(ctx, m.Chat.ID, formatOwnerLeaveDraft(draft, loc), OwnerLeaveConfirmKeyboard(draftID))
}

func (b *Bot) handleOwnerConfirm(ctx context.Context, q *CallbackQuery) error {
	draft, ok, err := b.loadOwnedOwnerDraft(ctx, q, ownerConfirmPrefix)
	if err != nil {
		return err
	}
	if !ok {
		return b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerConfirmExpired)
	}

	desc := formatOwnerLeaveDescription(draft)
	ruleJSON := map[string]any{
		"kind":          "day_off",
		"employee_id":   draft.EmployeeID.String(),
		"employee_name": draft.EmployeeName,
		"date":          draft.Date.Format("2006-01-02"),
		"scope":         draft.Scope,
	}
	if _, err := b.rules.Create(ctx, draft.ShopID, desc, ruleJSON, ownerLeaveRuleWeight); err != nil {
		return fmt.Errorf("telegram: create owner leave rule: %w", err)
	}
	_ = b.ownerDrafts.Delete(ctx, draft.ID)

	if err := b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerLeaveSaved); err != nil {
		return err
	}
	chatID := callbackChatID(q, draft.ChatID)
	if err := b.api.SendMessage(ctx, chatID, desc+"\n\nĐã lưu.", nil); err != nil {
		return err
	}

	shop, err := b.shops.ByID(ctx, draft.ShopID)
	if err == nil && shop.TelegramGroupID != 0 {
		_ = b.api.SendMessage(ctx, shop.TelegramGroupID, "📢 "+desc, nil)
	}
	return nil
}

func (b *Bot) handleOwnerCancel(ctx context.Context, q *CallbackQuery) error {
	draft, ok, err := b.loadOwnedOwnerDraft(ctx, q, ownerCancelPrefix)
	if err != nil {
		return err
	}
	if !ok {
		return b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerConfirmExpired)
	}
	_ = b.ownerDrafts.Delete(ctx, draft.ID)
	if err := b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerLeaveDiscarded); err != nil {
		return err
	}
	chatID := callbackChatID(q, draft.ChatID)
	return b.api.SendMessage(ctx, chatID, msgOwnerLeaveDiscarded+" Gửi lại khi cần nha.", nil)
}

func (b *Bot) loadOwnedOwnerDraft(ctx context.Context, q *CallbackQuery, prefix string) (*OwnerDraft, bool, error) {
	if b.ownerDrafts == nil {
		return nil, false, b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerConfirmExpired)
	}
	draftID, err := uuid.Parse(strings.TrimPrefix(q.Data, prefix))
	if err != nil {
		return nil, false, b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerConfirmInvalid)
	}
	draft, ok, err := b.ownerDrafts.Get(ctx, draftID)
	if err != nil {
		return nil, false, fmt.Errorf("telegram: load owner draft: %w", err)
	}
	if !ok {
		return nil, false, nil
	}
	if q.From.ID != draft.TelegramUserID {
		return nil, false, b.api.AnswerCallbackQuery(ctx, q.ID, msgOwnerConfirmNotYours)
	}
	return draft, true, nil
}

// OwnerLeaveConfirmKeyboard builds Confirm/Cancel buttons for an owner leave draft.
func OwnerLeaveConfirmKeyboard(draftID uuid.UUID) *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "Xác nhận", Data: ownerConfirmPrefix + draftID.String()},
		{Text: "Hủy", Data: ownerCancelPrefix + draftID.String()},
	}}}
}

type ownerLeaveIntent struct {
	EmployeeID   uuid.UUID
	EmployeeName string
	Date         time.Time
	Scope        string
}

func parseOwnerLeaveIntent(text string, emps []*store.Employee, now time.Time, loc *time.Location) (ownerLeaveIntent, bool) {
	lower := strings.ToLower(foldSpaces(text))
	if !containsLeaveKeyword(lower) {
		return ownerLeaveIntent{}, false
	}

	emp, ok := matchEmployeeName(text, emps)
	if !ok {
		return ownerLeaveIntent{}, false
	}

	scope := "full"
	if strings.Contains(lower, "chiều") || strings.Contains(lower, "afternoon") ||
		strings.Contains(lower, " buổi chiều") || hasWord(lower, "pm") {
		scope = "afternoon"
	}

	date := nextWeekdayOrRelative(lower, now, loc)
	return ownerLeaveIntent{
		EmployeeID:   emp.ID,
		EmployeeName: emp.DisplayName,
		Date:         date,
		Scope:        scope,
	}, true
}

func containsLeaveKeyword(lower string) bool {
	keywords := []string{"nghỉ", "xin nghỉ", "day off", "leave"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return hasWord(lower, "off")
}

func matchEmployeeName(text string, emps []*store.Employee) (*store.Employee, bool) {
	lower := strings.ToLower(text)
	var best *store.Employee
	bestLen := 0
	for _, emp := range emps {
		name := strings.TrimSpace(emp.DisplayName)
		if name == "" {
			continue
		}
		if idx := strings.Index(lower, strings.ToLower(name)); idx >= 0 {
			if len(name) > bestLen {
				best = emp
				bestLen = len(name)
			}
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

func nextWeekdayOrRelative(lower string, now time.Time, loc *time.Location) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch {
	case strings.Contains(lower, "hôm nay") || hasWord(lower, "today"):
		return today
	case strings.Contains(lower, "ngày mai") || hasWord(lower, "tomorrow") ||
		strings.Contains(lower, "chiều mai") || strings.Contains(lower, "off mai") ||
		strings.Contains(lower, "nghỉ mai"):
		return today.AddDate(0, 0, 1)
	}

	weekdays := []struct {
		names []string
		day   time.Weekday
	}{
		{[]string{"thứ 2", "thu 2", "thứ hai", "monday", "mon"}, time.Monday},
		{[]string{"thứ 3", "thu 3", "thứ ba", "tuesday", "tue"}, time.Tuesday},
		{[]string{"thứ 4", "thu 4", "thứ tư", "wednesday", "wed"}, time.Wednesday},
		{[]string{"thứ 5", "thu 5", "thứ năm", "thursday", "thu"}, time.Thursday},
		{[]string{"thứ 6", "thu 6", "thứ sáu", "friday", "fri"}, time.Friday},
		{[]string{"thứ 7", "thu 7", "thứ bảy", "saturday", "sat"}, time.Saturday},
		{[]string{"chủ nhật", "chu nhat", "sunday", "sun"}, time.Sunday},
	}
	for _, wd := range weekdays {
		for _, name := range wd.names {
			if strings.Contains(lower, name) {
				return nextWeekdayDate(today, wd.day)
			}
		}
	}
	return today.AddDate(0, 0, 1)
}

func nextWeekdayDate(from time.Time, want time.Weekday) time.Time {
	days := (int(want) - int(from.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return from.AddDate(0, 0, days)
}

func formatOwnerLeaveDraft(d OwnerDraft, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	scopeLabel := "cả ngày"
	if d.Scope == "afternoon" {
		scopeLabel = "buổi chiều"
	}
	return fmt.Sprintf(
		"Xác nhận quy tắc nghỉ?\n\nNhân viên: %s\nNgày: %s\nPhạm vi: %s\n\nLưu?",
		d.EmployeeName,
		d.Date.In(loc).Format("02/01/2006"),
		scopeLabel,
	)
}

func formatOwnerLeaveDescription(d *OwnerDraft) string {
	scopeLabel := "cả ngày"
	if d.Scope == "afternoon" {
		scopeLabel = "buổi chiều"
	}
	return fmt.Sprintf("%s nghỉ %s (%s)", d.EmployeeName, d.Date.Format("02/01/2006"), scopeLabel)
}

func foldSpaces(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

func hasWord(s, word string) bool {
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(".,!?;:", r)
	}) {
		if part == word {
			return true
		}
	}
	return false
}
