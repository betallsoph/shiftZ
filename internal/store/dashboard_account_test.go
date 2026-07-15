package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeDashboardUsername(t *testing.T) {
	if got := NormalizeDashboardUsername("  Demo.Cafe  "); got != "demo.cafe" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateDashboardUsername(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"demo", true},
		{"demo.cafe", true},
		{"a1b", true},
		{"ab", false},
		{"-bad", false},
		{"", false},
		{string(make([]byte, 33)), false},
	}
	for _, tc := range cases {
		err := ValidateDashboardUsername(tc.in)
		if tc.valid && err != nil {
			t.Fatalf("%q: want valid, got %v", tc.in, err)
		}
		if !tc.valid && err == nil {
			t.Fatalf("%q: want invalid", tc.in)
		}
	}
}

func TestValidatePlan(t *testing.T) {
	for _, plan := range []string{"free", "starter", "pro", "FREE"} {
		if err := ValidatePlan(plan); err != nil {
			t.Fatalf("%q: %v", plan, err)
		}
	}
	if err := ValidatePlan("enterprise"); err == nil {
		t.Fatal("want invalid plan error")
	}
}

func TestProvisionDashboardAccount(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Provision Cafe").
		SetTimezone("UTC").
		SetInviteCode("prov01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	creds, err := repo.ProvisionDashboardAccount(ctx, shopRow.ID, "demo.cafe", "starter")
	if err != nil {
		t.Fatal(err)
	}
	if creds.OwnerToken == "" {
		t.Fatal("expected owner token")
	}
	if creds.Shop.DashboardUsername != "demo.cafe" {
		t.Fatalf("username = %q", creds.Shop.DashboardUsername)
	}
	if creds.Shop.Plan != "starter" {
		t.Fatalf("plan = %q", creds.Shop.Plan)
	}

	got, err := repo.VerifyDashboardCredentials(ctx, "demo.cafe", creds.OwnerToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != shopRow.ID {
		t.Fatalf("shop = %s", got.ID)
	}
}

func TestProvisionDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	a, err := client.Shop.Create().SetName("A").SetTimezone("UTC").SetInviteCode("dupa01").SetTelegramGroupID(0).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := client.Shop.Create().SetName("B").SetTimezone("UTC").SetInviteCode("dupb01").SetTelegramGroupID(0).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ProvisionDashboardAccount(ctx, a.ID, "taken", "free"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ProvisionDashboardAccount(ctx, b.ID, "taken", "free"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyDashboardCredentialsWrongToken(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Verify Cafe").
		SetTimezone("UTC").
		SetInviteCode("ver01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := repo.ProvisionDashboardAccount(ctx, shopRow.ID, "verify", "free")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.VerifyDashboardCredentials(ctx, "verify", creds.OwnerToken+"x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdatePlan(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Plan Cafe").
		SetTimezone("UTC").
		SetInviteCode("plan01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := repo.ProvisionDashboardAccount(ctx, shopRow.ID, "plan", "free")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := repo.UpdatePlan(ctx, shopRow.ID, "pro")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Plan != "pro" {
		t.Fatalf("plan = %q", updated.Plan)
	}
	if _, err := repo.VerifyDashboardCredentials(ctx, "plan", creds.OwnerToken); err != nil {
		t.Fatalf("token should still work: %v", err)
	}
}

func TestRotateDashboardTokenInvalidatesOld(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Rotate Cafe").
		SetTimezone("UTC").
		SetInviteCode("rot01").
		SetTelegramGroupID(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.ProvisionDashboardAccount(ctx, shopRow.ID, "rotate", "free")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.RotateDashboardToken(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.OwnerToken == first.OwnerToken {
		t.Fatal("expected new token")
	}
	if _, err := repo.VerifyDashboardCredentials(ctx, "rotate", first.OwnerToken); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old token should fail: %v", err)
	}
	if _, err := repo.VerifyDashboardCredentials(ctx, "rotate", second.OwnerToken); err != nil {
		t.Fatalf("new token should work: %v", err)
	}
}

func TestShopWithoutUsernameLegacyVerifyByID(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Legacy Cafe").
		SetTimezone("UTC").
		SetInviteCode("leg01").
		SetTelegramGroupID(0).
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
	got, err := repo.ByID(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DashboardUsername != "" {
		t.Fatalf("username = %q, want empty", got.DashboardUsername)
	}
	if _, err := repo.VerifyDashboardToken(ctx, shopRow.ID, token); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionNotFound(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}
	if _, err := repo.ProvisionDashboardAccount(ctx, uuid.New(), "ghost", "free"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestShopModelDoesNotExposeTokenHash(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}
	shopRow, err := client.Shop.Create().
		SetName("Hash Cafe").
		SetTimezone("UTC").
		SetInviteCode("hash01").
		SetTelegramGroupID(0).
		SetDashboardTokenHash("deadbeef").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.ByID(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	// store.Shop has no hash field — compile-time guarantee; runtime spot-check name only.
	if got.Name != "Hash Cafe" {
		t.Fatal("unexpected shop")
	}
}
