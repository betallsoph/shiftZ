package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/availabilitydraft"
	"github.com/betallsoph/shiftz/internal/ent/schema"
)

// DefaultAvailabilityDraftTTL is how long a Telegram confirmation draft lives.
const DefaultAvailabilityDraftTTL = 30 * time.Minute

// AvailabilityDraftRepo stores pending availability confirmations.
type AvailabilityDraftRepo struct {
	client *ent.Client
	ttl    time.Duration
}

// Create stores a draft and returns its ID.
func (r *AvailabilityDraftRepo) Create(ctx context.Context, draft AvailabilityDraft) (uuid.UUID, error) {
	now := time.Now()
	if draft.ID == uuid.Nil {
		draft.ID = uuid.New()
	}
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = now
	}
	if draft.ExpiresAt.IsZero() {
		draft.ExpiresAt = draft.CreatedAt.Add(r.ttl)
	}

	stored := make([]schema.AvailabilitySlot, len(draft.Slots))
	for i, s := range draft.Slots {
		stored[i] = schema.AvailabilitySlot{
			Start:      s.Start,
			End:        s.End,
			Preference: s.Preference,
			Note:       s.Note,
		}
	}

	row, err := r.client.AvailabilityDraft.Create().
		SetID(draft.ID).
		SetShopID(draft.ShopID).
		SetEmployeeID(draft.EmployeeID).
		SetTelegramUserID(draft.TelegramUserID).
		SetChatID(draft.ChatID).
		SetWeekStart(draft.WeekStart).
		SetTimezone(draft.Timezone).
		SetSlots(stored).
		SetRawMessage(draft.RawMessage).
		SetCreatedAt(draft.CreatedAt).
		SetExpiresAt(draft.ExpiresAt).
		Save(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("store: create availability draft: %w", err)
	}
	return row.ID, nil
}

// Get returns a draft if it exists and has not expired.
func (r *AvailabilityDraftRepo) Get(ctx context.Context, id uuid.UUID) (*AvailabilityDraft, bool, error) {
	row, err := r.client.AvailabilityDraft.Get(ctx, id)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: get availability draft: %w", err)
	}
	if !row.ExpiresAt.After(time.Now()) {
		_ = r.client.AvailabilityDraft.DeleteOneID(id).Exec(ctx)
		return nil, false, nil
	}
	return availabilityDraftFromEnt(row), true, nil
}

// Delete removes a draft. Missing rows are ignored.
func (r *AvailabilityDraftRepo) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.client.AvailabilityDraft.DeleteOneID(id).Exec(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: delete availability draft: %w", err)
	}
	return nil
}

// DeleteExpired removes drafts whose expiry time has passed.
func (r *AvailabilityDraftRepo) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	n, err := r.client.AvailabilityDraft.Delete().
		Where(availabilitydraft.ExpiresAtLTE(now)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired availability drafts: %w", err)
	}
	return n, nil
}
