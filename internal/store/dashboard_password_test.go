package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestDashboardPasswordSetAndVerify(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Password Cafe").
		SetTimezone("UTC").
		SetInviteCode("pass01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	has, err := repo.HasDashboardPassword(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected no password yet")
	}

	if err := repo.SetDashboardPassword(ctx, shopRow.ID, "secret123"); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyDashboardPassword(ctx, shopRow.ID, "secret123"); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyDashboardPassword(ctx, shopRow.ID, "wrongpass"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func TestDashboardPasswordTooShort(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Short Cafe").
		SetTimezone("UTC").
		SetInviteCode("shrt01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardPassword(ctx, shopRow.ID, "short"); !errors.Is(err, ErrValidation) {
		t.Fatalf("got %v", err)
	}
}

func TestDashboardPasswordSetTwiceRejected(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Twice Cafe").
		SetTimezone("UTC").
		SetInviteCode("twc01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardPassword(ctx, shopRow.ID, "firstpass"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardPassword(ctx, shopRow.ID, "secondpass"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("got %v", err)
	}
}

func TestDashboardPasswordNotFound(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	if err := repo.SetDashboardPassword(ctx, uuid.New(), "secret123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set: got %v", err)
	}
	if err := repo.VerifyDashboardPassword(ctx, uuid.New(), "secret123"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("verify: got %v", err)
	}
}

func TestDashboardCredentialsSetEmailAndHint(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Email Cafe").
		SetTimezone("UTC").
		SetInviteCode("eml001").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.SetDashboardCredentials(ctx, shopRow.ID, "secret123", "Owner@Example.com", "mèo con"); err != nil {
		t.Fatal(err)
	}
	email, err := repo.DashboardEmail(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if email != "owner@example.com" {
		t.Fatalf("email = %q", email)
	}
	row, err := client.Shop.Get(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.DashboardPasswordHint == nil || *row.DashboardPasswordHint != "mèo con" {
		t.Fatalf("hint = %v", row.DashboardPasswordHint)
	}
}

func TestDashboardCredentialsRequireEmail(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("No Email Cafe").
		SetTimezone("UTC").
		SetInviteCode("noem01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardCredentials(ctx, shopRow.ID, "secret123", "", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("got %v", err)
	}
}

func TestDashboardPasswordResetFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Reset Cafe").
		SetTimezone("UTC").
		SetInviteCode("rst001").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetDashboardCredentials(ctx, shopRow.ID, "oldpass1", "owner@example.com", ""); err != nil {
		t.Fatal(err)
	}
	token, err := repo.IssueDashboardPasswordReset(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ResetDashboardPasswordWithToken(ctx, token, "newpass1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyDashboardPassword(ctx, shopRow.ID, "newpass1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyDashboardPassword(ctx, shopRow.ID, "oldpass1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
	if _, err := repo.ResetDashboardPasswordWithToken(ctx, token, "another1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("reuse token: got %v", err)
	}
}
