// Command seed populates a development database with a demo shop, a few
// employees and a week of shifts, using the ent-backed store. Run it after
// applying migrations; it is idempotent enough for repeated local use (each
// run creates a fresh shop with a new invite code).
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

	st, err := store.New(ctx, cfg.DatabaseURL)
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
	}{
		{1001, "Anna"},
		{1002, "Bob"},
		{1003, "Chi"},
		{1004, "Dave"},
	}
	for _, s := range staff {
		emp, err := st.Employees.Join(ctx, shop.InviteCode, s.tgID, s.name)
		if err != nil {
			return err
		}
		log.Info("created employee", "id", emp.ID, "name", emp.DisplayName)
	}

	// A week of morning/evening shifts starting next Monday. Shift rows are
	// created through the ent client directly; a ShiftRepo grows once the
	// bot needs to read them.
	monday := nextMonday(time.Now().UTC())
	for day := 0; day < 7; day++ {
		date := monday.AddDate(0, 0, day)
		for _, span := range []struct{ from, to int }{{8, 14}, {14, 20}} {
			_, err := st.Client.Shift.Create().
				SetShopID(int(shop.ID)).
				SetRole("floor").
				SetStartsAt(date.Add(time.Duration(span.from) * time.Hour)).
				SetEndsAt(date.Add(time.Duration(span.to) * time.Hour)).
				SetMinStaff(1).
				SetMaxStaff(2).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("seed shift: %w", err)
			}
		}
	}
	log.Info("created shifts", "week_start", monday.Format("2006-01-02"), "count", 14)
	fmt.Printf("\nSeeded. Join the demo shop in Telegram with:\n  /start %s\n", shop.InviteCode)
	return nil
}

func nextMonday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	days := (int(time.Monday) - int(t.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	return t.AddDate(0, 0, days)
}
