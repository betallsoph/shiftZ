package planner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/enttest"
	"github.com/betallsoph/shiftz/internal/solver"
	"github.com/betallsoph/shiftz/internal/store"
	_ "github.com/mattn/go-sqlite3"
)

func TestShiftDateMapping(t *testing.T) {
	loc := time.UTC
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, loc) // Monday

	mondayShift := &store.Shift{ID: uuid.New(), Weekday: 1, StartTime: "09:00", EndTime: "17:00", MinStaff: 1, MaxStaff: 1}
	sundayShift := &store.Shift{ID: uuid.New(), Weekday: 0, StartTime: "09:00", EndTime: "17:00", MinStaff: 1, MaxStaff: 1}

	emp := &store.Employee{ID: uuid.New(), DisplayName: "Anna", MaxHoursPerWeek: 40}
	avail := []*store.Availability{{
		EmployeeID: emp.ID,
		Slots: []store.AvailabilitySlot{{
			Start:      weekStart,
			End:        weekStart.AddDate(0, 0, 7),
			Preference: 1,
		}},
	}}

	problemMon, occMon, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{mondayShift}, avail)
	if err != nil {
		t.Fatal(err)
	}
	if len(problemMon.Shifts) != 1 {
		t.Fatalf("monday shifts = %d", len(problemMon.Shifts))
	}
	if !occMon[problemMon.Shifts[0].ID].Date.Equal(weekStart) {
		t.Fatalf("monday date = %v, want %v", occMon[problemMon.Shifts[0].ID].Date, weekStart)
	}

	problemSun, occSun, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{sundayShift}, avail)
	if err != nil {
		t.Fatal(err)
	}
	wantSunday := weekStart.AddDate(0, 0, 6)
	if !occSun[problemSun.Shifts[0].ID].Date.Equal(wantSunday) {
		t.Fatalf("sunday date = %v, want %v", occSun[problemSun.Shifts[0].ID].Date, wantSunday)
	}
}

func TestOvernightShift(t *testing.T) {
	loc := time.UTC
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)
	shift := &store.Shift{ID: uuid.New(), Weekday: 1, StartTime: "22:00", EndTime: "06:00", MinStaff: 1, MaxStaff: 1}
	emp := &store.Employee{ID: uuid.New(), DisplayName: "Anna", MaxHoursPerWeek: 40}
	avail := []*store.Availability{{
		EmployeeID: emp.ID,
		Slots: []store.AvailabilitySlot{{
			Start:      weekStart,
			End:        weekStart.AddDate(0, 0, 2),
			Preference: 1,
		}},
	}}

	problem, _, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{shift}, avail)
	if err != nil {
		t.Fatal(err)
	}
	sh := problem.Shifts[0]
	if sh.Hours() != 8 {
		t.Fatalf("overnight hours = %v, want 8", sh.Hours())
	}
}

func TestAvailabilityMapping(t *testing.T) {
	loc := time.UTC
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, loc)
	shift := &store.Shift{ID: uuid.New(), Weekday: 1, StartTime: "09:00", EndTime: "17:00", MinStaff: 1, MaxStaff: 1}
	emp := &store.Employee{ID: uuid.New(), DisplayName: "Anna", MaxHoursPerWeek: 40}

	t.Run("missing availability is unavailable", func(t *testing.T) {
		problem, _, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{shift}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if problem.Preference(solver.EmployeeID(emp.ID.String()), solver.ShiftID(shift.ID.String())) != solver.Unavailable {
			t.Fatal("expected unavailable")
		}
	})

	t.Run("covering available slot", func(t *testing.T) {
		avail := []*store.Availability{{
			EmployeeID: emp.ID,
			Slots: []store.AvailabilitySlot{{
				Start:      weekStart.Add(8 * time.Hour),
				End:        weekStart.Add(18 * time.Hour),
				Preference: 1,
			}},
		}}
		problem, _, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{shift}, avail)
		if err != nil {
			t.Fatal(err)
		}
		if problem.Preference(solver.EmployeeID(emp.ID.String()), solver.ShiftID(shift.ID.String())) != solver.Available {
			t.Fatal("expected available")
		}
	})

	t.Run("unavailable overlap overrides available", func(t *testing.T) {
		avail := []*store.Availability{{
			EmployeeID: emp.ID,
			Slots: []store.AvailabilitySlot{
				{
					Start:      weekStart.Add(8 * time.Hour),
					End:        weekStart.Add(18 * time.Hour),
					Preference: 1,
				},
				{
					Start:      weekStart.Add(10 * time.Hour),
					End:        weekStart.Add(12 * time.Hour),
					Preference: 0,
				},
			},
		}}
		problem, _, err := BuildProblem(weekStart, loc, []*store.Employee{emp}, []*store.Shift{shift}, avail)
		if err != nil {
			t.Fatal(err)
		}
		if problem.Preference(solver.EmployeeID(emp.ID.String()), solver.ShiftID(shift.ID.String())) != solver.Unavailable {
			t.Fatal("expected unavailable override")
		}
	})
}

