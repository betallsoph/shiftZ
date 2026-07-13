package onboarding

import (
	"context"
	"testing"

	"fmt"
	"github.com/betallsoph/shiftz/internal/ent/enttest"
	_ "github.com/mattn/go-sqlite3"
	"strings"

	"github.com/betallsoph/shiftz/internal/store"
)

func TestServiceCreateShopWithDefaultShifts(t *testing.T) {
	ctx := context.Background()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	st := store.NewWithClient(client)
	svc := New(st)

	result, err := svc.CreateShop(ctx, "Beta Cafe", "Asia/Ho_Chi_Minh", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.OwnerToken == "" {
		t.Fatal("expected owner token")
	}

	got, err := st.Shops.VerifyDashboardToken(ctx, result.Shop.ID, result.OwnerToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != result.Shop.ID {
		t.Fatalf("shop = %s", got.ID)
	}

	shifts, err := st.Shifts.ListByShop(ctx, result.Shop.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shifts) != 14 {
		t.Fatalf("shifts = %d, want 14", len(shifts))
	}
}

func TestServiceCreateShopSkipsDefaultShifts(t *testing.T) {
	ctx := context.Background()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	st := store.NewWithClient(client)
	svc := New(st)

	result, err := svc.CreateShop(ctx, "No Shift Cafe", "UTC", false)
	if err != nil {
		t.Fatal(err)
	}

	shifts, err := st.Shifts.ListByShop(ctx, result.Shop.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shifts) != 0 {
		t.Fatalf("shifts = %d, want 0", len(shifts))
	}
}
