package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestHandleOwnerStartLinksTelegram(t *testing.T) {
	shopID := uuid.New()
	shops := &fakeShops{
		consumeShop: &store.Shop{ID: shopID, Name: "Cafe", Timezone: "Asia/Ho_Chi_Minh"},
	}
	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, shops, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), &fakeRules{}, NewMemoryOwnerDraftStore(0), testLogger())

	msg := &Message{
		From: &User{ID: 9001, FirstName: "Owner"},
		Chat: Chat{ID: 55, Type: "private"},
		Text: "/start owner_sz_ownerlink_abc",
	}
	if err := bot.handleMessage(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if shops.setOwnerCalls != 1 || shops.lastOwnerID != 9001 {
		t.Fatalf("setOwner calls=%d id=%d", shops.setOwnerCalls, shops.lastOwnerID)
	}
	if len(msgAPI.messages) != 1 || !strings.Contains(msgAPI.messages[0].text, "liên kết tài khoản chủ quán") {
		t.Fatalf("message = %#v", msgAPI.messages)
	}
	if !strings.Contains(msgAPI.messages[0].text, "Thêm bot vào nhóm") {
		t.Fatalf("expected next-step guidance, got %q", msgAPI.messages[0].text)
	}
}

func TestHandleMyChatMemberAsksBindAndCallbackBindsBroadcast(t *testing.T) {
	shopID := uuid.New()
	ownerID := int64(9001)
	groupID := int64(-100555)
	shops := &fakeShops{
		byOwner: map[int64]*store.Shop{
			ownerID: {ID: shopID, OwnerTelegramID: ownerID, Timezone: "UTC"},
		},
	}
	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, shops, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), &fakeRules{}, NewMemoryOwnerDraftStore(0), testLogger())

	if err := bot.HandleUpdate(context.Background(), Update{
		MyChatMember: &ChatMemberUpdated{
			Chat:          Chat{ID: groupID, Type: "supergroup"},
			From:          User{ID: ownerID},
			OldChatMember: ChatMember{Status: "left"},
			NewChatMember: ChatMember{Status: "administrator"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if len(msgAPI.messages) != 1 || msgAPI.messages[0].markup == nil {
		t.Fatalf("expected bind keyboard, got %#v", msgAPI.messages)
	}
	rows := msgAPI.messages[0].markup.InlineKeyboard
	if len(rows) < 1 || len(rows[0]) < 2 {
		t.Fatalf("buttons = %#v", rows)
	}
	if rows[0][0].Text != "Thông báo" || !strings.HasPrefix(rows[0][0].Data, bindBroadcastPrefix) {
		t.Fatalf("broadcast button = %+v", rows[0][0])
	}

	q := &CallbackQuery{
		ID:      "bind1",
		From:    User{ID: ownerID},
		Data:    rows[0][0].Data,
		Message: &Message{Chat: Chat{ID: groupID, Type: "supergroup"}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if shops.bindGroupCalls != 1 || shops.boundGroupID != groupID {
		t.Fatalf("bind group calls=%d id=%d", shops.bindGroupCalls, shops.boundGroupID)
	}
	if !strings.Contains(msgAPI.messages[len(msgAPI.messages)-1].text, "Thông báo") {
		t.Fatalf("follow-up = %q", msgAPI.messages[len(msgAPI.messages)-1].text)
	}
}

func TestHandleOwnerLeaveConfirmCreatesRule(t *testing.T) {
	shopID := uuid.New()
	empID := uuid.New()
	ownerID := int64(9001)
	broadcastID := int64(-100777)
	shop := &store.Shop{
		ID:              shopID,
		OwnerTelegramID: ownerID,
		Timezone:        "Asia/Ho_Chi_Minh",
		TelegramGroupID: broadcastID,
	}
	shops := &fakeShops{
		shop:    shop,
		byOwner: map[int64]*store.Shop{ownerID: shop},
	}
	employees := &fakeEmployees{
		emps: []*store.Employee{{ID: empID, ShopID: shopID, DisplayName: "Lan", IsActive: true}},
	}
	rules := &fakeRules{}
	ownerDrafts := NewMemoryOwnerDraftStore(30 * time.Minute)
	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, shops, employees, &fakeAvailability{}, &fakeVotes{}, NewMemoryAvailabilityDraftStore(0), rules, ownerDrafts, testLogger())

	msg := &Message{
		From: &User{ID: ownerID},
		Chat: Chat{ID: 12, Type: "private"},
		Text: "Lan nghỉ chiều mai",
	}
	if err := bot.handleMessage(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if rules.createCalls != 0 {
		t.Fatal("rule should not persist before confirm")
	}
	if len(msgAPI.messages) != 1 || msgAPI.messages[0].markup == nil {
		t.Fatalf("expected confirm keyboard, got %#v", msgAPI.messages)
	}
	confirmData := msgAPI.messages[0].markup.InlineKeyboard[0][0].Data
	if !strings.HasPrefix(confirmData, ownerConfirmPrefix) {
		t.Fatalf("confirm data = %q", confirmData)
	}

	q := &CallbackQuery{
		ID:      "own1",
		From:    User{ID: ownerID},
		Data:    confirmData,
		Message: &Message{Chat: Chat{ID: 12, Type: "private"}},
	}
	if err := bot.handleCallback(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if rules.createCalls != 1 {
		t.Fatalf("createCalls = %d", rules.createCalls)
	}
	if rules.lastShopID != shopID {
		t.Fatalf("shop = %v", rules.lastShopID)
	}
	if rules.lastJSON["kind"] != "day_off" || rules.lastJSON["scope"] != "afternoon" {
		t.Fatalf("ruleJSON = %#v", rules.lastJSON)
	}
	if rules.lastJSON["employee_id"] != empID.String() {
		t.Fatalf("employee_id = %v", rules.lastJSON["employee_id"])
	}
	// private confirm + broadcast notify
	foundBroadcast := false
	for _, m := range msgAPI.messages {
		if m.chatID == broadcastID {
			foundBroadcast = true
			break
		}
	}
	if !foundBroadcast {
		t.Fatalf("expected broadcast notify, messages=%#v", msgAPI.messages)
	}
}
