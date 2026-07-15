package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/llm"
	"github.com/betallsoph/shiftz/internal/store"
)

const (
	availConfirmPrefix = "avail_confirm:"
	availCancelPrefix  = "avail_cancel:"
	votePrefix         = "vote:"
)

// Messenger sends Telegram messages and acknowledges callbacks.
type Messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) error
	AnswerCallbackQuery(ctx context.Context, callbackID, text string) error
}

// AvailabilityParser turns free text into structured slots. Satisfied by
// *llm.Service.
type AvailabilityParser interface {
	ParseAvailability(ctx context.Context, text string, weekStart time.Time, loc *time.Location) ([]llm.AvailabilitySlot, error)
}

// ShopDirectory loads shop metadata. Satisfied by *store.ShopRepo.
type ShopDirectory interface {
	ByID(ctx context.Context, id uuid.UUID) (*store.Shop, error)
}

// EmployeeDirectory is the slice of the store the bot needs to identify and
// enroll employees. Satisfied by *store.EmployeeRepo.
type EmployeeDirectory interface {
	ByTelegramID(ctx context.Context, telegramUserID int64) (*store.Employee, error)
	Join(ctx context.Context, inviteCode string, telegramUserID int64, displayName string) (*store.Employee, error)
}

// AvailabilityStore persists parsed availability. Satisfied by
// *store.AvailabilityRepo.
type AvailabilityStore interface {
	ReplaceWeek(ctx context.Context, shopID, employeeID uuid.UUID, weekStart time.Time, slots []store.AvailabilitySlot, rawMessage string) error
}

// VoteStore records schedule votes. Satisfied by *store.VoteRepo.
type VoteStore interface {
	Record(ctx context.Context, shopID, scheduleID, employeeID uuid.UUID) error
}

// Bot routes Telegram updates to handlers.
type Bot struct {
	api          Messenger
	parser       AvailabilityParser
	shops        ShopDirectory
	employees    EmployeeDirectory
	availability AvailabilityStore
	votes        VoteStore
	drafts       AvailabilityDraftStore
	log          *slog.Logger
}

// NewBot wires the bot's dependencies.
func NewBot(
	api Messenger,
	parser AvailabilityParser,
	shops ShopDirectory,
	employees EmployeeDirectory,
	availability AvailabilityStore,
	votes VoteStore,
	drafts AvailabilityDraftStore,
	log *slog.Logger,
) *Bot {
	if log == nil {
		log = slog.Default()
	}
	return &Bot{
		api:          api,
		parser:       parser,
		shops:        shops,
		employees:    employees,
		availability: availability,
		votes:        votes,
		drafts:       drafts,
		log:          log,
	}
}

// HandleUpdate dispatches one incoming update.
func (b *Bot) HandleUpdate(ctx context.Context, u Update) error {
	switch {
	case u.Message != nil:
		return b.handleMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		return b.handleCallback(ctx, u.CallbackQuery)
	default:
		return nil
	}
}

func (b *Bot) handleMessage(ctx context.Context, m *Message) error {
	if m.From == nil {
		return nil
	}
	text := strings.TrimSpace(m.Text)

	if isGroupChat(m.Chat) {
		return nil
	}

	switch {
	case strings.HasPrefix(text, "/start"):
		return b.handleStart(ctx, m, strings.TrimSpace(strings.TrimPrefix(text, "/start")))
	case text != "":
		return b.handleAvailabilityText(ctx, m, text)
	default:
		return nil
	}
}

func (b *Bot) handleStart(ctx context.Context, m *Message, inviteCode string) error {
	if inviteCode == "" {
		return b.api.SendMessage(ctx, m.Chat.ID,
			"Hi! Send /start <invite-code> to join your shop's schedule, then just message me your availability in plain words.", nil)
	}
	name := strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
	emp, err := b.employees.Join(ctx, inviteCode, m.From.ID, name)
	if errors.Is(err, store.ErrNotFound) {
		return b.api.SendMessage(ctx, m.Chat.ID, "That invite code doesn't match any shop. Double-check it with your manager.", nil)
	}
	if errors.Is(err, store.ErrEmployeeInactive) {
		return b.api.SendMessage(ctx, m.Chat.ID,
			"Tài khoản của bạn đang bị tạm ngưng trong quán này.\nHãy liên hệ chủ quán để được bật lại.", nil)
	}
	if err != nil {
		return fmt.Errorf("telegram: join shop: %w", err)
	}
	return b.api.SendMessage(ctx, m.Chat.ID,
		fmt.Sprintf("Welcome, %s! You're on the roster. Send me your availability for next week whenever you're ready — plain language is fine.", emp.DisplayName), nil)
}

