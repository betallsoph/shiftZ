package store

import (
	"context"
	"fmt"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/employee"
	"github.com/betallsoph/shiftz/internal/ent/shop"
)

// EmployeeRepo covers employees, who join a shop via its invite code and are
// identified by their Telegram user id.
type EmployeeRepo struct {
	client *ent.Client
}

// Join registers a new employee or updates an active employee on the shop
// matching the invite code. Inactive employees are not re-activated here.
func (r *EmployeeRepo) Join(ctx context.Context, inviteCode string, telegramUserID int64, displayName string) (*Employee, error) {
	sh, err := r.client.Shop.Query().Where(shop.InviteCode(inviteCode)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("store: no shop with invite code %q: %w", inviteCode, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: shop by invite code: %w", err)
	}

	existing, err := r.client.Employee.Query().
		Where(employee.ShopID(sh.ID), employee.TelegramUserID(telegramUserID)).
		Only(ctx)
	if err == nil {
		if !existing.IsActive {
			return nil, ErrEmployeeInactive
		}
		updated, err := r.client.Employee.UpdateOneID(existing.ID).
			SetDisplayName(strings.TrimSpace(displayName)).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("store: update joined employee: %w", err)
		}
		return employeeFromEnt(updated), nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("store: lookup employee for join: %w", err)
	}

	row, err := r.client.Employee.Create().
		SetShopID(sh.ID).
		SetTelegramUserID(telegramUserID).
		SetDisplayName(strings.TrimSpace(displayName)).
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: join shop: %w", err)
	}
	return employeeFromEnt(row), nil
}

// ByTelegramID returns the active employee linked to a Telegram account.
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

// ByID returns an employee by primary key.
func (r *EmployeeRepo) ByID(ctx context.Context, id uuid.UUID) (*Employee, error) {
	row, err := r.client.Employee.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: employee by id: %w", err)
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

// ListActiveByShop returns active employees for a shop, ordered by display name.
func (r *EmployeeRepo) ListActiveByShop(ctx context.Context, shopID uuid.UUID) ([]*Employee, error) {
	rows, err := r.client.Employee.Query().
		Where(employee.ShopID(shopID), employee.IsActive(true)).
		Order(employee.ByDisplayName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list active employees: %w", err)
	}
	out := make([]*Employee, len(rows))
	for i, row := range rows {
		out[i] = employeeFromEnt(row)
	}
	return out, nil
}

// ListAllByShop returns every employee for a shop; active first, then name.
func (r *EmployeeRepo) ListAllByShop(ctx context.Context, shopID uuid.UUID) ([]*Employee, error) {
	rows, err := r.client.Employee.Query().
		Where(employee.ShopID(shopID)).
		Order(employee.ByIsActive(sql.OrderDesc()), employee.ByDisplayName()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list all employees: %w", err)
	}
	out := make([]*Employee, len(rows))
	for i, row := range rows {
		out[i] = employeeFromEnt(row)
	}
	return out, nil
}

// Update changes owner-editable employee profile fields scoped to one shop.
func (r *EmployeeRepo) Update(ctx context.Context, shopID, employeeID uuid.UUID, input UpdateEmployeeInput) (*Employee, error) {
	if err := ValidateUpdateEmployeeInput(input); err != nil {
		return nil, err
	}
	row, err := r.client.Employee.Query().
		Where(employee.And(employee.ShopID(shopID), employee.ID(employeeID))).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: find employee: %w", err)
	}
	updated, err := r.client.Employee.UpdateOneID(row.ID).
		SetDisplayName(strings.TrimSpace(input.DisplayName)).
		SetRole(strings.TrimSpace(input.Role)).
		SetMaxHoursPerWeek(input.MaxHoursPerWeek).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: update employee: %w", err)
	}
	return employeeFromEnt(updated), nil
}

// SetActive enables or disables an employee scoped to one shop.
func (r *EmployeeRepo) SetActive(ctx context.Context, shopID, employeeID uuid.UUID, active bool) (*Employee, error) {
	row, err := r.client.Employee.Query().
		Where(employee.And(employee.ShopID(shopID), employee.ID(employeeID))).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: find employee: %w", err)
	}
	updated, err := r.client.Employee.UpdateOneID(row.ID).SetIsActive(active).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: set employee active: %w", err)
	}
	return employeeFromEnt(updated), nil
}
