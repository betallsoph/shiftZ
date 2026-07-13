package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRotateTelegramSetupCodeStoresHashOnly(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	row, err := client.Shop.Create().
		SetName("TG Cafe").
		SetTimezone("UTC").
		SetInviteCode("tg1234").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	expiresAt := time.Now().Add(30 * time.Minute)
	code, err := repo.RotateTelegramSetupCode(ctx, row.ID, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if code == "" || code[:len(telegramSetupPrefix)] != telegramSetupPrefix {
		t.Fatalf("code = %q", code)
	}

	got, err := client.Shop.Get(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TelegramSetupCodeHash == nil || *got.TelegramSetupCodeHash != HashTelegramSetupCode(code) {
		t.Fatal("expected hash stored, not plaintext")
	}
	if got.TelegramSetupCodeHash != nil && *got.TelegramSetupCodeHash == code {
		t.Fatal("plaintext code stored in database")
	}
}

func TestVerifyTelegramSetupCodeValid(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	row, err := client.Shop.Create().
		SetName("Verify Cafe").
		SetTimezone("UTC").
		SetInviteCode("vf1234").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().Add(30 * time.Minute)
	code, err := repo.RotateTelegramSetupCode(ctx, row.ID, expiresAt)
	if err != nil {
		t.Fatal(err)
	}

	shop, err := repo.VerifyTelegramSetupCode(ctx, code, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if shop.ID != row.ID {
		t.Fatalf("shop = %s", shop.ID)
	}
}

func TestVerifyTelegramSetupCodeWrong(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	row, err := client.Shop.Create().
		SetName("Wrong Cafe").
		SetTimezone("UTC").
		SetInviteCode("wr1234").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	code, err := repo.RotateTelegramSetupCode(ctx, row.ID, time.Now().Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.VerifyTelegramSetupCode(ctx, code+"x", time.Now()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyTelegramSetupCodeExpired(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	row, err := client.Shop.Create().
		SetName("Expired Cafe").
		SetTimezone("UTC").
		SetInviteCode("ex1234").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().Add(-time.Minute)
	code, err := repo.RotateTelegramSetupCode(ctx, row.ID, expiresAt)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.VerifyTelegramSetupCode(ctx, code, time.Now()); !errors.Is(err, ErrExpiredSetupCode) {
		t.Fatalf("got %v", err)
	}
}

func TestSetTelegramGroupClearsSetupCode(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	row, err := client.Shop.Create().
		SetName("Connect Cafe").
		SetTimezone("UTC").
		SetInviteCode("cn1234").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	code, err := repo.RotateTelegramSetupCode(ctx, row.ID, time.Now().Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetTelegramGroup(ctx, row.ID, -100123); err != nil {
		t.Fatal(err)
	}

	got, err := client.Shop.Get(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TelegramGroupID != -100123 {
		t.Fatalf("group id = %d", got.TelegramGroupID)
	}
	if got.TelegramSetupCodeHash != nil {
		t.Fatal("expected setup hash cleared")
	}
	if got.TelegramSetupCodeExpiresAt != nil {
		t.Fatal("expected setup expiry cleared")
	}
	if _, err := repo.VerifyTelegramSetupCode(ctx, code, time.Now()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("code still valid: %v", err)
	}
}
