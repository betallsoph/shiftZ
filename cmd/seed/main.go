// Command seed populates a development database so the solver can run
// locally: one demo shop, five employees, a week of shift templates and an
// availability submission per employee. Run it after applying migrations;
// each run creates a fresh shop with a new invite code.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/betallsoph/shiftz/internal/config"
	"github.com/betallsoph/shiftz/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.RequireDatabase(); err != nil {
		return err
	}
	ctx := context.Background()

	st, err := store.New(ctx, cfg.DatabaseURL, cfg.EntDebug)
	if err != nil {
		return err
	}
	defer st.Close()

	shop, err := st.Shops.Create(ctx, "Demo Cafe", "Asia/Ho_Chi_Minh", 0)
	if err != nil {
		return err
	}
	log.Info("created shop", "id", shop.ID, "invite_code", shop.InviteCode)

	// Fake Telegram ids; real employees enroll through /start <invite-code>.
	staff := []struct {
		tgID int64
		name string
		role string
	}{
		{1001, "Anna", "barista"},
		{1002, "Bob", "barista"},
		{1003, "Chi", "kitchen"},
		{1004, "Dave", "floor"},
		{1005, "Eve", "floor"},
	}
	employees := make([]*store.Employee, 0, len(staff))
	for _, s := range staff {
		emp, err := st.Employees.Join(ctx, shop.InviteCode, s.tgID, s.name)
		if err != nil {
			return err
		}
		// Role isn't part of the join flow (owners set it later); set it
		// directly through the ent client.
		if _, err := st.Client.Employee.UpdateOneID(emp.ID).SetRole(s.role).Save(ctx); err != nil {
			return fmt.Errorf("seed role: %w", err)
		}
		employees = append(employees, emp)
		log.Info("created employee", "id", emp.ID, "name", emp.DisplayName, "role", s.role)
	}

	// Weekly shift templates: morning and evening, every day of the week.
	// Shift templates are created through the ent client directly; a
	// ShiftRepo grows once the bot needs to read them.
	for weekday := 0; weekday < 7; weekday++ {
		for _, tpl := range []struct {
			name, from, to string
		}{
			{"morning", "08:00", "14:00"},
			{"evening", "14:00", "20:00"},
		} {
			_, err := st.Client.Shift.Create().
				SetShopID(shop.ID).
				SetName(tpl.name).
				SetWeekday(weekday).
				SetStartTime(tpl.from).
				SetEndTime(tpl.to).
				SetMinStaff(1).
				SetMaxStaff(2).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("seed shift: %w", err)
			}
		}
	}
	log.Info("created shift templates", "count", 14)

	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		return fmt.Errorf("seed timezone: %w", err)
	}
	monday := store.WeekStart(time.Now().In(loc).AddDate(0, 0, 7), loc)

	// One availability submission per employee for next week: everyone can
	// work every day 08:00-20:00, so the demo problem is trivially feasible.
	for _, emp := range employees {
		slots := make([]store.AvailabilitySlot, 0, 7)
		for day := 0; day < 7; day++ {
			date := monday.AddDate(0, 0, day)
			slots = append(slots, store.AvailabilitySlot{
				Start:      time.Date(date.Year(), date.Month(), date.Day(), 8, 0, 0, 0, loc),
				End:        time.Date(date.Year(), date.Month(), date.Day(), 20, 0, 0, 0, loc),
				Preference: 1,
			})
		}
		raw := fmt.Sprintf("(seed) %s: any day next week works", emp.DisplayName)
		if err := st.Availability.ReplaceWeek(ctx, shop.ID, emp.ID, monday, slots, raw); err != nil {
			return err
		}
	}
	log.Info("created availabilities", "week_start", monday.Format("2006-01-02"), "employees", len(employees))

	fmt.Printf("\nSeeded. Shop ID: %s\nJoin the demo shop in Telegram with:\n  /start %s\n", shop.ID, shop.InviteCode)
	return nil
}