func TestMapRules(t *testing.T) {
	annaID := uuid.New()
	bobID := uuid.New()
	employees := []*store.Employee{
		{ID: annaID, DisplayName: "Anna"},
		{ID: bobID, DisplayName: "bob"},
	}

	rules := []*store.Rule{
		{
			ID:       uuid.New(),
			IsActive: true,
			Weight:   2,
			RuleJSON: map[string]any{
				"kind": "avoid_pair",
				"params": map[string]any{
					"a": "Anna",
					"b": "Bob",
				},
			},
		},
		{
			ID:       uuid.New(),
			IsActive: true,
			RuleJSON: map[string]any{
				"kind": "day_off",
				"params": map[string]any{
					"employee": "Anna",
					"weekday":  float64(0),
				},
				"weight": float64(3),
			},
		},
		{
			ID:       uuid.New(),
			IsActive: true,
			RuleJSON: map[string]any{"kind": "custom"},
		},
	}

	penalty, warnings := mapRules(employees, rules)
	if len(penalty) != 2 {
		t.Fatalf("penalty rules = %d, want 2", len(penalty))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want 1 unknown kind", warnings)
	}
}

func TestGenerateWeekIntegration(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	st := store.NewWithClient(client)
	svc := New(st)

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	weekStart := store.WeekStart(time.Date(2026, 7, 8, 0, 0, 0, 0, loc), loc)

	shop, err := client.Shop.Create().
		SetName("Planner Cafe").
		SetTimezone("Asia/Ho_Chi_Minh").
		SetInviteCode("plan01").
		SetTelegramGroupID(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	emps := make([]*store.Employee, 0, 3)
	for i, name := range []string{"Anna", "Bob", "Chi"} {
		row, err := client.Employee.Create().
			SetShopID(shop.ID).
			SetTelegramUserID(int64(100 + i)).
			SetDisplayName(name).
			SetMaxHoursPerWeek(40).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		emps = append(emps, &store.Employee{ID: row.ID, DisplayName: name, MaxHoursPerWeek: 40})
	}

	for weekday := 1; weekday <= 5; weekday++ {
		_, err := client.Shift.Create().
			SetShopID(shop.ID).
			SetName("morning").
			SetWeekday(weekday).
			SetStartTime("08:00").
			SetEndTime("14:00").
			SetMinStaff(1).
			SetMaxStaff(2).
			Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, emp := range emps {
		slots := make([]store.AvailabilitySlot, 0, 7)
		for day := 0; day < 7; day++ {
			date := weekStart.AddDate(0, 0, day)
			slots = append(slots, store.AvailabilitySlot{
				Start:      time.Date(date.Year(), date.Month(), date.Day(), 7, 0, 0, 0, loc),
				End:        time.Date(date.Year(), date.Month(), date.Day(), 21, 0, 0, 0, loc),
				Preference: 1,
			})
		}
		if err := st.Availability.ReplaceWeek(ctx, shop.ID, emp.ID, weekStart, slots, "test"); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.GenerateWeek(ctx, shop.ID, weekStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) < 2 {
		t.Fatalf("candidates = %d, want at least 2", len(result.Candidates))
	}

	persisted, err := st.Schedules.ListByShopWeek(ctx, shop.ID, weekStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted) != len(result.Candidates) {
		t.Fatalf("persisted = %d, result = %d", len(persisted), len(result.Candidates))
	}
	for _, sched := range persisted {
		if len(sched.Assignments) == 0 {
			t.Fatalf("schedule %s has no assignments", sched.VariantLabel)
		}
	}

	if _, err := svc.GenerateWeek(ctx, shop.ID, weekStart); err != ErrSchedulesExist {
		t.Fatalf("second generate: got %v, want ErrSchedulesExist", err)
	}
}

func newTestClient(t *testing.T) *ent.Client {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	return enttest.Open(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
}
