package store

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/employee"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

// EmployeeRepo covers employees, who join a shop via its invite code and are
// identified by their Telegram user id.
type EmployeeRepo struct {
	client *ent.Client
}

// Join registers (or re-activates) an employee on the shop matching the
// invite code and returns the employee row.
func (r *EmployeeRepo) Join(ctx context.Context, inviteCode string, telegramUserID int64, displayName string) (*Employee, error) {
	sh, err := r.client.Shop.Query().Where(shop.InviteCode(inviteCode)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("store: no shop with invite code %q: %w", inviteCode, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: shop by invite code: %w", err)
	}

	id, err := r.client.Employee.Create().
		SetShopID(sh.ID).
		SetTelegramUserID(telegramUserID).
		SetDisplayName(displayName).
		OnConflict(
			sql.ConflictColumns(employee.FieldShopID, employee.FieldTelegramUserID),
		).
		Update(func(u *ent.EmployeeUpsert) {
			u.UpdateDisplayName()
			u.SetIsActive(true)
		}).
		ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: join shop: %w", err)
	}
	row, err := r.client.Employee.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("store: fetch joined employee: %w", err)
	}
	return employeeFromEnt(row), nil
}

// ByTelegramID returns the employee linked to a Telegram account. This is
// the webhook lookup: the bot resolves the sender before knowing the shop,
// hence the standalone telegram_user_id index.
// NOTE: assumes one shop per Telegram user for now; multi-shop membership
// would return a slice instead.
func (r *EmployeeRepo) ByTelegramID(ctx context.Context, telegramUserID int64) (*Employee, error) {
	row, err := r.client.Employee.Query().
		Where(employee.TelegramUserID(telegramUserID), employee.IsActive(true)).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: employee by telegram id: %w", err)
	}
	return employeeFromEnt(row), nil
}

// ActiveTelegramIDs lists the Telegram ids of all active employees across
// all shops; the scheduler uses it to fan out weekly reminders.
func (r *EmployeeRepo) ActiveTelegramIDs(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := r.client.Employee.Query().
		Where(employee.IsActive(true)).
		Select(employee.FieldTelegramUserID).
		Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("store: active telegram ids: %w", err)
	}
	return ids, nil
}
