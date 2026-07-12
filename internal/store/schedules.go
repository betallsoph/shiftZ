package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/schedule"
)

// NewScheduleAssignment is one row to insert under a schedule candidate.
type NewScheduleAssignment struct {
	ShiftID    uuid.UUID
	EmployeeID uuid.UUID
	Date       time.Time
}

// ScheduleRepo persists generated schedule candidates and their assignments.
type ScheduleRepo struct {
	client *ent.Client
}

// CreateCandidate inserts a draft schedule variant for a shop and week.
func (r *ScheduleRepo) CreateCandidate(
	ctx context.Context,
	shopID uuid.UUID,
	weekStart time.Time,
	variantLabel string,
	score float64,
) (*Schedule, error) {
	row, err := r.client.Schedule.Create().
		SetShopID(shopID).
		SetWeekStart(weekStart).
		SetVariantLabel(variantLabel).
		SetScore(score).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: create schedule candidate: %w", err)
	}
	return scheduleFromEnt(row), nil
}

// AddAssignments bulk-inserts assignments under a shop-scoped schedule.
func (r *ScheduleRepo) AddAssignments(
	ctx context.Context,
	shopID uuid.UUID,
	scheduleID uuid.UUID,
	assignments []NewScheduleAssignment,
) error {
	if len(assignments) == 0 {
		return nil
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("store: add assignments: begin tx: %w", err)
	}
	defer rollbackTx(tx)

	if _, err := tx.Schedule.Query().
		Where(schedule.ID(scheduleID), schedule.ShopID(shopID)).
		Only(ctx); err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: add assignments: verify schedule: %w", err)
	}

	builders := make([]*ent.ScheduleAssignmentCreate, len(assignments))
	for i, a := range assignments {
		builders[i] = tx.ScheduleAssignment.Create().
			SetShopID(shopID).
			SetScheduleID(scheduleID).
			SetShiftID(a.ShiftID).
			SetEmployeeID(a.EmployeeID).
			SetDate(a.Date)
	}
	if _, err := tx.ScheduleAssignment.CreateBulk(builders...).Save(ctx); err != nil {
		return fmt.Errorf("store: add assignments: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: add assignments: commit: %w", err)
	}
	return nil
}

// ListByShopWeek returns schedule candidates for a shop and week with
// assignments and display fields eager-loaded.
func (r *ScheduleRepo) ListByShopWeek(
	ctx context.Context,
	shopID uuid.UUID,
	weekStart time.Time,
) ([]*Schedule, error) {
	rows, err := r.client.Schedule.Query().
		Where(schedule.ShopID(shopID), schedule.WeekStart(weekStart)).
		Order(schedule.ByVariantLabel(), schedule.ByCreatedAt()).
		WithAssignments(func(q *ent.ScheduleAssignmentQuery) {
			q.WithShift().WithEmployee()
		}).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list schedules by shop week: %w", err)
	}
	out := make([]*Schedule, len(rows))
	for i, row := range rows {
		out[i] = scheduleFromEnt(row)
	}
	return out, nil
}

// Approve marks one schedule approved and demotes sibling variants to draft.
func (r *ScheduleRepo) Approve(
	ctx context.Context,
	shopID uuid.UUID,
	scheduleID uuid.UUID,
) (*Schedule, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: approve schedule: begin tx: %w", err)
	}
	defer rollbackTx(tx)

	target, err := tx.Schedule.Query().
		Where(schedule.ID(scheduleID), schedule.ShopID(shopID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("store: approve schedule: load target: %w", err)
	}

	if _, err := tx.Schedule.Update().
		Where(
			schedule.ShopID(shopID),
			schedule.WeekStart(target.WeekStart),
			schedule.IDNEQ(scheduleID),
		).
		SetStatus(schedule.StatusDraft).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("store: approve schedule: demote siblings: %w", err)
	}

	approved, err := tx.Schedule.UpdateOneID(scheduleID).
		SetStatus(schedule.StatusApproved).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: approve schedule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: approve schedule: commit: %w", err)
	}
	return scheduleFromEnt(approved), nil
}

func rollbackTx(tx *ent.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
