package store

import (
	"context"
	"testing"

	"github.com/betallsoph/shiftz/internal/ent/shop"
)

func TestCreateWithDashboardToken(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ShopRepo{client: client}

	creds, err := repo.CreateWithDashboardToken(ctx, "Onboard Cafe", "Asia/Ho_Chi_Minh", 0)
	if err != nil {
		t.Fatal(err)
	}
	if creds.OwnerToken == "" {
		t.Fatal("expected owner token")
	}
	if creds.Shop.Name != "Onboard Cafe" {
		t.Fatalf("name = %q", creds.Shop.Name)
	}

	row, err := client.Shop.Query().Where(shop.ID(creds.Shop.ID)).Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row.DashboardTokenHash == nil || *row.DashboardTokenHash == "" {
		t.Fatal("expected dashboard token hash")
	}
	if *row.DashboardTokenHash != HashDashboardToken(creds.OwnerToken) {
		t.Fatal("hash mismatch")
	}

	got, err := repo.VerifyDashboardToken(ctx, creds.Shop.ID, creds.OwnerToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != creds.Shop.ID {
		t.Fatalf("verify shop = %s", got.ID)
	}
}

func TestCreateDefaultsForShop(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRepo := &ShopRepo{client: client}
	shiftRepo := &ShiftRepo{client: client}

	creds, err := shopRepo.CreateWithDashboardToken(ctx, "Shift Cafe", "UTC", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := shiftRepo.CreateDefaultsForShop(ctx, creds.Shop.ID); err != nil {
		t.Fatal(err)
	}

	shifts, err := shiftRepo.ListByShop(ctx, creds.Shop.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shifts) != 14 {
		t.Fatalf("shifts = %d, want 14", len(shifts))
	}
}