func (b *Bot) handleAvailabilityText(ctx context.Context, m *Message, text string) error {
	if !isPrivateChat(m.Chat) {
		return nil
	}
	emp, err := b.employees.ByTelegramID(ctx, m.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		return b.api.SendMessage(ctx, m.Chat.ID, "I don't know you yet — join your shop first with /start <invite-code>.", nil)
	}
	if err != nil {
		return fmt.Errorf("telegram: lookup employee: %w", err)
	}

	shop, err := b.shops.ByID(ctx, emp.ShopID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return b.api.SendMessage(ctx, m.Chat.ID, "Your shop could not be found. Ask your manager for help.", nil)
		}
		return fmt.Errorf("telegram: lookup shop: %w", err)
	}

	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		b.log.Warn("invalid shop timezone, falling back to UTC", "timezone", shop.Timezone, "shop", shop.ID)
		loc = time.UTC
	}

	weekStart := nextMonday(time.Now().In(loc))
	parsed, err := b.parser.ParseAvailability(ctx, text, weekStart, loc)
	if errors.Is(err, llm.ErrNoProvider) {
		return b.api.SendMessage(ctx, m.Chat.ID, "Availability parsing isn't configured yet — ask your admin to set up the LLM provider.", nil)
	}
	var clarify *llm.ClarificationError
	if errors.As(err, &clarify) {
		return b.api.SendMessage(ctx, m.Chat.ID, formatClarificationMessage(clarify.Questions), nil)
	}
	if err != nil {
		b.log.Warn("availability parse failed", "err", err, "employee", emp.ID)
		return b.api.SendMessage(ctx, m.Chat.ID, "Sorry, I couldn't understand that. Try something like: \"I can work Mon-Fri mornings, not Wednesday, prefer Friday evening.\"", nil)
	}
	if len(parsed) == 0 {
		return b.api.SendMessage(ctx, m.Chat.ID, "I didn't find any availability in that message. Try again with days and times, e.g. \"Mon-Wed mornings, Thu off\".", nil)
	}

	slots := make([]store.AvailabilitySlot, len(parsed))
	for i, s := range parsed {
		slots[i] = store.AvailabilitySlot{
			Start:      s.Start,
			End:        s.End,
			Preference: s.Preference,
			Note:       s.Note,
		}
	}
	if err := validateAvailabilitySlots(slots, weekStart); err != nil {
		b.log.Warn("availability slots invalid", "err", err, "employee", emp.ID)
		return b.api.SendMessage(ctx, m.Chat.ID, "I couldn't turn that into valid availability for next week. Please rephrase with clear days and times.", nil)
	}

	draft := AvailabilityDraft{
		TelegramUserID: m.From.ID,
		ChatID:         m.Chat.ID,
		ShopID:         emp.ShopID,
		EmployeeID:     emp.ID,
		WeekStart:      weekStart,
		Timezone:       shop.Timezone,
		Slots:          slots,
		RawMessage:     text,
	}
	draftID, err := b.drafts.Create(ctx, draft)
	if err != nil {
		return fmt.Errorf("telegram: create availability draft: %w", err)
	}

	summary := formatAvailabilityDraft(draft, loc)
	return b.api.SendMessage(ctx, m.Chat.ID, summary, AvailabilityConfirmKeyboard(draftID))
}

func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) error {
	if isGroupChat(callbackChat(q)) {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "")
	}
	switch {
	case strings.HasPrefix(q.Data, availConfirmPrefix):
		return b.handleAvailabilityConfirm(ctx, q)
	case strings.HasPrefix(q.Data, availCancelPrefix):
		return b.handleAvailabilityCancel(ctx, q)
	case strings.HasPrefix(q.Data, votePrefix):
		return b.handleVote(ctx, q)
	default:
		return b.api.AnswerCallbackQuery(ctx, q.ID, "")
	}
}

func (b *Bot) handleAvailabilityConfirm(ctx context.Context, q *CallbackQuery) error {
	draft, ok, err := b.loadOwnedDraft(ctx, q, availConfirmPrefix)
	if err != nil {
		return err
	}
	if !ok {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "This confirmation expired. Please resend your availability.")
	}

	if err := b.availability.ReplaceWeek(ctx, draft.ShopID, draft.EmployeeID, draft.WeekStart, draft.Slots, draft.RawMessage); err != nil {
		return fmt.Errorf("telegram: save availability: %w", err)
	}
	_ = b.drafts.Delete(ctx, draft.ID)

	if err := b.api.AnswerCallbackQuery(ctx, q.ID, "Availability saved."); err != nil {
		return err
	}
	chatID := callbackChatID(q, draft.ChatID)
	return b.api.SendMessage(ctx, chatID,
		fmt.Sprintf("Saved your availability for the week of %s.", draft.WeekStart.Format("2006-01-02")), nil)
}

