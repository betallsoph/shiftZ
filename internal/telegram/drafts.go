package telegram

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

// MemoryAvailabilityDraftStore stores drafts in memory with TTL expiry.
type MemoryAvailabilityDraftStore struct {
	ttl    time.Duration
	mu     sync.Mutex
	drafts map[uuid.UUID]store.AvailabilityDraft
}

// NewMemoryAvailabilityDraftStore returns an in-memory draft store.
func NewMemoryAvailabilityDraftStore(ttl time.Duration) *MemoryAvailabilityDraftStore {
	if ttl <= 0 {
		ttl = store.DefaultAvailabilityDraftTTL
	}
	return &MemoryAvailabilityDraftStore{
		ttl:    ttl,
		drafts: make(map[uuid.UUID]store.AvailabilityDraft),
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
