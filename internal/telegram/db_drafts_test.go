package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/betallsoph/shiftz/internal/ent/enttest"
	"github.com/betallsoph/shiftz/internal/store"
	_ "github.com/mattn/go-sqlite3"
)

func TestStoreDraftConfirmAcrossBotInstances(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:telegramdraft?mode=memory&cache=shared&_fk=1")
	st := store.NewWithClient(client)

	shopRow, err := client.Shop.Create().
		SetName("Draft Shop").
		SetTimezone("UTC").
		SetInviteCode("draftshop").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	empRow, err := client.Employee.Create().
		SetShopID(shopRow.ID).
		SetTelegramUserID(42).
		SetDisplayName("Anna").
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	weekStart := nextMonday(time.Now().UTC())
	slot := store.AvailabilitySlot{
		Start:      weekStart.Add(8 * time.Hour),
		End:        weekStart.Add(14 * time.Hour),
		Preference: 1,
	}

	draftsA := NewStoreAvailabilityDraftStore(st.AvailabilityDrafts)
	draftID, err := draftsA.Create(ctx, AvailabilityDraft{
		TelegramUserID: 42,
		ChatID:         100,
		ShopID:         shopRow.ID,
		EmployeeID:     empRow.ID,
		WeekStart:      weekStart,
		Timezone:       "UTC",
		Slots:          []store.AvailabilitySlot{slot},
		RawMessage:     "Mon mornings",
	})
	if err != nil {
		t.Fatal(err)
	}

	draftsB := NewStoreAvailabilityDraftStore(st.AvailabilityDrafts)
	msgAPI := &fakeMessenger{}
	availability := &fakeAvailability{}
	employees := &fakeEmployees{emp: &store.Employee{ID: empRow.ID, ShopID: shopRow.ID, TelegramUserID: 42}}
	botB := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, employees, availability, &fakeVotes{}, draftsB, testLogger())

	q := &CallbackQuery{
		ID:      "cb-db",
		From:    User{ID: 42},
		Data:    availConfirmPrefix + draftID.String(),
		Message: &Message{Chat: Chat{ID: 100, Type: "private"}},
	}
	if err := botB.handleCallback(ctx, q); err != nil {
		t.Fatal(err)
	}
	if availability.replaceCalls != 1 {
		t.Fatalf("ReplaceWeek calls = %d, want 1", availability.replaceCalls)
	}
	if _, ok, _ := draftsB.Get(ctx, draftID); ok {
		t.Fatal("draft should be deleted after confirm")
	}
}

func TestStoreDraftConfirmWrongUser(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:telegramdraftwrong?mode=memory&cache=shared&_fk=1")
	st := store.NewWithClient(client)

	shopRow, err := client.Shop.Create().
		SetName("Wrong User Shop").
		SetTimezone("UTC").
		SetInviteCode("wrongshop").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	empRow, err := client.Employee.Create().
		SetShopID(shopRow.ID).
		SetTelegramUserID(42).
		SetDisplayName("Anna").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	drafts := NewStoreAvailabilityDraftStore(st.AvailabilityDrafts)
	draftID, err := drafts.Create(ctx, AvailabilityDraft{
		TelegramUserID: 42,
		ShopID:         shopRow.ID,
		EmployeeID:     empRow.ID,
		WeekStart:      nextMonday(time.Now().UTC()),
		Timezone:       "UTC",
		Slots:          []store.AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	msgAPI := &fakeMessenger{}
	bot := NewBot(msgAPI, &fakeParser{}, &fakeShops{}, &fakeEmployees{}, &fakeAvailability{}, &fakeVotes{}, drafts, testLogger())
	q := &CallbackQuery{ID: "cb-wrong", From: User{ID: 99}, Data: availConfirmPrefix + draftID.String()}
	if err := bot.handleCallback(ctx, q); err != nil {
		t.Fatal(err)
	}
	if msgAPI.answers[0] != "This confirmation is not yours." {
		t.Fatalf("answer = %q", msgAPI.answers[0])
	}
	if _, ok, _ := drafts.Get(ctx, draftID); !ok {
		t.Fatal("draft should remain for owner")
	}
}
