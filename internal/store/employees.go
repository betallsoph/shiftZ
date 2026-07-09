package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmployeeRepo covers employees, who join a shop via its invite code and are
// identified by their Telegram user id.
type EmployeeRepo struct {
	pool *pgxpool.Pool
}

// Join registers (or re-activates) an employee on the shop matching the
// invite code and returns the employee row.
func (r *EmployeeRepo) Join(ctx context.Context, inviteCode string, telegramUserID int64, displayName string) (*Employee, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO employees (shop_id, telegram_user_id, display_name)
		SELECT s.id, $2, $3 FROM shops s WHERE s.invite_code = $1
		ON CONFLICT (shop_id, telegram_user_id)
		DO UPDATE SET display_name = EXCLUDED.display_name, active = TRUE
		RETURNING id, shop_id, telegram_user_id, display_name, max_hours_per_week, active, created_at`,
		inviteCode, telegramUserID, displayName)
	e, err := scanEmployee(row)
	if errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("store: no shop with invite code %q: %w", inviteCode, ErrNotFound)
	}
	return e, err
}

// ByTelegramID returns the employee linked to a Telegram account.
// NOTE: assumes one shop per Telegram user for now; multi-shop membership
// would return a slice instead.
func (r *EmployeeRepo) ByTelegramID(ctx context.Context, telegramUserID int64) (*Employee, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, shop_id, telegram_user_id, display_name, max_hours_per_week, active, created_at
		FROM employees WHERE telegram_user_id = $1 AND active LIMIT 1`, telegramUserID)
	return scanEmployee(row)
}

// ActiveTelegramIDs lists the Telegram ids of all active employees across
// all shops; the scheduler uses it to fan out weekly reminders.
func (r *EmployeeRepo) ActiveTelegramIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT telegram_user_id FROM employees WHERE active`)
	if err != nil {
		return nil, fmt.Errorf("store: active telegram ids: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan telegram id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanEmployee(row pgx.Row) (*Employee, error) {
	var e Employee
	err := row.Scan(&e.ID, &e.ShopID, &e.TelegramUserID, &e.DisplayName, &e.MaxHoursPerWeek, &e.Active, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan employee: %w", err)
	}
	return &e, nil
}
