package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/enttest"
	"github.com/betallsoph/shiftz/internal/ent/schedule"
	_ "github.com/mattn/go-sqlite3"
)

func newTestClient(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	return enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
}

func TestScheduleRepo(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ScheduleRepo{client: client}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	weekStart := WeekStart(time.Date(2026, 7, 8, 0, 0, 0, 0, loc), loc)
	shiftDate := weekStart.AddDate(0, 0, 2) // Wednesday

	shopRow, err := client.Shop.Create().
		SetName("Test Cafe").
		SetTimezone("Asia/Ho_Chi_Minh").
		SetInviteCode("abc123").
		SetTelegramGroupID(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	otherShop, err := client.Shop.Create().
		SetName("Other Cafe").
		SetTimezone("UTC").
		SetInviteCode("xyz789").
		SetTelegramGroupID(2).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	emp, err := client.Employee.Create().
		SetShopID(shopRow.ID).
		SetTelegramUserID(42).
		SetDisplayName("Anna").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shift, err := client.Shift.Create().
		SetShopID(shopRow.ID).
		SetName("morning").
		SetWeekday(int(shiftDate.Weekday())).
		SetStartTime("08:00").
		SetEndTime("14:00").
		SetMinStaff(1).
		SetMaxStaff(2).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	candA, err := repo.CreateCandidate(ctx, shopRow.ID, weekStart, "A", 10.5)
	if err != nil {
		t.Fatalf("CreateCandidate A: %v", err)
	}
	if candA.Status != "draft" {
		t.Fatalf("status = %q, want draft", candA.Status)
	}

	candB, err := repo.CreateCandidate(ctx, shopRow.ID, weekStart, "B", 9.0)
	if err != nil {
		t.Fatalf("CreateCandidate B: %v", err)
	}

	if err := repo.AddAssignments(ctx, shopRow.ID, candA.ID, []NewScheduleAssignment{
		{ShiftID: shift.ID, EmployeeID: emp.ID, Date: shiftDate},
	}); err != nil {
		t.Fatalf("AddAssignments: %v", err)
	}
	if err := repo.AddAssignments(ctx, shopRow.ID, candA.ID, nil); err != nil {
		t.Fatalf("AddAssignments empty: %v", err)
	}

	if err := repo.AddAssignments(ctx, otherShop.ID, candA.ID, []NewScheduleAssignment{
		{ShiftID: shift.ID, EmployeeID: emp.ID, Date: shiftDate},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddAssignments wrong shop: got %v, want ErrNotFound", err)
	}

	list, err := repo.ListByShopWeek(ctx, shopRow.ID, weekStart)
	if err != nil {
		t.Fatalf("ListByShopWeek: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d candidates, want 2", len(list))
	}
	if list[0].VariantLabel != "A" {
		t.Fatalf("first variant = %q, want A", list[0].VariantLabel)
	}
	if len(list[0].Assignments) != 1 {
		t.Fatalf("candidate A assignments = %d, want 1", len(list[0].Assignments))
	}
	a0 := list[0].Assignments[0]
	if a0.ShiftName != "morning" || a0.EmployeeName != "Anna" {
		t.Fatalf("display fields not loaded: %+v", a0)
	}

	approved, err := repo.Approve(ctx, shopRow.ID, candB.ID)
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("approved status = %q", approved.Status)
	}

	list, err = repo.ListByShopWeek(ctx, shopRow.ID, weekStart)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		switch s.ID {
		case candB.ID:
			if s.Status != "approved" {
				t.Fatalf("candidate B status = %q, want approved", s.Status)
			}
		case candA.ID:
			if s.Status != "draft" {
				t.Fatalf("candidate A status = %q, want draft", s.Status)
			}
		}
	}

	if _, err := repo.Approve(ctx, otherShop.ID, candB.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Approve wrong shop: got %v, want ErrNotFound", err)
	}
	if _, err := repo.Approve(ctx, shopRow.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Approve missing: got %v, want ErrNotFound", err)
	}
}

