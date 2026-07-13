// Package onboarding coordinates shop creation for the owner signup flow.
package onboarding

import (
	"context"
	"fmt"
	"time"

	"github.com/betallsoph/shiftz/internal/store"
)

// Result is returned once when a shop is created through signup.
type Result struct {
	Shop       *store.Shop
	OwnerToken string
}

// Service coordinates shop and shift creation for owner onboarding.
type Service struct {
	shops  *store.ShopRepo
	shifts *store.ShiftRepo
}

// New wires onboarding on top of the store.
func New(st *store.Store) *Service {
	return &Service{
		shops:  st.Shops,
		shifts: st.Shifts,
	}
}

// CreateShop creates a shop with owner dashboard credentials and optional default shifts.
func (s *Service) CreateShop(ctx context.Context, name, timezone string, createDefaultShifts bool) (*Result, error) {
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, fmt.Errorf("onboarding: invalid timezone %q: %w", timezone, err)
	}
	creds, err := s.shops.CreateWithDashboardToken(ctx, name, timezone, 0)
	if err != nil {
		return nil, err
	}
	if createDefaultShifts {
		if err := s.shifts.CreateDefaultsForShop(ctx, creds.Shop.ID); err != nil {
			return nil, err
		}
	}
	return &Result{
		Shop:       creds.Shop,
		OwnerToken: creds.OwnerToken,
	}, nil
}
