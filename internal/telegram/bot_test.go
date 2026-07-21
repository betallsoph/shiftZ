package telegram

import (
	"context"
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
		Chat: Chat{ID: 100, Type: "private"},
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

	bot := NewBot(msgAPI, parser, shops, employees, availability, &fakeVotes{}, drafts, nil, nil, testLogger())
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
	if len(buttons) != 2 || buttons[0].Text != btnConfirm || buttons[1].Text != btnCancel {
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
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, availability, &fakeVotes{}, drafts, nil, nil, testLogger())

	q := &CallbackQuery{
		ID:      "cb1",
		From:    User{ID: 42},
		Data:    availConfirmPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: 100, Type: "private"}},
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
	if msgAPI.answers[0] != msgAvailabilitySaved {
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
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, availability, &fakeVotes{}, drafts, nil, nil, testLogger())

	q := &CallbackQuery{
		ID:      "cb2",
		From:    User{ID: 42},
		Data:    availCancelPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: 100, Type: "private"}},
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
	if msgAPI.answers[0] != msgDraftDiscarded {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
}

func TestHandleAvailabilityConfirmExpiredDraft(t *testing.T) {
	msgAPI := &fakeMessenger{}
	drafts := NewMemoryAvailabilityDraftStore(time.Millisecond)
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, drafts, nil, nil, testLogger())

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
	if !strings.Contains(msgAPI.answers[0], "hết hạn") {
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
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, drafts, nil, nil, testLogger())

	q := &CallbackQuery{ID: "cb4", From: User{ID: 99}, Data: availConfirmPrefix + draftID.String()}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if msgAPI.answers[0] != msgConfirmNotYours {
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
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, &fakeAvailability{}, votes, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	q := &CallbackQuery{ID: "vote1", From: User{ID: 42}, Data: votePrefix + scheduleID.String(), Message: &Message{Chat: Chat{ID: 100, Type: "private"}}}
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

func TestHandleAvailabilityTextClarificationNoDraft(t *testing.T) {
	shopID := uuid.New()
	msgAPI := &fakeMessenger{}
	parser := &fakeParser{err: &llm.ClarificationError{Questions: []string{"Did you mean next Monday morning or evening?"}}}
	shops := &fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}}
	employees := &fakeEmployees{emp: &store.Employee{ID: uuid.New(), ShopID: shopID}}
	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)
	bot := NewBot(msgAPI, parser, shops, employees, &fakeAvailability{}, &fakeVotes{}, drafts, nil, nil, testLogger())

	msg := &Message{From: &User{ID: 42}, Chat: Chat{ID: 1, Type: "private"}, Text: "maybe Monday"}
	if err := bot.handleAvailabilityText(context.Background(), msg, msg.Text); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 1 {
		t.Fatalf("messages = %d", len(msgAPI.messages))
	}
	if !strings.Contains(msgAPI.messages[0].text, "hỏi thêm") {
		t.Fatalf("message = %q", msgAPI.messages[0].text)
	}
	if msgAPI.messages[0].markup != nil {
		t.Fatal("should not send confirm keyboard")
	}
}

func TestHandleAvailabilityTextNoProvider(t *testing.T) {
	shopID := uuid.New()
	msgAPI := &fakeMessenger{}
	parser := &fakeParser{err: llm.ErrNoProvider}
	shops := &fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}}
	employees := &fakeEmployees{emp: &store.Employee{ID: uuid.New(), ShopID: shopID}}
	bot := NewBot(msgAPI, parser, shops, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	msg := &Message{From: &User{ID: 42}, Chat: Chat{ID: 1, Type: "private"}, Text: "Mon mornings"}
	if err := bot.handleAvailabilityText(context.Background(), msg, msg.Text); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msgAPI.messages[0].text, "Chưa cấu hình") {
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
	shop              *store.Shop
	byOwner           map[int64]*store.Shop
	consumeShop       *store.Shop
	consumeErr        error
	setOwnerErr       error
	setOwnerCalls     int
	lastOwnerID       int64
	boundGroupID      int64
	boundTeamChatID   int64
	bindGroupCalls    int
	bindTeamCalls     int
}

