package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/store"
)

// ProvisionService coordinates transactional shop creation for the admin portal.
type ProvisionService struct {
	client *ent.Client
}

// NewProvisionService wires provisioning on the ent client.
func NewProvisionService(st *store.Store) *ProvisionService {
	return &ProvisionService{client: st.Client}
}

// CreateShopWithAccount creates a shop, provisions dashboard access, and optional default shifts atomically.
func (p *ProvisionService) CreateShopWithAccount(ctx context.Context, name, timezone, username, plan string, createDefaultShifts bool) (*store.Shop, error) {
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("admin: invalid timezone %q: %w", timezone, err)
	}
	tx, err := p.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("admin: begin tx: %w", err)
	}
	rollback := func() {
		_ = tx.Rollback()
	}

	shops := store.ShopRepoFromClient(tx.Client())
	shifts := store.ShiftRepoFromClient(tx.Client())

	shop, err := shops.Create(ctx, name, timezone, 0)
	if err != nil {
		rollback()
		return nil, err
	}
	shop, err = shops.ProvisionDashboardAccount(ctx, shop.ID, username, plan)
	if err != nil {
		rollback()
		return nil, err
	}
	if createDefaultShifts {
		if err := shifts.CreateDefaultsForShop(ctx, shop.ID); err != nil {
			rollback()
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("admin: commit tx: %w", err)
	}
	return shop, nil
}

// ShopService adapts store.ShopRepo for admin handlers.
type ShopService struct {
	shops *store.ShopRepo
}

// NewShopService wires shop admin operations.
func NewShopService(st *store.Store) *ShopService {
	return &ShopService{shops: st.Shops}
}

func (s *ShopService) ListAll(ctx context.Context) ([]*store.Shop, error) {
	return s.shops.ListAll(ctx)
}

func (s *ShopService) ProvisionDashboardAccount(ctx context.Context, shopID, username, plan string) (*store.Shop, error) {
	id, err := uuid.Parse(shopID)
	if err != nil {
		return nil, store.ErrNotFound
	}
	return s.shops.ProvisionDashboardAccount(ctx, id, username, plan)
}

func (s *ShopService) UpdatePlan(ctx context.Context, shopID, plan string) (*store.Shop, error) {
	id, err := uuid.Parse(shopID)
	if err != nil {
		return nil, store.ErrNotFound
	}
	return s.shops.UpdatePlan(ctx, id, plan)
}
