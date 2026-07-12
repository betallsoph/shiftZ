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
