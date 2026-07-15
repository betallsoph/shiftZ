package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/availabilitydraft"
)

func TestAvailabilityDraftRepoCreateGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Draft Cafe", "dr1111")
	empRow := createTestEmployee(t, client, shopRow.ID, 1001, "Anna")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	weekStart := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	slot := AvailabilitySlot{
		Start:      weekStart.Add(8 * time.Hour),
		End:        weekStart.Add(14 * time.Hour),
		Preference: 1,
		Note:       "morning",
	}
	id, err := repo.Create(ctx, AvailabilityDraft{
		ShopID:         shopRow.ID,
		EmployeeID:     empRow.ID,
		TelegramUserID: 1001,
		ChatID:         200,
		WeekStart:      weekStart,
		Timezone:       "Asia/Ho_Chi_Minh",
		Slots:          []AvailabilitySlot{slot},
		RawMessage:     "Mon mornings",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, ok, err := repo.Get(ctx, id)
	if err != nil || !ok {
		t.Fatalf("get draft: ok=%v err=%v", ok, err)
	}
	if got.ShopID != shopRow.ID || got.EmployeeID != empRow.ID || got.TelegramUserID != 1001 {
		t.Fatalf("draft = %+v", got)
	}
	if len(got.Slots) != 1 || got.Slots[0].Preference != 1 || got.Slots[0].Note != "morning" {
		t.Fatalf("slots = %+v", got.Slots)
	}
	if got.RawMessage != "Mon mornings" || got.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("draft meta = %+v", got)
	}
}

func TestAvailabilityDraftRepoDefaultTTL(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "TTL Cafe", "dr2222")
	empRow := createTestEmployee(t, client, shopRow.ID, 1002, "Bob")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	before := time.Now()
	id, err := repo.Create(ctx, AvailabilityDraft{
		ShopID:         shopRow.ID,
		EmployeeID:     empRow.ID,
		TelegramUserID: 1002,
		ChatID:         201,
		WeekStart:      time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		Timezone:       "UTC",
		Slots:          []AvailabilitySlot{{Start: before, End: before.Add(time.Hour), Preference: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := repo.Get(ctx, id)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if got.ExpiresAt.Sub(got.CreatedAt) != DefaultAvailabilityDraftTTL {
		t.Fatalf("ttl = %v, want %v", got.ExpiresAt.Sub(got.CreatedAt), DefaultAvailabilityDraftTTL)
	}
}

func TestAvailabilityDraftRepoExpiredNotReturned(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Expired Cafe", "dr3333")
	empRow := createTestEmployee(t, client, shopRow.ID, 1003, "Cara")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	past := time.Now().Add(-time.Minute)
	id, err := repo.Create(ctx, AvailabilityDraft{
		ShopID:         shopRow.ID,
		EmployeeID:     empRow.ID,
		TelegramUserID: 1003,
		ChatID:         202,
		WeekStart:      time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		Timezone:       "UTC",
		Slots:          []AvailabilitySlot{{Start: past, End: past.Add(time.Hour), Preference: 1}},
		CreatedAt:      past.Add(-DefaultAvailabilityDraftTTL),
		ExpiresAt:      past,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.Get(ctx, id); err != nil || ok {
		t.Fatalf("expired draft should not be returned: ok=%v err=%v", ok, err)
	}
	count, err := client.AvailabilityDraft.Query().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired draft count = %d, want 0 after opportunistic delete", count)
	}
}

func TestAvailabilityDraftRepoDeleteIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Delete Cafe", "dr4444")
	empRow := createTestEmployee(t, client, shopRow.ID, 1004, "Dan")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	id, err := repo.Create(ctx, AvailabilityDraft{
		ShopID: shopRow.ID, EmployeeID: empRow.ID, TelegramUserID: 1004, ChatID: 203,
		WeekStart: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), Timezone: "UTC",
		Slots: []AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, id); err != nil {
		t.Fatal("second delete should be idempotent")
	}
}

func TestAvailabilityDraftRepoDeleteExpired(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Purge Cafe", "dr5555")
	empRow := createTestEmployee(t, client, shopRow.ID, 1005, "Eve")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	_, err := repo.Create(ctx, AvailabilityDraft{
		ShopID: shopRow.ID, EmployeeID: empRow.ID, TelegramUserID: 1005, ChatID: 204,
		WeekStart: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), Timezone: "UTC",
		Slots:     []AvailabilitySlot{{Start: past, End: past.Add(time.Hour), Preference: 1}},
		ExpiresAt: past,
	})
	if err != nil {
		t.Fatal(err)
	}
	activeID, err := repo.Create(ctx, AvailabilityDraft{
		ShopID: shopRow.ID, EmployeeID: empRow.ID, TelegramUserID: 1005, ChatID: 204,
		WeekStart: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), Timezone: "UTC",
		Slots:     []AvailabilitySlot{{Start: now, End: now.Add(time.Hour), Preference: 1}},
		ExpiresAt: future,
	})
	if err != nil {
		t.Fatal(err)
	}

	n, err := repo.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted = %d, want 1", n)
	}
	if _, ok, _ := repo.Get(ctx, activeID); !ok {
		t.Fatal("active draft should remain")
	}
}

func TestAvailabilityDraftRepoCascadeOnEmployeeDelete(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Cascade Cafe", "dr6666")
	empRow := createTestEmployee(t, client, shopRow.ID, 1006, "Finn")
	repo := &AvailabilityDraftRepo{client: client, ttl: DefaultAvailabilityDraftTTL}

	id, err := repo.Create(ctx, AvailabilityDraft{
		ShopID: shopRow.ID, EmployeeID: empRow.ID, TelegramUserID: 1006, ChatID: 205,
		WeekStart: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), Timezone: "UTC",
		Slots: []AvailabilitySlot{{Start: time.Now(), End: time.Now().Add(time.Hour), Preference: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Employee.DeleteOneID(empRow.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := repo.Get(ctx, id); ok {
		t.Fatal("draft should cascade delete with employee")
	}
	count, err := client.AvailabilityDraft.Query().Where(availabilitydraft.ID(id)).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("draft rows = %d", count)
	}
}

func createTestEmployee(t *testing.T, client *ent.Client, shopID uuid.UUID, tgID int64, name string) *ent.Employee {
	t.Helper()
	row, err := client.Employee.Create().
		SetShopID(shopID).
		SetTelegramUserID(tgID).
		SetDisplayName(name).
		SetIsActive(true).
		Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return row
}
