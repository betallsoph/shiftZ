package store

import (
	"context"
	"fmt"
	"time"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/availability"
)

// AvailabilityRepo stores the structured availability slots parsed from
// employees' free-text messages.
type AvailabilityRepo struct {
	client *ent.Client
}

// ReplaceWeek atomically replaces an employee's availability for the week
// starting at weekStart with the given slots. rawText preserves the original
// message for auditing and re-parsing.
func (r *AvailabilityRepo) ReplaceWeek(ctx context.Context, shopID, employeeID int64, weekStart time.Time, slots []AvailabilitySlot, rawText string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	_, err = tx.Availability.Delete().
		Where(
			availability.ShopID(int(shopID)),
			availability.EmployeeID(int(employeeID)),
			availability.WeekStart(weekStart),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("store: clear availability: %w", err)
	}

	builders := make([]*ent.AvailabilityCreate, len(slots))
	for i, slot := range slots {
		builders[i] = tx.Availability.Create().
			SetShopID(int(shopID)).
			SetEmployeeID(int(employeeID)).
			SetWeekStart(weekStart).
			SetStartsAt(slot.Start).
			SetEndsAt(slot.End).
			SetPreference(slot.Preference).
			SetNote(slot.Note).
			SetRawText(rawText)
	}
	if _, err := tx.Availability.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("store: insert availability: %w", err)
	}
	return tx.Commit()
}
