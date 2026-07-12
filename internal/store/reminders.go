package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/reminderdelivery"
)

const (
	ReminderKindAvailabilityReminder = "availability_reminder"
	ReminderKindAvailabilityNag      = "availability_nag"

	ReminderStatusPending = "pending"
	ReminderStatusSent    = "sent"
	ReminderStatusFailed  = "failed"
)

// ReminderDeliveryRepo persists reminder/nag delivery logs for idempotency.
type ReminderDeliveryRepo struct {
	client *ent.Client
}

// CreatePending inserts a pending delivery row. Returns created=false on dedupe conflict.
func (r *ReminderDeliveryRepo) CreatePending(
	ctx context.Context,
	shopID, employeeID uuid.UUID,
	weekStart time.Time,
	kind string,
) (bool, error) {
	_, err := r.client.ReminderDelivery.Create().
		SetShopID(shopID).
		SetEmployeeID(employeeID).
		SetWeekStart(weekStart).
		SetKind(kind).
		SetStatus(reminderdelivery.StatusPending).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: create pending reminder: %w", err)
	}
	return true, nil
}

// ListPending returns pending deliveries oldest first.
func (r *ReminderDeliveryRepo) ListPending(ctx context.Context, limit int) ([]*ReminderDelivery, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.client.ReminderDelivery.Query().
		Where(reminderdelivery.StatusEQ(reminderdelivery.StatusPending)).
		Order(reminderdelivery.ByCreatedAt()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list pending reminders: %w", err)
	}
	out := make([]*ReminderDelivery, len(rows))
	for i, row := range rows {
		out[i] = reminderDeliveryFromEnt(row)
	}
	return out, nil
}

// MarkSent marks a delivery as successfully sent.
func (r *ReminderDeliveryRepo) MarkSent(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.client.ReminderDelivery.UpdateOneID(id).
		SetStatus(reminderdelivery.StatusSent).
		SetSentAt(now).
		ClearLastError().
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: mark reminder sent: %w", err)
	}
	return nil
}

// MarkFailed records a failed send attempt.
func (r *ReminderDeliveryRepo) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastErr string) error {
	upd := r.client.ReminderDelivery.UpdateOneID(id).
		SetStatus(reminderdelivery.StatusFailed).
		SetAttempts(attempts)
	if lastErr != "" {
		upd = upd.SetLastError(lastErr)
	}
	_, err := upd.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("store: mark reminder failed: %w", err)
	}
	return nil
}

func reminderDeliveryFromEnt(m *ent.ReminderDelivery) *ReminderDelivery {
	d := &ReminderDelivery{
		ID:         m.ID,
		ShopID:     m.ShopID,
		EmployeeID: m.EmployeeID,
		WeekStart:  m.WeekStart,
		Kind:       m.Kind,
		Status:     string(m.Status),
		Attempts:   m.Attempts,
		CreatedAt:  m.CreatedAt,
	}
	if m.LastError != nil {
		d.LastError = *m.LastError
	}
	if m.SentAt != nil {
		t := *m.SentAt
		d.SentAt = &t
	}
	return d
}
