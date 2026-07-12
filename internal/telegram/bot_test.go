package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/llm"
	"github.com/betallsoph/shiftz/internal/store"
)

func TestHandleAvailabilityTextCreatesDraft(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	weekStart := nextMonday(time.Now().In(loc))

	msg := &Message{
		From: &User{ID: 42, FirstName: "Anna"},
		Chat: Chat{ID: 100},
		Text: "Mon mornings",
	}

	slots := []llm.AvailabilitySlot{{
		Start:      weekStart.Add(8 * time.Hour),
		End:        weekStart.Add(14 * time.Hour),
		Preference: 1,
	}}

	msgAPI := &fakeMessenger{}
	parser := &fakeParser{slots: slots}
	shops := &fakeShops{shop: &store.Shop{ID: shopID, Timezone: "Asia/Ho_Chi_Minh"}}
	employees := &fakeEmployees{emp: &store.Employee{ID: empID, ShopID: shopID}}
	availability := &fakeAvailability{}
	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)

	bot := NewBot(msgAPI, parser, shops, employees, availability, &fakeVotes{}, drafts, testLogger())
	if err := bot.handleAvailabilityText(context.Background(), msg, msg.Text); err != nil {
		t.Fatal(err)
	}
	if availability.replaceCalls != 0 {
		t.Fatalf("ReplaceWeek calls = %d, want 0", availability.replaceCalls)
	}
	if len(msgAPI.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgAPI.messages))
	}
	if msgAPI.messages[0].markup == nil || len(msgAPI.messages[0].markup.InlineKeyboard) == 0 {
		t.Fatal("expected confirm keyboard")
	}
	buttons := msgAPI.messages[0].markup.InlineKeyboard[0]
	if len(buttons) != 2 || buttons[0].Text != "Confirm" || buttons[1].Text != "Cancel" {
		t.Fatalf("buttons = %+v", buttons)
	}
	if !strings.HasPrefix(buttons[0].Data, availConfirmPrefix) {
		t.Fatalf("confirm data = %q", buttons[0].Data)
	}
	if parser.lastLoc == nil || parser.lastLoc.String() != "Asia/Ho_Chi_Minh" {
		t.Fatalf("parser loc = %v, want Asia/Ho_Chi_Minh", parser.lastLoc)
	}
}

