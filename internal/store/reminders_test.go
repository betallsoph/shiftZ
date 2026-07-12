package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/betallsoph/shiftz/internal/ent/enttest"
	_ "github.com/mattn/go-sqlite3"
)

func TestReminderDeliveryRepoCreatePendingIdempotent(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", strings.ReplaceAll(t.Name(), "/", "_")))
	repo := &ReminderDeliveryRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Reminder Shop").
		SetTimezone("UTC").
		SetInviteCode("remind1").
		SetTelegramGroupID(1).
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

	weekStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	created, err := repo.CreatePending(ctx, shopRow.ID, empRow.ID, weekStart, ReminderKindAvailabilityReminder)
	if err != nil || !created {
		t.Fatalf("first create: created=%v err=%v", created, err)
	}
	created, err = repo.CreatePending(ctx, shopRow.ID, empRow.ID, weekStart, ReminderKindAvailabilityReminder)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("second create should be deduped")
	}

	pending, err := repo.ListPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want 1", len(pending))
	}

	if err := repo.MarkSent(ctx, pending[0].ID); err != nil {
		t.Fatal(err)
	}
	pending, err = repo.ListPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after sent = %d", len(pending))
	}
}

func TestReminderDeliveryRepoMarkFailed(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", strings.ReplaceAll(t.Name(), "/", "_")))
	repo := &ReminderDeliveryRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Fail Shop").
		SetTimezone("UTC").
		SetInviteCode("fail01").
		SetTelegramGroupID(2).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	empRow, err := client.Employee.Create().
		SetShopID(shopRow.ID).
		SetTelegramUserID(99).
		SetDisplayName("Bob").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	weekStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	_, err = repo.CreatePending(ctx, shopRow.ID, empRow.ID, weekStart, ReminderKindAvailabilityNag)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := repo.ListPending(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkFailed(ctx, pending[0].ID, 1, "telegram down"); err != nil {
		t.Fatal(err)
	}
	pending, err = repo.ListPending(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatal("failed delivery should leave pending queue")
	}
}
