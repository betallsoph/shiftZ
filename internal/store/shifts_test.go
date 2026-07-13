package store

import (
	"context"
	"errors"
	"testing"

	"github.com/betallsoph/shiftz/internal/ent"
)

func TestShiftRepoCreate(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Shift Cafe", "sh1111")
	repo := &ShiftRepo{client: client}

	got, err := repo.Create(ctx, shopRow.ID, CreateShiftInput{
		Name: "morning", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsActive || got.Name != "morning" {
		t.Fatalf("shift = %+v", got)
	}
}

func TestShiftRepoCreateValidation(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Validate Cafe", "sh2222")
	repo := &ShiftRepo{client: client}

	cases := []CreateShiftInput{
		{Name: "  ", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2},
		{Name: "x", Weekday: 9, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2},
		{Name: "x", Weekday: 1, StartTime: "8:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2},
		{Name: "x", Weekday: 1, StartTime: "08:00", EndTime: "08:00", MinStaff: 1, MaxStaff: 2},
		{Name: "x", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 3, MaxStaff: 2},
		{Name: "x", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 0},
	}
	for i, input := range cases {
		if _, err := repo.Create(ctx, shopRow.ID, input); !errors.Is(err, ErrValidation) {
			t.Fatalf("case %d: got %v, want ErrValidation", i, err)
		}
	}
}

func TestShiftRepoListByShopActiveOnly(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "List Cafe", "sh3333")
	repo := &ShiftRepo{client: client}

	active, err := repo.Create(ctx, shopRow.ID, CreateShiftInput{
		Name: "active", Weekday: 2, StartTime: "08:00", EndTime: "12:00", MinStaff: 1, MaxStaff: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := repo.Create(ctx, shopRow.ID, CreateShiftInput{
		Name: "inactive", Weekday: 3, StartTime: "13:00", EndTime: "17:00", MinStaff: 1, MaxStaff: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetActive(ctx, shopRow.ID, inactive.ID, false); err != nil {
		t.Fatal(err)
	}

	activeOnly, err := repo.ListByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeOnly) != 1 || activeOnly[0].ID != active.ID {
		t.Fatalf("active only = %+v", activeOnly)
	}

	all, err := repo.ListAllByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all = %d, want 2", len(all))
	}
}

func TestShiftRepoSetActive(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Toggle Cafe", "sh6666")
	repo := &ShiftRepo{client: client}

	shift, err := repo.Create(ctx, shopRow.ID, CreateShiftInput{
		Name: "evening", Weekday: 5, StartTime: "14:00", EndTime: "20:00", MinStaff: 1, MaxStaff: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.SetActive(ctx, shopRow.ID, shift.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsActive {
		t.Fatal("expected inactive")
	}
}

func TestShiftRepoSetActiveCrossTenant(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopA := createTestShop(t, client, "Shop A", "sh4444")
	shopB := createTestShop(t, client, "Shop B", "sh5555")
	repo := &ShiftRepo{client: client}

	shift, err := repo.Create(ctx, shopA.ID, CreateShiftInput{
		Name: "morning", Weekday: 1, StartTime: "08:00", EndTime: "14:00", MinStaff: 1, MaxStaff: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetActive(ctx, shopB.ID, shift.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func createTestShop(t *testing.T, client *ent.Client, name, invite string) *ent.Shop {
	t.Helper()
	ctx := context.Background()
	row, err := client.Shop.Create().
		SetName(name).
		SetTimezone("UTC").
		SetInviteCode(invite).
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return row
}
