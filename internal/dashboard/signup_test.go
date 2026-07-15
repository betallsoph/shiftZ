package dashboard

import (
	"context"
	"errors"

	"github.com/betallsoph/shiftz/internal/onboarding"
)

// Public signup was replaced by admin-managed shop provisioning.
type noopOnboarder struct{}

func (noopOnboarder) CreateShop(context.Context, string, string, bool) (*onboarding.Result, error) {
	return nil, errors.New("onboarding not configured")
}
