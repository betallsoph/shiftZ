package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VoteRepo records employee votes on schedule candidates.
type VoteRepo struct {
	pool *pgxpool.Pool
}

// Record stores a vote; re-voting for the same schedule is a no-op.
func (r *VoteRepo) Record(ctx context.Context, shopID, scheduleID, employeeID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO schedule_votes (shop_id, schedule_id, employee_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (schedule_id, employee_id) DO NOTHING`,
		shopID, scheduleID, employeeID)
	if err != nil {
		return fmt.Errorf("store: record vote: %w", err)
	}
	return nil
}
