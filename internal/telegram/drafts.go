package telegram

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

// AvailabilityDraft is a pending availability confirmation.
type AvailabilityDraft struct {
	ID             uuid.UUID
	TelegramUserID int64
	ChatID         int64
	ShopID         uuid.UUID
	EmployeeID     uuid.UUID
	WeekStart      time.Time
	Timezone       string
	Slots          []store.AvailabilitySlot
	RawMessage     string
	CreatedAt      time.Time
}

// AvailabilityDraftStore holds pending availability confirmations.
type AvailabilityDraftStore interface {
	Create(ctx context.Context, draft AvailabilityDraft) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*AvailabilityDraft, bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// MemoryAvailabilityDraftStore stores drafts in memory with TTL expiry.
type MemoryAvailabilityDraftStore struct {
	ttl   time.Duration
	mu    sync.Mutex
	drafts map[uuid.UUID]AvailabilityDraft
}

// NewMemoryAvailabilityDraftStore returns an in-memory draft store.
func NewMemoryAvailabilityDraftStore(ttl time.Duration) *MemoryAvailabilityDraftStore {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &MemoryAvailabilityDraftStore{
		ttl:    ttl,
		drafts: make(map[uuid.UUID]AvailabilityDraft),
	}
}

// Create stores a draft and returns its ID.
func (s *MemoryAvailabilityDraftStore) Create(_ context.Context, draft AvailabilityDraft) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(time.Now())
	if draft.ID == uuid.Nil {
		draft.ID = uuid.New()
	}
	if draft.CreatedAt.IsZero() {
		draft.CreatedAt = time.Now()
	}
	s.drafts[draft.ID] = draft
	return draft.ID, nil
}

// Get returns a draft if it exists and has not expired.
func (s *MemoryAvailabilityDraftStore) Get(_ context.Context, id uuid.UUID) (*AvailabilityDraft, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.purgeExpiredLocked(now)
	draft, ok := s.drafts[id]
	if !ok {
		return nil, false, nil
	}
	if now.Sub(draft.CreatedAt) > s.ttl {
		delete(s.drafts, id)
		return nil, false, nil
	}
	copy := draft
	return &copy, true, nil
}

// Delete removes a draft.
func (s *MemoryAvailabilityDraftStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, id)
	return nil
}

func (s *MemoryAvailabilityDraftStore) purgeExpiredLocked(now time.Time) {
	for id, draft := range s.drafts {
		if now.Sub(draft.CreatedAt) > s.ttl {
			delete(s.drafts, id)
		}
	}
}
