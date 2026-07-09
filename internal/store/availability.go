package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AvailabilityRepo stores the structured availability slots parsed from
// employees' free-text messages.
type AvailabilityRepo struct {
	pool *pgxpool.Pool
}

// ReplaceWeek atomically replaces an employee's availability for the week
// starting at weekStart with the given slots. rawText preserves the original
// message for auditing and re-parsing.
func (r *AvailabilityRepo) ReplaceWeek(ctx context.Context, shopID, employeeID int64, weekStart time.Time, slots []AvailabilitySlot, rawText string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM availability
		WHERE shop_id = $1 AND employee_id = $2 AND week_start = $3`,
		shopID, employeeID, weekStart); err != nil {
		return fmt.Errorf("store: clear availability: %w", err)
	}
	for _, slot := range slots {
		if _, err := tx.Exec(ctx, `
			INSERT INTO availability (shop_id, employee_id, week_start, starts_at, ends_at, preference, note, raw_text)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			shopID, employeeID, weekStart, slot.Start, slot.End, slot.Preference, slot.Note, rawText); err != nil {
			return fmt.Errorf("store: insert availability: %w", err)
		}
	}
	return tx.Commit(ctx)
}
