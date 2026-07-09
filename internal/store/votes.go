package store

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/schedulevote"
)

// VoteRepo records employee votes on schedule candidates.
type VoteRepo struct {
	client *ent.Client
}

// Record stores a vote; re-voting for the same schedule is a no-op.
func (r *VoteRepo) Record(ctx context.Context, shopID, scheduleID, employeeID int64) error {
	err := r.client.ScheduleVote.Create().
		SetShopID(int(shopID)).
		SetScheduleID(int(scheduleID)).
		SetEmployeeID(int(employeeID)).
		OnConflict(
			sql.ConflictColumns(schedulevote.FieldScheduleID, schedulevote.FieldEmployeeID),
		).
		Ignore().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("store: record vote: %w", err)
	}
	return nil
}
