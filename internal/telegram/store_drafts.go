package telegram

import (
	"context"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

// AvailabilityDraft is a pending availability confirmation.
type AvailabilityDraft = store.AvailabilityDraft

// AvailabilityDraftStore holds pending availability confirmations.
type AvailabilityDraftStore interface {
	Create(ctx context.Context, draft AvailabilityDraft) (uuid.UUID, error)
	Get(ctx context.Context, id uuid.UUID) (*AvailabilityDraft, bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// StoreAvailabilityDraftStore adapts store.AvailabilityDraftRepo for the bot.
type StoreAvailabilityDraftStore struct {
	repo *store.AvailabilityDraftRepo
}

// NewStoreAvailabilityDraftStore returns a DB-backed draft store.
func NewStoreAvailabilityDraftStore(repo *store.AvailabilityDraftRepo) *StoreAvailabilityDraftStore {
	return &StoreAvailabilityDraftStore{repo: repo}
}

func (s *StoreAvailabilityDraftStore) Create(ctx context.Context, draft AvailabilityDraft) (uuid.UUID, error) {
	return s.repo.Create(ctx, draft)
}

func (s *StoreAvailabilityDraftStore) Get(ctx context.Context, id uuid.UUID) (*AvailabilityDraft, bool, error) {
	return s.repo.Get(ctx, id)
}

func (s *StoreAvailabilityDraftStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