func (b *Bot) handleAvailabilityCancel(ctx context.Context, q *CallbackQuery) error {
	draft, ok, err := b.loadOwnedDraft(ctx, q, availCancelPrefix)
	if err != nil {
		return err
	}
	if !ok {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "This confirmation expired. Please resend your availability.")
	}

	_ = b.drafts.Delete(ctx, draft.ID)
	if err := b.api.AnswerCallbackQuery(ctx, q.ID, "Discarded."); err != nil {
		return err
	}
	chatID := callbackChatID(q, draft.ChatID)
	return b.api.SendMessage(ctx, chatID, "Discarded. Send your availability again whenever you're ready.", nil)
}

func (b *Bot) loadOwnedDraft(ctx context.Context, q *CallbackQuery, prefix string) (*AvailabilityDraft, bool, error) {
	draftID, err := uuid.Parse(strings.TrimPrefix(q.Data, prefix))
	if err != nil {
		return nil, false, b.api.AnswerCallbackQuery(ctx, q.ID, "Invalid confirmation.")
	}
	draft, ok, err := b.drafts.Get(ctx, draftID)
	if err != nil {
		return nil, false, fmt.Errorf("telegram: load draft: %w", err)
	}
	if !ok {
		return nil, false, nil
	}
	if q.From.ID != draft.TelegramUserID {
		return nil, false, b.api.AnswerCallbackQuery(ctx, q.ID, "This confirmation is not yours.")
	}
	return draft, true, nil
}

func (b *Bot) handleVote(ctx context.Context, q *CallbackQuery) error {
	scheduleID, err := uuid.Parse(strings.TrimPrefix(q.Data, votePrefix))
	if err != nil {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "Invalid vote.")
	}
	emp, err := b.employees.ByTelegramID(ctx, q.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "Join the shop first with /start <invite-code>.")
	}
	if err != nil {
		return fmt.Errorf("telegram: lookup voter: %w", err)
	}
	if err := b.votes.Record(ctx, emp.ShopID, scheduleID, emp.ID); err != nil {
		return fmt.Errorf("telegram: record vote: %w", err)
	}
	return b.api.AnswerCallbackQuery(ctx, q.ID, "Vote counted!")
}

// AvailabilityConfirmKeyboard builds Confirm/Cancel buttons for a draft.
func AvailabilityConfirmKeyboard(draftID uuid.UUID) *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		{Text: "Confirm", Data: availConfirmPrefix + draftID.String()},
		{Text: "Cancel", Data: availCancelPrefix + draftID.String()},
	}}}
}

// VoteKeyboard builds the inline keyboard for choosing between schedule
// variants.
func VoteKeyboard(labels []string, scheduleIDs []uuid.UUID) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(labels))
	for i, label := range labels {
		rows = append(rows, []InlineKeyboardButton{{
			Text: label,
			Data: fmt.Sprintf("%s%s", votePrefix, scheduleIDs[i]),
		}})
	}
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatAvailabilityDraft(d AvailabilityDraft, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	var b strings.Builder
	fmt.Fprintf(&b, "I understood this for the week of %s:\n\n", d.WeekStart.In(loc).Format("2006-01-02"))
	for _, slot := range d.Slots {
		start := slot.Start.In(loc)
		end := slot.End.In(loc)
		fmt.Fprintf(&b, "%s %s-%s %s\n",
			start.Format("Mon"),
			start.Format("15:04"),
			end.Format("15:04"),
			preferenceLabel(slot.Preference),
		)
	}
	b.WriteString("\nConfirm?")
	return b.String()
}

func preferenceLabel(pref int) string {
	switch pref {
	case 0:
		return "unavailable"
	case 2:
		return "preferred"
	default:
		return "available"
	}
}

func formatClarificationMessage(questions []string) string {
	var b strings.Builder
	b.WriteString("I need one clarification before saving:\n")
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", q)
	}
	return strings.TrimSpace(b.String())
}

func validateAvailabilitySlots(slots []store.AvailabilitySlot, weekStart time.Time) error {
	weekEnd := weekStart.AddDate(0, 0, 7)
	for i, slot := range slots {
		if !slot.End.After(slot.Start) {
			return fmt.Errorf("slot %d: end before start", i)
		}
		if slot.Preference < 0 || slot.Preference > 2 {
			return fmt.Errorf("slot %d: preference out of range", i)
		}
		if !slotIntersectsWeek(slot.Start, slot.End, weekStart, weekEnd) {
			return fmt.Errorf("slot %d: outside target week", i)
		}
	}
	return nil
}

func slotIntersectsWeek(start, end, weekStart, weekEnd time.Time) bool {
	return start.Before(weekEnd) && end.After(weekStart)
}

func callbackChatID(q *CallbackQuery, fallback int64) int64 {
	if q.Message != nil {
		return q.Message.Chat.ID
	}
	return fallback
}

// nextMonday returns the upcoming Monday at midnight (t's location); if t is
// already Monday, the following one.
func nextMonday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	days := (int(time.Monday) - int(t.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return t.AddDate(0, 0, days)
}