func TestHandleAvailabilityConfirmSavesAndDeletesDraft(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	loc := time.FixedZone("ICT", 7*3600)
	weekStart := nextMonday(time.Now().In(loc))
	slot := store.AvailabilitySlot{
		Start:      weekStart.Add(8 * time.Hour),
		End:        weekStart.Add(14 * time.Hour),
		Preference: 1,
	}

	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)
	draftID, err := drafts.Create(context.Background(), AvailabilityDraft{
		TelegramUserID: 42,
		ChatID:         100,
		ShopID:         shopID,
		EmployeeID:     empID,
		WeekStart:      weekStart,
		Timezone:       "Asia/Ho_Chi_Minh",
		Slots:          []store.AvailabilitySlot{slot},
		RawMessage:     "Mon mornings",
	})
	if err != nil {
		t.Fatal(err)
	}

	msgAPI := &fakeMessenger{}
	availability := &fakeAvailability{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, availability, &fakeVotes{}, drafts, testLogger())

	q := &CallbackQuery{
		ID:   "cb1",
		From: User{ID: 42},
		Data: availConfirmPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: 100}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if availability.replaceCalls != 1 {
		t.Fatalf("ReplaceWeek calls = %d, want 1", availability.replaceCalls)
	}
	if availability.lastShopID != shopID || availability.lastEmployeeID != empID {
		t.Fatalf("saved for wrong employee/shop")
	}
	if _, ok, _ := drafts.Get(context.Background(), draftID); ok {
		t.Fatal("draft should be deleted after confirm")
	}
	if msgAPI.answers[0] != "Availability saved." {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleAvailabilityCancelDoesNotSave(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	weekStart := nextMonday(time.Now().UTC())

	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)
	draftID, err := drafts.Create(context.Background(), AvailabilityDraft{
		TelegramUserID: 42,
		ChatID:         100,
		ShopID:         shopID,
		EmployeeID:     empID,
		WeekStart:      weekStart,
		Slots:          []store.AvailabilitySlot{{Start: weekStart, End: weekStart.Add(time.Hour), Preference: 1}},
		RawMessage:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	msgAPI := &fakeMessenger{}
	availability := &fakeAvailability{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, availability, &fakeVotes{}, drafts, testLogger())

	q := &CallbackQuery{
		ID:   "cb2",
		From: User{ID: 42},
		Data: availCancelPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: 100}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if availability.replaceCalls != 0 {
		t.Fatalf("ReplaceWeek calls = %d, want 0", availability.replaceCalls)
	}
	if _, ok, _ := drafts.Get(context.Background(), draftID); ok {
		t.Fatal("draft should be deleted after cancel")
	}
	if msgAPI.answers[0] != "Discarded." {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleAvailabilityConfirmExpiredDraft(t *testing.T) {
	msgAPI := &fakeMessenger{}
	drafts := NewMemoryAvailabilityDraftStore(time.Millisecond)
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, drafts, testLogger())

	draftID := uuid.New()
	_, _ = drafts.Create(context.Background(), AvailabilityDraft{
		ID:             draftID,
		TelegramUserID: 42,
		Slots:          []store.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})
	time.Sleep(5 * time.Millisecond)

	q := &CallbackQuery{ID: "cb3", From: User{ID: 42}, Data: availConfirmPrefix + draftID.String()}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgAPI.answers[0], "expired") {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleAvailabilityConfirmWrongUser(t *testing.T) {
	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)
	draftID, _ := drafts.Create(context.Background(), AvailabilityDraft{
		TelegramUserID: 42,
		Slots:          []store.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})

	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, drafts, testLogger())

	q := &CallbackQuery{ID: "cb4", From: User{ID: 99}, Data: availConfirmPrefix + draftID.String()}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if msgAPI.answers[0] != "This confirmation is not yours." {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleVoteCallbackStillWorks(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	scheduleID := uuid.New()

	msgAPI := &fakeMessenger{}
	votes := &fakeVotes{}
	employees := &fakeEmployees{emp: &store.Employee{ID: empID, ShopID: shopID}}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, &fakeAvailability{}, votes, NewMemoryAvailabilityDraftStore(0), testLogger())

	q := &CallbackQuery{ID: "vote1", From: User{ID: 42}, Data: votePrefix + scheduleID.String()}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if votes.recordCalls != 1 {
		t.Fatalf("recordCalls = %d", votes.recordCalls)
	}
	if msgAPI.answers[0] != "Vote counted!" {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleAvailabilityTextNoProvider(t *testing.T) {
	shopID := uuid.New()
	msgAPI := &fakeMessenger{}
	parser := &fakeParser{err: llm.ErrNoProvider}
	shops := &fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}}
	employees := &fakeEmployees{emp: &store.Employee{ID: uuid.New(), ShopID: shopID}}
	bot := NewBot(msgAPI, parser, shops, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), testLogger())

	msg := &Message{From: &User{ID: 42}, Chat: Chat{ID: 1}, Text: "Mon mornings"}
	if err := bot.handleAvailabilityText(context.Background(), msg, msg.Text); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgAPI.messages[0].text, "isn't configured") {
		t.Fatalf("message = %q", msgAPI.messages[0].text)
	}
}

func TestValidateAvailabilitySlotsRejectsOutsideWeek(t *testing.T) {
	weekStart := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	slots := []store.AvailabilitySlot{{
		Start:      weekStart.AddDate(0, 0, -1),
		End:        weekStart.AddDate(0, 0, -1).Add(2 * time.Hour),
		Preference: 1,
	}}
	if err := validateAvailabilitySlots(slots, weekStart); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMemoryDraftStoreExpires(t *testing.T) {
	draftStore := NewMemoryAvailabilityDraftStore(time.Millisecond)
	id, err := draftStore.Create(context.Background(), AvailabilityDraft{
		Slots: []store.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok, err := draftStore.Get(context.Background(), id); err != nil || ok {
		t.Fatalf("expected expired draft, ok=%v err=%v", ok, err)
	}
}

type sentMessage struct {
	chatID int64
	text   string
	markup *InlineKeyboardMarkup
}

type fakeMessenger struct {
	messages []sentMessage
	answers  []string
}

func (f *fakeMessenger) SendMessage(_ context.Context, chatID int64, text string, markup *InlineKeyboardMarkup) error {
	f.messages = append(f.messages, sentMessage{chatID: chatID, text: text, markup: markup})
	return nil
}

func (f *fakeMessenger) AnswerCallbackQuery(_ context.Context, callbackID, text string) error {
	f.answers = append(f.answers, text)
	return nil
}

type fakeParser struct {
	slots         []llm.AvailabilitySlot
	err           error
	lastWeekStart time.Time
	lastLoc       *time.Location
}

func (f *fakeParser) ParseAvailability(_ context.Context, _ string, weekStart time.Time, loc *time.Location) ([]llm.AvailabilitySlot, error) {
	f.lastWeekStart = weekStart
	f.lastLoc = loc
	if f.err != nil {
		return nil, f.err
	}
	return f.slots, nil
}

type fakeShops struct {
	shop *store.Shop
}

func (f *fakeShops) ByID(_ context.Context, id uuid.UUID) (*store.Shop, error) {
	if f.shop != nil && f.shop.ID == id {
		return f.shop, nil
	}
	return nil, store.ErrNotFound
}

type fakeEmployees struct {
	emp *store.Employee
}

func (f *fakeEmployees) ByTelegramID(_ context.Context, telegramUserID int64) (*store.Employee, error) {
	if f.emp != nil {
		return f.emp, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeEmployees) Join(_ context.Context, _ string, _ int64, _ string) (*store.Employee, error) {
	return nil, errors.New("not implemented")
}

type fakeAvailability struct {
	replaceCalls     int
	lastShopID       uuid.UUID
	lastEmployeeID   uuid.UUID
	lastWeekStart    time.Time
	lastSlots        []store.AvailabilitySlot
	lastRawMessage   string
}

func (f *fakeAvailability) ReplaceWeek(_ context.Context, shopID, employeeID uuid.UUID, weekStart time.Time, slots []store.AvailabilitySlot, rawMessage string) error {
	f.replaceCalls++
	f.lastShopID = shopID
	f.lastEmployeeID = employeeID
	f.lastWeekStart = weekStart
	f.lastSlots = slots
	f.lastRawMessage = rawMessage
	return nil
}

type fakeVotes struct {
	recordCalls int
}

func (f *fakeVotes) Record(_ context.Context, _, _, _ uuid.UUID) error {
	f.recordCalls++
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
