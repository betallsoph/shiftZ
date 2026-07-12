package store

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/availability"
	"github.com/betallsoph/shiftz/internal/ent/schema"
)

// AvailabilityRepo stores weekly availability submissions: the structured
// slots parsed from an employee's free-text message plus the original text.
type AvailabilityRepo struct {
	client *ent.Client
}

// ReplaceWeek stores an employee's availability for the week starting at
// weekStart. One row per (shop, employee, week): resubmitting upserts,
// replacing the slots and raw message wholesale.
func (r *AvailabilityRepo) ReplaceWeek(ctx context.Context, shopID, employeeID uuid.UUID, weekStart time.Time, slots []AvailabilitySlot, rawMessage string) error {
	stored := make([]schema.AvailabilitySlot, len(slots))
	for i, s := range slots {
		stored[i] = schema.AvailabilitySlot{
			Start:      s.Start,
			End:        s.End,
			Preference: s.Preference,
			Note:       s.Note,
		}
	}
	err := r.client.Availability.Create().
		SetShopID(shopID).
		SetEmployeeID(employeeID).
		SetWeekStart(weekStart).
		SetSlots(stored).
		SetRawMessage(rawMessage).
		OnConflict(
			sql.ConflictColumns(
				availability.FieldShopID,
				availability.FieldEmployeeID,
				availability.FieldWeekStart,
			),
		).
		Update(func(u *ent.AvailabilityUpsert) {
			u.UpdateSlots()
			u.UpdateRawMessage()
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("store: replace availability week: %w", err)
	}
	return nil
}

// ListByShopWeek returns all availability submissions for a shop and week,
// ordered by employee id.
func (r *AvailabilityRepo) ListByShopWeek(ctx context.Context, shopID uuid.UUID, weekStart time.Time) ([]*Availability, error) {
	rows, err := r.client.Availability.Query().
		Where(availability.ShopID(shopID), availability.WeekStart(weekStart)).
		Order(availability.ByEmployeeID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list availability by shop week: %w", err)
	}
	out := make([]*Availability, len(rows))
	for i, row := range rows {
		out[i] = availabilityFromEnt(row)
	}
	return out, nil
}
