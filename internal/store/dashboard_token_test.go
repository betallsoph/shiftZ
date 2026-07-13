package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestVerifyDashboardToken(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Token Cafe").
		SetTimezone("UTC").
		SetInviteCode("tok123").
		SetTelegramGroupID(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	token, err := NewDashboardToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardTokenHash(ctx, shopRow.ID, HashDashboardToken(token)); err != nil {
		t.Fatal(err)
	}

	got, err := repo.VerifyDashboardToken(ctx, shopRow.ID, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != shopRow.ID {
		t.Fatalf("shop = %s, want %s", got.ID, shopRow.ID)
	}

	if _, err := repo.VerifyDashboardToken(ctx, shopRow.ID, token+"x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong token: got %v", err)
	}
	if _, err := repo.VerifyDashboardToken(ctx, uuid.New(), token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong shop: got %v", err)
	}
}