func TestCreateCandidatesAtomic(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &ScheduleRepo{client: client}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	weekStart := WeekStart(time.Date(2026, 7, 8, 0, 0, 0, 0, loc), loc)
	shiftDate := weekStart.AddDate(0, 0, 2)

	shopRow, err := client.Shop.Create().
		SetName("Atomic Cafe").
		SetTimezone("Asia/Ho_Chi_Minh").
		SetInviteCode("atomic1").
		SetTelegramGroupID(10).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	emp, err := client.Employee.Create().
		SetShopID(shopRow.ID).
		SetTelegramUserID(99).
		SetDisplayName("Anna").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shift, err := client.Shift.Create().
		SetShopID(shopRow.ID).
		SetName("morning").
		SetWeekday(int(shiftDate.Weekday())).
		SetStartTime("08:00").
		SetEndTime("14:00").
		SetMinStaff(1).
		SetMaxStaff(2).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	validAssignment := NewScheduleAssignment{ShiftID: shift.ID, EmployeeID: emp.ID, Date: shiftDate}
	saved, err := repo.CreateCandidates(ctx, shopRow.ID, weekStart, []NewScheduleCandidate{
		{VariantLabel: "A", Score: 10, Assignments: []NewScheduleAssignment{validAssignment}},
		{VariantLabel: "B", Score: 9, Assignments: []NewScheduleAssignment{validAssignment}},
		{VariantLabel: "C", Score: 8, Assignments: []NewScheduleAssignment{validAssignment}},
	})
	if err != nil {
		t.Fatalf("CreateCandidates: %v", err)
	}
	if len(saved) != 3 {
		t.Fatalf("saved = %d, want 3", len(saved))
	}

	// Rollback: invalid employee on second candidate must leave no rows.
	badWeek := weekStart.AddDate(0, 0, 7)
	_, err = repo.CreateCandidates(ctx, shopRow.ID, badWeek, []NewScheduleCandidate{
		{VariantLabel: "A", Score: 10, Assignments: []NewScheduleAssignment{validAssignment}},
		{VariantLabel: "B", Score: 9, Assignments: []NewScheduleAssignment{
			{ShiftID: shift.ID, EmployeeID: uuid.New(), Date: shiftDate},
		}},
	})
	if err == nil {
		t.Fatal("expected error for invalid assignment")
	}
	count, err := client.Schedule.Query().
		Where(schedule.ShopID(shopRow.ID), schedule.WeekStart(badWeek)).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("partial schedule persisted after failed CreateCandidates: count = %d", count)
	}

	// Duplicate protection.
	_, err = repo.CreateCandidates(ctx, shopRow.ID, weekStart, []NewScheduleCandidate{
		{VariantLabel: "A", Score: 1},
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate CreateCandidates: got %v, want ErrAlreadyExists", err)
	}
}

func TestRuleRepo_ListByShop(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &RuleRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Rule Shop").
		SetTimezone("UTC").
		SetInviteCode("rule01").
		SetTelegramGroupID(3).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Rule.Create().
		SetShopID(shopRow.ID).
		SetDescription("no doubles").
		SetRuleJSON(map[string]any{"kind": "avoid_pair"}).
		Save(ctx); err != nil {
		t.Fatal(err)
	}

	rules, err := repo.ListByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Description != "no doubles" {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestRuleRepo_Create(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	repo := &RuleRepo{client: client}

	shopRow, err := client.Shop.Create().
		SetName("Create Rule Shop").
		SetTimezone("UTC").
		SetInviteCode("rule02").
		SetTelegramGroupID(4).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.Create(ctx, shopRow.ID, "Lan off tomorrow", map[string]any{
		"kind":  "day_off",
		"scope": "afternoon",
	}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.ShopID != shopRow.ID || got.Description != "Lan off tomorrow" || !got.IsActive {
		t.Fatalf("got %+v", got)
	}
	if got.RuleJSON["kind"] != "day_off" || got.Weight != 5 {
		t.Fatalf("json/weight = %+v / %v", got.RuleJSON, got.Weight)
	}

	listed, err := repo.ListByShop(ctx, shopRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != got.ID {
		t.Fatalf("listed = %+v", listed)
	}
}
