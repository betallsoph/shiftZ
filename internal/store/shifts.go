package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/shift"
)

// ShiftRepo covers weekly shift templates for a shop.
type ShiftRepo struct {
	client *ent.Client
}

// ShiftRepoFromClient returns a ShiftRepo backed by an ent client (e.g. in a transaction).
func ShiftRepoFromClient(client *ent.Client) *ShiftRepo {
	return &ShiftRepo{client: client}
}

// ListByShop returns active shift templates for a shop, ordered by weekday,
// start time, then name.
func (r *ShiftRepo) ListByShop(ctx context.Context, shopID uuid.UUID) ([]*Shift, error) {
	rows, err := r.client.Shift.Query().
		Where(shift.ShopID(shopID), shift.IsActive(true)).
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

// ListAllByShop returns every shift template for a shop, including inactive.
func (r *ShiftRepo) ListAllByShop(ctx context.Context, shopID uuid.UUID) ([]*Shift, error) {
	rows, err := r.client.Shift.Query().
		Where(shift.ShopID(shopID)).
		Order(shift.ByWeekday(), shift.ByStartTime(), shift.ByName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list all shifts: %w", err)
	}
	out := make([]*Shift, len(rows))
	for i, row := range rows {
		out[i] = shiftFromEnt(row)
	}
	return out, nil
}

// Create inserts a new active shift template for a shop.
func (r *ShiftRepo) Create(ctx context.Context, shopID uuid.UUID, input CreateShiftInput) (*Shift, error) {
	if err := ValidateCreateShiftInput(input); err != nil {
		return nil, err
	}
	row, err := r.client.Shift.Create().
		SetShopID(shopID).
		SetName(strings.TrimSpace(input.Name)).
		SetWeekday(input.Weekday).
		SetStartTime(input.StartTime).
		SetEndTime(input.EndTime).
		SetMinStaff(input.MinStaff).
		SetMaxStaff(input.MaxStaff).
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: create shift: %w", err)
	}
	return shiftFromEnt(row), nil
}

// SetActive enables or disables a shift template scoped to one shop.
func (r *ShiftRepo) SetActive(ctx context.Context, shopID, shiftID uuid.UUID, active bool) (*Shift, error) {
	row, err := r.client.Shift.Query().
		Where(shift.And(shift.ShopID(shopID), shift.ID(shiftID))).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: find shift: %w", err)
	}
	updated, err := r.client.Shift.UpdateOneID(row.ID).SetIsActive(active).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: set shift active: %w", err)
	}
	return shiftFromEnt(updated), nil
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
				SetIsActive(true).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("store: create default shift: %w", err)
			}
		}
	}
	return nil
}
