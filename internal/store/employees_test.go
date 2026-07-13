package store

import (
	"context"
	"errors"
	"testing"

	"github.com/betallsoph/shiftz/internal/ent/employee"
)

func TestEmployeeRepoJoinAndListAll(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Roster Cafe", "em1111")
	repo := &EmployeeRepo{client: client}

	active, err := repo.Join(ctx, shopRow.InviteCode, 1001, "Anna")
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := repo.Join(ctx, shopRow.InviteCode, 1002, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetActive(ctx, shopRow.ID, inactive.ID, false); err != nil {
		t.Fatal(err)
	}

	all, err := repo.ListAllByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("all = %d, want 2", len(all))
	}
	if !all[0].IsActive || all[0].ID != active.ID {
		t.Fatalf("expected active first, got %+v", all[0])
	}
	if all[1].IsActive {
		t.Fatal("expected inactive second")
	}

	activeOnly, err := repo.ListActiveByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeOnly) != 1 || activeOnly[0].ID != active.ID {
		t.Fatalf("active only = %+v", activeOnly)
	}
}

func TestEmployeeRepoUpdate(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Update Cafe", "em2222")
	repo := &EmployeeRepo{client: client}

	emp, err := repo.Join(ctx, shopRow.InviteCode, 2001, "Anna")
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.Update(ctx, shopRow.ID, emp.ID, UpdateEmployeeInput{
		DisplayName: "Anna K", Role: "barista", MaxHoursPerWeek: 35,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Anna K" || got.Role != "barista" || got.MaxHoursPerWeek != 35 {
		t.Fatalf("employee = %+v", got)
	}
}

func TestEmployeeRepoUpdateValidation(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Validate Emp", "em3333")
	repo := &EmployeeRepo{client: client}

	emp, err := repo.Join(ctx, shopRow.InviteCode, 3001, "Anna")
	if err != nil {
		t.Fatal(err)
	}

	cases := []UpdateEmployeeInput{
		{DisplayName: "  ", Role: "", MaxHoursPerWeek: 40},
		{DisplayName: "x", Role: "", MaxHoursPerWeek: 0},
		{DisplayName: "x", Role: "", MaxHoursPerWeek: 169},
	}
	for i, input := range cases {
		if _, err := repo.Update(ctx, shopRow.ID, emp.ID, input); !errors.Is(err, ErrValidation) {
			t.Fatalf("case %d: got %v, want ErrValidation", i, err)
		}
	}
}

func TestEmployeeRepoSetActive(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Toggle Emp", "em4444")
	repo := &EmployeeRepo{client: client}

	emp, err := repo.Join(ctx, shopRow.InviteCode, 4001, "Anna")
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.SetActive(ctx, shopRow.ID, emp.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsActive {
		t.Fatal("expected inactive")
	}
	got, err = repo.SetActive(ctx, shopRow.ID, emp.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsActive {
		t.Fatal("expected active")
	}
}

func TestEmployeeRepoCrossTenant(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopA := createTestShop(t, client, "Shop A", "em5555")
	shopB := createTestShop(t, client, "Shop B", "em6666")
	repo := &EmployeeRepo{client: client}

	emp, err := repo.Join(ctx, shopA.InviteCode, 5001, "Anna")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Update(ctx, shopB.ID, emp.ID, UpdateEmployeeInput{
		DisplayName: "x", Role: "", MaxHoursPerWeek: 40,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update: got %v", err)
	}
	if _, err := repo.SetActive(ctx, shopB.ID, emp.ID, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set active: got %v", err)
	}
}

func TestEmployeeRepoJoinInactiveNotReactivated(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Inactive Join", "em7777")
	repo := &EmployeeRepo{client: client}

	emp, err := repo.Join(ctx, shopRow.InviteCode, 6001, "Anna")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetActive(ctx, shopRow.ID, emp.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Join(ctx, shopRow.InviteCode, 6001, "Anna Updated"); !errors.Is(err, ErrEmployeeInactive) {
		t.Fatalf("got %v, want ErrEmployeeInactive", err)
	}

	row, err := client.Employee.Get(ctx, emp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.IsActive {
		t.Fatal("employee should stay inactive")
	}
	if row.DisplayName != "Anna" {
		t.Fatalf("display name = %q, want unchanged", row.DisplayName)
	}
}

func TestEmployeeRepoJoinUpdatesActiveDisplayName(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	shopRow := createTestShop(t, client, "Rejoin Active", "em8888")
	repo := &EmployeeRepo{client: client}

	if _, err := repo.Join(ctx, shopRow.InviteCode, 7001, "Anna"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Join(ctx, shopRow.InviteCode, 7001, "Anna Lee")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Anna Lee" || !got.IsActive {
		t.Fatalf("employee = %+v", got)
	}
	count, err := client.Employee.Query().Where(employee.ShopID(shopRow.ID)).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}
