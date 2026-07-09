package store

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/schedulevote"
)

// VoteRepo records employee votes on schedule variants.
type VoteRepo struct {
	client *ent.Client
}

// Record stores a vote for a schedule variant. One vote per employee per
// voting round (shop + week), across variants: re-voting switches the vote
// to the new variant. The round's week_start is derived from the schedule,
// which must belong to shopID.
func (r *VoteRepo) Record(ctx context.Context, shopID, scheduleID, employeeID uuid.UUID) error {
	sched, err := r.client.Schedule.Get(ctx, scheduleID)
	if ent.IsNotFound(err) {
		return fmt.Errorf("store: schedule %s: %w", scheduleID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("store: fetch schedule for vote: %w", err)
	}
	if sched.ShopID != shopID {
		// Tenant check: a vote may only target a schedule of the voter's shop.
		return fmt.Errorf("store: schedule %s does not belong to shop %s: %w", scheduleID, shopID, ErrNotFound)
	}

	err = r.client.ScheduleVote.Create().
		SetShopID(shopID).
		SetScheduleID(scheduleID).
		SetEmployeeID(employeeID).
		SetWeekStart(sched.WeekStart).
		OnConflict(
			sql.ConflictColumns(
				schedulevote.FieldShopID,
				schedulevote.FieldEmployeeID,
				schedulevote.FieldWeekStart,
			),
		).
		Update(func(u *ent.ScheduleVoteUpsert) {
			u.UpdateScheduleID() // re-voting switches the chosen variant
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("store: record vote: %w", err)
	}
	return nil
}
