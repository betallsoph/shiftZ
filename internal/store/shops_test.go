package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestSetOwnerTelegramIDAndLookup(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Owner Cafe").
		SetTimezone("UTC").
		SetInviteCode("own01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SetOwnerTelegramID(ctx, shopRow.ID, 424242); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ByOwnerTelegramID(ctx, 424242)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != shopRow.ID {
		t.Fatalf("got shop %v, want %v", got.ID, shopRow.ID)
	}
	if got.OwnerTelegramID != 424242 {
		t.Fatalf("got owner %d, want 424242", got.OwnerTelegramID)
	}
}

func TestSetOwnerTelegramIDRejectsZero(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Zero Cafe").
		SetTimezone("UTC").
		SetInviteCode("zero01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetOwnerTelegramID(ctx, shopRow.ID, 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("got %v", err)
	}
}

func TestClearOwnerTelegramID(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Clear Cafe").
		SetTimezone("UTC").
		SetInviteCode("clr01").
		SetTelegramGroupID(0).
		SetOwnerTelegramID(999).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ClearOwnerTelegramID(ctx, shopRow.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByOwnerTelegramID(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestBindTelegramGroupAndTeamChat(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Bind Cafe").
		SetTimezone("UTC").
		SetInviteCode("bind01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.BindTelegramGroup(ctx, shopRow.ID, -100111); err != nil {
		t.Fatal(err)
	}
	if err := repo.BindTelegramTeamChat(ctx, shopRow.ID, -100222); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ByID(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TelegramGroupID != -100111 {
		t.Fatalf("broadcast group %d, want -100111", got.TelegramGroupID)
	}
	if got.TelegramTeamChatID != -100222 {
		t.Fatalf("team chat %d, want -100222", got.TelegramTeamChatID)
	}
}

func TestBindTelegramGroupRejectsZero(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Reject Cafe").
		SetTimezone("UTC").
		SetInviteCode("rej01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.BindTelegramGroup(ctx, shopRow.ID, 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("got %v", err)
	}
}

func TestOwnerLinkTokenIssueAndConsume(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Link Cafe").
		SetTimezone("UTC").
		SetInviteCode("lnk01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	token, err := repo.IssueOwnerLinkToken(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ConsumeOwnerLinkToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != shopRow.ID {
		t.Fatalf("got shop %v, want %v", got.ID, shopRow.ID)
	}
	if _, err := repo.ConsumeOwnerLinkToken(ctx, token); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("reuse got %v", err)
	}
}

func TestOwnerLinkTokenInvalid(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	if _, err := repo.ConsumeOwnerLinkToken(ctx, "sz_ownerlink_deadbeef"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func TestOwnerTelegramIDUniquePerShop(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	a, err := client.Shop.Create().
		SetName("A").
		SetTimezone("UTC").
		SetInviteCode("uniq01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := client.Shop.Create().
		SetName("B").
		SetTimezone("UTC").
		SetInviteCode("uniq02").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetOwnerTelegramID(ctx, a.ID, 777); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetOwnerTelegramID(ctx, b.ID, 777); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("got %v", err)
	}
}

func TestShopRepoNotFound(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}
	missing := uuid.New()

	if err := repo.SetOwnerTelegramID(ctx, missing, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set owner: got %v", err)
	}
	if err := repo.ClearOwnerTelegramID(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("clear owner: got %v", err)
	}
	if err := repo.BindTelegramGroup(ctx, missing, -100); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bind group: got %v", err)
	}
	if _, err := repo.IssueOwnerLinkToken(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("issue token: got %v", err)
	}
}