func (f *fakeShops) ByID(_ context.Context, id uuid.UUID) (*store.Shop, error) {
	if f.shop != nil && f.shop.ID == id {
		return f.shop, nil
	}
	if f.byOwner != nil {
		for _, s := range f.byOwner {
			if s.ID == id {
				return s, nil
			}
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeShops) ByOwnerTelegramID(_ context.Context, telegramUserID int64) (*store.Shop, error) {
	if f.byOwner != nil {
		if s, ok := f.byOwner[telegramUserID]; ok {
			return s, nil
		}
	}
	if f.shop != nil && f.shop.OwnerTelegramID == telegramUserID && telegramUserID != 0 {
		return f.shop, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeShops) ConsumeOwnerLinkToken(_ context.Context, _ string) (*store.Shop, error) {
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	if f.consumeShop != nil {
		return f.consumeShop, nil
	}
	return nil, store.ErrInvalidCredentials
}

func (f *fakeShops) SetOwnerTelegramID(_ context.Context, shopID uuid.UUID, telegramUserID int64) error {
	f.setOwnerCalls++
	f.lastOwnerID = telegramUserID
	if f.setOwnerErr != nil {
		return f.setOwnerErr
	}
	if f.consumeShop != nil && f.consumeShop.ID == shopID {
		f.consumeShop.OwnerTelegramID = telegramUserID
	}
	if f.shop != nil && f.shop.ID == shopID {
		f.shop.OwnerTelegramID = telegramUserID
	}
	return nil
}

func (f *fakeShops) BindTelegramGroup(_ context.Context, _ uuid.UUID, chatID int64) error {
	f.bindGroupCalls++
	f.boundGroupID = chatID
	return nil
}

func (f *fakeShops) BindTelegramTeamChat(_ context.Context, _ uuid.UUID, chatID int64) error {
	f.bindTeamCalls++
	f.boundTeamChatID = chatID
	return nil
}

type fakeEmployees struct {
	emp      *store.Employee
	emps     []*store.Employee
	joinErr  error
	joinEmp  *store.Employee
}

func (f *fakeEmployees) ByTelegramID(_ context.Context, telegramUserID int64) (*store.Employee, error) {
	if f.emp != nil {
		return f.emp, nil
	}
	return nil, store.ErrNotFound
}

func (f *fakeEmployees) Join(_ context.Context, _ string, _ int64, displayName string) (*store.Employee, error) {
	if f.joinErr != nil {
		return nil, f.joinErr
	}
	if f.joinEmp != nil {
		return f.joinEmp, nil
	}
	if f.emp != nil {
		emp := *f.emp
		emp.DisplayName = displayName
		return &emp, nil
	}
	return &store.Employee{ID: uuid.New(), DisplayName: displayName, IsActive: true}, nil
}

func (f *fakeEmployees) ListActiveByShop(_ context.Context, shopID uuid.UUID) ([]*store.Employee, error) {
	if len(f.emps) > 0 {
		return f.emps, nil
	}
	if f.emp != nil && f.emp.ShopID == shopID {
		return []*store.Employee{f.emp}, nil
	}
	return nil, nil
}

type fakeRules struct {
	createCalls int
	lastShopID  uuid.UUID
	lastDesc    string
	lastJSON    map[string]any
	lastWeight  float64
}

func (f *fakeRules) Create(_ context.Context, shopID uuid.UUID, description string, ruleJSON map[string]any, weight float64) (*store.Rule, error) {
	f.createCalls++
	f.lastShopID = shopID
	f.lastDesc = description
	f.lastJSON = ruleJSON
	f.lastWeight = weight
	return &store.Rule{ID: uuid.New(), ShopID: shopID, Description: description, RuleJSON: ruleJSON, Weight: weight, IsActive: true}, nil
}

type fakeAvailability struct {
	replaceCalls   int
	lastShopID     uuid.UUID
	lastEmployeeID uuid.UUID
	lastWeekStart  time.Time
	lastSlots      []store.AvailabilitySlot
	lastRawMessage string
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

func TestHandleAvailabilityTextInGroupIgnored(t *testing.T) {
	shopID := uuid.New()
	msgAPI := &fakeMessenger{}
	parser := &fakeParser{slots: []llm.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}}}
	shops := &fakeShops{shop: &store.Shop{ID: shopID, Timezone: "UTC"}}
	employees := &fakeEmployees{emp: &store.Employee{ID: uuid.New(), ShopID: shopID}}
	bot := NewBot(msgAPI, parser, shops, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	msg := &Message{From: &User{ID: 42}, Chat: Chat{ID: -1001, Type: "group"}, Text: "Mon mornings"}
	if err := bot.handleMessage(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 0 {
		t.Fatalf("expected no reply in group, got %d messages", len(msgAPI.messages))
	}
	if parser.lastLoc != nil {
		t.Fatal("parser should not run for group availability text")
	}
}

func TestHandleStartInGroupIgnored(t *testing.T) {
	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	msg := &Message{From: &User{ID: 42}, Chat: Chat{ID: -1001, Type: "supergroup"}, Text: "/start abc123"}
	if err := bot.handleMessage(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 0 {
		t.Fatalf("expected no reply for /start in group, got %v", msgAPI.messages)
	}
}

func TestHandleAvailabilityConfirmInGroupIgnored(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	drafts := NewMemoryAvailabilityDraftStore(30 * time.Minute)
	draftID, _ := drafts.Create(context.Background(), AvailabilityDraft{
		TelegramUserID: 42,
		ShopID:         shopID,
		EmployeeID:     empID,
		Slots:          []store.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})

	msgAPI := &fakeMessenger{}
	availability := &fakeAvailability{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, availability, &fakeVotes{}, drafts, nil, nil, testLogger())

	q := &CallbackQuery{
		ID:      "cb-group",
		From:    User{ID: 42},
		Data:    availConfirmPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: -1001, Type: "group"}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if availability.replaceCalls != 0 {
		t.Fatal("availability should not save from group callback")
	}
}

func TestHandleVoteInGroupIgnored(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	scheduleID := uuid.New()

	msgAPI := &fakeMessenger{}
	votes := &fakeVotes{}
	employees := &fakeEmployees{emp: &store.Employee{ID: empID, ShopID: shopID}}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, &fakeAvailability{}, votes, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	q := &CallbackQuery{
		ID:      "vote-group",
		From:    User{ID: 42},
		Data:    votePrefix + scheduleID.String(),
		Message: &Message{Chat: Chat{ID: -1001, Type: "supergroup"}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if votes.recordCalls != 0 {
		t.Fatal("vote should not record from group callback yet")
	}
}

func TestHandleStartInactiveEmployee(t *testing.T) {
	msgAPI := &fakeMessenger{}
	employees := &fakeEmployees{joinErr: store.ErrEmployeeInactive}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	msg := &Message{
		From: &User{ID: 42, FirstName: "Anna"},
		Chat: Chat{ID: 100, Type: "private"},
		Text: "/start invite123",
	}
	if err := bot.handleStart(context.Background(), msg, "invite123"); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 1 {
		t.Fatalf("messages = %d", len(msgAPI.messages))
	}
	body := msgAPI.messages[0].text
	if !strings.Contains(body, "đang bị tạm ngưng") || !strings.Contains(body, "liên hệ chủ quán") {
		t.Fatalf("message = %q", body)
	}
}

func TestHandleStartJoinSuccess(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	msgAPI := &fakeMessenger{}
	employees := &fakeEmployees{joinEmp: &store.Employee{ID: empID, ShopID: shopID, DisplayName: "Anna", IsActive: true}}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), nil, nil, testLogger())

	msg := &Message{
		From: &User{ID: 42, FirstName: "Anna"},
		Chat: Chat{ID: 100, Type: "private"},
		Text: "/start invite123",
	}
	if err := bot.handleStart(context.Background(), msg, "invite123"); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 1 {
		t.Fatalf("messages = %d", len(msgAPI.messages))
	}
	if !strings.Contains(msgAPI.messages[0].text, "Chào Anna") || !strings.Contains(msgAPI.messages[0].text, "nha") {
		t.Fatalf("message = %q", msgAPI.messages[0].text)
	}
}
