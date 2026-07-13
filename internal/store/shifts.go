package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shift"
)

// ShiftRepo covers weekly shift templates for a shop.
type ShiftRepo struct {
	client *ent.Client
}

// ListByShop returns all shift templates for a shop, ordered by weekday,
// start time, then name.
func (r *ShiftRepo) ListByShop(ctx context.Context, shopID uuid.UUID) ([]*Shift, error) {
	rows, err := r.client.Shift.Query().
		Where(shift.ShopID(shopID)).
		Order(shift.ByWeekday(), shift.ByStartTime(), shift.ByName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list shifts: %w", err)
	}
	out := make([]*Shift, len(rows))
	for i, row := range rows {
		out[i] = shiftFromEnt(row)
	}
	return out, nil
}

// CreateDefaultsForShop inserts morning/evening shift templates for every weekday.
func (r *ShiftRepo) CreateDefaultsForShop(ctx context.Context, shopID uuid.UUID) error {
	for weekday := 0; weekday < 7; weekday++ {
		for _, tpl := range []struct {
			name, from, to string
		}{
			{"morning", "08:00", "14:00"},
			{"evening", "14:00", "20:00"},
		} {
			_, err := r.client.Shift.Create().
				SetShopID(shopID).
				SetName(tpl.name).
				SetWeekday(weekday).
				SetStartTime(tpl.from).
				SetEndTime(tpl.to).
				SetMinStaff(1).
				SetMaxStaff(2).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("store: create default shift: %w", err)
			}
		}
	}
	return nil
}
