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

// AvailabilityParser turns free text into structured slots. Satisfied by
// *llm.Service.
type AvailabilityParser interface {
	ParseAvailability(ctx context.Context, text string, weekStart time.Time, loc *time.Location) ([]llm.AvailabilitySlot, error)
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
	api          *Client
	parser       AvailabilityParser
	employees    EmployeeDirectory
	availability AvailabilityStore
	votes        VoteStore
	log          *slog.Logger
}

// NewBot wires the bot's dependencies.
func NewBot(api *Client, parser AvailabilityParser, employees EmployeeDirectory, availability AvailabilityStore, votes VoteStore, log *slog.Logger) *Bot {
	if log == nil {
		log = slog.Default()
	}
	return &Bot{api: api, parser: parser, employees: employees, availability: availability, votes: votes, log: log}
}

// HandleUpdate dispatches one incoming update.
func (b *Bot) HandleUpdate(ctx context.Context, u Update) error {
	switch {
	case u.Message != nil:
		return b.handleMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		return b.handleCallback(ctx, u.CallbackQuery)
	default:
		return nil // update type we don't handle yet
	}
}

func (b *Bot) handleMessage(ctx context.Context, m *Message) error {
	if m.From == nil {
		return nil
	}
	text := strings.TrimSpace(m.Text)
	switch {
	case strings.HasPrefix(text, "/start"):
		return b.handleStart(ctx, m, strings.TrimSpace(strings.TrimPrefix(text, "/start")))
	case text != "":
		return b.handleAvailabilityText(ctx, m, text)
	default:
		return nil
	}
}

// handleStart enrolls an employee via "/start <invite-code>".
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
	if err != nil {
		return fmt.Errorf("telegram: join shop: %w", err)
	}
	return b.api.SendMessage(ctx, m.Chat.ID,
		fmt.Sprintf("Welcome, %s! You're on the roster. Send me your availability for next week whenever you're ready — plain language is fine.", emp.DisplayName), nil)
}

// handleAvailabilityText is the example end-to-end handler: identify the
// employee, parse the free text into slots with the LLM, persist them and
// confirm.
func (b *Bot) handleAvailabilityText(ctx context.Context, m *Message, text string) error {
	emp, err := b.employees.ByTelegramID(ctx, m.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		return b.api.SendMessage(ctx, m.Chat.ID, "I don't know you yet — join your shop first with /start <invite-code>.", nil)
	}
	if err != nil {
		return fmt.Errorf("telegram: lookup employee: %w", err)
	}

	weekStart := nextMonday(time.Now().UTC())
	// TODO: interpret times in the shop's timezone (shops.timezone) instead of UTC.
	slots, err := b.parser.ParseAvailability(ctx, text, weekStart, time.UTC)
	if errors.Is(err, llm.ErrNoProvider) {
		return b.api.SendMessage(ctx, m.Chat.ID, "Availability parsing isn't configured yet — ask your admin to set up the LLM provider.", nil)
	}
	if err != nil {
		b.log.Warn("availability parse failed", "err", err, "employee", emp.ID)
		return b.api.SendMessage(ctx, m.Chat.ID, "Sorry, I couldn't understand that. Try something like: \"I can work Mon-Fri mornings, not Wednesday, prefer Friday evening.\"", nil)
	}

	stored := make([]store.AvailabilitySlot, len(slots))
	for i, s := range slots {
		stored[i] = store.AvailabilitySlot{Start: s.Start, End: s.End, Preference: s.Preference, Note: s.Note}
	}
	if err := b.availability.ReplaceWeek(ctx, emp.ShopID, emp.ID, weekStart, stored, text); err != nil {
		return fmt.Errorf("telegram: save availability: %w", err)
	}
	return b.api.SendMessage(ctx, m.Chat.ID,
		fmt.Sprintf("Got it — recorded %d availability slot(s) for the week of %s. You can resend anytime to overwrite.", len(stored), weekStart.Format("Jan 2")), nil)
}

// handleCallback processes inline-button presses. Voting buttons carry data
// of the form "vote:<schedule-uuid>".
func (b *Bot) handleCallback(ctx context.Context, q *CallbackQuery) error {
	const votePrefix = "vote:"
	if !strings.HasPrefix(q.Data, votePrefix) {
		return b.api.AnswerCallbackQuery(ctx, q.ID, "")
	}
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

// VoteKeyboard builds the inline keyboard for choosing between schedule
// variants.
func VoteKeyboard(labels []string, scheduleIDs []uuid.UUID) *InlineKeyboardMarkup {
	rows := make([][]InlineKeyboardButton, 0, len(labels))
	for i, label := range labels {
		rows = append(rows, []InlineKeyboardButton{{
			Text: label,
			Data: fmt.Sprintf("vote:%s", scheduleIDs[i]),
		}})
	}
	return &InlineKeyboardMarkup{InlineKeyboard: rows}
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
